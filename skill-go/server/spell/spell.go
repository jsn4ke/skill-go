package spell

import (
	"fmt"
	"log"
	"sync"
	"time"

	"skill-go/server/effect"
	"skill-go/server/script"
	"skill-go/server/spelldef"
	"skill-go/server/unit"
)

// SpellState represents the current phase of a spell cast.
type SpellState int

const (
	StateNone      SpellState = iota // 0: just created
	StatePreparing                     // 1: casting bar in progress
	StateLaunched                      // 2: spell launched, effects in flight
	StateChanneling                    // 3: channeled spell active
	StateFinished                      // 4: spell complete, awaiting cleanup
	StateIdle                          // 5: waiting for auto-repeat trigger
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
	target   *unit.Unit
	eff      spelldef.SpellEffectInfo
	hitAt    time.Time
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

	// Cast time modifier chain
	CastModifiers ModifierChain

	// Spell history provider for cooldown/charge/GCD checks
	HistoryProvider SpellHistoryProvider

	// Script support
	ScriptRegistry *script.Registry

	// Delayed execution
	delayedHits  []delayedHit
	pendingHits  sync.WaitGroup

	// Channeled spell
	channelTicker *time.Ticker
	channelDone   chan struct{}
	channelStop   chan struct{}

	// Empower spell
	empowerStart   time.Time
	empowerStage   int
	empowerActive  bool
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
	}
}

// Prepare performs the validation chain, consumes resources, and transitions to Preparing.
func (s *SpellContext) Prepare() spelldef.CastResult {
	log.Printf("[%s] prepare() — state: %s", s.Info.Name, s.State)

	if !s.Caster.IsAlive() {
		log.Printf("[%s] prepare FAILED: caster is dead", s.Info.Name)
		s.LastCastErr = spelldef.CastErrDead
		s.State = StateFinished
		return spelldef.CastResultFailed
	}

	// Full validation chain
	if err := CheckCast(s.Info, s.Caster, s.Targets, s.HistoryProvider); err != spelldef.CastErrNone {
		log.Printf("[%s] prepare FAILED: CheckCast error %d", s.Info.Name, err)
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
				log.Printf("[%s] prepare FAILED: script prevented cast", s.Info.Name)
				s.LastCastErr = spelldef.CastErrInterrupted
				s.State = StateFinished
				return spelldef.CastResultFailed
			}
		}
	}

	if s.Info.PowerCost > 0 {
		if !s.Caster.ConsumeMana(s.Info.PowerCost) {
			log.Printf("[%s] prepare FAILED: not enough mana (need %d, have %d)",
				s.Info.Name, s.Info.PowerCost, s.Caster.Mana)
			s.LastCastErr = spelldef.CastErrNoMana
			s.State = StateFinished
			return spelldef.CastResultFailed
		}
		s.ManaPaid = true
		log.Printf("[%s] consumed %d mana → %d remaining", s.Info.Name, s.Info.PowerCost, s.Caster.Mana)
	}

	// Apply cast time modifier chain
	baseCastTime := s.Info.CastTime
	finalCastTime := baseCastTime
	if len(s.CastModifiers) > 0 {
		finalCastTime = s.CastModifiers.Apply(baseCastTime)
		if finalCastTime != baseCastTime {
			log.Printf("[%s] cast time modified: %dms → %dms", s.Info.Name, baseCastTime, finalCastTime)
		}
	}

	s.CastDuration = time.Duration(finalCastTime) * time.Millisecond
	s.CastStart = time.Now()
	s.State = StatePreparing

	log.Printf("[%s] state → Preparing (cast time: %v)", s.Info.Name, s.CastDuration)

	if s.CastDuration == 0 {
		return s.Cast()
	}

	return spelldef.CastResultSuccess
}

