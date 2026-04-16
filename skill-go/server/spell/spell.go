package spell

import (
	"fmt"
	"time"

	"skill-go/server/aura"
	"skill-go/server/effect"
	"skill-go/server/script"
	"skill-go/server/spelldef"
	"skill-go/server/trace"
	"skill-go/server/unit"
)

// SpellState represents the current phase of a spell cast.
type SpellState int

const (
	StateNone       SpellState = iota // 0: just created
	StatePreparing                    // 1: casting bar in progress
	StateLaunched                     // 2: spell launched, effects in flight
	StateChanneling                   // 3: channeled spell active
	StateFinished                     // 4: spell complete, awaiting cleanup
	StateIdle                         // 5: waiting for auto-repeat trigger
)

func (s SpellState) String() string {
	switch s {
	case StateNone:
		return "None"
	case StatePreparing:
		return "Preparing"
	case StateLaunched:
		return "Launched"
	case StateChanneling:
		return "Channeling"
	case StateFinished:
		return "Finished"
	case StateIdle:
		return "Idle"
	default:
		return "Unknown"
	}
}

// delayedHit holds a pending hit to be executed at a future time.
type delayedHit struct {
	target *unit.Unit
	eff    spelldef.SpellEffectInfo
	hitAt  time.Time
}

// SpellContext holds all information about an in-progress spell cast.
type SpellContext struct {
	ID           uint32
	Info         *spelldef.SpellInfo
	Caster       *unit.Unit
	Targets      []*unit.Unit
	State        SpellState
	CastStart    time.Time
	CastDuration time.Duration
	ManaCost     int32
	ManaPaid     bool
	Cancelled    bool
	EffectStore  *effect.Store
	LastCastErr  spelldef.CastError // last validation error

	// Flow tracing
	Trace *trace.Trace

	// Cast time modifier chain
	CastModifiers ModifierChain

	// Spell history provider for cooldown/charge/GCD checks
	HistoryProvider SpellHistoryProvider

	// Cooldown/Aura providers for auto-integration
	CooldownProvider CooldownProvider
	AuraProvider     AuraProvider

	// Script support
	ScriptRegistry *script.Registry

	// Delayed execution
	delayedHits []delayedHit

	// Channeled spell
	channelStop chan struct{}

	// Empower spell
	empowerStart    time.Time
	empowerStage    int
	empowerActive   bool
	empowerReleased bool
}

// New creates a new SpellContext in the None state.
func New(id uint32, info *spelldef.SpellInfo, caster *unit.Unit, targets []*unit.Unit) *SpellContext {
	return &SpellContext{
		ID:          id,
		Info:        info,
		Caster:      caster,
		Targets:     targets,
		State:       StateNone,
		EffectStore: effect.NewStore(),
		Trace:       trace.NewTrace(),
	}
}

