package aura

import (
	"testing"

	"skill-go/server/spelldef"
	"skill-go/server/unit"
)

// helper to create a basic buff aura
func newBuffAura(spellID uint32, casterGUID uint64, maxStack int32) *Aura {
	return &Aura{
		SpellID:     spellID,
		CasterGUID:  casterGUID,
		AuraType:    AuraTypeBuff,
		Duration:    10000,
		MaxStack:    maxStack,
		StackAmount: 1,
		Effects:     nil,
	}
}

// helper to create a debuff aura that applies a control effect
func newDebuffAura(spellID uint32, casterGUID uint64, controlState spelldef.UnitState) *Aura {
	return &Aura{
		SpellID:     spellID,
		CasterGUID:  casterGUID,
		AuraType:    AuraTypeDebuff,
		Duration:    10000,
		MaxStack:    1,
		StackAmount: 1,
		Effects: []*AuraEffect{
			{
				AuraType:    AuraTypeDebuff,
				SpellID:     spellID,
				EffectIndex: 0,
				BaseAmount:  0,
				MiscValue:   int32(controlState),
			},
		},
	}
}

// helper to create a proc aura with configurable proc chance
func newProcAura(spellID uint32, casterGUID uint64, procChance float64, procCharges int32) *Aura {
	remaining := procCharges
	if remaining == 0 {
		remaining = -1 // sentinel for unlimited
	}
	return &Aura{
		SpellID:        spellID,
		CasterGUID:     casterGUID,
		AuraType:       AuraTypeProc,
		Duration:       30000,
		MaxStack:       1,
		StackAmount:    1,
		ProcChance:     procChance,
		ProcCharges:    procCharges,
		RemainingProcs: remaining,
		Effects: []*AuraEffect{
			{
				AuraType:    AuraTypeProc,
				SpellID:     spellID,
				EffectIndex: 0,
				BaseAmount:  100,
			},
		},
	}
}

func TestApplyAura_Refresh(t *testing.T) {
	owner := unit.NewUnit(1, "Target", 1000, 500)
	caster := unit.NewUnit(2, "Caster", 1000, 500)
	mgr := NewAuraManager(owner)

	const spellID uint32 = 1001
	a1 := newBuffAura(spellID, caster.GUID, 1)

	mgr.ApplyAura(a1, nil, 0, "")
	if !mgr.HasAura(spellID) {
		t.Fatal("expected aura to be present after first apply")
	}

	// Re-apply same spell from same caster: refresh duration
	a2 := newBuffAura(spellID, caster.GUID, 1)
	mgr.ApplyAura(a2, nil, 0, "")

	if !mgr.HasAura(spellID) {
		t.Fatal("expected aura still present after refresh")
	}

	got := mgr.GetAura(spellID)
	if got.Duration != a2.Duration {
		t.Errorf("expected duration %d after refresh, got %d", a2.Duration, got.Duration)
	}
}

func TestApplyAura_Stacking(t *testing.T) {
	owner := unit.NewUnit(1, "Target", 1000, 500)
	caster := unit.NewUnit(2, "Caster", 1000, 500)
	mgr := NewAuraManager(owner)

	const spellID uint32 = 2001
	const maxStack int32 = 3

	tests := []struct {
		name        string
		applyCount  int
		wantStacks  int32
		wantHasAura bool
	}{
		{"1st apply", 1, 1, true},
		{"2nd apply", 2, 2, true},
		{"3rd apply (max)", 3, 3, true},
		{"4th apply (over max)", 4, 3, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := newBuffAura(spellID, caster.GUID, maxStack)
			mgr.ApplyAura(a, nil, 0, "")

			got := mgr.GetAura(spellID)
			if got == nil {
				if tt.wantHasAura {
					t.Fatal("expected aura to exist")
				}
				return
			}

			if got.StackAmount != tt.wantStacks {
				t.Errorf("StackAmount = %d, want %d", got.StackAmount, tt.wantStacks)
			}
		})
	}
}

func TestApplyAura_DifferentCasterReplaces(t *testing.T) {
	owner := unit.NewUnit(1, "Target", 1000, 500)
	caster1 := unit.NewUnit(10, "Caster1", 1000, 500)
	caster2 := unit.NewUnit(20, "Caster2", 1000, 500)
	mgr := NewAuraManager(owner)

	const spellID uint32 = 3001

	a1 := newBuffAura(spellID, caster1.GUID, 1)
	mgr.ApplyAura(a1, nil, 0, "")

	if !mgr.HasAura(spellID) {
		t.Fatal("expected aura after first caster apply")
	}
	got1 := mgr.GetAura(spellID)
	if got1.CasterGUID != caster1.GUID {
		t.Errorf("expected caster GUID %d, got %d", caster1.GUID, got1.CasterGUID)
	}

	// Second caster applies same spell
	a2 := newBuffAura(spellID, caster2.GUID, 1)
	mgr.ApplyAura(a2, nil, 0, "")

	if !mgr.HasAura(spellID) {
		t.Fatal("expected aura still present after second caster apply")
	}
	got2 := mgr.GetAura(spellID)
	if got2.CasterGUID != caster2.GUID {
		t.Errorf("expected caster GUID %d after replacement, got %d", caster2.GUID, got2.CasterGUID)
	}
}

