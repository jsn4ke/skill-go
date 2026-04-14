package spelldef

// UnitState represents control effects that can be applied to a unit (bitfield).
type UnitState uint32

const (
	UnitStateNone        UnitState = 0
	UnitStateSilenced    UnitState = 1 << iota // cannot cast spells
	UnitStateDisarmed                          // cannot use melee attacks
	UnitStateStunned                           // cannot act
	UnitStateRooted                            // cannot move
	UnitStateConfused                          // acts randomly
	UnitStateFleeing                           // runs in fear
	UnitStateCharmed                           // controlled by another
	UnitStateDisoriented                       // disoriented
	UnitStatePacified                          // cannot attack
	UnitStateDead                              // unit is dead
)

// Has returns true if the state bit is set.
func (s UnitState) Has(flag UnitState) bool {
	return s&flag != 0
}
