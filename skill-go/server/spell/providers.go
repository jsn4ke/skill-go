package spell

import (
	"skill-go/server/aura"
	"skill-go/server/trace"
)

// CooldownProvider abstracts cooldown operations for spell→cooldown integration.
type CooldownProvider interface {
	AddCooldown(spellID uint32, durationMs int32, category int32)
	StartGCD(category int32, durationMs int32)
	ConsumeCharge(spellID uint32) bool
	GetChargeRemaining(spellID uint32) int32
}

// TracingCooldownProvider extends CooldownProvider with trace-aware methods.
type TracingCooldownProvider interface {
	CooldownProvider
	TraceAddCooldown(spellID uint32, durationMs int32, category int32, t *trace.Trace)
	TraceConsumeCharge(spellID uint32, t *trace.Trace) bool
	TraceStartGCD(category int32, durationMs int32, t *trace.Trace)
}

// AuraProvider abstracts aura access for spell→aura integration.
type AuraProvider interface {
	GetAuraManager(target interface{}) *aura.AuraManager
}
