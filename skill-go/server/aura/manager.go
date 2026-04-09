package aura

import (
	"math/rand"
	"time"

	"skill-go/server/spelldef"
	"skill-go/server/trace"
	"skill-go/server/unit"
)

// AuraManager manages all auras on a unit.
type AuraManager struct {
	Owner  *unit.Unit
	Auras  map[uint32]*Aura // keyed by spellID
}

// NewAuraManager creates an AuraManager for a unit.
func NewAuraManager(owner *unit.Unit) *AuraManager {
	return &AuraManager{
		Owner: owner,
		Auras: make(map[uint32]*Aura),
	}
}

// ApplyAura applies an aura to the target unit.
func (m *AuraManager) ApplyAura(aura *Aura, t *trace.Trace, spellID uint32, spellName string) {
	existing, ok := m.Auras[aura.SpellID]

	if ok && existing.CasterGUID == aura.CasterGUID {
		// Same caster, same aura → refresh duration or stack
		if existing.MaxStack > 1 && existing.StackAmount < existing.MaxStack {
			existing.StackAmount++
		} else {
			t.Event(trace.SpanAura, "refreshed", spellID, spellName, map[string]interface{}{
				"target":  m.Owner.Name,
				"auraID":  aura.SpellID,
			})
			existing.Duration = aura.Duration
			return
		}
		t.Event(trace.SpanAura, "stacked", spellID, spellName, map[string]interface{}{
			"target":      m.Owner.Name,
			"auraID":      aura.SpellID,
			"stacks":      existing.StackAmount,
		})
		existing.Duration = aura.Duration
		return
	}

	if ok && existing.CasterGUID != aura.CasterGUID {
		t.Event(trace.SpanAura, "replacing", spellID, spellName, map[string]interface{}{
			"target":       m.Owner.Name,
			"auraID":       aura.SpellID,
			"oldCaster":    existing.CasterGUID,
			"newCaster":    aura.CasterGUID,
		})
		m.RemoveAura(existing, RemoveModeDefault, t, spellID, spellName)
	}

	// Create application
	app := &AuraApplication{
		Target:         m.Owner,
		RemoveMode:     RemoveModeDefault,
		NeedClientUpdate: true,
		TimerStart:     time.Now().UnixMilli(),
	}

	aura.Applications = append(aura.Applications, app)
	m.Auras[aura.SpellID] = aura

	// Apply aura effects
	for _, eff := range aura.Effects {
		applyEffect(eff, m.Owner)
	}

	t.Event(trace.SpanAura, "applied", spellID, spellName, map[string]interface{}{
		"target":   m.Owner.Name,
		"auraID":   aura.SpellID,
		"auraType": aura.AuraType,
		"duration": aura.Duration,
		"stacks":   aura.StackAmount,
	})
}

// RemoveAura removes an aura from the target.
func (m *AuraManager) RemoveAura(aura *Aura, mode RemoveMode, t *trace.Trace, spellID uint32, spellName string) {
	app := findApplication(aura, m.Owner)
	if app == nil {
		return
	}

	app.RemoveMode = mode

	// Remove aura effects
	for _, eff := range aura.Effects {
		removeEffect(eff, m.Owner)
	}

	delete(m.Auras, aura.SpellID)
	t.Event(trace.SpanAura, "removed", spellID, spellName, map[string]interface{}{
		"target":  m.Owner.Name,
		"auraID":  aura.SpellID,
		"mode":    mode,
	})
}

// HasAura checks if the unit has a specific aura.
func (m *AuraManager) HasAura(spellID uint32) bool {
	_, ok := m.Auras[spellID]
	return ok
}

// GetAura returns the aura by spellID, or nil.
func (m *AuraManager) GetAura(spellID uint32) *Aura {
	return m.Auras[spellID]
}

// --- Stacking rules ---

// StackRule determines how a new aura interacts with existing ones.
type StackRule int

const (
	StackRefresh   StackRule = iota // refresh duration only
	StackAddStack                    // add a stack
	StackReplace                     // replace entirely
)

// --- Proc pipeline ---

// ProcCheckResult is the result of a proc attempt.
type ProcCheckResult struct {
	Triggered bool
	Aura      *Aura
	Effect    *AuraEffect
}

