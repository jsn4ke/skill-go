package cooldown

import (
	"testing"
	"time"

	"skill-go/server/spelldef"
)

func TestNewSpellHistory(t *testing.T) {
	h := NewSpellHistory()
	if h == nil {
		t.Fatal("NewSpellHistory returned nil")
	}
	if len(h.cooldowns) != 0 {
		t.Errorf("expected empty cooldowns map, got %d entries", len(h.cooldowns))
	}
	if len(h.charges) != 0 {
		t.Errorf("expected empty charges map, got %d entries", len(h.charges))
	}
	if len(h.gcds) != 0 {
		t.Errorf("expected empty gcds map, got %d entries", len(h.gcds))
	}
	if len(h.lockouts) != 0 {
		t.Errorf("expected empty lockouts slice, got %d entries", len(h.lockouts))
	}
}

func TestAddCooldownAndGetRemaining(t *testing.T) {
	tests := []struct {
		name     string
		spellID  uint32
		duration int32
		category int32
		wantZero bool
	}{
		{"6 second cooldown", 1001, 6000, 0, false},
		{"1 second cooldown", 1002, 1000, 1, false},
		{"very long cooldown", 1003, 60000, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewSpellHistory()
			h.AddCooldown(tt.spellID, tt.duration, tt.category)

			remaining := h.GetCooldownRemaining(tt.spellID)
			if tt.wantZero && remaining != 0 {
				t.Errorf("expected 0 remaining, got %v", remaining)
			}
			if !tt.wantZero && remaining <= 0 {
				t.Errorf("expected remaining > 0, got %v", remaining)
			}
		})
	}
}

func TestGetCooldownRemaining_NotFound(t *testing.T) {
	h := NewSpellHistory()
	remaining := h.GetCooldownRemaining(9999)
	if remaining != 0 {
		t.Errorf("expected 0 for unknown spell, got %v", remaining)
	}
}

func TestConsumeCharge(t *testing.T) {
	h := NewSpellHistory()
	const spellID uint32 = 2001
	const maxCharges int32 = 3
	const recoveryMs int32 = 20000

	h.InitCharges(spellID, maxCharges, recoveryMs)

	tests := []struct {
		name          string
		consumptions  int
		wantRemaining int32
		wantSuccess   bool
	}{
		{"consume 1 of 3", 1, 2, true},
		{"consume 2 of 3", 2, 1, true},
		{"consume 3 of 3", 3, 0, true},
		{"overconsume (0 left)", 4, 0, false},
	}

	remaining := maxCharges
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Each test consumes from current state
			ok := h.ConsumeCharge(spellID)
			if ok != tt.wantSuccess {
				t.Errorf("ConsumeCharge() = %v, want %v", ok, tt.wantSuccess)
			}

			if ok {
				remaining--
			}

			got := h.GetChargeRemaining(spellID)
			if got != remaining {
				t.Errorf("GetChargeRemaining() = %d, want %d", got, remaining)
			}
		})
	}
}

func TestConsumeCharge_NoRecord(t *testing.T) {
	h := NewSpellHistory()
	ok := h.ConsumeCharge(9999)
	if ok {
		t.Error("expected false for non-existent charge record")
	}
}

func TestInitCharges(t *testing.T) {
	h := NewSpellHistory()
	const spellID uint32 = 2002
	const maxCharges int32 = 5
	const recoveryMs int32 = 10000

	h.InitCharges(spellID, maxCharges, recoveryMs)

	got := h.GetChargeRemaining(spellID)
	if got != maxCharges {
		t.Errorf("GetChargeRemaining() = %d, want %d", got, maxCharges)
	}
}

func TestGCD(t *testing.T) {
	h := NewSpellHistory()

	// Start GCD for category 1 with 1500ms duration
	h.StartGCD(1, 1500)

	tests := []struct {
		name     string
		category int32
		wantGCD  bool
	}{
		{"same category is on GCD", 1, true},
		{"different category is not on GCD", 2, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := h.IsOnGCD(tt.category)
			if got != tt.wantGCD {
				t.Errorf("IsOnGCD(%d) = %v, want %v", tt.category, got, tt.wantGCD)
			}
		})
	}
}

