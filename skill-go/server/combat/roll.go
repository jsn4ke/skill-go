package combat

import (
	"math"

	"skill-go/server/spelldef"
	"skill-go/server/trace"
	"skill-go/server/unit"
)

// ResolveMeleeHit performs a single-roll melee hit resolution.
// Table order: Miss → Dodge → Parry → Glancing → Block → Crit → Hit.
func ResolveMeleeHit(attacker, target *unit.Unit, t *trace.Trace) spelldef.CombatResult {
	roll := rng.Float64() * 100.0

	// Level-based miss chance: +1% per level the target is above attacker
	levelDiff := int(target.Level) - int(attacker.Level)
	if levelDiff < 0 {
		levelDiff = 0
	}
	baseMiss := 5.0 + float64(levelDiff)*1.0
	// Player hit rating reduces miss
	missChance := baseMiss - attacker.HitMelee
	if missChance < 0 {
		missChance = 0
	}

	dodge := target.Dodge
	parry := target.Parry
	glancing := glancingChance(levelDiff)
	block := target.Block
	crit := attacker.CritMelee

	result := spelldef.CombatResultHit

	cumulative := missChance
	if roll < cumulative {
		result = spelldef.CombatResultMiss
	} else {
		cumulative += dodge
		if roll < cumulative {
			result = spelldef.CombatResultDodge
		} else {
			cumulative += parry
			if roll < cumulative {
				result = spelldef.CombatResultParry
			} else {
				cumulative += glancing
				if roll < cumulative {
					result = spelldef.CombatResultGlancing
				} else {
					cumulative += block
					if roll < cumulative {
						result = spelldef.CombatResultBlock
					} else {
						cumulative += crit
						if roll < cumulative {
							result = spelldef.CombatResultCrit
						}
					}
				}
			}
		}
	}

	if t != nil {
		t.Event(trace.SpanCombat, "melee_roll",
			0, "", map[string]interface{}{
				"roll":          math.Round(roll*100) / 100,
				"result":        result,
				"attackerLevel": attacker.Level,
				"targetLevel":   target.Level,
				"missChance":    missChance,
				"dodge":         dodge,
				"parry":         parry,
				"glancing":      glancing,
				"block":         block,
				"crit":          crit,
			})
	}

	return result
}

// ResolveSpellHit performs a two-phase spell hit resolution.
// Phase 1: hit/miss/crit roll. Phase 2: resist roll.
func ResolveSpellHit(
	attacker, target *unit.Unit,
	school spelldef.SchoolMask,
	t *trace.Trace,
) spelldef.CombatResult {
	roll := rng.Float64() * 100.0

	// Level-based miss chance: +2% per level difference
	levelDiff := int(target.Level) - int(attacker.Level)
	if levelDiff < 0 {
		levelDiff = 0
	}
	baseMiss := float64(levelDiff) * 2.0
	missChance := baseMiss - attacker.HitSpell
	if missChance < 0 {
		missChance = 0
	}

	crit := attacker.CritSpell

	// Phase 1: roll table: Miss → Crit → Hit
	cumulative := missChance
	if roll < cumulative {
		if t != nil {
			t.Event(trace.SpanCombat, "spell_roll",
				0, "", map[string]interface{}{
					"roll":       math.Round(roll*100) / 100,
					"result":     "miss",
					"missChance": missChance,
					"hitSpell":   attacker.HitSpell,
				})
		}
		return spelldef.CombatResultMiss
	}
	cumulative += crit
	if roll < cumulative {
		if t != nil {
			t.Event(trace.SpanCombat, "spell_roll",
				0, "", map[string]interface{}{
					"roll":      math.Round(roll*100) / 100,
					"result":    "crit",
					"critSpell": crit,
				})
		}
		return spelldef.CombatResultCrit
	}

	// Phase 2: resist check (only for non-physical schools)
	if school != spelldef.SchoolMaskPhysical {
		resistance := target.GetResistance(school)
		if resistance > 0 {
			avgReduction := resistReduction(resistance, attacker.Level)
			rollFactor := resistRoll(avgReduction)
			if t != nil {
				t.Event(trace.SpanCombat, "resist_roll",
					0, "", map[string]interface{}{
						"resistance":   resistance,
						"avgReduction": avgReduction,
						"rollFactor":   rollFactor,
					})
			}
			if rollFactor >= 1.0 {
				return spelldef.CombatResultFullResist
			}
			if rollFactor > 0 {
				return spelldef.CombatResultResist
			}
		}
	}

	if t != nil {
		t.Event(trace.SpanCombat, "spell_roll",
			0, "", map[string]interface{}{
				"roll":       math.Round(roll*100) / 100,
				"result":     "hit",
				"missChance": missChance,
				"hitSpell":   attacker.HitSpell,
			})
	}

	return spelldef.CombatResultHit
}

// glancingChance returns the glancing blow chance.
// Only applies when the target is 3+ levels above the attacker (e.g., boss).
func glancingChance(levelDiff int) float64 {
	if levelDiff >= 3 {
		return 40.0
	}
	return 0.0
}

// GlancingDamageMultiplier returns the damage multiplier for a glancing blow.
func GlancingDamageMultiplier(levelDiff int) float64 {
	if levelDiff < 1 {
		return 1.0
	}
	if levelDiff >= 4 {
		return 0.55
	}
	if levelDiff == 3 {
		return 0.65
	}
	if levelDiff == 2 {
		return 0.75
	}
	return 0.91 // level diff 1
}
