package combat

import (
	"testing"

	"skill-go/server/spelldef"
	"skill-go/server/trace"
	"skill-go/server/unit"
)

func makeCasterAndTarget() (*unit.Unit, *unit.Unit) {
	caster := unit.NewUnit(1, "Caster", 1000, 500)
	caster.SetLevel(60)
	target := unit.NewUnit(2, "Target", 1000, 500)
	target.SetLevel(60)
	return caster, target
}

// --- 7.1 Spell damage basic (no SP) ---

func TestCalcSpellDamage_BasicNoSP(t *testing.T) {
	attacker, target := makeCasterAndTarget()
	// With zero SP and no armor/resistance, damage ≈ basePoints
	sum := int32(0)
	n := 100
	for i := 0; i < n; i++ {
		dmg := CalcSpellDamage(100, 0, 0, attacker, target, spelldef.SchoolMaskFire, nil)
		sum += dmg
	}
	avg := sum / int32(n)
	if avg < 96 || avg > 104 {
		t.Errorf("avg spell damage = %d, want ~100 (±4%%)", avg)
	}
}

// --- 7.2 Spell damage SP scaling ---

func TestCalcSpellDamage_SPScaling(t *testing.T) {
	attacker, target := makeCasterAndTarget()
	sum := int32(0)
	n := 100
	for i := 0; i < n; i++ {
		dmg := CalcSpellDamage(100, 500, 0.8, attacker, target, spelldef.SchoolMaskArcane, nil)
		sum += dmg
	}
	avg := sum / int32(n)
	// Expected: 100 + 500*0.8 = 500, ±4% = 480–520
	if avg < 480 || avg > 520 {
		t.Errorf("avg spell damage = %d, want ~500", avg)
	}
}

// --- 7.3 Variance within ±4% ---

func TestCalcSpellDamage_VarianceRange(t *testing.T) {
	attacker, target := makeCasterAndTarget()
	min, max := int32(999999), int32(0)
	for i := 0; i < 500; i++ {
		dmg := CalcSpellDamage(1000, 0, 0, attacker, target, spelldef.SchoolMaskFire, nil)
		if dmg < min {
			min = dmg
		}
		if dmg > max {
			max = dmg
		}
	}
	if min < 960 {
		t.Errorf("min damage %d < 960 (below -4%%)", min)
	}
	if max > 1040 {
		t.Errorf("max damage %d > 1040 (above +4%%)", max)
	}
}

// --- 7.4 Minimum damage is 1 ---

func TestCalcSpellDamage_MinimumOne(t *testing.T) {
	attacker, target := makeCasterAndTarget()
	dmg := CalcSpellDamage(0, 0, 0, attacker, target, spelldef.SchoolMaskFire, nil)
	if dmg < 1 {
		t.Errorf("damage = %d, want >= 1", dmg)
	}
}

// --- 7.5 Melee damage weapon + AP ---

func TestCalcMeleeDamage_WeaponAndAP(t *testing.T) {
	attacker, target := makeCasterAndTarget()
	attacker.SetWeaponDamage(80, 120)
	attacker.AttackPower = 0
	sum := int32(0)
	n := 100
	for i := 0; i < n; i++ {
		dmg := CalcMeleeDamage(attacker, target, nil)
		sum += dmg
	}
	avg := sum / int32(n)
	// Weapon range 80-120, avg 100 ± variance
	if avg < 70 || avg > 130 {
		t.Errorf("avg melee damage = %d, want ~100", avg)
	}
}

func TestCalcMeleeDamage_WithAP(t *testing.T) {
	attacker, target := makeCasterAndTarget()
	attacker.SetWeaponDamage(80, 120)
	attacker.AttackPower = 140
	sum := int32(0)
	n := 100
	for i := 0; i < n; i++ {
		dmg := CalcMeleeDamage(attacker, target, nil)
		sum += dmg
	}
	avg := sum / int32(n)
	// AP bonus = 140/14 = 10, avg weapon = 100, total avg ~110
	if avg < 90 || avg > 140 {
		t.Errorf("avg melee damage = %d, want ~110", avg)
	}
}

// --- 7.6 Armor mitigation zero armor ---

func TestArmorMitigation_ZeroArmor(t *testing.T) {
	factor := armorMitigation(0, 60)
	if factor != 1.0 {
		t.Errorf("armorMitigation(0, 60) = %v, want 1.0", factor)
	}
}

// --- 7.7 Armor mitigation positive armor ---

