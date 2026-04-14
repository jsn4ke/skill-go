package script

import (
	"sync"
)

// Registry holds all registered spell and aura scripts.
type Registry struct {
	spellScripts map[uint32]*SpellScript
	auraScripts  map[uint32]*AuraScript
	mu           sync.RWMutex
}

// NewRegistry creates a new script registry.
func NewRegistry() *Registry {
	return &Registry{
		spellScripts: make(map[uint32]*SpellScript),
		auraScripts:  make(map[uint32]*AuraScript),
	}
}

// RegisterSpellScript registers a spell script setup function for a spell ID.
func (r *Registry) RegisterSpellScript(spellID uint32, setup SpellSetupFunc) {
	r.mu.Lock()
	defer r.mu.Unlock()

	ss := NewSpellScript()
	setup(ss)
	r.spellScripts[spellID] = ss
}

// RegisterAuraScript registers an aura script setup function for a spell ID.
func (r *Registry) RegisterAuraScript(spellID uint32, setup AuraSetupFunc) {
	r.mu.Lock()
	defer r.mu.Unlock()

	as := NewAuraScript()
	setup(as)
	r.auraScripts[spellID] = as
}

// GetSpellScript returns the spell script for a spell ID, or nil.
func (r *Registry) GetSpellScript(spellID uint32) *SpellScript {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.spellScripts[spellID]
}

// GetAuraScript returns the aura script for a spell ID, or nil.
func (r *Registry) GetAuraScript(spellID uint32) *AuraScript {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.auraScripts[spellID]
}

// Phase represents the current lifecycle phase of a spell.
type Phase int

const (
	PhaseNone    Phase = iota
	PhasePrepare       // can read SpellInfo only
	PhaseCast          // can read SpellInfo + targets
	PhaseLaunch        // can read/write launch data
	PhaseHit           // can read/write hit data + targets
	PhaseChannel       // channel tick context
	PhaseFinish        // post-cast context
)

// PhaseGuard restricts what data can be accessed at each phase.
type PhaseGuard struct {
	CurrentPhase Phase
}

// NewPhaseGuard creates a guard starting at the given phase.
func NewPhaseGuard(phase Phase) *PhaseGuard {
	return &PhaseGuard{CurrentPhase: phase}
}

// SetPhase changes the current phase.
func (pg *PhaseGuard) SetPhase(phase Phase) {
	pg.CurrentPhase = phase
}

// CanAccessTargets returns true if the current phase allows target access.
func (pg *PhaseGuard) CanAccessTargets() bool {
	switch pg.CurrentPhase {
	case PhasePrepare:
		return false
	case PhaseCast, PhaseLaunch, PhaseHit, PhaseChannel:
		return true
	default:
		return true
	}
}

// CanModifyHit returns true if the current phase allows modifying hit results.
func (pg *PhaseGuard) CanModifyHit() bool {
	return pg.CurrentPhase == PhaseHit
}

// CanPreventDefault returns true if the current phase allows preventing default behavior.
func (pg *PhaseGuard) CanPreventDefault() bool {
	switch pg.CurrentPhase {
	case PhasePrepare, PhaseHit, PhaseLaunch:
		return true
	default:
		return false
	}
}
