package api

import (
	"fmt"
	"math"
	"time"

	"skill-go/server/aura"
	"skill-go/server/cooldown"
	"skill-go/server/effect"
	"skill-go/server/script"
	"skill-go/server/spell"
	"skill-go/server/spelldef"
	"skill-go/server/trace"
	"skill-go/server/unit"
)

// ---------------------------------------------------------------------------
// Command protocol
// ---------------------------------------------------------------------------

// Command represents an operation sent to the game loop.
type Command struct {
	Op      string
	Payload interface{}
	Reply   chan Result
}

// Result is the reply from the game loop.
type Result struct {
	Data interface{}
	Err  string
}

// ---------------------------------------------------------------------------
// Payload types
// ---------------------------------------------------------------------------

type castPayload struct {
	SpellID   uint32
	TargetIDs []uint64
}

type pushbackPayload struct {
	PushbackMs int32
}

type moveUnitPayload struct {
	GUID uint64
	X, Z float64
}

type updateUnitPayload struct {
	GUID  uint64
	Level uint8
}

type addUnitPayload struct {
	Name  string
	Level uint8
}

type removeUnitPayload struct {
	GUID uint64
}

type createSpellPayload struct {
	Req CreateSpellRequest
}

type updateSpellPayload struct {
	ID  uint32
	Req UpdateSpellRequest
}

type delayedHitPayload struct {
	Ctx    *spell.SpellContext
	Target *unit.Unit
	Eff    spelldef.SpellEffectInfo
}

type channelTickPayload struct {
	Ctx        *spell.SpellContext
	TickCount  int
	TotalTicks int
}

// ---------------------------------------------------------------------------
// GameLoop — single goroutine owning all mutable state
// ---------------------------------------------------------------------------

// GameLoop owns all mutable game state and processes commands sequentially.
// Only the run() goroutine accesses state fields — zero locks, zero races.
type GameLoop struct {
	cmds   chan Command
	stopCh chan struct{}

	// State — only accessed by the run() goroutine
	caster       *unit.Unit
	targets      []*unit.Unit
	allUnits     []*unit.Unit
	history      *cooldown.SpellHistory
	auraMgrs     map[uint64]*aura.AuraManager
	auraProvider *simpleAuraProvider
	recorder     *trace.FlowRecorder
	store        *effect.Store
	registry     *script.Registry
	spellBook    []*spelldef.SpellInfo
	nextSpellID  uint32
	tr           *trace.Trace
	hub          *trace.StreamHub
	fileSink     *trace.FileSink
	pending      *pendingCast
	nextGUID     uint64
}

// NewGameLoop creates a game loop with predefined units and spells.
func NewGameLoop(fileSink *trace.FileSink) *GameLoop {
	mage := unit.NewUnit(1, "Mage", 5000, 20000)
	mage.SetLevel(60)
	mage.SpellPower = 500
	mage.SetWeaponDamage(30, 30)
	mage.HitSpell = 100.0
	mage.CritSpell = 20.0
	mage.Position = unit.Position{X: 0, Y: 0, Z: 0}

	warrior := unit.NewUnit(2, "Warrior", 10000, 5000)
	warrior.SetLevel(60)
	warrior.Armor = 3000
	warrior.Block = 15.0
	warrior.BlockValue = 200
	warrior.Dodge = 5.0
	warrior.Parry = 10.0
	warrior.SetWeaponDamage(80, 120)
	warrior.HitMelee = 100.0
	warrior.Position = unit.Position{X: 20, Y: 0, Z: 0}

	target := unit.NewUnit(3, "Target Dummy", 15000, 0)
	target.SetLevel(63)
	target.Armor = 5000
	target.SetResistance(spelldef.SchoolMaskFire, 100)
	target.Position = unit.Position{X: 40, Y: 0, Z: 0}

	allUnits := []*unit.Unit{mage, warrior, target}

	targetAuraMgr := aura.NewAuraManager(target)
	warriorAuraMgr := aura.NewAuraManager(warrior)
	auraMgrs := map[uint64]*aura.AuraManager{
		target.GUID:  targetAuraMgr,
		warrior.GUID: warriorAuraMgr,
	}

	recorder := trace.NewFlowRecorder()
	hub := trace.NewStreamHub(10000)
	streamSink := trace.NewStreamSink(hub)

	var sinks []trace.TraceSink
	sinks = append(sinks, recorder, streamSink)
	if fileSink != nil {
		sinks = append(sinks, fileSink)
	}
	tr := trace.NewTraceWithSinks(sinks...)

	store := effect.NewStore()
	registry := script.NewRegistry()
	history := cooldown.NewSpellHistory()

	auraProvider := &simpleAuraProvider{managers: auraMgrs}

	gl := &GameLoop{
		stopCh:       make(chan struct{}),
		caster:       mage,
		targets:      []*unit.Unit{warrior, target},
		allUnits:     allUnits,
		history:      history,
		auraMgrs:     auraMgrs,
		auraProvider: auraProvider,
		recorder:     recorder,
		store:        store,
		registry:     registry,
		tr:           tr,
		hub:          hub,
		fileSink:     fileSink,
		nextGUID:     100,
	}

	effect.RegisterExtended(store, makeAuraHandler(auraProvider), nil)
	gl.initSpellBook()

	return gl
}

