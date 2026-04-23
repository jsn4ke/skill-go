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
	GUID             uint64
	Name             string
	Health           int32
	MaxHealth        int32
	Mana             int32
	MaxMana          int32
	Rage             int32
	MaxRage          int32
	PrimaryPowerType spelldef.PowerType
	Alive            bool
	Position         Position

	// State management
	UnitStates  spelldef.UnitState     // bitfield of control effects
	CurrentForm spelldef.ShapeshiftForm // current shapeshift form (0 = none)
	MountID     uint32                 // current mount/vehicle (0 = none)

	// Identity
	Level  uint8
	TeamID uint32

	// Defense
	Armor       int32
	Resistances [spelldef.SchoolMax]float64

	// Base attributes
	Str, Agi, Sta, Int, Spi int32

	// Combat statistics
	AttackPower int32
	SpellPower  int32
	CritMelee   float64 // 0–100%
	CritSpell   float64 // 0–100%
	HitMelee    float64 // 0–100%
	HitSpell    float64 // 0–100%
	Dodge       float64 // 0–100%
	Parry       float64 // 0–100%
	Block       float64 // 0–100%
	BlockValue  int32

	// Weapon
	MinWeaponDamage int32
	MaxWeaponDamage int32

	// Movement speed modifier (multiplicative). 1.0 = normal, 0.5 = 50% slow.
	SpeedMod float64

	// OnDamageTaken is called after this unit takes damage. Set by the game loop.
	OnDamageTaken func(u *Unit, amount int32)
}

// Position is a simple 3D coordinate.
type Position struct {
	X, Y, Z float64
}

// NewUnit creates a unit with the given parameters. New fields default to zero.
func NewUnit(guid uint64, name string, health, mana int32) *Unit {
	return &Unit{
		GUID:      guid,
		Name:      name,
		Health:    health,
		MaxHealth: health,
		Mana:      mana,
		MaxMana:   mana,
		Alive:     true,
		Level:     1,
		SpeedMod:  1.0,
	}
}

// NewUnitWithStats creates a unit with level and team specified.
func NewUnitWithStats(guid uint64, name string, health, mana int32, level uint8, teamID uint32) *Unit {
	u := NewUnit(guid, name, health, mana)
	u.Level = level
	u.TeamID = teamID
	return u
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
	if u.OnDamageTaken != nil {
		u.OnDamageTaken(u, amount)
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

// ConsumeRage reduces rage if available, returns true if successful.
func (u *Unit) ConsumeRage(amount int32) bool {
	if u.Rage < amount {
		return false
	}
	u.Rage -= amount
	return true
}

// RestoreRage increases rage up to max.
func (u *Unit) RestoreRage(amount int32) {
	u.Rage += amount
	if u.Rage > u.MaxRage {
		u.Rage = u.MaxRage
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

// --- Stat modification ---

// ModifyStat adjusts a combat stat by the given amount.
// For percentage stats (Crit/Dodge/Parry/Block/Hit), amount is treated as
// a percentage value (e.g. +5 means +5%).
func (u *Unit) ModifyStat(stat StatType, amount int32) {
	switch stat {
	case StatStrength:
		u.Str += amount
	case StatAgility:
		u.Agi += amount
	case StatStamina:
		u.Sta += amount
	case StatIntellect:
		u.Int += amount
	case StatSpirit:
		u.Spi += amount
	case StatAttackPower:
		u.AttackPower += amount
	case StatSpellPower:
		u.SpellPower += amount
	case StatArmor:
		u.Armor += amount
	case StatCritMelee:
		u.CritMelee += float64(amount)
	case StatCritSpell:
		u.CritSpell += float64(amount)
	case StatHitMelee:
		u.HitMelee += float64(amount)
	case StatHitSpell:
		u.HitSpell += float64(amount)
	case StatDodge:
		u.Dodge += float64(amount)
	case StatParry:
		u.Parry += float64(amount)
	case StatBlock:
		u.Block += float64(amount)
	case StatBlockValue:
		u.BlockValue += amount
	}
}

// --- Armor ---

// SetArmor sets the unit's armor value.
func (u *Unit) SetArmor(amount int32) {
	u.Armor = amount
}

// --- Resistances ---

// SetResistance sets the resistance for a specific school.
func (u *Unit) SetResistance(school spelldef.SchoolMask, value float64) {
	idx := schoolIndex(school)
	u.Resistances[idx] = value
}

// GetResistance returns the resistance for a specific school.
func (u *Unit) GetResistance(school spelldef.SchoolMask) float64 {
	idx := schoolIndex(school)
	return u.Resistances[idx]
}

// schoolIndex maps a single-bit SchoolMask to an array index (0–6).
func schoolIndex(school spelldef.SchoolMask) int {
	switch school {
	case spelldef.SchoolMaskFire:
		return 0
	case spelldef.SchoolMaskFrost:
		return 1
	case spelldef.SchoolMaskArcane:
		return 2
	case spelldef.SchoolMaskNature:
		return 3
	case spelldef.SchoolMaskShadow:
		return 4
	case spelldef.SchoolMaskHoly:
		return 5
	case spelldef.SchoolMaskPhysical:
		return 6
	default:
		return 0
	}
}

// --- Level ---

// SetLevel sets the unit's level.
func (u *Unit) SetLevel(level uint8) {
	u.Level = level
}

// --- Faction ---

// IsFriendly returns true if both units share the same TeamID.
func (u *Unit) IsFriendly(other *Unit) bool {
	return u.TeamID == other.TeamID
}

// --- Weapon ---

// SetWeaponDamage sets the weapon damage range.
func (u *Unit) SetWeaponDamage(minDmg, maxDmg int32) {
	u.MinWeaponDamage = minDmg
	u.MaxWeaponDamage = maxDmg
}

// RecalcSpeedMod recomputes the multiplicative speed modifier from a list of
// (amount, pct) pairs where amount is the slow percentage (e.g. 65 = 65% slow).
// Slows stack multiplicatively: final = product of (1 - amount/100) for each slow.
func (u *Unit) RecalcSpeedMod(slows []int32) {
	if len(slows) == 0 {
		u.SpeedMod = 1.0
		return
	}
	mod := 1.0
	for _, pct := range slows {
		mod *= (1.0 - float64(pct)/100.0)
	}
	if mod < 0.0 {
		mod = 0.0
	}
	u.SpeedMod = mod
}

// String returns a debug representation.
func (u *Unit) String() string {
	return fmt.Sprintf("%s(HP:%d/%d MP:%d/%d)", u.Name, u.Health, u.MaxHealth, u.Mana, u.MaxMana)
}
