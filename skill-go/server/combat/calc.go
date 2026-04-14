package combat

import (
	"math/rand"
	"time"

	"skill-go/server/spelldef"
	"skill-go/server/trace"
	"skill-go/server/unit"
)

var rng = rand.New(rand.NewSource(time.Now().UnixNano()))

// CalcSpellDamage computes final spell damage after scaling, variance, armor, and
// resistance. It returns the damage amount only — hit resolution (miss/crit) is
// handled separately by ResolveSpellHit in roll.go.
func CalcSpellDamage(
	basePoints int32,
	spellPower int32,
	coef float64,
	attacker *unit.Unit,
	target *unit.Unit,
	school spelldef.SchoolMask,
	t *trace.Trace,
) int32 {
	// Base damage: flat + spell power scaling
	baseDamage := basePoints + int32(float64(spellPower)*coef)

	// Apply ±4% random variance
	varyFactor := 1.0 + (rng.Float64()*0.08 - 0.04)
	damage := float64(baseDamage) * varyFactor

	// Armor mitigation for physical school only
	if school == spelldef.SchoolMaskPhysical {
		factor := armorMitigation(target.Armor, attacker.Level)
		damage *= factor
		if t != nil {
			t.Event(trace.SpanCombat, "armor_mitigation",
				0, "", map[string]interface{}{
					"armor":     target.Armor,
					"reduction": 1.0 - factor,
					"factor":    factor,
				})
		}
	}

	// Spell resistance reduction for non-physical schools
	if school != spelldef.SchoolMaskPhysical {
		resistance := target.GetResistance(school)
		if resistance > 0 {
			avgReduction := resistReduction(resistance, attacker.Level)
			factor := resistRoll(avgReduction)
			damage *= (1.0 - factor)
			if t != nil {
				t.Event(trace.SpanCombat, "resist_reduction",
					0, "", map[string]interface{}{
						"school":       school,
						"resistance":   resistance,
						"avgReduction": avgReduction,
						"rollFactor":   factor,
					})
			}
		}
	}

	// Minimum 1 damage
	result := int32(damage)
	if result < 1 {
		result = 1
	}

	if t != nil {
		t.Event(trace.SpanCombat, "damage_calc",
			0, "", map[string]interface{}{
				"baseDamage":  baseDamage,
				"finalDamage": result,
				"school":      school,
				"variance":    varyFactor,
			})
	}

	return result
}

// CalcMeleeDamage computes base melee damage from weapon + AP, with ±4%
// variance and armor mitigation applied. Returns the damage amount only —
// hit resolution and crit/block multiplier are handled in roll.go / effect handlers.
func CalcMeleeDamage(
	attacker *unit.Unit,
	target *unit.Unit,
	t *trace.Trace,
) int32 {
	// Weapon damage range
	weaponRange := attacker.MaxWeaponDamage - attacker.MinWeaponDamage
	if weaponRange < 0 {
		weaponRange = 0
	}
	weaponDamage := attacker.MinWeaponDamage
	if weaponRange > 0 {
		weaponDamage += int32(rng.Intn(int(weaponRange) + 1))
	}

	// AP bonus (normalized to weapon speed 1.0)
	apBonus := int32(float64(attacker.AttackPower) / 14.0)
	baseDamage := weaponDamage + apBonus

	// ±4% variance
	varyFactor := 1.0 + (rng.Float64()*0.08 - 0.04)
	damage := float64(baseDamage) * varyFactor

	// Armor mitigation
	factor := armorMitigation(target.Armor, attacker.Level)
	damage *= factor

	if t != nil {
		t.Event(trace.SpanCombat, "armor_mitigation",
			0, "", map[string]interface{}{
				"armor":     target.Armor,
				"reduction": 1.0 - factor,
				"factor":    factor,
			})
	}

	result := int32(damage)
	if result < 1 {
		result = 1
	}

	if t != nil {
		t.Event(trace.SpanCombat, "damage_calc",
			0, "", map[string]interface{}{
				"baseDamage":  baseDamage,
				"finalDamage": result,
				"school":      "physical",
				"variance":    varyFactor,
				"type":        "melee",
			})
	}

	return result
}

// armorMitigation returns the damage multiplier after armor reduction.
// Formula: damage * (1 - Armor / (Armor + 400 + 85 * attackerLevel)).
func armorMitigation(armor int32, attackerLevel uint8) float64 {
	if armor <= 0 {
		return 1.0
	}
	reduction := float64(armor) / (float64(armor) + 400.0 + 85.0*float64(attackerLevel))
	return 1.0 - reduction
}

// resistReduction returns the average fractional damage reduction for a given
// resistance value against a caster of the given level.
// Formula: resistance / (resistance + 150 * casterLevel).
func resistReduction(resistance float64, casterLevel uint8) float64 {
	if resistance <= 0 {
		return 0.0
	}
	return resistance / (resistance + 150.0*float64(casterLevel))
}

// resistRoll performs a single roll to determine actual resistance factor.
// Returns 0.0 (no resist), 0.25, 0.5, 0.75, or 1.0 (full resist).
// The average returned value approximates avgReduction.
func resistRoll(avgReduction float64) float64 {
	roll := rng.Float64() * 100.0
	switch {
	case roll < avgReduction*10.0:
		return 1.0 // full resist (10% weight)
	case roll < avgReduction*10.0+avgReduction*30.0:
		return 0.75 // 75% resist (30% weight)
	case roll < avgReduction*10.0+avgReduction*60.0:
		return 0.50 // 50% resist (30% weight)
	case roll < avgReduction*10.0+avgReduction*90.0:
		return 0.25 // 25% resist (30% weight)
	default:
		return 0.0 // no resist
	}
}