func (gl *GameLoop) initSpellBook() {
	gl.spellBook = []*spelldef.SpellInfo{
		{
			ID:             42833,
			Name:           "Fireball",
			SchoolMask:     spelldef.SchoolMaskFire,
			RecoveryTime:   0,
			CastTime:       3500,
			PowerCost:      400,
			MaxTargets:     1,
			Effects: []spelldef.SpellEffectInfo{
				{
					EffectIndex: 0,
					EffectType:  spelldef.SpellEffectSchoolDamage,
					SchoolMask:  spelldef.SchoolMaskFire,
					BasePoints:  888,
					Coef:        1.0,
				},
				{
					EffectIndex: 1,
					EffectType:  spelldef.SpellEffectApplyAura,
					AuraType:    int32(aura.AuraTypeDebuff),
					AuraDuration: 8000,
				},
			},
		},
	}

	for _, s := range gl.spellBook {
		if s.ID >= gl.nextSpellID {
			gl.nextSpellID = s.ID + 1
		}
	}
}

// Start launches the event loop goroutine and the aura update ticker.
func (gl *GameLoop) Start() {
	go gl.run()
	go gl.auraTicker()
}

// Stop shuts down the event loop and all background tickers.
func (gl *GameLoop) Stop() {
	close(gl.stopCh)
}

// auraTicker sends periodic aura_update commands to the event loop.
func (gl *GameLoop) auraTicker() {
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			gl.SendAsync(Command{Op: "aura_update"})
		case <-gl.stopCh:
			return
		}
	}
}

// Send sends a command and blocks until the event loop replies.
func (gl *GameLoop) Send(cmd Command) Result {
	cmd.Reply = make(chan Result, 1)
	gl.cmds <- cmd
	return <-cmd.Reply
}

// SendAsync sends a command without waiting for a reply.
func (gl *GameLoop) SendAsync(cmd Command) {
	gl.cmds <- cmd
}

// Hub returns the StreamHub for SSE streaming.
func (gl *GameLoop) Hub() *trace.StreamHub {
	return gl.hub
}

func (gl *GameLoop) run() {
	gl.cmds = make(chan Command, 256)

	for cmd := range gl.cmds {
		gl.dispatch(cmd)
	}
}