// Prepare performs the validation chain, consumes resources, and transitions to Preparing.
func (s *SpellContext) Prepare() spelldef.CastResult {
	spellID, spellName := s.Info.ID, s.Info.Name
	s.Trace.Event(trace.SpanSpell, "prepare", spellID, spellName, map[string]interface{}{
		"state":       s.State.String(),
		"targetCount": len(s.Targets),
	})

	if !s.Caster.IsAlive() {
		s.Trace.Event(trace.SpanSpell, "prepare_failed", spellID, spellName, map[string]interface{}{
			"reason": "caster_dead",
		})
		s.LastCastErr = spelldef.CastErrDead
		s.State = StateFinished
		return spelldef.CastResultFailed
	}

	// Full validation chain
	if err := CheckCast(s.Info, s.Caster, s.Targets, s.HistoryProvider, s.Trace); err != spelldef.CastErrNone {
		s.Trace.Event(trace.SpanSpell, "prepare_failed", spellID, spellName, map[string]interface{}{
			"reason":     "checkcast",
			"error_code": int(err),
		})
		s.LastCastErr = err
		s.State = StateFinished
		return spelldef.CastResultFailed
	}

	// Script hook: OnCheckCast
	if s.ScriptRegistry != nil {
		if ss := s.ScriptRegistry.GetSpellScript(s.ID); ss != nil {
			ss.ClearPrevented()
			ss.Fire(script.HookOnCheckCast, s.Info)
			if ss.IsPrevented(script.HookOnCheckCast) {
				s.Trace.Event(trace.SpanSpell, "prepare_failed", spellID, spellName, map[string]interface{}{
					"reason": "script_prevented",
				})
				s.LastCastErr = spelldef.CastErrInterrupted
				s.State = StateFinished
				return spelldef.CastResultFailed
			}
		}
	}

	if s.Info.PowerCost > 0 {
		var paid bool
		switch s.Info.PowerType {
		case spelldef.PowerTypeRage:
			paid = s.Caster.ConsumeRage(s.Info.PowerCost)
			if !paid {
				s.Trace.Event(trace.SpanSpell, "prepare_failed", spellID, spellName, map[string]interface{}{
					"reason": "no_rage",
					"need":   s.Info.PowerCost,
					"have":   s.Caster.Rage + s.Info.PowerCost,
				})
				s.LastCastErr = spelldef.CastErrNoRage
				s.State = StateFinished
				return spelldef.CastResultFailed
			}
			s.Trace.Event(trace.SpanSpell, "rage_consumed", spellID, spellName, map[string]interface{}{
				"amount":    s.Info.PowerCost,
				"remaining": s.Caster.Rage,
			})
		default:
			paid = s.Caster.ConsumeMana(s.Info.PowerCost)
			if !paid {
				s.Trace.Event(trace.SpanSpell, "prepare_failed", spellID, spellName, map[string]interface{}{
					"reason": "no_mana",
					"need":   s.Info.PowerCost,
					"have":   s.Caster.Mana + s.Info.PowerCost,
				})
				s.LastCastErr = spelldef.CastErrNoMana
				s.State = StateFinished
				return spelldef.CastResultFailed
			}
			s.Trace.Event(trace.SpanSpell, "mana_consumed", spellID, spellName, map[string]interface{}{
				"amount":    s.Info.PowerCost,
				"remaining": s.Caster.Mana,
			})
		}
		s.ManaPaid = paid
	}

	// Apply cast time modifier chain
	baseCastTime := s.Info.CastTime
	finalCastTime := baseCastTime
	if len(s.CastModifiers) > 0 {
		finalCastTime = s.CastModifiers.Apply(baseCastTime)
		if finalCastTime != baseCastTime {
			s.Trace.Event(trace.SpanSpell, "cast_time_modified", spellID, spellName, map[string]interface{}{
				"base_ms":  baseCastTime,
				"final_ms": finalCastTime,
			})
		}
	}

	s.CastDuration = time.Duration(finalCastTime) * time.Millisecond
	s.CastStart = time.Now()
	s.State = StatePreparing

	s.Trace.Event(trace.SpanSpell, "state_change", spellID, spellName, map[string]interface{}{
		"from":        "None",
		"to":          "Preparing",
		"castTime_ms": finalCastTime,
	})

	if s.CastDuration == 0 {
		return s.Cast()
	}

	return spelldef.CastResultSuccess
}

