package effect

import (
	"log"

	"skill-go/server/spelldef"
	"skill-go/server/unit"
)

// CasterInfo is the interface that effect handlers need from the spell context.
type CasterInfo interface {
	Caster() *unit.Unit
	Targets() []*unit.Unit
}

// LaunchHandler is called during the Launch phase of an effect.
type LaunchHandler func(ctx CasterInfo, eff spelldef.SpellEffectInfo)

// HitHandler is called during the Hit phase of an effect, targeting a specific unit.
type HitHandler func(ctx CasterInfo, eff spelldef.SpellEffectInfo, target *unit.Unit)

// Store holds registered effect handlers.
type Store struct {
	launchHandlers map[spelldef.SpellEffectType]LaunchHandler
	hitHandlers    map[spelldef.SpellEffectType]HitHandler
}

// NewStore creates an empty effect store with default handlers registered.
func NewStore() *Store {
	store := &Store{
		launchHandlers: make(map[spelldef.SpellEffectType]LaunchHandler),
		hitHandlers:    make(map[spelldef.SpellEffectType]HitHandler),
	}
	RegisterDefaults(store)
	return store
}

// RegisterLaunch adds a launch handler for an effect type.
func (s *Store) RegisterLaunch(effectType spelldef.SpellEffectType, handler LaunchHandler) {
	s.launchHandlers[effectType] = handler
}

// RegisterHit adds a hit handler for an effect type.
func (s *Store) RegisterHit(effectType spelldef.SpellEffectType, handler HitHandler) {
	s.hitHandlers[effectType] = handler
}

// GetLaunchHandler returns the launch handler for an effect type, or nil.
func (s *Store) GetLaunchHandler(effectType spelldef.SpellEffectType) LaunchHandler {
	return s.launchHandlers[effectType]
}

// GetHitHandler returns the hit handler for an effect type, or nil.
func (s *Store) GetHitHandler(effectType spelldef.SpellEffectType) HitHandler {
	return s.hitHandlers[effectType]
}

// RegisterDefaults registers built-in effect handlers.
func RegisterDefaults(store *Store) {
	store.RegisterLaunch(spelldef.SpellEffectSchoolDamage, handleSchoolDamageLaunch)
	store.RegisterHit(spelldef.SpellEffectSchoolDamage, handleSchoolDamageHit)

	store.RegisterLaunch(spelldef.SpellEffectHeal, handleHealLaunch)
	store.RegisterHit(spelldef.SpellEffectHeal, handleHealHit)
}

func handleSchoolDamageLaunch(ctx CasterInfo, eff spelldef.SpellEffectInfo) {
	log.Printf("  [Launch] SchoolDamage: base=%d school=%d", eff.BasePoints, eff.SchoolMask)
}

func handleSchoolDamageHit(ctx CasterInfo, eff spelldef.SpellEffectInfo, target *unit.Unit) {
	damage := eff.BasePoints
	log.Printf("  [Hit] SchoolDamage → %s: %d damage (school=%d)", target.Name, damage, eff.SchoolMask)
	target.TakeDamage(damage)
	log.Printf("  [Hit] %s health: %d/%d (alive=%v)", target.Name, target.Health, target.MaxHealth, target.Alive)
}

func handleHealLaunch(ctx CasterInfo, eff spelldef.SpellEffectInfo) {
	log.Printf("  [Launch] Heal: base=%d", eff.BasePoints)
}

func handleHealHit(ctx CasterInfo, eff spelldef.SpellEffectInfo, target *unit.Unit) {
	healAmount := eff.BasePoints
	log.Printf("  [Hit] Heal → %s: %d healing", target.Name, healAmount)
	target.Heal(healAmount)
	log.Printf("  [Hit] %s health: %d/%d", target.Name, target.Health, target.MaxHealth)
}