func (gl *GameLoop) dispatch(cmd Command) {
	switch cmd.Op {
	case "cast":
		gl.handleCast(cmd)
	case "cast_complete":
		gl.handleCastComplete(cmd)
	case "cast_cancel":
		gl.handleCastCancel(cmd)
	case "cast_pushback":
		gl.handleCastPushback(cmd)
	case "delayed_hit":
		gl.handleDelayedHit(cmd)
	case "channel_tick":
		gl.handleChannelTick(cmd)
	case "channel_elapsed":
		gl.handleChannelElapsed(cmd)
	case "get_units":
		gl.handleGetUnits(cmd)
	case "get_spells":
		gl.handleGetSpells(cmd)
	case "get_spell":
		gl.handleGetSpell(cmd)
	case "create_spell":
		gl.handleCreateSpell(cmd)
	case "update_spell":
		gl.handleUpdateSpell(cmd)
	case "delete_spell":
		gl.handleDeleteSpell(cmd)
	case "add_unit":
		gl.handleAddUnit(cmd)
	case "remove_unit":
		gl.handleRemoveUnit(cmd)
	case "move_unit":
		gl.handleMoveUnit(cmd)
	case "update_unit":
		gl.handleUpdateUnit(cmd)
	case "reset":
		gl.handleReset(cmd)
	case "trace_clear":
		gl.handleTraceClear(cmd)
	case "aura_update":
		gl.handleAuraUpdate(cmd)
	default:
		reply(cmd, Result{Err: fmt.Sprintf("unknown command: %s", cmd.Op)})
	}
}

func reply(cmd Command, r Result) {
	if cmd.Reply != nil {
		cmd.Reply <- r
	}
}

// ---------------------------------------------------------------------------
// Cast command handlers
// ---------------------------------------------------------------------------

func (gl *GameLoop) handleCast(cmd Command) {
	req := cmd.Payload.(castPayload)

	spellInfo := gl.findSpell(req.SpellID)
	if spellInfo == nil {
		reply(cmd, Result{Err: fmt.Sprintf("unknown spell ID %d", req.SpellID)})
		return
	}

	// Resolve targets
	var targets []*unit.Unit
	for _, guid := range req.TargetIDs {
		u := gl.findUnit(guid)
		if u != nil {
			targets = append(targets, u)
		}
	}
	if len(targets) == 0 {
		targets = gl.targets
	}

	// Create trace for this cast
	castTrace := gl.newCastTrace()

	// Create spell context
	ctx := spell.New(spellInfo.ID, spellInfo, gl.caster, targets)
	ctx.EffectStore = gl.store
	ctx.HistoryProvider = gl.history
	ctx.CooldownProvider = &tracingCooldownHistory{SpellHistory: gl.history}
	ctx.AuraProvider = gl.auraProvider
	ctx.ScriptRegistry = gl.registry
	ctx.Trace = castTrace

	// Prepare
	result := ctx.Prepare()

	if result != spelldef.CastResultSuccess {
		events := gl.collectAndResetEvents()
		reply(cmd, Result{Data: gl.buildCastResponse(result, ctx, events)})
		return
	}

	if ctx.CastDuration == 0 {
		// Instant spell: execute immediately
		result = ctx.Cast()
		events := gl.collectAndResetEvents()
		reply(cmd, Result{Data: gl.buildCastResponse(result, ctx, events)})
		return
	}

	// Cast-time spell: store pending, client will call cast_complete
	gl.pending = &pendingCast{
		ctx:             ctx,
		spellInfo:       spellInfo,
		targetIDs:       req.TargetIDs,
		castTimeMs:      spellInfo.CastTime,
		pushbackTotalMs: 0,
	}

	// Events already pushed to StreamHub via trace sinks; clean up recorder
	gl.recorder.Reset()

	reply(cmd, Result{Data: CastPrepareResponse{
		Result:     "preparing",
		CastTimeMs: spellInfo.CastTime,
		SpellID:    spellInfo.ID,
		SpellName:  spellInfo.Name,
		SchoolName: schoolName(spellInfo.SchoolMask),
	}})
}

