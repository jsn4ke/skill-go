package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"skill-go/server/api"
	"skill-go/server/aura"
	"skill-go/server/cooldown"
	"skill-go/server/effect"
	"skill-go/server/script"
	"skill-go/server/spell"
	"skill-go/server/spelldef"
	"skill-go/server/targeting"
	"skill-go/server/trace"
	"skill-go/server/unit"
)

func main() {
	cli := flag.Bool("cli", false, "run in CLI demo mode instead of HTTP server")
	port := flag.String("port", ":13001", "HTTP listen address")
	flag.Parse()

	log.SetFlags(log.Ltime | log.Lmicroseconds)

	if *cli {
		runCLIDemo()
	} else {
		runHTTPServer(*port)
	}
}

// ---------------------------------------------------------------------------
// HTTP server mode (default)
// ---------------------------------------------------------------------------

func runHTTPServer(addr string) {
	// Create file sink for spell trace logging
	fileSink, err := trace.NewFileSink("server/log")
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: file logging disabled: %v\n", err)
	}

	gs := api.NewGameState(fileSink)
	srv := api.NewServer(addr, gs)
	fmt.Printf("=== skill-go Spell Demo ===\n")
	fmt.Printf("Open http://localhost%s in your browser\n", addr)
	fmt.Printf("Trace log: server/log/trace-*.log\n")
	fmt.Printf("SSE stream: http://localhost%s/api/trace/stream\n\n", addr)

	// Graceful shutdown
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		fmt.Println("\nShutting down...")
		if fileSink != nil {
			fileSink.Close()
		}
		srv.Close()
	}()

	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		fmt.Fprintf(os.Stderr, "server error: %v\n", err)
		os.Exit(1)
	}
}

// ---------------------------------------------------------------------------
// CLI demo mode (--cli flag)
// ---------------------------------------------------------------------------

