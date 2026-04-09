package cooldown

import (
	"log"
	"time"

	"skill-go/server/spelldef"
)

// --- Record types ---

// SpellCooldownRecord tracks a single spell cooldown.
type SpellCooldownRecord struct {
	SpellID   uint32
	StartTime time.Time
	Duration  time.Duration
	Category  int32
}

// ChargeRecord tracks charge-based recovery for a spell.
type ChargeRecord struct {
	MaxCharges      int32
	CurrentCharges  int32
	RecoveryDuration time.Duration
	RecoveryQueue   []time.Time // timestamps when each recovery completes
}

// GCDRecord tracks a global cooldown entry.
type GCDRecord struct {
	Category  int32
	StartTime time.Time
	Duration  time.Duration
}

// SchoolLockoutRecord tracks a school lockout (silence effect).
type SchoolLockoutRecord struct {
	SchoolMask spelldef.SchoolMask
	ExpireTime time.Time
}

// --- SpellHistory: central cooldown/charge/GCD/lockout manager ---

// SpellHistory manages all cooldown-related state for a unit.
type SpellHistory struct {
	cooldowns map[uint32]*SpellCooldownRecord
	charges   map[uint32]*ChargeRecord
	gcds      map[int32]*GCDRecord // keyed by recovery category
	lockouts  []*SchoolLockoutRecord
}

// NewSpellHistory creates an empty SpellHistory.
func NewSpellHistory() *SpellHistory {
	return &SpellHistory{
		cooldowns: make(map[uint32]*SpellCooldownRecord),
		charges:   make(map[uint32]*ChargeRecord),
		gcds:      make(map[int32]*GCDRecord),
	}
}

// --- Cooldown subsystem ---

// AddCooldown adds a cooldown for a spell.
func (h *SpellHistory) AddCooldown(spellID uint32, durationMs int32, category int32) {
	h.cooldowns[spellID] = &SpellCooldownRecord{
		SpellID:   spellID,
		StartTime: time.Now(),
		Duration:  time.Duration(durationMs) * time.Millisecond,
		Category:  category,
	}
	log.Printf("[Cooldown] added spell %d: %dms, category=%d", spellID, durationMs, category)
}

// GetCooldownRemaining returns the remaining cooldown time for a spell (0 if ready).
func (h *SpellHistory) GetCooldownRemaining(spellID uint32) time.Duration {
	rec, ok := h.cooldowns[spellID]
	if !ok {
		return 0
	}
	remaining := rec.Duration - time.Since(rec.StartTime)
	if remaining <= 0 {
		delete(h.cooldowns, spellID)
		return 0
	}
	return remaining
}

// --- Charge subsystem ---

// InitCharges initializes a charge record for a spell.
func (h *SpellHistory) InitCharges(spellID uint32, maxCharges int32, recoveryMs int32) {
	h.charges[spellID] = &ChargeRecord{
		MaxCharges:      maxCharges,
		CurrentCharges:  maxCharges,
		RecoveryDuration: time.Duration(recoveryMs) * time.Millisecond,
	}
	log.Printf("[Charges] initialized spell %d: %d charges, recovery=%dms", spellID, maxCharges, recoveryMs)
}

// ConsumeCharge consumes one charge of a spell. Returns false if no charges available.
func (h *SpellHistory) ConsumeCharge(spellID uint32) bool {
	rec, ok := h.charges[spellID]
	if !ok {
		return false
	}

	if rec.CurrentCharges <= 0 {
		return false
	}

	rec.CurrentCharges--
	log.Printf("[Charges] consumed spell %d: %d charges remaining", spellID, rec.CurrentCharges)

	// If not at max, queue a recovery
	if int32(len(rec.RecoveryQueue)) < (rec.MaxCharges - rec.CurrentCharges) {
		startTime := time.Now()
		if len(rec.RecoveryQueue) > 0 {
			// Serial recovery: next starts when previous ends
			lastEnd := rec.RecoveryQueue[len(rec.RecoveryQueue)-1]
			startTime = lastEnd.Add(rec.RecoveryDuration)
		}
		endTime := startTime.Add(rec.RecoveryDuration)
		rec.RecoveryQueue = append(rec.RecoveryQueue, endTime)
		log.Printf("[Charges] spell %d: recovery queued, ready at %v", spellID, endTime)
	}

	return true
}

// GetChargeRemaining returns the number of available charges.
func (h *SpellHistory) GetChargeRemaining(spellID uint32) int32 {
	rec, ok := h.charges[spellID]
	if !ok {
		return 0
	}
	return rec.CurrentCharges
}

// --- GCD subsystem ---

// StartGCD starts a GCD for a recovery category.
func (h *SpellHistory) StartGCD(category int32, durationMs int32) {
	h.gcds[category] = &GCDRecord{
		Category:  category,
		StartTime: time.Now(),
		Duration:  time.Duration(durationMs) * time.Millisecond,
	}
	log.Printf("[GCD] started category=%d: %dms", category, durationMs)
}