func (gl *GameLoop) handleCastComplete(cmd Command) {
	if gl.pending == nil {
		reply(cmd, Result{Err: "no pending cast"})
		return
	}

	ctx := gl.pending.ctx
	result := ctx.Cast()

	// Schedule delayed hits if any
	schedules := ctx.GetDelayedHitSchedules()
	for _, s := range schedules {
		gl.scheduleDelayedHit(ctx, s)
	}

	// Start channel ticker if channeling
	if ctx.State == spell.StateChanneling {
		gl.startChannelTicker(ctx)
		// Keep pending so cancel can stop the channel
	} else {
		gl.pending = nil
	}

	events := gl.collectAndResetEvents()
	reply(cmd, Result{Data: gl.buildCastResponse(result, ctx, events)})
}

func (gl *GameLoop) handleCastCancel(cmd Command) {
	if gl.pending == nil {
		reply(cmd, Result{Data: map[string]interface{}{"result": "no_pending"}})
		return
	}

	ctx := gl.pending.ctx
	ctx.Cancel() // handles StatePreparing/Channeling/Launched

	eventsJSON := gl.collectPrepareEvents()
	gl.recorder.Reset()
	gl.pending = nil

	reply(cmd, Result{Data: map[string]interface{}{
		"result": "cancelled",
		"events": eventsJSON,
	}})
}

func (gl *GameLoop) handleCastPushback(cmd Command) {
	if gl.pending == nil {
		reply(cmd, Result{Data: map[string]string{"result": "no_pending"}})
		return
	}

	req := cmd.Payload.(pushbackPayload)
	maxPushback := gl.pending.castTimeMs
	gl.pending.pushbackTotalMs += req.PushbackMs

	if gl.pending.pushbackTotalMs >= maxPushback {
		ctx := gl.pending.ctx
		ctx.Cancel()
		gl.recorder.Reset()
		gl.pending = nil
		reply(cmd, Result{Data: map[string]interface{}{
			"result":         "interrupted",
			"reason":         "pushback_limit",
			"pushbackTotalMs": maxPushback,
			"maxPushbackMs":   maxPushback,
		}})
		return
	}

	newRemainingMs := gl.pending.castTimeMs + gl.pending.pushbackTotalMs
	reply(cmd, Result{Data: map[string]interface{}{
		"result":          "pushed",
		"newRemainingMs":  newRemainingMs,
		"pushbackTotalMs": gl.pending.pushbackTotalMs,
		"maxPushbackMs":   maxPushback,
	}})
}

// ---------------------------------------------------------------------------
// Delayed hit and channel handlers
// ---------------------------------------------------------------------------

func (gl *GameLoop) scheduleDelayedHit(ctx *spell.SpellContext, s spell.DelayedHitSchedule) {
	if s.Delay <= 0 {
		gl.SendAsync(Command{
			Op:      "delayed_hit",
			Payload: delayedHitPayload{Ctx: ctx, Target: s.Target, Eff: s.Eff},
		})
		return
	}

	time.AfterFunc(s.Delay, func() {
		gl.SendAsync(Command{
			Op:      "delayed_hit",
			Payload: delayedHitPayload{Ctx: ctx, Target: s.Target, Eff: s.Eff},
		})
	})
}

func (gl *GameLoop) handleDelayedHit(cmd Command) {
	p := cmd.Payload.(delayedHitPayload)
	ctx := p.Ctx

	if ctx.Cancelled || !p.Target.IsAlive() {
		ctx.Trace.Event(trace.SpanSpell, "delayed_hit_skipped", ctx.Info.ID, ctx.Info.Name, map[string]interface{}{
			"cancelled":   ctx.Cancelled,
			"targetAlive": p.Target.IsAlive(),
		})
		return
	}

	ctx.Trace.Event(trace.SpanSpell, "delayed_hit_arrived", ctx.Info.ID, ctx.Info.Name, map[string]interface{}{
		"target":      p.Target.Name,
		"effectIndex": p.Eff.EffectIndex,
	})

	ctx.ExecuteHit(p.Target, p.Eff)
	ctx.TriggerHitProc(p.Target)
}

