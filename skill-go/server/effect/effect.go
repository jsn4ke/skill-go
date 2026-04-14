package effect

import (
	"math"

	"skill-go/server/combat"
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

	store.RegisterLaunch(spelldef.SpellEffectCharge, handleChargeLaunch)
	store.RegisterHit(spelldef.SpellEffectCharge, handleChargeHit)
}

func handleSchoolDamageLaunch(ctx CasterInfo, eff spelldef.SpellEffectInfo) {
	ctx.GetTrace().Event(trace.SpanEffectLaunch, "school_damage_launch", ctx.GetSpellID(), ctx.GetSpellName(), map[string]interface{}{
		"base":   eff.BasePoints,
		"school": eff.SchoolMask,
	})
}

func handleSchoolDamageHit(ctx CasterInfo, eff spelldef.SpellEffectInfo, target *unit.Unit) {
	caster := ctx.Caster()
	t := ctx.GetTrace()

	// Phase 1: hit resolution
	result := combat.ResolveSpellHit(caster, target, eff.SchoolMask, t)

	if result == spelldef.CombatResultMiss || result == spelldef.CombatResultFullResist {
		t.Event(trace.SpanEffectHit, "school_damage_miss", ctx.GetSpellID(), ctx.GetSpellName(), map[string]interface{}{
			"target": target.Name,
			"result": result,
		})
		return
	}

	// Phase 2: damage calculation
	damage := combat.CalcSpellDamage(eff.BasePoints, caster.SpellPower, eff.Coef, caster, target, eff.SchoolMask, t)

	// Crit multiplier for spells
	if result == spelldef.CombatResultCrit {
		damage = int32(float64(damage) * 1.5)
	}

	target.TakeDamage(damage)
	t.Event(trace.SpanEffectHit, "school_damage_hit", ctx.GetSpellID(), ctx.GetSpellName(), map[string]interface{}{
		"target":  target.Name,
		"damage":  damage,
		"school":  eff.SchoolMask,
		"result":  result,
		"hp":      target.Health,
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

func handleChargeLaunch(ctx CasterInfo, eff spelldef.SpellEffectInfo) {
	ctx.GetTrace().Event(trace.SpanEffectLaunch, "charge_launch", ctx.GetSpellID(), ctx.GetSpellName(), map[string]interface{}{
		"offset": eff.BasePoints,
	})
}

func handleChargeHit(ctx CasterInfo, eff spelldef.SpellEffectInfo, target *unit.Unit) {
	caster := ctx.Caster()
	offset := float64(eff.BasePoints)
	if offset <= 0 {
		offset = 1.0
	}

	dx := caster.Position.X - target.Position.X
	dz := caster.Position.Z - target.Position.Z
	dist := math.Sqrt(dx*dx + dz*dz)
	if dist == 0 {
		dist = 1
	}
	nx := dx / dist
	nz := dz / dist

	caster.Position = unit.Position{
		X: target.Position.X + nx*offset,
		Y: 0,
		Z: target.Position.Z + nz*offset,
	}

	ctx.GetTrace().Event(trace.SpanEffectHit, "charge_teleport", ctx.GetSpellID(), ctx.GetSpellName(), map[string]interface{}{
		"caster":  caster.Name,
		"target":  target.Name,
		"offset":  offset,
		"newPos":  map[string]float64{"x": caster.Position.X, "z": caster.Position.Z},
	})
}