// CheckProc runs the full proc pipeline: prepare → identify → determine → post-process.
func (m *AuraManager) CheckProc(event ProcEvent, t *trace.Trace, spellID uint32, spellName string) []*ProcCheckResult {
	var results []*ProcCheckResult

	for _, aura := range m.Auras {
		for _, eff := range aura.Effects {
			result := m.checkProcForEffect(aura, eff, event, t, spellID, spellName)
			if result != nil {
				results = append(results, result)
			}
		}
	}

	return results
}

func (m *AuraManager) checkProcForEffect(aura *Aura, eff *AuraEffect, event ProcEvent, t *trace.Trace, spellID uint32, spellName string) *ProcCheckResult {
	if aura.ProcCharges > 0 && aura.RemainingProcs <= 0 {
		return nil
	}

	// Check event match (simplified: any proc aura triggers on any event)
	// In full implementation, each effect would have a ProcEventMask

	// Determine trigger
	triggered := false
	if aura.PPM > 0 {
		// PPM: convert to per-swing probability
		// PPM * weaponSpeed / 60 = chance per swing
		// Assume 2.6s weapon speed
		weaponSpeed := 2.6
		chance := (aura.PPM * weaponSpeed) / 60.0 * 100.0
		if rand.Float64()*100 < chance {
			triggered = true
		}
	} else if aura.ProcChance > 0 {
		if rand.Float64()*100 < aura.ProcChance {
			triggered = true
		}
	} else {
		triggered = true // 100% proc
	}

	if !triggered {
		return nil
	}

	// Post-process: consume proc charges
	if aura.ProcCharges > 0 {
		aura.RemainingProcs--
		if aura.RemainingProcs <= 0 {
			m.RemoveAura(aura, RemoveModeExpired, t, spellID, spellName)
		}
	}

	t.Event(trace.SpanProc, "check", spellID, spellName, map[string]interface{}{
		"target":     m.Owner.Name,
		"auraID":     aura.SpellID,
		"procEvent":  int(event),
		"remaining":  aura.RemainingProcs,
	})

	return &ProcCheckResult{
		Triggered: true,
		Aura:      aura,
		Effect:    eff,
	}
}

// --- Aura effect application ---

func applyEffect(eff *AuraEffect, target *unit.Unit) {
	switch eff.AuraType {
	case AuraTypeBuff:
		// Check if this is a stat modifier (MiscValue maps to StatType)
		if statType := unit.StatType(eff.MiscValue); isValidStatType(statType) {
			target.ModifyStat(statType, eff.BaseAmount)
		}
	case AuraTypeDebuff:
		// Apply control effect
		applyControlEffect(eff, target)
	default:
		// Passive/Proc auras don't apply stat changes
	}
}

func removeEffect(eff *AuraEffect, target *unit.Unit) {
	switch eff.AuraType {
	case AuraTypeBuff:
		// Reverse stat modifier
		if statType := unit.StatType(eff.MiscValue); isValidStatType(statType) {
			target.ModifyStat(statType, -eff.BaseAmount)
		}
	case AuraTypeDebuff:
		removeControlEffect(eff, target)
	}
}

// isValidStatType returns true if the value is a valid StatType.
func isValidStatType(s unit.StatType) bool {
	return s >= unit.StatStrength && s <= unit.StatBlockValue
}

func applyControlEffect(eff *AuraEffect, target *unit.Unit) {
	switch eff.MiscValue {
	case int32(spelldef.UnitStateSilenced):
		target.ApplyUnitState(spelldef.UnitStateSilenced)
	case int32(spelldef.UnitStateDisarmed):
		target.ApplyUnitState(spelldef.UnitStateDisarmed)
	case int32(spelldef.UnitStateStunned):
		target.ApplyUnitState(spelldef.UnitStateStunned)
	}
}

func removeControlEffect(eff *AuraEffect, target *unit.Unit) {
	switch eff.MiscValue {
	case int32(spelldef.UnitStateSilenced):
		target.RemoveUnitState(spelldef.UnitStateSilenced)
	case int32(spelldef.UnitStateDisarmed):
		target.RemoveUnitState(spelldef.UnitStateDisarmed)
	case int32(spelldef.UnitStateStunned):
		target.RemoveUnitState(spelldef.UnitStateStunned)
	}
}

func findApplication(aura *Aura, target *unit.Unit) *AuraApplication {
	for _, app := range aura.Applications {
		if app.Target.GUID == target.GUID {
			return app
		}
	}
	return nil
}
