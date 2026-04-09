package targeting

import "skill-go/server/unit"

// SelectCategory describes how targets are selected.
type SelectCategory int

const (
	SelectSelf     SelectCategory = iota // caster is the target
	SelectSingle                         // single explicit target
	SelectFriendly                       // friendly units in range
	SelectEnemy                          // enemy units in range
	SelectArea                           // all units in an area
	SelectCone                           // units in a cone
	SelectChain                          // chain bounce between targets
)

// ReferenceFrame describes the origin point for target selection.
type ReferenceFrame int

const (
	RefCaster  ReferenceFrame = iota // origin is the caster
	RefTarget                        // origin is the explicit target
	RefPosition                      // origin is a world position
	RefDefault                       // use spell default
)

// ObjectType describes what kind of entity to select.
type ObjectType int

const (
	ObjUnit       ObjectType = iota
	ObjGameObject
	ObjCorpse
	ObjItem
)

// ValidationRule defines filters and limits for target selection.
type ValidationRule struct {
	MaxTargets int      // maximum number of targets (0 = unlimited)
	AliveOnly  bool     // only select living units
	DeadOnly   bool     // only select dead units
	Conditions []ConditionFunc // additional filter conditions
}

// ConditionFunc returns true if a unit passes the condition check.
type ConditionFunc func(u *unit.Unit) bool

// Direction defines spatial constraints for area/cone selection.
type Direction struct {
	Forward   bool    // use caster facing direction
	ConeAngle float64 // cone half-angle in radians
	Radius    float64 // selection radius in yards
}

// TargetDescriptor is the 5-dimension orthogonal description of target selection.
type TargetDescriptor struct {
	Category      SelectCategory
	Reference     ReferenceFrame
	ObjType       ObjectType
	Validation    ValidationRule
	Dir           Direction
}

// FilterFunc allows scripts to intercept and modify the target list.
type FilterFunc func(targets []*unit.Unit) []*unit.Unit

// FilterPoint represents an interception point in the selection pipeline.
type FilterPoint int

const (
	FilterArea   FilterPoint = iota // after area selection
	FilterUnit                      // after unit-level validation
	FilterDest                      // after destination validation
)

// SelectionContext holds the context for a target selection operation.
type SelectionContext struct {
	Caster       *unit.Unit
	ExplicitTargets []*unit.Unit
	Descriptor   TargetDescriptor
	OriginPos    unit.Position // override position for RefPosition
	Filters      map[FilterPoint][]FilterFunc
}