// Cast performs re-validation, launches effects, then routes to immediate/delay/channel/empower path.
func (s *SpellContext) Cast() spelldef.CastResult {
	log.Printf("[%s] cast() — state: %s", s.Info.Name, s.State)

	if !s.Caster.IsAlive() {
		log.Printf("[%s] cast FAILED: caster died during cast", s.Info.Name)
		s.refundMana()
		s.State = StateFinished
		return spelldef.CastResultFailed
	}

	// Re-check range at launch time
	if !ReCheckRange(s.Info, s.Caster, s.Targets) {
		log.Printf("[%s] cast FAILED: target out of range at launch", s.Info.Name)
		s.refundMana()
		s.LastCastErr = spelldef.CastErrOutOfRange
		s.State = StateFinished
		return spelldef.CastResultFailed
	}

	for _, t := range s.Targets {
		if !t.IsAlive() {
			log.Printf("[%s] cast FAILED: target %s died during cast", s.Info.Name, t.Name)
			s.refundMana()
			s.State = StateFinished
			return spelldef.CastResultFailed
		}
	}

	s.State = StateLaunched
	log.Printf("[%s] state → Launched", s.Info.Name)

	// Launch phase
	ctx := &effectAdapter{caster: s.Caster, targets: s.Targets}

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
			log.Printf("[%s] launch — effect[%d] type=%d", s.Info.Name, eff.EffectIndex, eff.EffectType)
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
	case s.Info.DelayMs > 0:
		return s.startDelayedHit()
	default:
		// Immediate path
		s.executeHitAll()
		return s.Finish()
	}
}

// Cancel interrupts the spell from Preparing, Channeling, or Empower state.
func (s *SpellContext) Cancel() {
	log.Printf("[%s] cancel() — state: %s", s.Info.Name, s.State)

	switch s.State {
	case StatePreparing:
		s.Cancelled = true
		s.refundMana()
		s.State = StateFinished
		log.Printf("[%s] state → Finished (cancelled from Preparing)", s.Info.Name)

	case StateChanneling:
		s.Cancelled = true
		s.stopChannel()
		s.State = StateFinished
		log.Printf("[%s] state → Finished (cancelled from Channeling)", s.Info.Name)

	case StateLaunched:
		// Cancel pending delayed hits
		s.Cancelled = true
		s.delayedHits = nil
		s.State = StateFinished
		log.Printf("[%s] state → Finished (cancelled from Launched)", s.Info.Name)

	default:
		log.Printf("[%s] cancel ignored: not in cancellable state", s.Info.Name)
	}
}

// Finish handles post-cast cleanup.
func (s *SpellContext) Finish() spelldef.CastResult {
	log.Printf("[%s] finish() — state: %s", s.Info.Name, s.State)

	if s.Info.IsAutoRepeat && !s.Cancelled {
		s.State = StateIdle
		log.Printf("[%s] state → Idle (auto-repeat)", s.Info.Name)
	} else {
		s.State = StateFinished
		log.Printf("[%s] state → Finished (spell complete)", s.Info.Name)
	}

	return spelldef.CastResultSuccess
}

// --- Delayed execution path ---

func (s *SpellContext) startDelayedHit() spelldef.CastResult {
	log.Printf("[%s] delayed hit path — %d ms delay, %d targets", s.Info.Name, s.Info.DelayMs, len(s.Targets))

	now := time.Now()
	delay := time.Duration(s.Info.DelayMs) * time.Millisecond

	for _, target := range s.Targets {
		for _, eff := range s.Info.Effects {
			dh := delayedHit{
				target: target,
				eff:    eff,
				hitAt:  now.Add(delay),
			}
			s.delayedHits = append(s.delayedHits, dh)
			s.pendingHits.Add(1)

			go func(d delayedHit) {
				remaining := time.Until(d.hitAt)
				if remaining > 0 {
					time.Sleep(remaining)
				}
				log.Printf("[%s] delayed hit arrived — target=%s, effect[%d]", s.Info.Name, d.target.Name, d.eff.EffectIndex)
				s.executeHit(d)
				s.pendingHits.Done()
			}(dh)
		}
	}

	// Launch phase is done, but we don't finish yet — wait for delayed hits
	// For demo: we return success and caller can call WaitDelayedHits()
	log.Printf("[%s] state → Launched (waiting for %d delayed hits)", s.Info.Name, len(s.delayedHits))
	return spelldef.CastResultSuccess
}