func TestRemoveAura_DebuffWithSilence(t *testing.T) {
	owner := unit.NewUnit(1, "Target", 1000, 500)
	caster := unit.NewUnit(2, "Caster", 1000, 500)
	mgr := NewAuraManager(owner)

	const spellID uint32 = 4001

	// Apply silence debuff
	silenceAura := newDebuffAura(spellID, caster.GUID, spelldef.UnitStateSilenced)
	mgr.ApplyAura(silenceAura, nil, 0, "")

	if !owner.HasUnitState(spelldef.UnitStateSilenced) {
		t.Error("expected target to be silenced after debuff applied")
	}

	// Remove the aura
	mgr.RemoveAura(silenceAura, RemoveModeDispel, nil, 0, "")

	if owner.HasUnitState(spelldef.UnitStateSilenced) {
		t.Error("expected target to no longer be silenced after aura removed")
	}

	if mgr.HasAura(spellID) {
		t.Error("expected aura to be gone after removal")
	}
}

func TestRemoveAura_NotApplied(t *testing.T) {
	owner := unit.NewUnit(1, "Target", 1000, 500)
	mgr := NewAuraManager(owner)

	// Removing an aura that was never applied should not panic
	a := newBuffAura(9999, 1, 1)
	mgr.RemoveAura(a, RemoveModeDefault, nil, 0, "")
}

func TestCheckProc_ProcChance100(t *testing.T) {
	owner := unit.NewUnit(1, "Target", 1000, 500)
	caster := unit.NewUnit(2, "Caster", 1000, 500)
	mgr := NewAuraManager(owner)

	const spellID uint32 = 5001
	procAura := newProcAura(spellID, caster.GUID, 100.0, 0)
	procAura.RemainingProcs = -1 // unlimited
	mgr.ApplyAura(procAura, nil, 0, "")

	results := mgr.CheckProc(ProcEventOnHit, nil, 0, "")

	if len(results) == 0 {
		t.Fatal("expected at least one proc result, got none")
	}

	found := false
	for _, r := range results {
		if r.Aura.SpellID == spellID && r.Triggered {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected proc aura with 100% chance to trigger")
	}
}

func TestCheckProc_ProcCharges(t *testing.T) {
	owner := unit.NewUnit(1, "Target", 1000, 500)
	caster := unit.NewUnit(2, "Caster", 1000, 500)
	mgr := NewAuraManager(owner)

	const spellID uint32 = 6001
	const procCharges int32 = 2

	procAura := newProcAura(spellID, caster.GUID, 100.0, procCharges)
	mgr.ApplyAura(procAura, nil, 0, "")

	tests := []struct {
		name        string
		procNumber  int
		wantTrigger bool
		wantHasAura bool
	}{
		{"1st proc triggers", 1, true, true},
		{"2nd proc triggers", 2, true, false},
		{"3rd proc: aura gone", 3, false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := mgr.CheckProc(ProcEventOnHit, nil, 0, "")

			if tt.wantTrigger && len(results) == 0 {
				t.Error("expected proc to trigger")
			}
			if !tt.wantTrigger && len(results) > 0 {
				t.Error("expected no proc result")
			}

			hasAura := mgr.HasAura(spellID)
			if hasAura != tt.wantHasAura {
				t.Errorf("HasAura(%d) = %v, want %v", spellID, hasAura, tt.wantHasAura)
			}
		})
	}
}

func TestHasAura_NotApplied(t *testing.T) {
	owner := unit.NewUnit(1, "Target", 1000, 500)
	mgr := NewAuraManager(owner)

	if mgr.HasAura(9999) {
		t.Error("expected false for aura that was never applied")
	}
}

func TestGetAura_NotApplied(t *testing.T) {
	owner := unit.NewUnit(1, "Target", 1000, 500)
	mgr := NewAuraManager(owner)

	got := mgr.GetAura(9999)
	if got != nil {
		t.Error("expected nil for aura that was never applied")
	}
}

func TestNewAuraManager(t *testing.T) {
	owner := unit.NewUnit(1, "Owner", 1000, 500)
	mgr := NewAuraManager(owner)

	if mgr == nil {
		t.Fatal("NewAuraManager returned nil")
	}
	if mgr.Owner.GUID != owner.GUID {
		t.Errorf("expected owner GUID %d, got %d", owner.GUID, mgr.Owner.GUID)
	}
	if len(mgr.Auras) != 0 {
		t.Errorf("expected empty auras map, got %d entries", len(mgr.Auras))
	}
}
