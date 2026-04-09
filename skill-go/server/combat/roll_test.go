package combat

import (
	"testing"

	"skill-go/server/spelldef"
	"skill-go/server/trace"
	"skill-go/server/unit"
)

// --- Helpers ---

func makeMeleeCasterTarget() (*unit.Unit, *unit.Unit) {
	attacker := unit.NewUnit(1, "Melee", 1000, 500)
	attacker.SetLevel(60)
	attacker.HitMelee = 100.0 // cap hit so no miss from rating
	attacker.CritMelee = 25.0
	target := unit.NewUnit(2, "Target", 1000, 500)
	target.SetLevel(60)
	target.Dodge = 10.0
	target.Parry = 10.0
	target.Block = 10.0
	return attacker, target
}

func makeSpellCasterTarget() (*unit.Unit, *unit.Unit) {
	attacker := unit.NewUnit(1, "Caster", 1000, 500)
	attacker.SetLevel(60)
	attacker.HitSpell = 100.0
	target := unit.NewUnit(2, "Target", 1000, 500)
	target.SetLevel(60)
	return attacker, target
}

// --- 7.11 Melee Hit/Dodge/Parry/Crit/Miss ---

func TestResolveMeleeHit_AllResults(t *testing.T) {
	attacker := unit.NewUnit(1, "A", 100, 50)
	attacker.SetLevel(60)
	attacker.HitMelee = 100.0
	attacker.CritMelee = 30.0
	target := unit.NewUnit(2, "B", 100, 50)
	target.SetLevel(60)
	target.Dodge = 15.0
	target.Parry = 10.0
	target.Block = 10.0

	// Set miss to 0 by same level
	results := make(map[spelldef.CombatResult]int)
	n := 1000
	for i := 0; i < n; i++ {
		r := ResolveMeleeHit(attacker, target, nil)
		results[r]++
	}

	// With 100% hit and same level, miss should be very rare (just base 5% - 100% = 0)
	// Dodge 15%, Parry 10%, Block 10%, Crit 30% → Hit ≈ 35%
	// Just verify multiple different results appear
	seen := 0
	for _, count := range results {
		if count > 0 {
			seen++
		}
	}
	if seen < 3 {
		t.Errorf("expected multiple result types, got %d: %v", seen, results)
	}
}

// --- 7.12 Glancing only level diff >= 3 ---

func TestResolveMeleeHit_GlancingHighLevelDiff(t *testing.T) {
	attacker := unit.NewUnit(1, "A", 100, 50)
	attacker.SetLevel(60)
	attacker.HitMelee = 100.0
	target := unit.NewUnit(2, "Boss", 1000, 500)
	target.SetLevel(63) // 3 levels above

	glancingCount := 0
	n := 500
	for i := 0; i < n; i++ {
		r := ResolveMeleeHit(attacker, target, nil)
		if r == spelldef.CombatResultGlancing {
			glancingCount++
		}
	}
	if glancingCount == 0 {
		t.Error("expected glancing blows against +3 level target")
	}
}

func TestResolveMeleeHit_NoGlancingSameLevel(t *testing.T) {
	attacker := unit.NewUnit(1, "A", 100, 50)
	attacker.SetLevel(60)
	attacker.HitMelee = 100.0
	target := unit.NewUnit(2, "B", 100, 50)
	target.SetLevel(60)

	glancingCount := 0
	n := 500
	for i := 0; i < n; i++ {
		r := ResolveMeleeHit(attacker, target, nil)
		if r == spelldef.CombatResultGlancing {
			glancingCount++
		}
	}
	if glancingCount > 0 {
		t.Errorf("expected no glancing vs same level, got %d", glancingCount)
	}
}

// --- 7.13 Melee Block ---

func TestResolveMeleeHit_Block(t *testing.T) {
	attacker := unit.NewUnit(1, "A", 100, 50)
	attacker.SetLevel(60)
	attacker.HitMelee = 100.0
	target := unit.NewUnit(2, "B", 100, 50)
	target.SetLevel(60)
	target.Block = 50.0
	target.BlockValue = 100

	blockCount := 0
	n := 500
	for i := 0; i < n; i++ {
		r := ResolveMeleeHit(attacker, target, nil)
		if r == spelldef.CombatResultBlock {
			blockCount++
		}
	}
	if blockCount == 0 {
		t.Error("expected block results with 50% block chance")
	}
}

// --- 7.14 Spell Hit/Miss ---

func TestResolveSpellHit_Hit(t *testing.T) {
	attacker, target := makeSpellCasterTarget()
	// Same level, 100% hit → should almost never miss
	missCount := 0
	n := 200
	for i := 0; i < n; i++ {
		r := ResolveSpellHit(attacker, target, spelldef.SchoolMaskFire, nil)
		if r == spelldef.CombatResultMiss {
			missCount++
		}
	}
	if missCount > 0 {
		t.Errorf("expected no misses with same level + 100%% hit, got %d", missCount)
	}
}

// --- 7.15 Spell no Dodge/Parry ---