// WaitDelayedHits blocks until all delayed hits have been processed.
func (s *SpellContext) WaitDelayedHits() {
	s.pendingHits.Wait()
	log.Printf("[%s] all delayed hits processed", s.Info.Name)
	s.Finish()
}

// executeHit runs the Hit phase for a single delayed hit entry.
func (s *SpellContext) executeHit(d delayedHit) {
	if s.Cancelled || !d.target.IsAlive() {
		log.Printf("[%s] delayed hit skipped (cancelled=%v, target alive=%v)", s.Info.Name, s.Cancelled, d.target.IsAlive())
		return
	}
	ctx := &effectAdapter{caster: s.Caster, targets: []*unit.Unit{d.target}}
	handler := s.EffectStore.GetHitHandler(d.eff.EffectType)
	if handler != nil {
		log.Printf("[%s] hit — effect[%d] type=%d → %s", s.Info.Name, d.eff.EffectIndex, d.eff.EffectType, d.target.Name)
		handler(ctx, d.eff, d.target)
	}
}

// executeHitAll runs the Hit phase for all effects on all targets (immediate path).
func (s *SpellContext) executeHitAll() {
	ctx := &effectAdapter{caster: s.Caster, targets: s.Targets}
	for _, eff := range s.Info.Effects {
		// Script hook: OnEffectHit (per effect)
		if s.ScriptRegistry != nil {
			if ss := s.ScriptRegistry.GetSpellScript(s.ID); ss != nil {
				ss.Fire(script.HookOnEffectHit, &eff)
				if ss.IsPrevented(script.HookOnEffectHit) {
					log.Printf("[%s] effect[%d] hit prevented by script", s.Info.Name, eff.EffectIndex)
					continue
				}
			}
		}

		handler := s.EffectStore.GetHitHandler(eff.EffectType)
		if handler != nil {
			for _, target := range s.Targets {
				log.Printf("[%s] hit — effect[%d] type=%d → %s", s.Info.Name, eff.EffectIndex, eff.EffectType, target.Name)
				handler(ctx, eff, target)

				// Script hook: OnHit (per target)
				if s.ScriptRegistry != nil {
					if ss := s.ScriptRegistry.GetSpellScript(s.ID); ss != nil {
						ss.Fire(script.HookOnHit, target)
					}
				}
			}
		}
	}
}

// --- Channeling ---

func (s *SpellContext) startChannel() spelldef.CastResult {
	s.channelDone = make(chan struct{})
	s.channelStop = make(chan struct{})
	duration := time.Duration(s.Info.ChannelDuration) * time.Millisecond
	interval := time.Duration(s.Info.TickInterval) * time.Millisecond

	if interval == 0 {
		interval = 1000 * time.Millisecond
	}

	s.channelTicker = time.NewTicker(interval)
	s.State = StateChanneling
	log.Printf("[%s] state → Channeling (duration=%v, interval=%v)", s.Info.Name, duration, interval)

	tickCount := 0

	go func() {
		defer func() {
			s.channelTicker.Stop()
			close(s.channelDone)
		}()

		timer := time.NewTimer(duration)
		defer timer.Stop()

		for {
			select {
			case _, ok := <-s.channelTicker.C:
				if !ok {
					return
				}
				tickCount++
				log.Printf("[%s] channel tick #%d", s.Info.Name, tickCount)

					// Check all targets still alive — stop channel if all dead
					allDead := true
					for _, target := range s.Targets {
						if target.IsAlive() {
							allDead = false
							break
						}
					}
					if allDead {
						log.Printf("[%s] all targets dead, stopping channel", s.Info.Name)
						return
					}

				ctx := &effectAdapter{caster: s.Caster, targets: s.Targets}
				for _, eff := range s.Info.Effects {
					handler := s.EffectStore.GetHitHandler(eff.EffectType)
					if handler != nil {
						for _, target := range s.Targets {
							if target.IsAlive() {
								log.Printf("[%s] hit — effect[%d] type=%d → %s",
									s.Info.Name, eff.EffectIndex, eff.EffectType, target.Name)
								handler(ctx, eff, target)
							}
						}
					}
				}

			case <-timer.C:
				log.Printf("[%s] channel duration elapsed (%d ticks)", s.Info.Name, tickCount)
				s.State = StateFinished
				return

			case <-s.channelStop:
				log.Printf("[%s] channel stopped (%d ticks)", s.Info.Name, tickCount)
				s.State = StateFinished
				return
			}
		}
	}()

	return spelldef.CastResultSuccess
}