func runCLIDemo() {
	fmt.Println("=== skill-go server: spell flow tracing demo ===")
	fmt.Println()

	// --- Setup ---
	mage := unit.NewUnit(1, "Mage", 5000, 20000)
	mage.Position = unit.Position{X: 0, Y: 0, Z: 0}
	fmt.Printf("Caster: %s\n\n", mage)

	target := unit.NewUnit(2, "Target Dummy", 10000, 5000)
	target.Position = unit.Position{X: 20, Y: 0, Z: 0}
	fmt.Printf("Target: %s\n", target)
	fmt.Println()

	// --- Trace ---
	recorder := trace.NewFlowRecorder()

	// --- Providers ---
	history := cooldown.NewSpellHistory()
	targetAuraMgr := aura.NewAuraManager(target)
	auraMap := map[uint64]*aura.AuraManager{target.GUID: targetAuraMgr}
	auraProvider := &simpleAuraProvider{managers: auraMap}
	registry := script.NewRegistry()

	store := effect.NewStore()
	effect.RegisterExtended(store, makeAuraHandler(auraProvider), nil)

	newCtx := func(info *spelldef.SpellInfo, targets []*unit.Unit) *spell.SpellContext {
		ctx := spell.New(info.ID, info, mage, targets)
		ctx.EffectStore = store
		ctx.HistoryProvider = history
		ctx.CooldownProvider = &tracingCooldownHistory{SpellHistory: history}
		ctx.AuraProvider = auraProvider
		ctx.ScriptRegistry = registry
		// Add recorder to trace
		ctx.Trace.AddSink(recorder)
		return ctx
	}

	// ===== INTEGRATION: Cast → Cooldown =====
	fmt.Println("===== Integration: Cast → Cooldown =====")
	cdSpell := &spelldef.SpellInfo{
		ID:                    1001,
		Name:                  "Fireball",
		SchoolMask:            spelldef.SchoolMaskFire,
		CastTime:              0,
		RecoveryTime:          6000,
		CategoryRecoveryTime: 1500,
		PowerCost:             0,
		Effects: []spelldef.SpellEffectInfo{
			{EffectIndex: 0, EffectType: spelldef.SpellEffectSchoolDamage, BasePoints: 100},
		},
	}
	ctx := newCtx(cdSpell, []*unit.Unit{target})
	ctx.Prepare()
	fmt.Printf("  Cast fireball → CD remaining: %v\n", history.GetCooldownRemaining(1001))
	fmt.Printf("  On GCD (cat=0): %v\n", history.IsOnGCD(0))

	// ===== INTEGRATION: Charges =====
	fmt.Println("\n--- Charges integration ---")
	chargeSpell := &spelldef.SpellInfo{
		ID:            3001,
		Name:          "ChargeSpell",
		MaxCharges:    3,
		CastTime:      0,
		PowerCost:     0,
		Effects:       []spelldef.SpellEffectInfo{},
	}
	history.InitCharges(3001, 3, 20000)
	fmt.Printf("  Charges before: %d\n", history.GetChargeRemaining(3001))
	ctx = newCtx(chargeSpell, []*unit.Unit{target})
	ctx.Prepare()
	fmt.Printf("  Charges after cast: %d\n", history.GetChargeRemaining(3001))

	// ===== INTEGRATION: Hit → Proc =====
	fmt.Println("\n===== Integration: Hit → Proc =====")
	procAura := &aura.Aura{
		SpellID:        5003,
		CasterGUID:     mage.GUID,
		Caster:         mage,
		AuraType:       aura.AuraTypeProc,
		ProcChance:     100.0,
		ProcCharges:    3,
		RemainingProcs: 3,
		Effects:        []*aura.AuraEffect{{AuraType: aura.AuraTypeProc}},
	}
	targetAuraMgr.ApplyAura(procAura, nil, 5003, "")
	fmt.Printf("  Proc aura applied, charges: %d\n", procAura.RemainingProcs)

	procSpell := &spelldef.SpellInfo{
		ID:         7001,
		Name:       "ProcTriggerSpell",
		CastTime:   0,
		PowerCost:  0,
		Effects: []spelldef.SpellEffectInfo{
			{EffectIndex: 0, EffectType: spelldef.SpellEffectSchoolDamage, BasePoints: 50},
		},
	}
	ctx = newCtx(procSpell, []*unit.Unit{target})
	ctx.Prepare()
	fmt.Printf("  After hit, proc charges: %d\n", procAura.RemainingProcs)
	fmt.Printf("  Aura still active: %v\n", targetAuraMgr.HasAura(5003))

	// ===== INTEGRATION: ApplyAura effect =====
	fmt.Println("\n===== Integration: ApplyAura Effect =====")
	buffSpell := &spelldef.SpellInfo{
		ID:         5001,
		Name:       "ApplyBuffSpell",
		CastTime:   0,
		PowerCost:  0,
		Effects: []spelldef.SpellEffectInfo{
			{EffectIndex: 0, EffectType: spelldef.SpellEffectApplyAura, AuraType: int32(aura.AuraTypeBuff), AuraDuration: 10000},
		},
	}
	ctx = newCtx(buffSpell, []*unit.Unit{target})
	ctx.Prepare()
	fmt.Printf("  After cast, target has buff aura: %v\n", targetAuraMgr.HasAura(9000))

	// ===== INTEGRATION: Script auto-trigger on cast =====
	fmt.Println("\n===== Integration: Script Auto-Trigger =====")
	registry.RegisterSpellScript(8001, func(ss *script.SpellScript) {
		ss.OnCast(func(arg interface{}) {
			fmt.Printf("  [Script] OnCast auto-triggered!\n")
		})
	})
	scriptedSpell := &spelldef.SpellInfo{
		ID:         8001,
		Name:       "ScriptedSpell",
		CastTime:   0,
		PowerCost:  0,
		Effects: []spelldef.SpellEffectInfo{
			{EffectIndex: 0, EffectType: spelldef.SpellEffectSchoolDamage, BasePoints: 50},
		},
	}
	ctx = newCtx(scriptedSpell, []*unit.Unit{target})
	ctx.Prepare()

	// ===== Line selection =====
	fmt.Println("\n===== Line Selection =====")
	units := []*unit.Unit{
		mage,
		target,
		unit.NewUnit(4, "Unit4", 8000, 5000),
		unit.NewUnit(5, "Unit5", 6000, 5000),
	}
	units[2].Position = unit.Position{X: 0, Y: 5, Z: 0}  // on the line
	units[3].Position = unit.Position{X: 10, Y: 10, Z: 0} // off the line

	world := &mockUnitProvider{units: units}
	lineCtx := &targeting.SelectionContext{
		Caster:          mage,
		ExplicitTargets: []*unit.Unit{},
		Descriptor: targeting.TargetDescriptor{
			Category:  targeting.SelectLine,
			Reference: targeting.RefCaster,
			Dir:       targeting.Direction{Length: 20, Width: 4},
		},
	}
	selected := targeting.Select(lineCtx, world, nil, 0, "")
	fmt.Printf("  Line (length=20, width=4): selected %d units\n", len(selected))
	for _, u := range selected {
		fmt.Printf("    - %s at (%.0f, %.0f)\n", u.Name, u.Position.X, u.Position.Y)
	}

	// ===== Trajectory selection =====
	fmt.Println("\n===== Trajectory Selection =====")
	trajCtx := &targeting.SelectionContext{
		Caster:          mage,
		ExplicitTargets: []*unit.Unit{target},
		Descriptor: targeting.TargetDescriptor{
			Category:  targeting.SelectTrajectory,
			Reference: targeting.RefTarget,
			Dir:       targeting.Direction{Width: 4},
		},
	}
	selected = targeting.Select(trajCtx, world, nil, 0, "")
	fmt.Printf("  Trajectory (caster→target, width=4): selected %d units\n", len(selected))

	// ===== Phase guard =====
	fmt.Println("\n===== Phase Guard =====")
	guard := script.NewPhaseGuard(script.PhasePrepare)
	fmt.Printf("  Prepare — can access targets: %v\n", guard.CanAccessTargets())
	guard.SetPhase(script.PhaseHit)
	fmt.Printf("  Hit — can access targets: %v, can modify hit: %v\n", guard.CanAccessTargets(), guard.CanModifyHit())

	// ===== Flow trace summary =====
	fmt.Println("\n===== Flow Trace Summary =====")
	fmt.Printf("  Total events captured: %d\n", recorder.Count("", ""))
	fmt.Printf("  Spell events: %d\n", recorder.Count(trace.SpanSpell, ""))
	fmt.Printf("  CheckCast events: %d\n", recorder.Count(trace.SpanCheckCast, ""))
	fmt.Printf("  Effect launch events: %d\n", recorder.Count(trace.SpanEffectLaunch, ""))
	fmt.Printf("  Effect hit events: %d\n", recorder.Count(trace.SpanEffectHit, ""))
	fmt.Printf("  Proc events: %d\n", recorder.Count(trace.SpanProc, ""))

	fmt.Println()
	fmt.Println("=== demo complete ===")
}