// Cast performs re-validation, launches effects, then routes to immediate/delay/channel/empower path.
func (s *SpellContext) Cast() spelldef.CastResult {
	spellID, spellName := s.Info.ID, s.Info.Name
	s.Trace.Event(trace.SpanSpell, "cast", spellID, spellName, map[string]interface{}{
		"state": s.State.String(),
	})

	if !s.Caster.IsAlive() {
		s.Trace.Event(trace.SpanSpell, "cast_failed", spellID, spellName, map[string]interface{}{
			"reason": "caster_died_during_cast",
		})
		s.refundMana()
		s.State = StateFinished
		return spelldef.CastResultFailed
	}

	// Re-check range at launch time
	if !ReCheckRange(s.Info, s.Caster, s.Targets, s.Trace) {
		s.Trace.Event(trace.SpanSpell, "cast_failed", spellID, spellName, map[string]interface{}{
			"reason": "target_out_of_range_at_launch",
		})
		s.refundMana()
		s.LastCastErr = spelldef.CastErrOutOfRange
		s.State = StateFinished
		return spelldef.CastResultFailed
	}

	for _, t := range s.Targets {
		if !t.IsAlive() {
			s.Trace.Event(trace.SpanSpell, "cast_failed", spellID, spellName, map[string]interface{}{
				"reason":      "target_died_during_cast",
				"target_name": t.Name,
			})
			s.refundMana()
			s.State = StateFinished
			return spelldef.CastResultFailed
		}
	}

	s.State = StateLaunched
	s.Trace.Event(trace.SpanSpell, "state_change", spellID, spellName, map[string]interface{}{
		"from": "Preparing",
		"to":   "Launched",
	})

	// Auto-trigger cooldowns
	if s.CooldownProvider != nil {
		if tcp, ok := s.CooldownProvider.(TracingCooldownProvider); ok {
			if s.Info.MaxCharges > 0 {
				tcp.TraceConsumeCharge(s.ID, s.Trace)
			}
			if s.Info.RecoveryTime > 0 {
				tcp.TraceAddCooldown(s.ID, s.Info.RecoveryTime, s.Info.RecoveryCategory, s.Trace)
			}
			if s.Info.CategoryRecoveryTime > 0 {
				tcp.TraceStartGCD(s.Info.RecoveryCategory, s.Info.CategoryRecoveryTime, s.Trace)
			}
		} else {
			if s.Info.MaxCharges > 0 {
				s.CooldownProvider.ConsumeCharge(s.ID)
			}
			if s.Info.RecoveryTime > 0 {
				s.CooldownProvider.AddCooldown(s.ID, s.Info.RecoveryTime, s.Info.RecoveryCategory)
			}
			if s.Info.CategoryRecoveryTime > 0 {
				s.CooldownProvider.StartGCD(s.Info.RecoveryCategory, s.Info.CategoryRecoveryTime)
			}
		}
	}

	// Launch phase
	ctx := &effectAdapter{caster: s.Caster, targets: s.Targets, trace: s.Trace, spellID: spellID, spellName: spellName}

	// Script hook: BeforeCast
	if s.ScriptRegistry != nil {
		if ss := s.ScriptRegistry.GetSpellScript(s.ID); ss != nil {
			ss.ClearPrevented()
			ss.Fire(script.HookBeforeCast, s)
		}
	}

	for _, eff := range s.Info.Effects {
		handler := s.EffectStore.GetLaunchHandler(eff.EffectType)
		if handler != nil {
			s.Trace.Event(trace.SpanEffectLaunch, "launch", spellID, spellName, map[string]interface{}{
				"effectIndex": eff.EffectIndex,
				"effectType":  int(eff.EffectType),
			})
			handler(ctx, eff)
		}
	}

	// Script hook: OnLaunch
	if s.ScriptRegistry != nil {
		if ss := s.ScriptRegistry.GetSpellScript(s.ID); ss != nil {
			ss.Fire(script.HookOnLaunch, s)
		}
	}

	// Route to execution path
	switch {
	case s.Info.IsChanneled && s.Info.ChannelDuration > 0:
		return s.startChannel()
	case s.Info.IsEmpower:
		return s.startEmpower()
	case s.Info.MissileSpeed > 0:
		return s.startDelayedHit()
	default:
		// Immediate path
		s.executeHitAll()
		return s.Finish()
	}
}

// Cancel interrupts the spell from Preparing, Channeling, or Empower state.
func (s *SpellContext) Cancel() {
	spellID, spellName := s.Info.ID, s.Info.Name
	s.Trace.Event(trace.SpanSpell, "cancel", spellID, spellName, map[string]interface{}{
		"state": s.State.String(),
	})

	switch s.State {
	case StatePreparing:
		s.Cancelled = true
		s.refundMana()
		s.State = StateFinished
		s.Trace.Event(trace.SpanSpell, "state_change", spellID, spellName, map[string]interface{}{
			"from":   "Preparing",
			"to":     "Finished",
			"reason": "cancelled",
		})

	case StateChanneling:
		s.Cancelled = true
		s.stopChannel()
		s.State = StateFinished
		s.Trace.Event(trace.SpanSpell, "state_change", spellID, spellName, map[string]interface{}{
			"from":   "Channeling",
			"to":     "Finished",
			"reason": "cancelled",
		})

	case StateLaunched:
		// Cancel pending delayed hits
		s.Cancelled = true
		s.delayedHits = nil
		s.State = StateFinished
		s.Trace.Event(trace.SpanSpell, "state_change", spellID, spellName, map[string]interface{}{
			"from":   "Launched",
			"to":     "Finished",
			"reason": "cancelled",
		})

	default:
		s.Trace.Event(trace.SpanSpell, "cancel_ignored", spellID, spellName, map[string]interface{}{
			"state": s.State.String(),
		})
	}
}