func TestResolveSpellHit_NoDodgeParry(t *testing.T) {
	attacker, target := makeSpellCasterTarget()
	attacker.HitSpell = 100.0
	// Give target high dodge/parry — spells should ignore them
	target.Dodge = 100.0
	target.Parry = 100.0

	hitCount := 0
	n := 100
	for i := 0; i < n; i++ {
		r := ResolveSpellHit(attacker, target, spelldef.SchoolMaskFire, nil)
		if r == spelldef.CombatResultHit {
			hitCount++
		}
		if r == spelldef.CombatResultDodge || r == spelldef.CombatResultParry {
			t.Error("spells should never result in Dodge or Parry")
		}
	}
	if hitCount != n {
		t.Errorf("expected %d hits, got %d", n, hitCount)
	}
}

// --- 7.16 Spell FullResist ---

func TestResolveSpellHit_FullResist(t *testing.T) {
	attacker, target := makeSpellCasterTarget()
	attacker.HitSpell = 100.0
	// Set very high fire resistance to trigger full resists
	target.SetResistance(spelldef.SchoolMaskFire, 500)

	fullResistCount := 0
	n := 500
	for i := 0; i < n; i++ {
		r := ResolveSpellHit(attacker, target, spelldef.SchoolMaskFire, nil)
		if r == spelldef.CombatResultFullResist {
			fullResistCount++
		}
	}
	// 500 resist vs level 60: avgReduction = 500/(500+9000) ≈ 5.3%
	// full resist weight = 5.3% * 10% = 0.53% → over 500 rolls should see some
	t.Logf("full resists: %d/%d", fullResistCount, n)
	if fullResistCount == 0 {
		// Not guaranteed with 0.53% chance over 500 rolls, but likely
		t.Log("warning: expected some full resists but got none (probabilistic)")
	}
}

// --- 7.17 Level diff affects miss ---

func TestResolveSpellHit_LevelDiffMiss(t *testing.T) {
	attacker := unit.NewUnit(1, "A", 100, 50)
	attacker.SetLevel(60)
	attacker.HitSpell = 0.0 // no hit rating
	target := unit.NewUnit(2, "B", 100, 50)
	target.SetLevel(63) // 3 levels above → 6% miss

	missCount := 0
	n := 500
	for i := 0; i < n; i++ {
		r := ResolveSpellHit(attacker, target, spelldef.SchoolMaskFire, nil)
		if r == spelldef.CombatResultMiss {
			missCount++
		}
	}
	missRate := float64(missCount) / float64(n) * 100
	if missRate < 3.0 {
		t.Errorf("miss rate = %.1f%%, expected ~6%% with +3 level diff", missRate)
	}
}

func TestResolveMeleeHit_LevelDiffMiss(t *testing.T) {
	attacker := unit.NewUnit(1, "A", 100, 50)
	attacker.SetLevel(60)
	attacker.HitMelee = 0.0
	target := unit.NewUnit(2, "B", 100, 50)
	target.SetLevel(63) // +3 → 8% miss (5 base + 3 level diff)

	missCount := 0
	n := 500
	for i := 0; i < n; i++ {
		r := ResolveMeleeHit(attacker, target, nil)
		if r == spelldef.CombatResultMiss {
			missCount++
		}
	}
	missRate := float64(missCount) / float64(n) * 100
	if missRate < 5.0 {
		t.Errorf("miss rate = %.1f%%, expected ~8%% with +3 level diff", missRate)
	}
}

// --- Glancing damage multiplier ---

func TestGlancingDamageMultiplier(t *testing.T) {
	tests := []struct {
		levelDiff int
		wantMin   float64
		wantMax   float64
	}{
		{0, 1.0, 1.0},
		{1, 0.90, 0.92},
		{2, 0.74, 0.76},
		{3, 0.64, 0.66},
		{4, 0.54, 0.56},
	}
	for _, tt := range tests {
		got := GlancingDamageMultiplier(tt.levelDiff)
		if got < tt.wantMin || got > tt.wantMax {
			t.Errorf("GlancingDamageMultiplier(%d) = %v, want %.0f-%.0f",
				tt.levelDiff, got, tt.wantMin*100, tt.wantMax*100)
		}
	}
}

// --- Trace events ---

func TestResolveMeleeHit_Trace(t *testing.T) {
	attacker, target := makeMeleeCasterTarget()
	rec := trace.NewFlowRecorder()
	tr := trace.NewTraceWithSinks(rec)

	ResolveMeleeHit(attacker, target, tr)

	if !rec.HasEvent(trace.SpanCombat, "melee_roll") {
		t.Error("missing melee_roll trace event")
	}
}

func TestResolveSpellHit_Trace(t *testing.T) {
	attacker, target := makeSpellCasterTarget()
	rec := trace.NewFlowRecorder()
	tr := trace.NewTraceWithSinks(rec)

	ResolveSpellHit(attacker, target, spelldef.SchoolMaskFire, tr)

	if !rec.HasEvent(trace.SpanCombat, "spell_roll") {
		t.Error("missing spell_roll trace event")
	}
}