func (gl *GameLoop) startChannelTicker(ctx *spell.SpellContext) {
	interval := time.Duration(ctx.Info.TickInterval) * time.Millisecond
	if interval == 0 {
		interval = time.Second
	}
	duration := time.Duration(ctx.Info.ChannelDuration) * time.Millisecond
	totalTicks := ctx.TotalTicks()
	stopCh := ctx.ChannelStop()

	ticker := time.NewTicker(interval)
	tickCount := 0

	go func() {
		defer ticker.Stop()
		timer := time.NewTimer(duration)
		defer timer.Stop()

		for {
			select {
			case <-ticker.C:
				tickCount++
				gl.SendAsync(Command{
					Op: "channel_tick",
					Payload: channelTickPayload{
						Ctx:        ctx,
						TickCount:  tickCount,
						TotalTicks: totalTicks,
					},
				})
			case <-timer.C:
				gl.SendAsync(Command{Op: "channel_elapsed", Payload: ctx})
				return
			case <-stopCh:
				return
			}
		}
	}()
}

func (gl *GameLoop) handleChannelTick(cmd Command) {
	p := cmd.Payload.(channelTickPayload)
	ctx := p.Ctx

	ctx.Trace.Event(trace.SpanSpell, "channel_tick", ctx.Info.ID, ctx.Info.Name, map[string]interface{}{
		"tick":        p.TickCount,
		"totalTicks":  p.TotalTicks,
		"spellID":    ctx.Info.ID,
		"spellName":  ctx.Info.Name,
	})

	if !ctx.ExecuteChannelTick() {
		ctx.Trace.Event(trace.SpanSpell, "channel_stopped", ctx.Info.ID, ctx.Info.Name, map[string]interface{}{
			"reason":      "all_targets_dead_or_cancelled",
			"total_ticks": p.TickCount,
		})
		ctx.FinishChannel()
		gl.pending = nil
	}
}

func (gl *GameLoop) handleChannelElapsed(cmd Command) {
	ctx := cmd.Payload.(*spell.SpellContext)
	ctx.Trace.Event(trace.SpanSpell, "channel_elapsed", ctx.Info.ID, ctx.Info.Name, map[string]interface{}{
		"total_ticks": ctx.TotalTicks(),
	})
	ctx.FinishChannel()
	gl.pending = nil
}

// ---------------------------------------------------------------------------
// Unit command handlers
// ---------------------------------------------------------------------------

func (gl *GameLoop) handleGetUnits(cmd Command) {
	reply(cmd, Result{Data: gl.unitListJSON()})
}

func (gl *GameLoop) handleAddUnit(cmd Command) {
	req := cmd.Payload.(addUnitPayload)
	gl.addUnit(req.Name, req.Level)
	reply(cmd, Result{Data: gl.unitListJSON()})
}

func (gl *GameLoop) handleRemoveUnit(cmd Command) {
	req := cmd.Payload.(removeUnitPayload)
	err := gl.removeUnit(req.GUID)
	if err != nil {
		reply(cmd, Result{Err: err.Error()})
		return
	}
	reply(cmd, Result{Data: gl.unitListJSON()})
}

func (gl *GameLoop) handleMoveUnit(cmd Command) {
	req := cmd.Payload.(moveUnitPayload)
	err := gl.moveUnit(req.GUID, req.X, req.Z)
	if err != nil {
		reply(cmd, Result{Err: err.Error()})
		return
	}
	reply(cmd, Result{Data: gl.unitListJSON()})
}

func (gl *GameLoop) handleUpdateUnit(cmd Command) {
	req := cmd.Payload.(updateUnitPayload)
	err := gl.updateUnitLevel(req.GUID, req.Level)
	if err != nil {
		reply(cmd, Result{Err: err.Error()})
		return
	}
	reply(cmd, Result{Data: gl.unitListJSON()})
}

// ---------------------------------------------------------------------------
// Spell command handlers
// ---------------------------------------------------------------------------

func (gl *GameLoop) handleGetSpells(cmd Command) {
	reply(cmd, Result{Data: gl.spellListJSON()})
}