func (s *SpellContext) stopChannel() {
	if s.channelStop != nil {
		close(s.channelStop)
	}
}

// WaitChannel blocks until the channel finishes.
func (s *SpellContext) WaitChannel() {
	if s.channelDone != nil {
		<-s.channelDone
		log.Printf("[%s] channel finished", s.Info.Name)
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
	log.Printf("[%s] state → Preparing (empower, stages=%d)", s.Info.Name, len(s.Info.EmpowerStages))

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
		log.Printf("[%s] empower stage changed: %d → %d (elapsed=%v)",
			s.Info.Name, oldStage, s.empowerStage, elapsed.Round(time.Millisecond))
	}

	return s.empowerStage, changed
}

// ReleaseEmpower releases the empower at the current stage, applying the empower multiplier.
// Returns the cast result.
func (s *SpellContext) ReleaseEmpower() spelldef.CastResult {
	if !s.empowerActive {
		log.Printf("[%s] release ignored: not empowering", s.Info.Name)
		return spelldef.CastResultFailed
	}

	elapsed := time.Since(s.empowerStart)
	if elapsed < time.Duration(s.Info.EmpowerMinTime)*time.Millisecond {
		log.Printf("[%s] release FAILED: below min empower time (elapsed=%v, min=%vms)",
			s.Info.Name, elapsed.Round(time.Millisecond), s.Info.EmpowerMinTime)
		return spelldef.CastResultFailed
	}

	s.empowerActive = false
	s.empowerReleased = true
	s.Cancelled = false

	stage := s.empowerStage
	log.Printf("[%s] empower released at stage %d (elapsed=%v)", s.Info.Name, stage, elapsed.Round(time.Millisecond))

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
	log.Printf("[%s] state → Launched (empower multiplier: %.1fx)", s.Info.Name, multiplier)

	ctx := &effectAdapter{caster: s.Caster, targets: s.Targets}
	for _, eff := range s.Info.Effects {
		handler := s.EffectStore.GetLaunchHandler(eff.EffectType)
		if handler != nil {
			log.Printf("[%s] launch — effect[%d] type=%d", s.Info.Name, eff.EffectIndex, eff.EffectType)
			handler(ctx, eff)
		}
	}

	if s.Info.DelayMs > 0 {
		return s.startDelayedHit()
	}

	s.executeHitAll()
	return s.Finish()
}

// --- Helpers ---

func (s *SpellContext) refundMana() {
	if s.ManaPaid && s.Info.PowerCost > 0 {
		s.Caster.Mana += s.Info.PowerCost
		if s.Caster.Mana > s.Caster.MaxMana {
			s.Caster.Mana = s.Caster.MaxMana
		}
		log.Printf("[%s] refunded %d mana → %d", s.Info.Name, s.Info.PowerCost, s.Caster.Mana)
		s.ManaPaid = false
	}
}

// effectAdapter wraps SpellContext to satisfy effect.CasterInfo.
type effectAdapter struct {
	caster  *unit.Unit
	targets []*unit.Unit
}

func (a *effectAdapter) Caster() *unit.Unit  { return a.caster }
func (a *effectAdapter) Targets() []*unit.Unit { return a.targets }

// String returns a debug representation.
func (s *SpellContext) String() string {
	return fmt.Sprintf("Spell(%s, state=%s, caster=%s, targets=%d)",
		s.Info.Name, s.State, s.Caster.Name, len(s.Targets))
}
