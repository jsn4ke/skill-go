package aura

import (
	"skill-go/server/unit"
)

// AuraType enumerates aura classification.
type AuraType int

const (
	AuraTypeBuff    AuraType = iota // beneficial aura
	AuraTypeDebuff                   // harmful aura
	AuraTypePassive                  // always-on aura
	AuraTypeProc                     // on-event trigger aura
)

// RemoveMode describes why an aura was removed.
type RemoveMode int

const (
	RemoveModeDefault RemoveMode = iota
	RemoveModeExpired
	RemoveModeCancelled
	RemoveModeDispel
	RemoveModeDeath
)

// Aura is the top-level container — holds effects and metadata.
type Aura struct {
	SpellID      uint32
	SourceName   string // name of the spell that created this aura
	CasterGUID   uint64
	Caster       *unit.Unit
	AuraType     AuraType
	MaxCharges   int32
	Charges      int32
	Duration     int32  // ms, 0 = permanent
	StackAmount  int32  // current stack count
	MaxStack     int32  // max stacks
	ProcChance   float64 // 0-100
	PPM          float64 // procs per minute, 0 = disabled
	ProcCharges  int32   // max procs before aura expires
	RemainingProcs int32
	Effects      []*AuraEffect
	Applications []*AuraApplication
}

// AuraEffect is the middle layer — describes a single effect within an aura.
type AuraEffect struct {
	AuraType      AuraType
	SpellID       uint32
	EffectIndex   int
	AuraName      string
	BaseAmount    int32
	MiscValue     int32
	PeriodicTimer int32 // ms between ticks, 0 = not periodic
}

// AuraApplication is the bottom layer — binds an aura to a specific target.
type AuraApplication struct {
	Target         *unit.Unit
	BaseAmount     int32
	RemoveMode     RemoveMode
	NeedClientUpdate bool
	TimerStart     int64 // ms timestamp when applied
}

// ProcEvent enumerates events that can trigger a proc.
type ProcEvent int

const (
	ProcEventOnHit       ProcEvent = iota // when dealing damage
	ProcEventOnCrit                        // when dealing critical damage
	ProcEventOnCast                        // when casting a spell
	ProcEventOnTakeDamage                  // when taking damage
	ProcEventOnHeal                        // when healing
	ProcEventOnBeHealed                    // when being healed
)