func (gl *GameLoop) handleGetSpell(cmd Command) {
	id := cmd.Payload.(uint32)
	s := gl.findSpell(id)
	if s == nil {
		reply(cmd, Result{Err: fmt.Sprintf("spell %d not found", id)})
		return
	}
	reply(cmd, Result{Data: spellToJSON(s)})
}

func (gl *GameLoop) handleCreateSpell(cmd Command) {
	req := cmd.Payload.(createSpellPayload)

	if req.Req.Name == "" {
		reply(cmd, Result{Err: "name is required"})
		return
	}

	schoolMask := schoolMaskFromName(req.Req.SchoolName)

	effects := make([]spelldef.SpellEffectInfo, len(req.Req.Effects))
	for i, ce := range req.Req.Effects {
		effects[i] = spelldef.SpellEffectInfo{
			EffectIndex:   i,
			EffectType:    effectTypeFromName(ce.EffectType),
			SchoolMask:    schoolMask,
			BasePoints:    ce.BasePoints,
			Coef:          ce.Coef,
			WeaponPercent: ce.WeaponPercent,
			AuraDuration:  ce.AuraDuration,
			AuraType:      ce.AuraType,
		}
		if effects[i].EffectType == spelldef.SpellEffectEnergize {
			effects[i].EnergizeType = spelldef.PowerTypeMana
			effects[i].EnergizeAmount = ce.BasePoints
		}
	}

	s := &spelldef.SpellInfo{
		ID:                   gl.nextSpellID,
		Name:                 req.Req.Name,
		SchoolMask:           schoolMask,
		CastTime:             req.Req.CastTime,
		RecoveryTime:         req.Req.RecoveryTime,
		CategoryRecoveryTime: req.Req.CategoryRecoveryTime,
		PowerCost:            req.Req.PowerCost,
		MaxTargets:           req.Req.MaxTargets,
		Effects:              effects,
	}

	gl.spellBook = append(gl.spellBook, s)
	gl.nextSpellID++

	reply(cmd, Result{Data: spellToJSON(s)})
}

func (gl *GameLoop) handleUpdateSpell(cmd Command) {
	p := cmd.Payload.(updateSpellPayload)
	s := gl.findSpell(p.ID)
	if s == nil {
		reply(cmd, Result{Err: fmt.Sprintf("spell %d not found", p.ID)})
		return
	}

	req := p.Req
	if req.Name != "" {
		s.Name = req.Name
	}
	s.CastTime = req.CastTime
	s.RecoveryTime = req.RecoveryTime
	s.CategoryRecoveryTime = req.CategoryRecoveryTime
	s.PowerCost = req.PowerCost
	s.MaxTargets = req.MaxTargets

	gl.history.RemoveCooldown(uint32(p.ID))

	for _, ue := range req.Effects {
		if ue.EffectIndex < 0 || ue.EffectIndex >= len(s.Effects) {
			continue
		}
		eff := &s.Effects[ue.EffectIndex]
		eff.BasePoints = ue.BasePoints
		eff.Coef = ue.Coef
		eff.WeaponPercent = ue.WeaponPercent
		eff.AuraDuration = ue.AuraDuration
		eff.AuraType = ue.AuraType
	}

	reply(cmd, Result{Data: map[string]string{"status": "ok"}})
}

func (gl *GameLoop) handleDeleteSpell(cmd Command) {
	id := cmd.Payload.(uint32)

	idx := -1
	for i, s := range gl.spellBook {
		if s.ID == id {
			idx = i
			break
		}
	}
	if idx == -1 {
		reply(cmd, Result{Err: fmt.Sprintf("spell %d not found", id)})
		return
	}

	gl.spellBook = append(gl.spellBook[:idx], gl.spellBook[idx+1:]...)
	gl.history.RemoveCooldown(id)

	reply(cmd, Result{Data: map[string]string{"status": "deleted"}})
}

// ---------------------------------------------------------------------------
// Reset and trace handlers
// ---------------------------------------------------------------------------

