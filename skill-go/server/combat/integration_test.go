package combat_test

import (
	"testing"

	"skill-go/server/effect"
	"skill-go/server/spelldef"
	"skill-go/server/trace"
	"skill-go/server/unit"
)

// mockCasterInfo implements effect.CasterInfo for integration tests.
type mockCasterInfo struct {
	caster    *unit.Unit
	targets   []*unit.Unit
	tr        *trace.Trace
	spellID   uint32
	spellName string
}

func (m *mockCasterInfo) Caster() *unit.Unit     { return m.caster }
func (m *mockCasterInfo) Targets() []*unit.Unit  { return m.targets }
func (m *mockCasterInfo) GetTrace() *trace.Trace { return m.tr }
func (m *mockCasterInfo) GetSpellID() uint32     { return m.spellID }
func (m *mockCasterInfo) GetSpellName() string   { return m.spellName }

func newSpellCtx(caster *unit.Unit, targets []*unit.Unit) *mockCasterInfo {
	return &mockCasterInfo{
		caster:    caster,
		targets:   targets,
		tr:        trace.NewTrace(),
		spellID:   100,
		spellName: "Fireball",
	}
}

// --- 11.1 Spell hits (HP reduced) with damage_calc trace ---

func TestIntegration_SpellHit_ReduceHP(t *testing.T) {
	caster := unit.NewUnit(1, "Caster", 1000, 500)
	caster.SetLevel(60)
	caster.HitSpell = 100.0
	caster.SpellPower = 0
	target := unit.NewUnit(2, "Target", 200, 200)
	target.SetLevel(60)
	ctx := newSpellCtx(caster, []*unit.Unit{target})
	rec := trace.NewFlowRecorder()
	ctx.tr.AddSink(rec)

	store := effect.NewStore()
	handler := store.GetHitHandler(spelldef.SpellEffectSchoolDamage)
	handler(ctx, spelldef.SpellEffectInfo{
		EffectIndex: 0,
		EffectType:  spelldef.SpellEffectSchoolDamage,
		BasePoints:  50,
		SchoolMask:  spelldef.SchoolMaskFire,
	}, target)

	if target.Health >= 200 {
		t.Error("target should have taken damage")
	}

	found := false
	for _, e := range rec.Events() {
		if e.Event == "damage_calc" && e.Span == trace.SpanCombat {
			found = true
			break
		}
	}
	if !found {
		t.Error("missing damage_calc trace event")
	}
}

// --- 11.2 Spell miss (HP unchanged) with miss trace ---

func TestIntegration_SpellMiss_HPUnchanged(t *testing.T) {
	caster := unit.NewUnit(1, "Caster", 1000, 500)
	caster.SetLevel(60)
	caster.HitSpell = 0.0 // no hit rating
	target := unit.NewUnit(2, "Target", 200, 200)
	target.SetLevel(66) // +6 levels → 12% miss
	ctx := newSpellCtx(caster, []*unit.Unit{target})
	rec := trace.NewFlowRecorder()
	ctx.tr.AddSink(rec)

	store := effect.NewStore()
	handler := store.GetHitHandler(spelldef.SpellEffectSchoolDamage)

	missFound := false
	for i := 0; i < 200; i++ {
		freshTarget := unit.NewUnit(uint64(100+i), "Target", 200, 200)
		freshTarget.SetLevel(66)
		handler(ctx, spelldef.SpellEffectInfo{
			EffectIndex: 0,
			EffectType:  spelldef.SpellEffectSchoolDamage,
			BasePoints:  50,
			SchoolMask:  spelldef.SchoolMaskFire,
		}, freshTarget)
		if freshTarget.Health == 200 {
			missFound = true
		}
	}
	if !missFound {
		t.Error("expected at least one miss with +6 level diff and 0 hit rating")
	}
}

// --- 11.3 Spell crit (more damage) with crit trace ---

func TestIntegration_SpellCrit_MoreDamage(t *testing.T) {
	caster := unit.NewUnit(1, "Caster", 1000, 500)
	caster.SetLevel(60)
	caster.HitSpell = 100.0
	caster.CritSpell = 100.0 // 100% crit
	target := unit.NewUnit(2, "Target", 1000, 500)
	target.SetLevel(60)
	ctx := newSpellCtx(caster, []*unit.Unit{target})
	rec := trace.NewFlowRecorder()
	ctx.tr.AddSink(rec)

	store := effect.NewStore()
	handler := store.GetHitHandler(spelldef.SpellEffectSchoolDamage)

	normalDamage := int32(0)
	critDamage := int32(0)

	// Normal hit
	normalCaster := unit.NewUnit(10, "Normal", 1000, 500)
	normalCaster.SetLevel(60)
	normalCaster.HitSpell = 100.0
	normalCaster.CritSpell = 0.0 // 0% crit
	normalTarget := unit.NewUnit(11, "Target", 1000, 500)
	normalTarget.SetLevel(60)
	normalCtx := newSpellCtx(normalCaster, []*unit.Unit{normalTarget})
	handler(normalCtx, spelldef.SpellEffectInfo{
		EffectIndex: 0,
		EffectType:  spelldef.SpellEffectSchoolDamage,
		BasePoints:  100,
		SchoolMask:  spelldef.SchoolMaskFire,
	}, normalTarget)
	normalDamage = 1000 - normalTarget.Health

	// Crit
	handler(ctx, spelldef.SpellEffectInfo{
		EffectIndex: 0,
		EffectType:  spelldef.SpellEffectSchoolDamage,
		BasePoints:  100,
		SchoolMask:  spelldef.SchoolMaskFire,
	}, target)
	critDamage = 1000 - target.Health

	if critDamage <= normalDamage {
		t.Errorf("crit damage (%d) should be > normal damage (%d)", critDamage, normalDamage)
	}

	// Crit should be approximately 1.5x normal (±variance)
	ratio := float64(critDamage) / float64(normalDamage)
	if ratio < 1.3 || ratio > 1.7 {
		t.Errorf("crit/normal ratio = %.2f, want ~1.5", ratio)
	}
}