// IsOnGCD checks if a recovery category is on GCD.
func (h *SpellHistory) IsOnGCD(category int32) bool {
	rec, ok := h.gcds[category]
	if !ok {
		return false
	}
	remaining := rec.Duration - time.Since(rec.StartTime)
	if remaining <= 0 {
		delete(h.gcds, category)
		return false
	}
	return true
}

// --- School lockout subsystem ---

// AddSchoolLockout locks a school of magic for a duration.
func (h *SpellHistory) AddSchoolLockout(schoolMask spelldef.SchoolMask, durationMs int32) {
	h.lockouts = append(h.lockouts, &SchoolLockoutRecord{
		SchoolMask: schoolMask,
		ExpireTime: time.Now().Add(time.Duration(durationMs) * time.Millisecond),
	})
	log.Printf("[Lockout] added school=%d: %dms", schoolMask, durationMs)
}

// IsSchoolLocked checks if any school in the mask is locked.
func (h *SpellHistory) IsSchoolLocked(schoolMask spelldef.SchoolMask) bool {
	for _, lo := range h.lockouts {
		if lo.SchoolMask&schoolMask != 0 && time.Now().Before(lo.ExpireTime) {
			return true
		}
	}
	return false
}

// --- Update (tick) ---

// Update refreshes all expired records. Call periodically.
func (h *SpellHistory) Update() {
	now := time.Now()

	// Expire cooldowns
	for id, rec := range h.cooldowns {
		if now.Sub(rec.StartTime) >= rec.Duration {
			delete(h.cooldowns, id)
		}
	}

	// Expire GCDs
	for cat, rec := range h.gcds {
		if now.Sub(rec.StartTime) >= rec.Duration {
			delete(h.gcds, cat)
		}
	}

	// Process charge recoveries
	for id, rec := range h.charges {
		for len(rec.RecoveryQueue) > 0 && now.After(rec.RecoveryQueue[0]) {
			rec.RecoveryQueue = rec.RecoveryQueue[1:]
			if rec.CurrentCharges < rec.MaxCharges {
				rec.CurrentCharges++
				log.Printf("[Charges] spell %d: charge recovered, now %d", id, rec.CurrentCharges)
			}
		}
	}

	// Expire lockouts
	var active []*SchoolLockoutRecord
	for _, lo := range h.lockouts {
		if now.Before(lo.ExpireTime) {
			active = append(active, lo)
		}
	}
	h.lockouts = active
}

// --- Cooldown modifiers ---

// CooldownModifier modifies cooldown durations.
type CooldownModifier interface {
	ModifyCooldown(baseMs int32) int32
	ModifyRecovery(baseMs int32) int32
}

// HasteCooldownModifier reduces cooldown by haste percentage.
type HasteCooldownModifier struct {
	HastePercent float64
}

func (m HasteCooldownModifier) ModifyCooldown(baseMs int32) int32 {
	if m.HastePercent <= 0 {
		return baseMs
	}
	return int32(float64(baseMs) / (1.0 + m.HastePercent/100.0))
}

func (m HasteCooldownModifier) ModifyRecovery(baseMs int32) int32 {
	return m.ModifyCooldown(baseMs)
}

// RecoverySpeedModifier modifies charge recovery speed.
type RecoverySpeedModifier struct {
	Speed float64 // multiplier, e.g. 1.5 = 50% faster
}

func (m RecoverySpeedModifier) ModifyCooldown(baseMs int32) int32 {
	return baseMs
}

func (m RecoverySpeedModifier) ModifyRecovery(baseMs int32) int32 {
	if m.Speed <= 0 {
		return baseMs
	}
	return int32(float64(baseMs) / m.Speed)
}

// FlatCooldownModifier adds/subtracts flat ms from cooldowns.
type FlatCooldownModifier struct {
	FlatMs int32
}

func (m FlatCooldownModifier) ModifyCooldown(baseMs int32) int32 {
	return baseMs + m.FlatMs
}

func (m FlatCooldownModifier) ModifyRecovery(baseMs int32) int32 {
	return baseMs + m.FlatMs
}

// --- IsReady: comprehensive readiness check ---

// IsReady returns true if a spell can be cast (no CD, charges available, not on GCD, school not locked).
// Implements spell.SpellHistoryProvider.
func (h *SpellHistory) IsReady(spellID uint32, schoolMask spelldef.SchoolMask) bool {
	// Check charge-based first
	if rec, ok := h.charges[spellID]; ok {
		if rec.CurrentCharges <= 0 {
			return false
		}
		return true // charges bypass normal cooldown
	}

	// Check cooldown
	if h.GetCooldownRemaining(spellID) > 0 {
		return false
	}

	// Check school lockout
	if h.IsSchoolLocked(schoolMask) {
		return false
	}

	return true
}

// OnHold applies an extra cooldown when interrupted.
func (h *SpellHistory) OnHold(spellID uint32, durationMs int32) {
	h.AddCooldown(spellID, durationMs, 0)
	log.Printf("[OnHold] spell %d: added %dms hold cooldown", spellID, durationMs)
}