// Finish handles post-cast cleanup.
func (s *SpellContext) Finish() spelldef.CastResult {
	spellID, spellName := s.Info.ID, s.Info.Name

	if s.Info.IsAutoRepeat && !s.Cancelled {
		s.State = StateIdle
		s.Trace.Event(trace.SpanSpell, "state_change", spellID, spellName, map[string]interface{}{
			"from":   s.State.String(),
			"to":     "Idle",
			"reason": "auto_repeat",
		})
	} else {
		s.State = StateFinished
		s.Trace.Event(trace.SpanSpell, "finish", spellID, spellName, map[string]interface{}{
			"cancelled": s.Cancelled,
		})
	}

	return spelldef.CastResultSuccess
}

// --- Delayed execution path ---

func (s *SpellContext) startDelayedHit() spelldef.CastResult {
	spellID, spellName := s.Info.ID, s.Info.Name
	now := time.Now()
	const minDelayMs = 150 // server scheduling granularity

	s.Trace.Event(trace.SpanSpell, "delayed_hit_path", spellID, spellName, map[string]interface{}{
		"missileSpeed": s.Info.MissileSpeed,
		"targetCount":  len(s.Targets),
	})

	for _, target := range s.Targets {
		dist := s.Caster.DistanceTo(target)
		delayMs := dist/s.Info.MissileSpeed*1000
		if delayMs < minDelayMs {
			delayMs = minDelayMs
		}
		delay := time.Duration(delayMs) * time.Millisecond

		for _, eff := range s.Info.Effects {
			dh := delayedHit{
				target: target,
				eff:    eff,
				hitAt:  now.Add(delay),
			}
			s.delayedHits = append(s.delayedHits, dh)
		}

		s.Trace.Event(trace.SpanSpell, "delayed_hit_scheduled", spellID, spellName, map[string]interface{}{
			"target":    target.Name,
			"distance":  dist,
			"delay_ms":  delayMs,
		})
	}

	s.Trace.Event(trace.SpanSpell, "state_change", spellID, spellName, map[string]interface{}{
		"from":         "Launched",
		"to":           "Launched",
		"pending_hits": len(s.delayedHits),
	})
	return spelldef.CastResultSuccess
}

// ExecuteHit runs the Hit phase for a single delayed hit entry.
func (s *SpellContext) ExecuteHit(target *unit.Unit, eff spelldef.SpellEffectInfo) {
	if s.Cancelled || !target.IsAlive() {
		s.Trace.Event(trace.SpanSpell, "delayed_hit_skipped", s.Info.ID, s.Info.Name, map[string]interface{}{
			"cancelled":   s.Cancelled,
			"targetAlive": target.IsAlive(),
		})
		return
	}
	s.Trace.Event(trace.SpanSpell, "delayed_hit_arrived", s.Info.ID, s.Info.Name, map[string]interface{}{
		"target":      target.Name,
		"effectIndex": eff.EffectIndex,
	})
	ctx := &effectAdapter{caster: s.Caster, targets: []*unit.Unit{target}, trace: s.Trace, spellID: s.Info.ID, spellName: s.Info.Name}
	handler := s.EffectStore.GetHitHandler(eff.EffectType)
	if handler != nil {
		s.Trace.Event(trace.SpanEffectHit, "hit", s.Info.ID, s.Info.Name, map[string]interface{}{
			"effectIndex": eff.EffectIndex,
			"effectType":  int(eff.EffectType),
			"target":      target.Name,
		})
		handler(ctx, eff, target)
	}
	s.TriggerHitProc(target)
}

