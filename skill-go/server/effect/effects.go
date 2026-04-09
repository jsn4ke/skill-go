package effect

import (
	"log"

	"skill-go/server/spelldef"
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
	log.Printf("  [Launch] TriggerSpell: triggerSpellID=%d", eff.TriggerSpellID)
}

func makeTriggerSpellHit(fn TriggerSpellHandler) HitHandler {
	return func(ctx CasterInfo, eff spelldef.SpellEffectInfo, target *unit.Unit) {
		log.Printf("  [Hit] TriggerSpell → %s: triggering spell %d", target.Name, eff.TriggerSpellID)
		if eff.TriggerSpellID != 0 {
			fn(ctx.Caster(), eff.TriggerSpellID, []*unit.Unit{target})
		}
	}
}

func handleEnergizeLaunch(ctx CasterInfo, eff spelldef.SpellEffectInfo) {
	log.Printf("  [Launch] Energize: amount=%d powerType=%d", eff.EnergizeAmount, eff.EnergizeType)
}

func handleEnergizeHit(ctx CasterInfo, eff spelldef.SpellEffectInfo, target *unit.Unit) {
	log.Printf("  [Hit] Energize → %s: restoring %d mana", target.Name, eff.EnergizeAmount)
	target.RestoreMana(eff.EnergizeAmount)
	log.Printf("  [Hit] %s mana: %d/%d", target.Name, target.Mana, target.MaxMana)
}

func handleWeaponDamageLaunch(ctx CasterInfo, eff spelldef.SpellEffectInfo) {
	log.Printf("  [Launch] WeaponDamage: basePoints=%d weaponPercent=%.1f", eff.BasePoints, eff.WeaponPercent)
}

func handleWeaponDamageHit(ctx CasterInfo, eff spelldef.SpellEffectInfo, target *unit.Unit) {
	// Weapon damage = basePoints + weaponDamage * weaponPercent
	weaponDamage := int32(100) // placeholder weapon damage
	total := eff.BasePoints + int32(float64(weaponDamage)*eff.WeaponPercent)
	log.Printf("  [Hit] WeaponDamage → %s: %d total damage (base=%d + weapon=%d * %.1f)",
		target.Name, total, eff.BasePoints, weaponDamage, eff.WeaponPercent)
	target.TakeDamage(total)
	log.Printf("  [Hit] %s health: %d/%d (alive=%v)", target.Name, target.Health, target.MaxHealth, target.Alive)
}
