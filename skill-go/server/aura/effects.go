package aura

import (
	"log"

	"skill-go/server/unit"
)

// StatModifierEffect modifies a stat while the aura is active.
type StatModifierEffect struct {
	StatType StatType
	Amount   int32
}

// StatType enumerates character stats.
type StatType int

const (
	StatStrength StatType = iota
	StatAgility
	StatStamina
	StatIntellect
	StatSpirit
	StatAttackPower
	StatSpellPower
)

// StatTracker tracks stat modifications from auras.
type StatTracker struct {
	bonuses map[StatType]int32
}

// NewStatTracker creates a StatTracker.
func NewStatTracker() *StatTracker {
	return &StatTracker{
		bonuses: make(map[StatType]int32),
	}
}

// AddBonus adds a stat bonus.
func (t *StatTracker) AddBonus(stat StatType, amount int32) {
	t.bonuses[stat] += amount
	log.Printf("[Stat] +%d to stat %d", amount, stat)
}

// RemoveBonus removes a stat bonus.
func (t *StatTracker) RemoveBonus(stat StatType, amount int32) {
	t.bonuses[stat] -= amount
}

// GetBonus returns the current bonus for a stat.
func (t *StatTracker) GetBonus(stat StatType) int32 {
	return t.bonuses[stat]
}

// --- Periodic effects ---

// PeriodicEffect handles damage/heal ticks on a timer.
// Deprecated: use event loop aura_update ticker instead of this struct.
type PeriodicEffect struct {
	Aura     *Aura
	Effect   *AuraEffect
	Target   *unit.Unit
	Caster   *unit.Unit
	IsDamage bool // true = periodic damage, false = periodic heal
}

// Start is deprecated. Periodic effects must be handled via the event loop
// (aura_update ticker) to avoid race conditions. This method is a no-op.
func (pe *PeriodicEffect) Start() {
	log.Printf("[Periodic] WARNING: PeriodicEffect.Start() is deprecated — use event loop aura_update instead")
}

// Stop is deprecated along with Start().
func (pe *PeriodicEffect) Stop() {}
