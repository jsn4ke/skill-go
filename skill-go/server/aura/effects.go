package aura

import (
	"log"
	"time"

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
type PeriodicEffect struct {
	Aura      *Aura
	Effect    *AuraEffect
	Target    *unit.Unit
	Caster    *unit.Unit
	IsDamage  bool // true = periodic damage, false = periodic heal
	Stopped   chan struct{}
}

// Start begins the periodic tick loop.
func (pe *PeriodicEffect) Start() {
	interval := time.Duration(pe.Effect.PeriodicTimer) * time.Millisecond
	if interval <= 0 {
		interval = 3000 * time.Millisecond
	}

	pe.Stopped = make(chan struct{})

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if !pe.Target.IsAlive() {
					return
				}
				pe.tick()

			case <-pe.Stopped:
				return
			}
		}
	}()

	log.Printf("[Periodic] started %d on %s (interval=%v, damage=%v)",
		pe.Aura.SpellID, pe.Target.Name, interval, pe.IsDamage)
}

// Stop stops the periodic effect.
func (pe *PeriodicEffect) Stop() {
	if pe.Stopped != nil {
		close(pe.Stopped)
	}
}

func (pe *PeriodicEffect) tick() {
	amount := pe.Effect.BaseAmount
	if pe.IsDamage {
		pe.Target.TakeDamage(amount)
		log.Printf("[Periodic] %d: damage tick %d → %s (HP: %d/%d)",
			pe.Aura.SpellID, amount, pe.Target.Name, pe.Target.Health, pe.Target.MaxHealth)
	} else {
		pe.Target.Heal(amount)
		log.Printf("[Periodic] %d: heal tick %d → %s (HP: %d/%d)",
			pe.Aura.SpellID, amount, pe.Target.Name, pe.Target.Health, pe.Target.MaxHealth)
	}
}