// executeHitAll runs the Hit phase for all effects on all targets (immediate path).
func (s *SpellContext) executeHitAll() {
	spellID, spellName := s.Info.ID, s.Info.Name
	ctx := &effectAdapter{caster: s.Caster, targets: s.Targets, trace: s.Trace, spellID: spellID, spellName: spellName}
	for _, eff := range s.Info.Effects {
		// Script hook: OnEffectHit (per effect)
		if s.ScriptRegistry != nil {
			if ss := s.ScriptRegistry.GetSpellScript(s.ID); ss != nil {
				ss.Fire(script.HookOnEffectHit, &eff)
				if ss.IsPrevented(script.HookOnEffectHit) {
					s.Trace.Event(trace.SpanScript, "effect_prevented", spellID, spellName, map[string]interface{}{
						"effectIndex": eff.EffectIndex,
					})
					continue
				}
			}
		}

		handler := s.EffectStore.GetHitHandler(eff.EffectType)
		if handler != nil {
			for _, target := range s.Targets {
				s.Trace.Event(trace.SpanEffectHit, "hit", spellID, spellName, map[string]interface{}{
					"effectIndex": eff.EffectIndex,
					"effectType":  int(eff.EffectType),
					"target":      target.Name,
				})
				handler(ctx, eff, target)

				// Script hook: OnHit (per target)
				if s.ScriptRegistry != nil {
					if ss := s.ScriptRegistry.GetSpellScript(s.ID); ss != nil {
						ss.Fire(script.HookOnHit, target)
					}
				}

				// Auto-trigger proc on target
				s.TriggerHitProc(target)
			}
		}
	}
}

// --- Channeling ---

func (s *SpellContext) startChannel() spelldef.CastResult {
	s.channelStop = make(chan struct{})
	duration := time.Duration(s.Info.ChannelDuration) * time.Millisecond
	interval := time.Duration(s.Info.TickInterval) * time.Millisecond

	if interval == 0 {
		interval = 1000 * time.Millisecond
	}

	s.State = StateChanneling
	s.Trace.Event(trace.SpanSpell, "state_change", s.Info.ID, s.Info.Name, map[string]interface{}{
		"from":     "Launched",
		"to":       "Channeling",
		"duration": duration.String(),
		"interval": interval.String(),
	})

	// No goroutine — the event loop manages the ticker via time.AfterFunc
	return spelldef.CastResultSuccess
}

func (s *SpellContext) stopChannel() {
	if s.channelStop != nil {
		close(s.channelStop)
	}
}

// CancelChannel cancels an active channel.
func (s *SpellContext) CancelChannel() {
	if s.State == StateChanneling {
		s.Cancel()
	}
}

// --- Empower ---

func (s *SpellContext) startEmpower() spelldef.CastResult {
	s.empowerStart = time.Now()
	s.empowerStage = 0
	s.empowerActive = true
	s.empowerReleased = false
	s.State = StatePreparing
	s.Trace.Event(trace.SpanSpell, "state_change", s.Info.ID, s.Info.Name, map[string]interface{}{
		"from":   "Launched",
		"to":     "Preparing",
		"reason": "empower",
		"stages": len(s.Info.EmpowerStages),
	})

	return spelldef.CastResultSuccess
}

// UpdateEmpower advances the empower timer. Call this in a loop to simulate hold-to-cast.
// Returns the current stage and whether the stage changed.
func (s *SpellContext) UpdateEmpower(elapsed time.Duration) (stage int, changed bool) {
	if !s.empowerActive {
		return s.empowerStage, false
	}

	elapsed = time.Since(s.empowerStart)
	oldStage := s.empowerStage

	for i, threshold := range s.Info.EmpowerStages {
		if elapsed >= time.Duration(threshold)*time.Millisecond {
			s.empowerStage = i + 1
		}
	}

	changed = s.empowerStage != oldStage
	if changed {
		s.Trace.Event(trace.SpanSpell, "empower_stage_changed", s.Info.ID, s.Info.Name, map[string]interface{}{
			"from":    oldStage,
			"to":      s.empowerStage,
			"elapsed": elapsed.Round(time.Millisecond).String(),
		})
	}

	return s.empowerStage, changed
}

