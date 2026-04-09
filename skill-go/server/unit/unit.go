package unit

import (
	"fmt"

	"skill-go/server/spelldef"
)

// PowerType matches spelldef.PowerType values.
type PowerType int

const (
	PowerTypeMana PowerType = 0
)

// Unit represents a game entity (player, NPC, etc).
type Unit struct {
	GUID      uint64
	Name      string
	Health    int32
	MaxHealth int32
	Mana      int32
	MaxMana   int32
	Alive     bool
	Position  Position

	// State management (Phase 1)
	UnitStates spelldef.UnitState // bitfield of control effects
	CurrentForm uint32         // current shapeshift form (0 = none)
	MountID     uint32         // current mount/vehicle (0 = none)
}

// Position is a simple 3D coordinate.
type Position struct {
	X, Y, Z float64
}

// NewUnit creates a unit with the given parameters.
func NewUnit(guid uint64, name string, health, mana int32) *Unit {
	return &Unit{
		GUID:      guid,
		Name:      name,
		Health:    health,
		MaxHealth: health,
		Mana:      mana,
		MaxMana:   mana,
		Alive:     true,
	}
}

// IsAlive returns true if the unit has positive health.
func (u *Unit) IsAlive() bool {
	return u.Alive && u.Health > 0
}

// TakeDamage reduces health and kills the unit if HP <= 0.
func (u *Unit) TakeDamage(amount int32) {
	u.Health -= amount
	if u.Health <= 0 {
		u.Health = 0
		u.Alive = false
	}
}

// Heal increases health up to max.
func (u *Unit) Heal(amount int32) {
	u.Health += amount
	if u.Health > u.MaxHealth {
		u.Health = u.MaxHealth
	}
}

// ConsumeMana reduces mana if available, returns true if successful.
func (u *Unit) ConsumeMana(amount int32) bool {
	if u.Mana < amount {
		return false
	}
	u.Mana -= amount
	return true
}

// RestoreMana increases mana up to max.
func (u *Unit) RestoreMana(amount int32) {
	u.Mana += amount
	if u.Mana > u.MaxMana {
		u.Mana = u.MaxMana
	}
}

// DistanceTo calculates distance to another unit.
func (u *Unit) DistanceTo(other *Unit) float64 {
	dx := u.Position.X - other.Position.X
	dy := u.Position.Y - other.Position.Y
	dz := u.Position.Z - other.Position.Z
	return sqrt(dx*dx + dy*dy + dz*dz)
}

func sqrt(x float64) float64 {
	if x <= 0 {
		return 0
	}
	z := 1.0
	for i := 0; i < 20; i++ {
		z -= (z*z - x) / (2 * z)
	}
	return z
}

// --- Unit state management ---

// HasUnitState returns true if the unit has the specified state flag.
func (u *Unit) HasUnitState(state spelldef.UnitState) bool {
	return u.UnitStates.Has(state)
}

// ApplyUnitState sets a state flag.
func (u *Unit) ApplyUnitState(state spelldef.UnitState) {
	u.UnitStates |= state
}

// RemoveUnitState clears a state flag.
func (u *Unit) RemoveUnitState(state spelldef.UnitState) {
	u.UnitStates &^= state
}

// IsMounted returns true if the unit is on a mount or vehicle.
func (u *Unit) IsMounted() bool {
	return u.MountID != 0
}

// String returns a debug representation.
func (u *Unit) String() string {
	return fmt.Sprintf("%s(HP:%d/%d MP:%d/%d)", u.Name, u.Health, u.MaxHealth, u.Mana, u.MaxMana)
}
