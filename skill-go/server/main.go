package main

import (
	"fmt"
	"log"

	"skill-go/server/aura"
	"skill-go/server/cooldown"
	"skill-go/server/effect"
	"skill-go/server/script"
	"skill-go/server/spell"
	"skill-go/server/spelldef"
	"skill-go/server/targeting"
	"skill-go/server/unit"
)

func main() {
	log.SetFlags(log.Ltime | log.Lmicroseconds)
	fmt.Println("=== skill-go server: full spell system replication demo ===")
	fmt.Println()

	// --- Setup ---
	mage := unit.NewUnit(1, "Mage", 5000, 20000)
	mage.Position = unit.Position{X: 0, Y: 0, Z: 0}
	fmt.Printf("Caster: %s\n\n", mage)

	target := unit.NewUnit(2, "Target Dummy", 10000, 5000)
	target.Position = unit.Position{X: 20, Y: 0, Z: 0}
	fmt.Printf("Target: %s\n", target)
	fmt.Println()

	// ===== Phase 1: Validation Chain =====
	fmt.Println("===== Phase 1: Validation Chain =====")

	// Scenario 1a: Silence check
	fmt.Println("--- 1a: Silence check ---")
	silenceSpell := &spelldef.SpellInfo{
		ID:             1001,
		Name:           "Fireball",
		SchoolMask:     spelldef.SchoolMaskFire,
		CastTime:       2000,
		PreventionType: spelldef.PreventionTypeSilence,
		PowerCost:      350,
		RangeMax:       40,
		Effects: []spelldef.SpellEffectInfo{
			{EffectIndex: 0, EffectType: spelldef.SpellEffectSchoolDamage, BasePoints: 500},
		},
	}
	mage.ApplyUnitState(spelldef.UnitStateSilenced)
	ctx := spell.New(silenceSpell.ID, silenceSpell, mage, []*unit.Unit{target})
	result := ctx.Prepare()
	log.Printf("  Cast while silenced → result=%d, err=%d\n", result, ctx.LastCastErr)
	mage.RemoveUnitState(spelldef.UnitStateSilenced)

	// Scenario 1b: Range check
	fmt.Println("--- 1b: Range check ---")
	farTarget := unit.NewUnit(3, "Far Target", 10000, 5000)
	farTarget.Position = unit.Position{X: 100, Y: 0, Z: 0}
	ctx = spell.New(silenceSpell.ID, silenceSpell, mage, []*unit.Unit{farTarget})
	result = ctx.Prepare()
	log.Printf("  Cast out of range → result=%d, err=%d\n", result, ctx.LastCastErr)

	// Scenario 1c: Cast time modifier (haste 50%)
	fmt.Println("--- 1c: Cast time modifier ---")
	hasteSpell := *silenceSpell
	hasteSpell.Name = "Hasted Fireball"
	hasteSpell.CastTime = 3000
	hasteSpell.RangeMax = 40
	ctx = spell.New(1002, &hasteSpell, mage, []*unit.Unit{target})
	ctx.CastModifiers = spell.ModifierChain{
		spell.HasteModifier{HastePercent: 50},
	}
	ctx.Prepare()
	log.Printf("  Base 3000ms with 50%% haste → %v\n", ctx.CastDuration)
	ctx.Cancel()

	// ===== Phase 1: Effect types =====
	fmt.Println("\n--- 1d: Energize effect ---")
	energizeSpell := &spelldef.SpellInfo{
		ID:         2001,
		Name:       "Mana Restore",
		CastTime:   0,
		RangeMax:   40,
		PowerCost:  0,
		Effects: []spelldef.SpellEffectInfo{
			{EffectIndex: 0, EffectType: spelldef.SpellEffectEnergize, EnergizeType: spelldef.PowerTypeMana, EnergizeAmount: 500},
		},
	}
	store := effect.NewStore()
	effect.RegisterExtended(store, nil, nil)
	ctx = spell.New(energizeSpell.ID, energizeSpell, mage, []*unit.Unit{target})
	ctx.EffectStore = store
	fmt.Printf("  Before: %s\n", target)
	ctx.Prepare()
	fmt.Printf("  After:  %s\n", target)

	fmt.Println()

	// ===== Phase 2: Target Selection =====
	fmt.Println("===== Phase 2: Target Selection =====")

	// Create multiple units for area/chain testing
	units := []*unit.Unit{
		mage,
		target,
		unit.NewUnit(4, "Enemy1", 8000, 5000),
		unit.NewUnit(5, "Enemy2", 6000, 5000),
		unit.NewUnit(6, "Enemy3", 9000, 5000),
	}
	units[2].Position = unit.Position{X: 5, Y: 5, Z: 0}
	units[3].Position = unit.Position{X: 8, Y: 3, Z: 0}
	units[4].Position = unit.Position{X: 3, Y: 7, Z: 0}

	world := &mockUnitProvider{units: units}

	// Area selection
	fmt.Println("--- 2a: Area selection ---")
	areaCtx := &targeting.SelectionContext{
		Caster:          mage,
		ExplicitTargets: []*unit.Unit{},
		Descriptor: targeting.TargetDescriptor{
			Category:  targeting.SelectArea,
			Reference: targeting.RefCaster,
			ObjType:   targeting.ObjUnit,
			Validation: targeting.ValidationRule{
				MaxTargets: 5,
				AliveOnly:  true,
			},
			Dir: targeting.Direction{Radius: 10},
		},
	}
	selected := targeting.Select(areaCtx, world)
	fmt.Printf("  Area (radius=10): selected %d units\n", len(selected))
	for _, u := range selected {
		fmt.Printf("    - %s (HP:%d)\n", u.Name, u.Health)
	}

	// Chain selection
	fmt.Println("--- 2b: Chain selection ---")
	chainCtx := &targeting.SelectionContext{
		Caster:          mage,
		ExplicitTargets: []*unit.Unit{target},
		Descriptor: targeting.TargetDescriptor{
			Category:  targeting.SelectChain,
			Reference: targeting.RefTarget,
			ObjType:   targeting.ObjUnit,
			Validation: targeting.ValidationRule{MaxTargets: 3},
			Dir:       targeting.Direction{Radius: 15},
		},
	}
	selected = targeting.Select(chainCtx, world)
	fmt.Printf("  Chain (max=3, range=15): selected %d units\n", len(selected))
	for _, u := range selected {
		fmt.Printf("    - %s (HP:%d)\n", u.Name, u.Health)
	}

	// Script filter interception
	fmt.Println("--- 2c: Script filter interception ---")
	filterCtx := &targeting.SelectionContext{
		Caster:          mage,
		ExplicitTargets: []*unit.Unit{},
		Descriptor: targeting.TargetDescriptor{
			Category:  targeting.SelectArea,
			Reference: targeting.RefCaster,
			ObjType:   targeting.ObjUnit,
			Validation: targeting.ValidationRule{AliveOnly: true},
			Dir:       targeting.Direction{Radius: 50},
		},
		Filters: map[targeting.FilterPoint][]targeting.FilterFunc{
			targeting.FilterUnit: {
				func(targets []*unit.Unit) []*unit.Unit {
					var result []*unit.Unit
					for _, u := range targets {
						if u.Health < 10000 {
							result = append(result, u)
						}
					}
					log.Printf("  [Filter] script filter: %d → %d units (HP < 10000)", len(targets), len(result))
					return result
				},
			},
		},
	}
	selected = targeting.Select(filterCtx, world)
	fmt.Printf("  After filter: %d units\n", len(selected))

	fmt.Println()

	// ===== Phase 3: Cooldown & Charge =====
	fmt.Println("===== Phase 3: Cooldown & Charge =====")

	history := cooldown.NewSpellHistory()

	// Cooldown
	fmt.Println("--- 3a: Spell cooldown ---")
	history.AddCooldown(1001, 6000, 1)
	fmt.Printf("  Added 6s CD for spell 1001\n")
	fmt.Printf("  Remaining: %v\n", history.GetCooldownRemaining(1001))
	history.Update()
	fmt.Printf("  After update: IsReady=%v\n", history.IsReady(1001, spelldef.SchoolMaskFire))

	// Charges
	fmt.Println("--- 3b: Charge-based spell ---")
	history.InitCharges(3001, 3, 20000) // 3 charges, 20s recovery
	fmt.Printf("  Charges: %d\n", history.GetChargeRemaining(3001))
	history.ConsumeCharge(3001)
	history.ConsumeCharge(3001)
	fmt.Printf("  After 2 uses: %d charges\n", history.GetChargeRemaining(3001))
	fmt.Printf("  IsReady (has charges): %v\n", history.IsReady(3001, 0))
	history.ConsumeCharge(3001)
	fmt.Printf("  After 3 uses: %d charges\n", history.GetChargeRemaining(3001))
	fmt.Printf("  IsReady (no charges): %v\n", history.IsReady(3001, 0))

	// School lockout
	fmt.Println("--- 3c: School lockout ---")
	history.AddSchoolLockout(spelldef.SchoolMaskFire, 5000)
	fmt.Printf("  Fire locked: %v\n", history.IsSchoolLocked(spelldef.SchoolMaskFire))
	fmt.Printf("  Frost locked: %v\n", history.IsSchoolLocked(spelldef.SchoolMaskFrost))
	fmt.Printf("  Fire spell ready: %v\n", history.IsReady(1001, spelldef.SchoolMaskFire))

	// GCD
	fmt.Println("--- 3d: GCD ---")
	history.StartGCD(1, 1500)
	fmt.Printf("  On GCD (cat=1): %v\n", history.IsOnGCD(1))
	fmt.Printf("  On GCD (cat=2): %v\n", history.IsOnGCD(2))

	// OnHold
	fmt.Println("--- 3e: OnHold ---")
	history.OnHold(4001, 3000)
	fmt.Printf("  After interrupt, hold CD remaining: %v\n", history.GetCooldownRemaining(4001))

	// Cooldown modifier
	fmt.Println("--- 3f: Cooldown modifier ---")
	modifier := cooldown.HasteCooldownModifier{HastePercent: 50}
	modified := modifier.ModifyCooldown(6000)
	fmt.Printf("  6000ms with 50%% haste → %dms\n", modified)

	fmt.Println()

	// ===== Phase 4: Aura System =====
	fmt.Println("===== Phase 4: Aura System =====")

	auraMgr := aura.NewAuraManager(target)

	// Apply buff aura
	fmt.Println("--- 4a: Apply and stack aura ---")
	buffAura := &aura.Aura{
		SpellID:    5001,
		CasterGUID: mage.GUID,
		Caster:     mage,
		AuraType:   aura.AuraTypeBuff,
		Duration:   10000,
		MaxStack:   3,
		StackAmount: 1,
		Effects: []*aura.AuraEffect{
			{AuraType: aura.AuraTypeBuff, BaseAmount: 50},
		},
	}
	auraMgr.ApplyAura(buffAura)
	fmt.Printf("  Has aura 5001: %v\n", auraMgr.HasAura(5001))

	// Refresh same aura from same caster
	auraMgr.ApplyAura(buffAura)
	fmt.Printf("  After refresh: stacks=%d\n", buffAura.StackAmount)

	// Different caster replaces
	caster2 := unit.NewUnit(10, "Other Mage", 5000, 20000)
	replaceAura := &aura.Aura{
		SpellID:    5001,
		CasterGUID: caster2.GUID,
		Caster:     caster2,
		AuraType:   aura.AuraTypeBuff,
		Duration:   10000,
		StackAmount: 1,
		Effects: []*aura.AuraEffect{
			{AuraType: aura.AuraTypeBuff, BaseAmount: 60},
		},
	}
	auraMgr.ApplyAura(replaceAura)
	fmt.Printf("  After replace by other caster\n")

	// Debuff (silence)
	fmt.Println("--- 4b: Debuff (silence) ---")
	silenceAura := &aura.Aura{
		SpellID:    5002,
		CasterGUID: mage.GUID,
		Caster:     mage,
		AuraType:   aura.AuraTypeDebuff,
		Duration:   5000,
		StackAmount: 1,
		Effects: []*aura.AuraEffect{
			{AuraType: aura.AuraTypeDebuff, MiscValue: int32(spelldef.UnitStateSilenced)},
		},
	}
	auraMgr.ApplyAura(silenceAura)
	fmt.Printf("  Target silenced: %v\n", target.HasUnitState(spelldef.UnitStateSilenced))

	// Remove debuff
	auraMgr.RemoveAura(silenceAura, aura.RemoveModeDispel)
	fmt.Printf("  After dispel, silenced: %v\n", target.HasUnitState(spelldef.UnitStateSilenced))

	// Proc check
	fmt.Println("--- 4c: Proc ---")
	procAura := &aura.Aura{
		SpellID:        5003,
		CasterGUID:     mage.GUID,
		Caster:         mage,
		AuraType:       aura.AuraTypeProc,
		ProcChance:     50.0,
		ProcCharges:    3,
		RemainingProcs: 3,
		Effects: []*aura.AuraEffect{
			{AuraType: aura.AuraTypeProc, BaseAmount: 100},
		},
	}
	auraMgr.ApplyAura(procAura)
	for i := 0; i < 5; i++ {
		results := auraMgr.CheckProc(aura.ProcEventOnHit)
		if len(results) > 0 {
			fmt.Printf("  Proc attempt %d: triggered!\n", i+1)
		} else {
			fmt.Printf("  Proc attempt %d: no trigger\n", i+1)
		}
	}

	fmt.Println()

	// ===== Phase 5: Script System =====
	fmt.Println("===== Phase 5: Script System =====")

	// Register a spell script that prevents cast
	fmt.Println("--- 5a: Script intercepts cast ---")
	registry := script.NewRegistry()
	registry.RegisterSpellScript(6001, func(ss *script.SpellScript) {
		ss.OnCheckCast(func(arg interface{}) {
			ss.PreventDefault(script.HookOnCheckCast)
			log.Printf("  [Script] OnCheckCast: preventing cast!")
		})
	})

	blockSpell := &spelldef.SpellInfo{
		ID:         6001,
		Name:       "BlockedSpell",
		CastTime:   0,
		PowerCost:  0,
		Effects:    []spelldef.SpellEffectInfo{},
	}
	ctx = spell.New(6001, blockSpell, mage, []*unit.Unit{target})
	ctx.ScriptRegistry = registry
	result = ctx.Prepare()
	fmt.Printf("  Script-blocked spell → result=%d, err=%d\n", result, ctx.LastCastErr)

	// Register a spell script with OnHit hook
	fmt.Println("--- 5b: Script OnHit hook ---")
	registry.RegisterSpellScript(6002, func(ss *script.SpellScript) {
		ss.OnHit(func(arg interface{}) {
			if hitTarget, ok := arg.(*unit.Unit); ok {
				log.Printf("  [Script] OnHit: spell hit %s!", hitTarget.Name)
			}
		})
	})

	hitSpell := &spelldef.SpellInfo{
		ID:         6002,
		Name:       "ScriptedFireball",
		CastTime:   0,
		PowerCost:  0,
		RangeMax:   40,
		Effects: []spelldef.SpellEffectInfo{
			{EffectIndex: 0, EffectType: spelldef.SpellEffectSchoolDamage, BasePoints: 200},
		},
	}
	target.Health = 10000
	ctx = spell.New(6002, hitSpell, mage, []*unit.Unit{target})
	ctx.EffectStore = effect.NewStore()
	ctx.ScriptRegistry = registry
	fmt.Printf("  Before: %s\n", target)
	ctx.Prepare()
	fmt.Printf("  After:  %s\n", target)

	// PreventHitEffect
	fmt.Println("--- 5c: Script PreventHitEffect ---")
	registry.RegisterSpellScript(6003, func(ss *script.SpellScript) {
		ss.OnEffectHit(func(arg interface{}) {
			log.Printf("  [Script] OnEffectHit: preventing hit effect!")
			ss.PreventDefault(script.HookOnEffectHit)
		})
	})

	preventSpell := &spelldef.SpellInfo{
		ID:         6003,
		Name:       "PreventableSpell",
		CastTime:   0,
		PowerCost:  0,
		RangeMax:   40,
		Effects: []spelldef.SpellEffectInfo{
			{EffectIndex: 0, EffectType: spelldef.SpellEffectSchoolDamage, BasePoints: 999},
		},
	}
	target.Health = 10000
	ctx = spell.New(6003, preventSpell, mage, []*unit.Unit{target})
	ctx.EffectStore = effect.NewStore()
	ctx.ScriptRegistry = registry
	fmt.Printf("  Before: %s\n", target)
	ctx.Prepare()
	fmt.Printf("  After:  %s (should be unchanged)\n", target)

	// Phase guard
	fmt.Println("--- 5d: Phase guard ---")
	guard := script.NewPhaseGuard(script.PhasePrepare)
	fmt.Printf("  Prepare phase — can access targets: %v\n", guard.CanAccessTargets())
	guard.SetPhase(script.PhaseHit)
	fmt.Printf("  Hit phase — can access targets: %v\n", guard.CanAccessTargets())
	fmt.Printf("  Hit phase — can modify hit: %v\n", guard.CanModifyHit())

	fmt.Println()
	fmt.Println("=== demo complete ===")
}

// mockUnitProvider implements targeting.UnitProvider for demo purposes.
type mockUnitProvider struct {
	units []*unit.Unit
}

func (m *mockUnitProvider) GetAllUnits() []*unit.Unit {
	return m.units
}