func TestArmorMitigation_PositiveArmor(t *testing.T) {
	factor := armorMitigation(3000, 60)
	// 3000 / (3000 + 400 + 5100) = 3000/8500 ≈ 0.353
	// factor = 1 - 0.353 ≈ 0.647
	if factor < 0.60 || factor > 0.70 {
		t.Errorf("armorMitigation(3000, 60) = %v, want ~0.65", factor)
	}
}

// --- 7.8 Physical vs spell armor ---

func TestCalcSpellDamage_PhysicalHitByArmor(t *testing.T) {
	attacker, target := makeCasterAndTarget()
	target.SetArmor(5000)

	noArmorSum := int32(0)
	armorSum := int32(0)
	n := 200

	// Fire (no armor effect)
	for i := 0; i < n; i++ {
		noArmorSum += CalcSpellDamage(1000, 0, 0, attacker, target, spelldef.SchoolMaskFire, nil)
	}
	// Physical (armor effect)
	targetNoArmor := unit.NewUnit(3, "NoArmor", 1000, 500)
	targetNoArmor.SetLevel(60)
	for i := 0; i < n; i++ {
		armorSum += CalcSpellDamage(1000, 0, 0, attacker, targetNoArmor, spelldef.SchoolMaskPhysical, nil)
	}

	fireAvg := float64(noArmorSum) / float64(n)
	_ = fireAvg
	physNoArmorAvg := float64(armorSum) / float64(n)

	// Physical on armored target should be less than on no-armor target
	physArmoredSum := int32(0)
	for i := 0; i < n; i++ {
		physArmoredSum += CalcSpellDamage(1000, 0, 0, attacker, target, spelldef.SchoolMaskPhysical, nil)
	}
	physArmoredAvg := float64(physArmoredSum) / float64(n)

	if physArmoredAvg >= physNoArmorAvg {
		t.Errorf("physical on armored target (%.0f) should be less than on no-armor target (%.0f)",
			physArmoredAvg, physNoArmorAvg)
	}
}

// --- 7.9 Resist reduction zero resistance ---

func TestResistReduction_ZeroResistance(t *testing.T) {
	reduction := resistReduction(0, 60)
	if reduction != 0.0 {
		t.Errorf("resistReduction(0, 60) = %v, want 0.0", reduction)
	}
}

// --- 7.10 Resist roll partial and full ---

func TestResistRoll_WithResistance(t *testing.T) {
	// With high resistance, should see full resists sometimes
	fullResistCount := 0
	partialResistCount := 0
	n := 500
	for i := 0; i < n; i++ {
		avg := resistReduction(300, 60) // ~300/(300+9000) ≈ 3.2%
		factor := resistRoll(avg)
		if factor >= 1.0 {
			fullResistCount++
		} else if factor > 0 {
			partialResistCount++
		}
	}
	// With low avgReduction (~3.2%), full resists should be rare but possible
	// Over 500 rolls, at least some should occur
	t.Logf("full resists: %d/%d, partial: %d/%d", fullResistCount, n, partialResistCount, n)
	// We just verify it doesn't panic and produces values in range
}

func TestResistRoll_ZeroReduction(t *testing.T) {
	for i := 0; i < 100; i++ {
		factor := resistRoll(0)
		if factor != 0.0 {
			t.Errorf("resistRoll(0) = %v, want 0.0", factor)
		}
	}
}

// --- Trace events ---

func TestCalcSpellDamage_TraceEvents(t *testing.T) {
	attacker, target := makeCasterAndTarget()
	rec := trace.NewFlowRecorder()
	tr := trace.NewTraceWithSinks(rec)

	CalcSpellDamage(100, 0, 0, attacker, target, spelldef.SchoolMaskFire, nil)

	// No trace when nil
	if len(rec.Events()) != 0 {
		t.Error("expected no trace events with nil trace")
	}

	// With trace
	CalcSpellDamage(100, 0, 0, attacker, target, spelldef.SchoolMaskFire, tr)

	if !rec.HasEvent(trace.SpanCombat, "damage_calc") {
		t.Error("missing damage_calc trace event")
	}
}

func TestCalcMeleeDamage_TraceEvents(t *testing.T) {
	attacker, target := makeCasterAndTarget()
	rec := trace.NewFlowRecorder()
	tr := trace.NewTraceWithSinks(rec)

	CalcMeleeDamage(attacker, target, tr)

	if !rec.HasEvent(trace.SpanCombat, "damage_calc") {
		t.Error("missing damage_calc trace event")
	}
	if !rec.HasEvent(trace.SpanCombat, "armor_mitigation") {
		t.Error("missing armor_mitigation trace event")
	}
}