// ReleaseEmpower releases the empower at the current stage, applying the empower multiplier.
// Returns the cast result.
func (s *SpellContext) ReleaseEmpower() spelldef.CastResult {
	spellID, spellName := s.Info.ID, s.Info.Name

	if !s.empowerActive {
		s.Trace.Event(trace.SpanSpell, "empower_release_ignored", spellID, spellName, nil)
		return spelldef.CastResultFailed
	}

	elapsed := time.Since(s.empowerStart)
	if elapsed < time.Duration(s.Info.EmpowerMinTime)*time.Millisecond {
		s.Trace.Event(trace.SpanSpell, "empower_release_failed", spellID, spellName, map[string]interface{}{
			"reason":  "below_min_time",
			"elapsed": elapsed.Round(time.Millisecond).String(),
			"min":     s.Info.EmpowerMinTime,
		})
		return spelldef.CastResultFailed
	}

	s.empowerActive = false
	s.empowerReleased = true
	s.Cancelled = false

	stage := s.empowerStage
	s.Trace.Event(trace.SpanSpell, "empower_released", spellID, spellName, map[string]interface{}{
		"stage":   stage,
		"elapsed": elapsed.Round(time.Millisecond).String(),
	})

	// Apply empower multiplier to effects
	empoweredInfo := *s.Info
	multiplier := 1.0 + 0.5*float64(stage)
	for i := range empoweredInfo.Effects {
		empoweredInfo.Effects[i] = spelldef.SpellEffectInfo{
			EffectIndex: empoweredInfo.Effects[i].EffectIndex,
			EffectType:  empoweredInfo.Effects[i].EffectType,
			SchoolMask:  empoweredInfo.Effects[i].SchoolMask,
			BasePoints:  int32(float64(empoweredInfo.Effects[i].BasePoints) * multiplier),
			Coef:        empoweredInfo.Effects[i].Coef,
			TargetA:     empoweredInfo.Effects[i].TargetA,
			TargetB:     empoweredInfo.Effects[i].TargetB,
		}
	}
	s.Info = &empoweredInfo

	// Transition to Launched and execute
	s.State = StateLaunched
	s.Trace.Event(trace.SpanSpell, "state_change", spellID, spellName, map[string]interface{}{
		"from":       "Preparing",
		"to":         "Launched",
		"reason":     "empower_release",
		"multiplier": fmt.Sprintf("%.1fx", multiplier),
	})

	ctx := &effectAdapter{caster: s.Caster, targets: s.Targets, trace: s.Trace, spellID: spellID, spellName: spellName}
	for _, eff := range s.Info.Effects {
		handler := s.EffectStore.GetLaunchHandler(eff.EffectType)
		if handler != nil {
			s.Trace.Event(trace.SpanEffectLaunch, "launch", spellID, spellName, map[string]interface{}{
				"effectIndex": eff.EffectIndex,
				"effectType":  int(eff.EffectType),
			})
			handler(ctx, eff)
		}
	}

	if s.Info.MissileSpeed > 0 {
		return s.startDelayedHit()
	}

	s.executeHitAll()
	return s.Finish()
}

// --- Event loop interface ---

// DelayedHitSchedule describes a delayed hit to be scheduled by the event loop.
type DelayedHitSchedule struct {
	Target *unit.Unit
	Eff    spelldef.SpellEffectInfo
	Delay  time.Duration
}

// GetDelayedHitSchedules returns the list of delayed hits to be scheduled.
func (s *SpellContext) GetDelayedHitSchedules() []DelayedHitSchedule {
	schedules := make([]DelayedHitSchedule, len(s.delayedHits))
	for i, dh := range s.delayedHits {
		schedules[i] = DelayedHitSchedule{
			Target: dh.target,
			Eff:    dh.eff,
			Delay:  time.Until(dh.hitAt),
		}
	}
	return schedules
}

// SetTargets replaces the internal target slice. Used for per-tick re-resolution in channeled AoE spells.
func (s *SpellContext) SetTargets(targets []*unit.Unit) {
	s.Targets = targets
}

// ChannelStop returns the stop channel for the active channel.
func (s *SpellContext) ChannelStop() chan struct{} {
	return s.channelStop
}

// TotalTicks returns the total number of channel ticks for this spell.
func (s *SpellContext) TotalTicks() int {
	duration := time.Duration(s.Info.ChannelDuration) * time.Millisecond
	interval := time.Duration(s.Info.TickInterval) * time.Millisecond
	if interval == 0 {
		interval = time.Second
	}
	return int(duration/interval) + 1
}