// simpleAuraProvider implements spell.AuraProvider.
type simpleAuraProvider struct {
	managers map[uint64]*aura.AuraManager
}

func (p *simpleAuraProvider) GetAuraManager(target interface{}) *aura.AuraManager {
	if u, ok := target.(*unit.Unit); ok {
		return p.managers[u.GUID]
	}
	return nil
}

// makeAuraHandler creates an effect.AuraHandler that applies auras through the provider.
func makeAuraHandler(provider *simpleAuraProvider) effect.AuraHandler {
	return func(ctx effect.CasterInfo, eff spelldef.SpellEffectInfo, target *unit.Unit) {
		if provider == nil {
			return
		}
		mgr := provider.GetAuraManager(target)
		if mgr == nil {
			return
		}
		a := &aura.Aura{
			SpellID:    uint32(eff.EffectIndex) + 9000,
			CasterGUID: ctx.Caster().GUID,
			Caster:     ctx.Caster(),
			AuraType:   aura.AuraType(eff.AuraType),
			Duration:   eff.AuraDuration,
			StackAmount: 1,
			Effects: []*aura.AuraEffect{
				{AuraType: aura.AuraType(eff.AuraType), BaseAmount: eff.BasePoints},
			},
		}
		mgr.ApplyAura(a, ctx.GetTrace(), ctx.GetSpellID(), ctx.GetSpellName())
	}
}

// mockUnitProvider implements targeting.UnitProvider.
type mockUnitProvider struct {
	units []*unit.Unit
}

func (m *mockUnitProvider) GetAllUnits() []*unit.Unit {
	return m.units
}

// tracingCooldownHistory wraps cooldown.SpellHistory with trace-aware methods.
type tracingCooldownHistory struct {
	*cooldown.SpellHistory
}

func (t *tracingCooldownHistory) TraceAddCooldown(spellID uint32, durationMs int32, category int32, tr *trace.Trace) {
	t.SpellHistory.TraceAddCooldown(spellID, durationMs, category, tr)
}

func (t *tracingCooldownHistory) TraceConsumeCharge(spellID uint32, tr *trace.Trace) bool {
	return t.SpellHistory.TraceConsumeCharge(spellID, tr)
}

func (t *tracingCooldownHistory) TraceStartGCD(category int32, durationMs int32, tr *trace.Trace) {
	t.SpellHistory.TraceStartGCD(category, durationMs, tr)
}