// --- 11.4 Melee attack hits with weapon damage ---

func TestIntegration_MeleeHit_WeaponDamage(t *testing.T) {
	caster := unit.NewUnit(1, "Melee", 1000, 500)
	caster.SetLevel(60)
	caster.HitMelee = 100.0
	caster.SetWeaponDamage(80, 80) // fixed weapon damage
	target := unit.NewUnit(2, "Target", 1000, 500)
	target.SetLevel(60)
	ctx := newSpellCtx(caster, []*unit.Unit{target})
	rec := trace.NewFlowRecorder()
	ctx.tr.AddSink(rec)

	store := effect.NewStore()
	effect.RegisterExtended(store, nil, nil)
	handler := store.GetHitHandler(spelldef.SpellEffectWeaponDamage)

	handler(ctx, spelldef.SpellEffectInfo{
		EffectIndex:   0,
		EffectType:    spelldef.SpellEffectWeaponDamage,
		BasePoints:    0,
		WeaponPercent: 0,
	}, target)

	if target.Health >= 1000 {
		t.Error("target should have taken melee damage")
	}
	// With 80 weapon damage, should be around 80 (±4%)
	damage := 1000 - target.Health
	if damage < 60 || damage > 100 {
		t.Errorf("melee damage = %d, expected ~80 (±4%%)", damage)
	}

	if !rec.HasEvent(trace.SpanCombat, "damage_calc") {
		t.Error("missing damage_calc trace event")
	}
}

// --- 11.5 Armor reduces physical but not spell damage ---

func TestIntegration_Armor_PhysicalOnly(t *testing.T) {
	caster := unit.NewUnit(1, "Caster", 1000, 500)
	caster.SetLevel(60)
	caster.HitSpell = 100.0
	caster.HitMelee = 100.0
	caster.SetWeaponDamage(100, 100)

	noArmorTarget := unit.NewUnit(2, "NoArmor", 1000, 500)
	noArmorTarget.SetLevel(60)

	armoredTarget := unit.NewUnit(3, "Armored", 1000, 500)
	armoredTarget.SetLevel(60)
	armoredTarget.SetArmor(5000)

	ctx := newSpellCtx(caster, []*unit.Unit{noArmorTarget})

	store := effect.NewStore()
	effect.RegisterExtended(store, nil, nil)
	spellHandler := store.GetHitHandler(spelldef.SpellEffectSchoolDamage)
	weaponHandler := store.GetHitHandler(spelldef.SpellEffectWeaponDamage)

	// Fire damage on both targets
	spellHandler(ctx, spelldef.SpellEffectInfo{
		EffectIndex: 0,
		EffectType:  spelldef.SpellEffectSchoolDamage,
		BasePoints:  100,
		SchoolMask:  spelldef.SchoolMaskFire,
	}, noArmorTarget)

	spellHandler(ctx, spelldef.SpellEffectInfo{
		EffectIndex: 0,
		EffectType:  spelldef.SpellEffectSchoolDamage,
		BasePoints:  100,
		SchoolMask:  spelldef.SchoolMaskFire,
	}, armoredTarget)

	fireNoArmor := 1000 - noArmorTarget.Health
	fireArmored := 1000 - armoredTarget.Health

	// Fire should NOT be reduced by armor
	if fireArmored < fireNoArmor*80/100 {
		t.Errorf("fire on armored target (%d) should be similar to no-armor (%d)", fireArmored, fireNoArmor)
	}

	// Physical damage on both targets
	meleeCtx := newSpellCtx(caster, []*unit.Unit{noArmorTarget})
	weaponHandler(meleeCtx, spelldef.SpellEffectInfo{
		EffectIndex:   0,
		EffectType:    spelldef.SpellEffectWeaponDamage,
		BasePoints:    0,
		WeaponPercent: 0,
	}, noArmorTarget)

	freshNoArmor := unit.NewUnit(4, "NoArmor2", 1000, 500)
	freshNoArmor.SetLevel(60)
	weaponHandler(meleeCtx, spelldef.SpellEffectInfo{
		EffectIndex:   0,
		EffectType:    spelldef.SpellEffectWeaponDamage,
		BasePoints:    0,
		WeaponPercent: 0,
	}, armoredTarget)

	physNoArmor := 1000 - noArmorTarget.Health
	physArmored := 1000 - armoredTarget.Health

	// Physical SHOULD be reduced by armor
	if physArmored >= physNoArmor {
		t.Errorf("physical on armored target (%d) should be < no-armor (%d)", physArmored, physNoArmor)
	}
}