func (gl *GameLoop) handleReset(cmd Command) {
	gl.caster.Health = gl.caster.MaxHealth
	gl.caster.Mana = gl.caster.MaxMana
	gl.caster.Alive = true
	for _, u := range gl.allUnits {
		u.Health = u.MaxHealth
		u.Mana = u.MaxMana
		u.Alive = true
	}

	gl.history = cooldown.NewSpellHistory()

	for _, mgr := range gl.auraMgrs {
		for _, a := range mgr.Auras {
			mgr.RemoveAura(a, aura.RemoveModeDefault, nil, 0, "")
		}
	}

	gl.recorder.Reset()
	gl.hub.Clear()
	gl.hub.ClearSubscribers()

	var sinks []trace.TraceSink
	sinks = append(sinks, gl.recorder, trace.NewStreamSink(gl.hub))
	if gl.fileSink != nil {
		sinks = append(sinks, gl.fileSink)
	}
	gl.tr = trace.NewTraceWithSinks(sinks...)

	reply(cmd, Result{Data: map[string]string{"status": "ok"}})
}

func (gl *GameLoop) handleTraceClear(cmd Command) {
	gl.recorder.Reset()
	gl.hub.Clear()
	reply(cmd, Result{})
}

// handleAuraUpdate expires auras whose duration has elapsed.
func (gl *GameLoop) handleAuraUpdate(cmd Command) {
	now := time.Now().UnixMilli()
	for guid, mgr := range gl.auraMgrs {
		for _, a := range mgr.Auras {
			if a.Duration <= 0 {
				continue // permanent aura
			}
			timerStart := int64(0)
			if len(a.Applications) > 0 {
				timerStart = a.Applications[len(a.Applications)-1].TimerStart
			}
			if timerStart == 0 {
				continue
			}
			if (now - timerStart) >= int64(a.Duration) {
				t := trace.NewTraceWithSinks(gl.recorder, trace.NewStreamSink(gl.hub))
				mgr.RemoveAura(a, aura.RemoveModeExpired, t, a.SpellID, a.SourceName)
			}
		}
		// Clean up empty aura managers from removed units
		if len(mgr.Auras) == 0 {
			// Keep the manager — the unit still exists
			_ = guid
		}
	}
}

// ---------------------------------------------------------------------------
// State operations
// ---------------------------------------------------------------------------

func (gl *GameLoop) findSpell(id uint32) *spelldef.SpellInfo {
	for _, s := range gl.spellBook {
		if s.ID == id {
			return s
		}
	}
	return nil
}

func (gl *GameLoop) findUnit(guid uint64) *unit.Unit {
	for _, u := range gl.allUnits {
		if u.GUID == guid {
			return u
		}
	}
	return nil
}

func (gl *GameLoop) addUnit(name string, level uint8) *unit.Unit {
	if name == "" {
		name = "Unknown"
	}
	if level == 0 {
		level = 60
	}

	lvl := float64(level)
	maxHP := int32(100 + lvl*50)
	maxMana := int32(50 + lvl*20)
	armor := int32(lvl * 30)
	spellPower := int32(lvl * 5)

	u := unit.NewUnit(gl.nextGUID, name, maxHP, maxMana)
	gl.nextGUID++
	u.SetLevel(level)
	u.Armor = armor
	u.SpellPower = spellPower

	offsetX := 25.0 + float64(len(gl.allUnits))*5 + math.Round(float64(len(gl.allUnits)%3)*3)
	offsetZ := float64(((len(gl.allUnits)*7+3)%11) - 5) * 1.5
	u.Position = unit.Position{X: offsetX, Y: 0, Z: offsetZ}

	auraMgr := aura.NewAuraManager(u)
	gl.auraMgrs[u.GUID] = auraMgr

	gl.allUnits = append(gl.allUnits, u)
	gl.targets = append(gl.targets, u)

	return u
}

