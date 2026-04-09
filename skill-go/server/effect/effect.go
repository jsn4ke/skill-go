package effect

import (
	"skill-go/server/spelldef"
	"skill-go/server/trace"
	"skill-go/server/unit"
)

// CasterInfo is the interface that effect handlers need from the spell context.
type CasterInfo interface {
	Caster() *unit.Unit
	Targets() []*unit.Unit
	GetTrace() *trace.Trace
	GetSpellID() uint32
	GetSpellName() string
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
	ctx.GetTrace().Event(trace.SpanEffectLaunch, "school_damage_launch", ctx.GetSpellID(), ctx.GetSpellName(), map[string]interface{}{
		"base":   eff.BasePoints,
		"school": eff.SchoolMask,
	})
}

func handleSchoolDamageHit(ctx CasterInfo, eff spelldef.SpellEffectInfo, target *unit.Unit) {
	damage := eff.BasePoints
	target.TakeDamage(damage)
	ctx.GetTrace().Event(trace.SpanEffectHit, "school_damage_hit", ctx.GetSpellID(), ctx.GetSpellName(), map[string]interface{}{
		"target": target.Name,
		"damage": damage,
		"school": eff.SchoolMask,
		"hp":     target.Health,
	})
}

func handleHealLaunch(ctx CasterInfo, eff spelldef.SpellEffectInfo) {
	ctx.GetTrace().Event(trace.SpanEffectLaunch, "heal_launch", ctx.GetSpellID(), ctx.GetSpellName(), map[string]interface{}{
		"base": eff.BasePoints,
	})
}

func handleHealHit(ctx CasterInfo, eff spelldef.SpellEffectInfo, target *unit.Unit) {
	healAmount := eff.BasePoints
	target.Heal(healAmount)
	ctx.GetTrace().Event(trace.SpanEffectHit, "heal_hit", ctx.GetSpellID(), ctx.GetSpellName(), map[string]interface{}{
		"target": target.Name,
		"amount": healAmount,
		"hp":     target.Health,
	})
}