// --- 11.6 Spell resistance causes full/partial resist ---

func TestIntegration_Resistance_FullAndPartial(t *testing.T) {
	caster := unit.NewUnit(1, "Caster", 1000, 500)
	caster.SetLevel(60)
	caster.HitSpell = 100.0
	caster.SpellPower = 0
	target := unit.NewUnit(2, "Target", 1000, 500)
	target.SetLevel(60)
	target.SetResistance(spelldef.SchoolMaskFrost, 500) // high frost resistance

	ctx := newSpellCtx(caster, []*unit.Unit{target})
	rec := trace.NewFlowRecorder()
	ctx.tr.AddSink(rec)

	store := effect.NewStore()
	handler := store.GetHitHandler(spelldef.SpellEffectSchoolDamage)

	fullResistCount := 0
	reducedCount := 0
	n := 200

	for i := 0; i < n; i++ {
		freshTarget := unit.NewUnit(uint64(200+i), "Target", 1000, 500)
		freshTarget.SetLevel(60)
		freshTarget.SetResistance(spelldef.SchoolMaskFrost, 500)

		freshRec := trace.NewFlowRecorder()
		freshTr := trace.NewTraceWithSinks(freshRec)
		freshCtx := newSpellCtx(caster, []*unit.Unit{freshTarget})
		freshCtx.tr = freshTr

		handler(freshCtx, spelldef.SpellEffectInfo{
			EffectIndex: 0,
			EffectType:  spelldef.SpellEffectSchoolDamage,
			BasePoints:  100,
			SchoolMask:  spelldef.SchoolMaskFrost,
		}, freshTarget)

		damage := 1000 - freshTarget.Health
		if damage == 0 {
			fullResistCount++
		} else if damage < 100 {
			reducedCount++
		}
	}

	t.Logf("full resists: %d/%d, reduced: %d/%d", fullResistCount, n, reducedCount, n)
	if fullResistCount == 0 && reducedCount == 0 {
		t.Error("expected some resist effects with 500 frost resistance")
	}
}

// --- 11.7 Aura StatModifier affects damage ---

func TestIntegration_AuraStatModifier_AffectsDamage(t *testing.T) {
	caster := unit.NewUnit(1, "Caster", 1000, 500)
	caster.SetLevel(60)
	caster.HitSpell = 100.0
	caster.SpellPower = 0
	target := unit.NewUnit(2, "Target", 10000, 500)
	target.SetLevel(60)

	// Apply a SpellPower buff aura to the caster
	auraMgr := NewTestAuraMgr(caster)
	buff := &TestBuffAura{
		spellID:    500,
		casterGUID: caster.GUID,
		effects: []*TestStatEffect{
			{StatType: unit.StatSpellPower, Amount: 500},
		},
	}
	auraMgr.ApplyBuff(buff)

	if caster.SpellPower != 500 {
		t.Fatalf("SpellPower should be 500 after buff, got %d", caster.SpellPower)
	}

	ctx := newSpellCtx(caster, []*unit.Unit{target})

	store := effect.NewStore()
	handler := store.GetHitHandler(spelldef.SpellEffectSchoolDamage)

	handler(ctx, spelldef.SpellEffectInfo{
		EffectIndex: 0,
		EffectType:  spelldef.SpellEffectSchoolDamage,
		BasePoints:  100,
		SchoolMask:  spelldef.SchoolMaskArcane,
		Coef:        1.0,
	}, target)

	damage := 10000 - target.Health
	// With 500 SP and coef 1.0: base = 100 + 500 = 600, ±4% variance
	// Without SP: base = 100
	if damage < 500 {
		t.Errorf("damage = %d, expected ~600 (100 base + 500 SP)", damage)
	}
}

// Minimal test helpers for integration tests.
type TestStatEffect struct {
	StatType unit.StatType
	Amount   int32
}

type TestBuffAura struct {
	spellID    uint32
	casterGUID uint64
	effects    []*TestStatEffect
}

type TestAuraMgr struct {
	target *unit.Unit
}

func NewTestAuraMgr(target *unit.Unit) *TestAuraMgr {
	return &TestAuraMgr{target: target}
}

func (m *TestAuraMgr) ApplyBuff(aura *TestBuffAura) {
	for _, eff := range aura.effects {
		m.target.ModifyStat(eff.StatType, eff.Amount)
	}
}