func (gl *GameLoop) removeUnit(guid uint64) error {
	if guid == gl.caster.GUID {
		return fmt.Errorf("cannot remove caster")
	}

	found := false
	var newAll []*unit.Unit
	for _, u := range gl.allUnits {
		if u.GUID == guid {
			found = true
			continue
		}
		newAll = append(newAll, u)
	}
	if !found {
		return fmt.Errorf("unit not found")
	}
	gl.allUnits = newAll

	var newTargets []*unit.Unit
	for _, u := range gl.targets {
		if u.GUID != guid {
			newTargets = append(newTargets, u)
		}
	}
	gl.targets = newTargets

	if mgr, ok := gl.auraMgrs[guid]; ok {
		for _, a := range mgr.Auras {
			mgr.RemoveAura(a, aura.RemoveModeDefault, nil, 0, "")
		}
		delete(gl.auraMgrs, guid)
	}

	return nil
}

func (gl *GameLoop) moveUnit(guid uint64, x, z float64) error {
	u := gl.findUnit(guid)
	if u == nil {
		return fmt.Errorf("unit not found")
	}
	u.Position = unit.Position{X: x, Y: 0, Z: z}
	return nil
}

func (gl *GameLoop) updateUnitLevel(guid uint64, level uint8) error {
	if guid == gl.caster.GUID {
		return fmt.Errorf("cannot modify caster")
	}
	u := gl.findUnit(guid)
	if u == nil {
		return fmt.Errorf("unit not found")
	}
	if level == 0 {
		level = 60
	}
	u.SetLevel(level)
	lvl := float64(level)
	u.MaxHealth = int32(100 + lvl*50)
	u.Health = u.MaxHealth
	u.MaxMana = int32(50 + lvl*20)
	u.Mana = u.MaxMana
	u.Armor = int32(lvl * 30)
	u.SpellPower = int32(lvl * 5)
	return nil
}

// ---------------------------------------------------------------------------
// JSON helpers
// ---------------------------------------------------------------------------

func (gl *GameLoop) unitListJSON() []UnitJSON {
	unitsJSON := make([]UnitJSON, len(gl.allUnits))
	for i, u := range gl.allUnits {
		auraMgr := gl.auraMgrs[u.GUID]
		unitsJSON[i] = unitToJSON(u, auraMgr)
	}
	return unitsJSON
}

func (gl *GameLoop) spellListJSON() []SpellJSON {
	spellsJSON := make([]SpellJSON, len(gl.spellBook))
	for i, s := range gl.spellBook {
		spellsJSON[i] = spellToJSON(s)
	}
	return spellsJSON
}

func (gl *GameLoop) newCastTrace() *trace.Trace {
	var sinks []trace.TraceSink
	sinks = append(sinks, gl.recorder, trace.NewStreamSink(gl.hub))
	if gl.fileSink != nil {
		sinks = append(sinks, gl.fileSink)
	}
	return trace.NewTraceWithSinks(sinks...)
}

func (gl *GameLoop) collectAndResetEvents() []trace.FlowEvent {
	events := gl.recorder.Events()
	gl.recorder.Reset()
	return events
}

func (gl *GameLoop) collectPrepareEvents() []TraceEventJSON {
	events := gl.recorder.Events()
	eventsJSON := make([]TraceEventJSON, len(events))
	for i, e := range events {
		eventsJSON[i] = eventToJSON(e)
	}
	return eventsJSON
}

func (gl *GameLoop) buildCastResponse(result spelldef.CastResult, ctx *spell.SpellContext, events []trace.FlowEvent) CastResponse {
	unitsJSON := gl.unitListJSON()
	eventsJSON := make([]TraceEventJSON, len(events))
	for i, e := range events {
		eventsJSON[i] = eventToJSON(e)
	}

	resp := CastResponse{
		Result: castResultName(result),
		Units:  unitsJSON,
		Events: eventsJSON,
	}
	if result != spelldef.CastResultSuccess {
		resp.Error = castErrorName(ctx.LastCastErr)
	}
	return resp
}
