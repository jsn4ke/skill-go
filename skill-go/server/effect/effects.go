package effect

import (
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
				"target":    target.Name,
				"auraType":  eff.AuraType,
				"duration":  eff.AuraDuration,
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
	target.RestoreMana(eff.EnergizeAmount)
	ctx.GetTrace().Event(trace.SpanEffectHit, "energize_hit", ctx.GetSpellID(), ctx.GetSpellName(), map[string]interface{}{
		"target": target.Name,
		"amount": eff.EnergizeAmount,
		"mana":   target.Mana,
	})
}

func handleWeaponDamageLaunch(ctx CasterInfo, eff spelldef.SpellEffectInfo) {
	ctx.GetTrace().Event(trace.SpanEffectLaunch, "weapon_damage_launch", ctx.GetSpellID(), ctx.GetSpellName(), map[string]interface{}{
		"basePoints":    eff.BasePoints,
		"weaponPercent": eff.WeaponPercent,
	})
}

func handleWeaponDamageHit(ctx CasterInfo, eff spelldef.SpellEffectInfo, target *unit.Unit) {
	// Weapon damage = basePoints + weaponDamage * weaponPercent
	weaponDamage := int32(100) // placeholder weapon damage
	total := eff.BasePoints + int32(float64(weaponDamage)*eff.WeaponPercent)
	target.TakeDamage(total)
	ctx.GetTrace().Event(trace.SpanEffectHit, "weapon_damage_hit", ctx.GetSpellID(), ctx.GetSpellName(), map[string]interface{}{
		"target":         target.Name,
		"totalDamage":    total,
		"basePoints":     eff.BasePoints,
		"weaponDamage":   weaponDamage,
		"weaponPercent":  eff.WeaponPercent,
		"hp":             target.Health,
	})
}
