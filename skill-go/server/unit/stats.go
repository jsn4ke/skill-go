package unit

import "fmt"

// StatType enumerates modifiable unit statistics.
type StatType int

const (
	StatStrength StatType = iota
	StatAgility
	StatStamina
	StatIntellect
	StatSpirit
	StatAttackPower
	StatSpellPower
	StatArmor
	StatCritMelee
	StatCritSpell
	StatHitMelee
	StatHitSpell
	StatDodge
	StatParry
	StatBlock
	StatBlockValue
)

// String returns a human-readable name for the stat type.
func (s StatType) String() string {
	switch s {
	case StatStrength:
		return "Strength"
	case StatAgility:
		return "Agility"
	case StatStamina:
		return "Stamina"
	case StatIntellect:
		return "Intellect"
	case StatSpirit:
		return "Spirit"
	case StatAttackPower:
		return "AttackPower"
	case StatSpellPower:
		return "SpellPower"
	case StatArmor:
		return "Armor"
	case StatCritMelee:
		return "CritMelee"
	case StatCritSpell:
		return "CritSpell"
	case StatHitMelee:
		return "HitMelee"
	case StatHitSpell:
		return "HitSpell"
	case StatDodge:
		return "Dodge"
	case StatParry:
		return "Parry"
	case StatBlock:
		return "Block"
	case StatBlockValue:
		return "BlockValue"
	default:
		return fmt.Sprintf("StatType(%d)", int(s))
	}
}