// ExecuteChannelTick executes one channel tick. Returns false if the channel should stop.
func (s *SpellContext) ExecuteChannelTick() bool {
	if s.Cancelled {
		return false
	}

	allDead := true
	for _, target := range s.Targets {
		if target.IsAlive() {
			allDead = false
			break
		}
	}
	if allDead {
		return false
	}

	ctx := &effectAdapter{caster: s.Caster, targets: s.Targets, trace: s.Trace, spellID: s.Info.ID, spellName: s.Info.Name}
	for _, eff := range s.Info.Effects {
		handler := s.EffectStore.GetHitHandler(eff.EffectType)
		if handler != nil {
			for _, target := range s.Targets {
				if target.IsAlive() {
					s.Trace.Event(trace.SpanEffectHit, "hit", s.Info.ID, s.Info.Name, map[string]interface{}{
						"effectIndex": eff.EffectIndex,
						"effectType":  int(eff.EffectType),
						"target":      target.Name,
					})
					handler(ctx, eff, target)
				}
			}
		}
	}

	if s.ScriptRegistry != nil {
		if ss := s.ScriptRegistry.GetSpellScript(s.ID); ss != nil {
			ss.Fire(script.HookOnChannelTick, s)
		}
	}

	return true
}

// FinishChannel marks the channel as finished.
func (s *SpellContext) FinishChannel() {
	s.State = StateFinished
}

// --- Helpers ---

// TriggerHitProc checks for proc auras on the target and triggers them.
func (s *SpellContext) TriggerHitProc(target *unit.Unit) {
	if s.AuraProvider == nil {
		return
	}
	mgr := s.AuraProvider.GetAuraManager(target)
	if mgr == nil {
		return
	}
	results := mgr.CheckProc(aura.ProcEventOnHit, s.Trace, s.Info.ID, s.Info.Name)
	for _, r := range results {
		if r.Triggered {
			s.Trace.Event(trace.SpanProc, "triggered", s.Info.ID, s.Info.Name, map[string]interface{}{
				"target":    target.Name,
				"auraSpell": r.Aura.SpellID,
			})
		}
	}
}

func (s *SpellContext) refundMana() {
	if s.ManaPaid && s.Info.PowerCost > 0 {
		switch s.Info.PowerType {
		case spelldef.PowerTypeRage:
			s.Caster.Rage += s.Info.PowerCost
			if s.Caster.Rage > s.Caster.MaxRage {
				s.Caster.Rage = s.Caster.MaxRage
			}
			s.Trace.Event(trace.SpanSpell, "rage_refunded", s.Info.ID, s.Info.Name, map[string]interface{}{
				"amount":    s.Info.PowerCost,
				"remaining": s.Caster.Rage,
			})
		default:
			s.Caster.Mana += s.Info.PowerCost
			if s.Caster.Mana > s.Caster.MaxMana {
				s.Caster.Mana = s.Caster.MaxMana
			}
			s.Trace.Event(trace.SpanSpell, "mana_refunded", s.Info.ID, s.Info.Name, map[string]interface{}{
				"amount":    s.Info.PowerCost,
				"remaining": s.Caster.Mana,
			})
		}
		s.ManaPaid = false
	}
}

// effectAdapter wraps SpellContext to satisfy effect.CasterInfo.
type effectAdapter struct {
	caster    *unit.Unit
	targets   []*unit.Unit
	trace     *trace.Trace
	spellID   uint32
	spellName string
}

func (a *effectAdapter) Caster() *unit.Unit     { return a.caster }
func (a *effectAdapter) Targets() []*unit.Unit  { return a.targets }
func (a *effectAdapter) GetTrace() *trace.Trace { return a.trace }
func (a *effectAdapter) GetSpellID() uint32     { return a.spellID }
func (a *effectAdapter) GetSpellName() string   { return a.spellName }

// String returns a debug representation.
func (s *SpellContext) String() string {
	return fmt.Sprintf("Spell(%s, state=%s, caster=%s, targets=%d)",
		s.Info.Name, s.State, s.Caster.Name, len(s.Targets))
}
