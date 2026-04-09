package spelldef

// SchoolMask represents a bitfield of magic schools.
type SchoolMask uint32

const (
	SchoolMaskNone   SchoolMask = 0
	SchoolMaskFire    SchoolMask = 1 << 0
	SchoolMaskFrost   SchoolMask = 1 << 1
	SchoolMaskArcane  SchoolMask = 1 << 2
	SchoolMaskNature  SchoolMask = 1 << 3
	SchoolMaskShadow  SchoolMask = 1 << 4
	SchoolMaskHoly    SchoolMask = 1 << 5
)

// SpellEffectType enumerates all possible effect types.
type SpellEffectType int

const (
	SpellEffectNone          SpellEffectType = 0
	SpellEffectSchoolDamage  SpellEffectType = 1
	SpellEffectHeal          SpellEffectType = 2
	SpellEffectApplyAura     SpellEffectType = 3
	SpellEffectTriggerSpell  SpellEffectType = 4
	SpellEffectEnergize      SpellEffectType = 5
	SpellEffectWeaponDamage  SpellEffectType = 6
)

// CastResult represents the outcome of a cast attempt.
type CastResult int

const (
	CastResultSuccess      CastResult = 0
	CastResultFailed       CastResult = 1
	CastResultInterrupted  CastResult = 2
)

// SpellEffectInfo defines a single effect within a spell.
type SpellEffectInfo struct {
	EffectIndex   int
	EffectType    SpellEffectType
	SchoolMask    SchoolMask
	BasePoints    int32 // base damage/heal value
	Coef          float64
	TargetA       TargetReference
	TargetB       TargetReference

	// Extended fields (Phase 1)
	TriggerSpellID uint32    // for SpellEffectTriggerSpell
	EnergizeType   PowerType // for SpellEffectEnergize
	EnergizeAmount int32     // for SpellEffectEnergize
	WeaponPercent  float64   // for SpellEffectWeaponDamage
	AuraType       int32     // for SpellEffectApplyAura
	AuraDuration   int32     // for SpellEffectApplyAura (ms)
}

// TargetReference describes how to select targets for an effect.
type TargetReference struct {
	Type     TargetType
	Reference TargetReferenceType
}

// TargetType describes the object type to select.
type TargetType int

const (
	TargetTypeUnit       TargetType = 0
	TargetTypeGameObject TargetType = 1
	TargetTypeItem       TargetType = 2
	TargetTypeCorpse     TargetType = 3
)

// TargetReferenceType describes who the targeting is relative to.
type TargetReferenceType int

const (
	TargetRefCaster   TargetReferenceType = 0
	TargetRefTarget   TargetReferenceType = 1
	TargetRefDest     TargetReferenceType = 2
)

// SpellInfo is the read-only definition of a spell, loaded from data.
type SpellInfo struct {
	ID             uint32
	Name           string
	SchoolMask     SchoolMask
	RecoveryTime   int32 // cooldown in ms
	CategoryRecoveryTime int32
	CastTime       int32 // base cast time in ms, 0 = instant
	RangeMin       float64
	RangeMax       float64
	MaxTargets     int
	PowerCost      int32 // mana cost
	Effects        []SpellEffectInfo
	IsAutoRepeat   bool
	PreventionType PreventionType

	// Delayed execution
	DelayMs int32 // > 0 = delayed hit path (projectile travel time in ms)

	// Channeled spell
	IsChanneled     bool
	ChannelDuration int32 // total channel duration in ms
	TickInterval    int32 // ms between channel ticks

	// Empower spell
	IsEmpower       bool
	EmpowerStages   []int32 // threshold in ms for each empower stage
	EmpowerMinTime  int32 // minimum hold time before release allowed

	// Validation fields (Phase 1)
	RequiresShapeshiftMask uint32  // bitfield of allowed shapeshift forms
	RequiredAuraState      uint32  // required aura state on caster
	RequiredAreaID         int32   // area restriction
	MaxCharges             int32   // > 0 = charge-based spell
	ChargeRecoveryTime     int32   // ms to recover one charge
	RecoveryCategory       int32   // cooldown category for shared CD
	Reflectable            bool    // can be reflected
}

// PreventionType indicates what kind of interrupts can block this spell.
type PreventionType uint32

const (
	PreventionTypeNone   PreventionType = 0
	PreventionTypeSilence PreventionType = 1 << 0
	PreventionTypePacify PreventionType = 1 << 1
)

// PowerType enumerates resource types.
type PowerType int

const (
	PowerTypeMana  PowerType = 0
	PowerTypeRage  PowerType = 1
	PowerTypeEnergy PowerType = 2
)

// CastError represents specific failure reasons for spell casting.
type CastError int

const (
	CastErrNone            CastError = 0
	CastErrNotReady        CastError = 1  // on cooldown
	CastErrOutOfRange      CastError = 2
	CastErrSilenced        CastError = 3
	CastErrDisarmed        CastError = 4
	CastErrShapeshifted    CastError = 5
	CastErrNoItems         CastError = 6
	CastErrWrongArea       CastError = 7
	CastErrMounted         CastError = 8
	CastErrNoMana          CastError = 9
	CastErrDead            CastError = 10
	CastErrTargetDead      CastError = 11
	CastErrSchoolLocked    CastError = 12
	CastErrNoCharges       CastError = 13
	CastErrOnGCD           CastError = 14
	CastErrInterrupted     CastError = 15
)

// CastResultWithCode pairs a CastResult with a specific error reason.
type CastResultWithCode struct {
	Result CastResult
	Err    CastError
}