func TestGCD_NotStarted(t *testing.T) {
	h := NewSpellHistory()
	if h.IsOnGCD(1) {
		t.Error("expected no GCD for category that was never started")
	}
}

func TestSchoolLockout(t *testing.T) {
	h := NewSpellHistory()

	h.AddSchoolLockout(spelldef.SchoolMaskFire, 5000)

	tests := []struct {
		name       string
		schoolMask spelldef.SchoolMask
		wantLocked bool
	}{
		{"fire school is locked", spelldef.SchoolMaskFire, true},
		{"frost school is not locked", spelldef.SchoolMaskFrost, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := h.IsSchoolLocked(tt.schoolMask)
			if got != tt.wantLocked {
				t.Errorf("IsSchoolLocked(%d) = %v, want %v", tt.schoolMask, got, tt.wantLocked)
			}
		})
	}
}

func TestSchoolLockout_MultipleSchools(t *testing.T) {
	h := NewSpellHistory()

	combinedMask := spelldef.SchoolMaskFire | spelldef.SchoolMaskFrost
	h.AddSchoolLockout(combinedMask, 5000)

	if !h.IsSchoolLocked(spelldef.SchoolMaskFire) {
		t.Error("expected fire to be locked via combined mask")
	}
	if !h.IsSchoolLocked(spelldef.SchoolMaskFrost) {
		t.Error("expected frost to be locked via combined mask")
	}
	if h.IsSchoolLocked(spelldef.SchoolMaskArcane) {
		t.Error("expected arcane to not be locked")
	}
}

func TestCooldownModifiers(t *testing.T) {
	tests := []struct {
		name       string
		modifier   CooldownModifier
		baseMs     int32
		method     string // "cooldown" or "recovery"
		wantResult int32
		tolerance  int32 // allowed rounding error
	}{
		{
			name:       "haste 50% reduces cooldown to 4000",
			modifier:   HasteCooldownModifier{HastePercent: 50},
			baseMs:     6000,
			method:     "cooldown",
			wantResult: 4000,
		},
		{
			name:       "haste 50% reduces recovery to 4000",
			modifier:   HasteCooldownModifier{HastePercent: 50},
			baseMs:     6000,
			method:     "recovery",
			wantResult: 4000,
		},
		{
			name:       "haste 0% returns base unchanged",
			modifier:   HasteCooldownModifier{HastePercent: 0},
			baseMs:     6000,
			method:     "cooldown",
			wantResult: 6000,
		},
		{
			name:       "recovery speed 1.5 reduces 20000 to ~13333",
			modifier:   RecoverySpeedModifier{Speed: 1.5},
			baseMs:     20000,
			method:     "recovery",
			wantResult: 13333,
			tolerance:  1,
		},
		{
			name:       "recovery speed does not affect cooldown",
			modifier:   RecoverySpeedModifier{Speed: 1.5},
			baseMs:     6000,
			method:     "cooldown",
			wantResult: 6000,
		},
		{
			name:       "flat -500 reduces 6000 to 5500",
			modifier:   FlatCooldownModifier{FlatMs: -500},
			baseMs:     6000,
			method:     "cooldown",
			wantResult: 5500,
		},
		{
			name:       "flat +1000 increases 6000 to 7000",
			modifier:   FlatCooldownModifier{FlatMs: 1000},
			baseMs:     6000,
			method:     "cooldown",
			wantResult: 7000,
		},
		{
			name:       "recovery speed 0 returns base unchanged",
			modifier:   RecoverySpeedModifier{Speed: 0},
			baseMs:     20000,
			method:     "recovery",
			wantResult: 20000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got int32
			if tt.method == "cooldown" {
				got = tt.modifier.ModifyCooldown(tt.baseMs)
			} else {
				got = tt.modifier.ModifyRecovery(tt.baseMs)
			}

			diff := got - tt.wantResult
			if diff < 0 {
				diff = -diff
			}
			if diff > tt.tolerance {
				t.Errorf("result = %d, want %d (tolerance %d)", got, tt.wantResult, tt.tolerance)
			}
		})
	}
}

