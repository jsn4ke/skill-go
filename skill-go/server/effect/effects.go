package effect

import (
	"skill-go/server/combat"
	"skill-go/server/spelldef"
	"skill-go/server/trace"
	"skill-go/server/unit"
)

// AuraHandler is a callback for creating/applying auras, set by the aura package.
type AuraHandler func(ctx CasterInfo, eff spelldef.SpellEffectInfo, target *unit.Unit)

// TriggerSpellHandler is a callback for triggering new spells, set by the spell package.
type TriggerSpellHandler func(caster *unit.Unit, spellID uint32, targets []*unit.Unit)

// RegisterExtended registers the Phase 1 extended effect handlers.
func RegisterExtended(store *Store, auraHandler AuraHandler, triggerHandler TriggerSpellHandler) {
	if auraHandler != nil {
		store.RegisterHit(spelldef.SpellEffectApplyAura, func(ctx CasterInfo, eff spelldef.SpellEffectInfo, target *unit.Unit) {
			ctx.GetTrace().Event(trace.SpanEffectHit, "apply_aura", ctx.GetSpellID(), ctx.GetSpellName(), map[string]interface{}{
				"target":   target.Name,
				"auraType": eff.AuraType,
				"duration": eff.AuraDuration,
			})
			auraHandler(ctx, eff, target)
		})
	}
	if triggerHandler != nil {
		store.RegisterHit(spelldef.SpellEffectTriggerSpell, makeTriggerSpellHit(triggerHandler))
	}
	store.RegisterHit(spelldef.SpellEffectEnergize, handleEnergizeHit)
	store.RegisterHit(spelldef.SpellEffectWeaponDamage, handleWeaponDamageHit)
	store.RegisterLaunch(spelldef.SpellEffectEnergize, handleEnergizeLaunch)
	store.RegisterLaunch(spelldef.SpellEffectWeaponDamage, handleWeaponDamageLaunch)
	store.RegisterLaunch(spelldef.SpellEffectTriggerSpell, handleTriggerSpellLaunch)
}

func handleTriggerSpellLaunch(ctx CasterInfo, eff spelldef.SpellEffectInfo) {
	ctx.GetTrace().Event(trace.SpanEffectLaunch, "trigger_spell_launch", ctx.GetSpellID(), ctx.GetSpellName(), map[string]interface{}{
		"triggerSpellID": eff.TriggerSpellID,
	})
}

func makeTriggerSpellHit(fn TriggerSpellHandler) HitHandler {
	return func(ctx CasterInfo, eff spelldef.SpellEffectInfo, target *unit.Unit) {
		ctx.GetTrace().Event(trace.SpanEffectHit, "trigger_spell_hit", ctx.GetSpellID(), ctx.GetSpellName(), map[string]interface{}{
			"target":         target.Name,
			"triggerSpellID": eff.TriggerSpellID,
		})
		if eff.TriggerSpellID != 0 {
			fn(ctx.Caster(), eff.TriggerSpellID, []*unit.Unit{target})
		}
	}
}

func handleEnergizeLaunch(ctx CasterInfo, eff spelldef.SpellEffectInfo) {
	ctx.GetTrace().Event(trace.SpanEffectLaunch, "energize_launch", ctx.GetSpellID(), ctx.GetSpellName(), map[string]interface{}{
		"amount":    eff.EnergizeAmount,
		"powerType": eff.EnergizeType,
	})
}

func handleEnergizeHit(ctx CasterInfo, eff spelldef.SpellEffectInfo, target *unit.Unit) {
	// Energize with rage applies to caster (e.g. Charge), others apply to target
	receiver := target
	if eff.EnergizeType == spelldef.PowerTypeRage {
		receiver = ctx.Caster()
	}
	switch eff.EnergizeType {
	case spelldef.PowerTypeRage:
		receiver.RestoreRage(eff.EnergizeAmount)
	default:
		receiver.RestoreMana(eff.EnergizeAmount)
	}
	ctx.GetTrace().Event(trace.SpanEffectHit, "energize_hit", ctx.GetSpellID(), ctx.GetSpellName(), map[string]interface{}{
		"target":    receiver.Name,
		"amount":    eff.EnergizeAmount,
		"mana":      receiver.Mana,
		"rage":      receiver.Rage,
		"powerType": eff.EnergizeType,
	})
}

func handleWeaponDamageLaunch(ctx CasterInfo, eff spelldef.SpellEffectInfo) {
	ctx.GetTrace().Event(trace.SpanEffectLaunch, "weapon_damage_launch", ctx.GetSpellID(), ctx.GetSpellName(), map[string]interface{}{
		"basePoints":    eff.BasePoints,
		"weaponPercent": eff.WeaponPercent,
	})
}

func handleWeaponDamageHit(ctx CasterInfo, eff spelldef.SpellEffectInfo, target *unit.Unit) {
	caster := ctx.Caster()
	t := ctx.GetTrace()

	// Phase 1: hit resolution
	result := combat.ResolveMeleeHit(caster, target, t)

	if result == spelldef.CombatResultMiss || result == spelldef.CombatResultDodge || result == spelldef.CombatResultParry {
		t.Event(trace.SpanEffectHit, "weapon_damage_miss", ctx.GetSpellID(), ctx.GetSpellName(), map[string]interface{}{
			"target": target.Name,
			"result": result,
		})
		return
	}

	// Phase 2: damage calculation
	damage := combat.CalcMeleeDamage(caster, target, t)

	// Add basePoints on top of weapon damage
	damage += eff.BasePoints

	// Crit multiplier for melee (2.0x)
	if result == spelldef.CombatResultCrit {
		damage = int32(float64(damage) * 2.0)
	}

	// Glancing damage reduction
	if result == spelldef.CombatResultGlancing {
		levelDiff := int(target.Level) - int(caster.Level)
		if levelDiff < 0 {
			levelDiff = 0
		}
		damage = int32(float64(damage) * combat.GlancingDamageMultiplier(levelDiff))
	}

	// Block reduces damage
	if result == spelldef.CombatResultBlock {
		damage -= target.BlockValue
		if damage < 1 {
			damage = 1
		}
	}

	target.TakeDamage(damage)
	t.Event(trace.SpanEffectHit, "weapon_damage_hit", ctx.GetSpellID(), ctx.GetSpellName(), map[string]interface{}{
		"target":        target.Name,
		"totalDamage":   damage,
		"basePoints":    eff.BasePoints,
		"weaponPercent": eff.WeaponPercent,
		"result":        result,
		"hp":            target.Health,
	})
}