func TestOnHold(t *testing.T) {
	h := NewSpellHistory()
	const spellID uint32 = 4001

	h.OnHold(spellID, 3000)

	remaining := h.GetCooldownRemaining(spellID)
	if remaining <= 0 {
		t.Errorf("expected cooldown remaining > 0 after OnHold, got %v", remaining)
	}

	// Verify remaining is close to 3000ms (within 100ms tolerance for test timing)
	expected := 3000 * time.Millisecond
	diff := expected - remaining
	if diff < 0 {
		diff = -diff
	}
	if diff > 100*time.Millisecond {
		t.Errorf("remaining = %v, expected close to %v", remaining, expected)
	}
}

func TestIsReady(t *testing.T) {
	t.Run("ready when no cooldown exists", func(t *testing.T) {
		h := NewSpellHistory()
		if !h.IsReady(5001, spelldef.SchoolMaskFire) {
			t.Error("expected spell to be ready with no cooldown")
		}
	})

	t.Run("not ready after AddCooldown", func(t *testing.T) {
		h := NewSpellHistory()
		h.AddCooldown(5002, 6000, 0)
		if h.IsReady(5002, spelldef.SchoolMaskFire) {
			t.Error("expected spell not ready while on cooldown")
		}
	})

	t.Run("charges: ready with charges remaining", func(t *testing.T) {
		h := NewSpellHistory()
		const spellID uint32 = 5003
		h.InitCharges(spellID, 3, 20000)

		if !h.IsReady(spellID, spelldef.SchoolMaskFire) {
			t.Error("expected spell ready with 3 charges")
		}

		h.ConsumeCharge(spellID)
		if !h.IsReady(spellID, spelldef.SchoolMaskFire) {
			t.Error("expected spell ready with 2 charges remaining")
		}
	})

	t.Run("charges: not ready when depleted", func(t *testing.T) {
		h := NewSpellHistory()
		const spellID uint32 = 5004
		h.InitCharges(spellID, 3, 20000)

		h.ConsumeCharge(spellID)
		h.ConsumeCharge(spellID)
		h.ConsumeCharge(spellID)

		if h.IsReady(spellID, spelldef.SchoolMaskFire) {
			t.Error("expected spell not ready with 0 charges")
		}

		// Extra consume should fail
		if h.ConsumeCharge(spellID) {
			t.Error("expected false from ConsumeCharge with 0 charges")
		}
	})

	t.Run("not ready when school is locked", func(t *testing.T) {
		h := NewSpellHistory()
		h.AddSchoolLockout(spelldef.SchoolMaskFire, 5000)

		if h.IsReady(5005, spelldef.SchoolMaskFire) {
			t.Error("expected spell not ready when school is locked")
		}
	})

	t.Run("ready when different school is locked", func(t *testing.T) {
		h := NewSpellHistory()
		h.AddSchoolLockout(spelldef.SchoolMaskFire, 5000)

		if !h.IsReady(5006, spelldef.SchoolMaskFrost) {
			t.Error("expected frost spell ready when only fire is locked")
		}
	})

	t.Run("charges bypass cooldown", func(t *testing.T) {
		h := NewSpellHistory()
		const spellID uint32 = 5007
		h.InitCharges(spellID, 1, 20000)
		// Also add a cooldown -- charges should take priority in IsReady
		h.AddCooldown(spellID, 6000, 0)

		if !h.IsReady(spellID, spelldef.SchoolMaskFire) {
			t.Error("expected spell ready via charges even with cooldown present")
		}
	})
}
