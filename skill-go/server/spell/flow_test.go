package spell

import (
	"testing"
	"time"

	"skill-go/server/aura"
	"skill-go/server/cooldown"
	"skill-go/server/spelldef"
	"skill-go/server/trace"
	"skill-go/server/unit"
)

// ---------------------------------------------------------------------------
// Mock helpers
// ---------------------------------------------------------------------------

// mockCooldownProvider implements CooldownProvider for tests.
type mockCooldownProvider struct {
	addCooldownCalled bool
	startGDCCalled    bool
}

func (m *mockCooldownProvider) AddCooldown(_ uint32, _ int32, _ int32) {
	m.addCooldownCalled = true
}

func (m *mockCooldownProvider) StartGCD(_ int32, _ int32) {
	m.startGDCCalled = true
}

func (m *mockCooldownProvider) ConsumeCharge(_ uint32) bool {
	return false
}

func (m *mockCooldownProvider) GetChargeRemaining(_ uint32) int32 {
	return 0
}

// mockAuraProvider implements AuraProvider for tests.
type mockAuraProvider struct {
	managers map[uint64]*aura.AuraManager
}

func (m *mockAuraProvider) GetAuraManager(target interface{}) *aura.AuraManager {
	u, ok := target.(*unit.Unit)
	if !ok {
		return nil
	}
	return m.managers[u.GUID]
}

// tracingCooldownHistory wraps cooldown.SpellHistory with trace-aware methods,
// satisfying the TracingCooldownProvider interface.
type tracingCooldownHistory struct {
	*cooldown.SpellHistory
}

func (t *tracingCooldownHistory) TraceAddCooldown(spellID uint32, durationMs int32, category int32, tr *trace.Trace) {
	t.SpellHistory.TraceAddCooldown(spellID, durationMs, category, tr)
}

func (t *tracingCooldownHistory) TraceConsumeCharge(spellID uint32, tr *trace.Trace) bool {
	return t.SpellHistory.TraceConsumeCharge(spellID, tr)
}

func (t *tracingCooldownHistory) TraceStartGCD(category int32, durationMs int32, tr *trace.Trace) {
	t.SpellHistory.TraceStartGCD(category, durationMs, tr)
}

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// newSpellContextWithRecorder creates a SpellContext with a FlowRecorder-backed
// Trace (no stdout output). The caller should use recorder to assert events.
func newSpellContextWithRecorder(
	info *spelldef.SpellInfo,
	caster *unit.Unit,
	targets []*unit.Unit,
) (*SpellContext, *trace.FlowRecorder) {
	recorder := trace.NewFlowRecorder()
	tr := trace.NewTraceWithSinks(recorder)
	sc := New(info.ID, info, caster, targets)
	sc.Trace = tr
	return sc, recorder
}

// basicCasterAndTarget creates a living caster at the origin and a target at
// the given distance along the Y axis.
func basicCasterAndTarget(casterMana int32, targetDist float64) (*unit.Unit, *unit.Unit) {
	caster := unit.NewUnit(1, "Caster", 100, casterMana)
	caster.Position = unit.Position{X: 0, Y: 0, Z: 0}
	target := unit.NewUnit(2, "Target", 100, 100)
	target.Position = unit.Position{X: 0, Y: targetDist, Z: 0}
	return caster, target
}

// ---------------------------------------------------------------------------
// Test scenarios
// ---------------------------------------------------------------------------

// TestFlow_InstantCastNormal verifies the full event sequence for an instant
// cast spell: prepare -> state_change(Preparing) -> cast -> state_change(Launched)
// -> finish. Because CastTime=0, Prepare() calls Cast() then Finish()
// internally.
func TestFlow_InstantCastNormal(t *testing.T) {
	info := &spelldef.SpellInfo{
		ID:       100,
		Name:     "InstantFireball",
		CastTime: 0,
		Effects: []spelldef.SpellEffectInfo{
			{EffectIndex: 0, EffectType: spelldef.SpellEffectSchoolDamage},
		},
	}
	caster, target := basicCasterAndTarget(500, 5)
	targets := []*unit.Unit{target}

	sc, rec := newSpellContextWithRecorder(info, caster, targets)

	result := sc.Prepare()

	if result != spelldef.CastResultSuccess {
		t.Fatalf("expected CastResultSuccess, got %d (err=%d)", result, sc.LastCastErr)
	}

	// Verify the flow event sequence.
	events := rec.Events()
	if len(events) == 0 {
		t.Fatal("expected events, got none")
	}

	// 1. prepare
	if !rec.HasEvent(trace.SpanSpell, "prepare") {
		t.Error("missing prepare event")
	}

	// 2. state_change -> Preparing
	if !rec.HasEvent(trace.SpanSpell, "state_change") {
		t.Error("missing state_change(Preparing) event")
	}

	// 3. cast
	if !rec.HasEvent(trace.SpanSpell, "cast") {
		t.Error("missing cast event")
	}

	// 4. state_change -> Launched
	launchedChanges := 0
	for _, e := range rec.ByEvent("state_change") {
		if to, ok := e.Fields["to"]; ok && to == "Launched" {
			launchedChanges++
		}
	}
	if launchedChanges != 1 {
		t.Errorf("expected exactly 1 state_change to Launched, got %d", launchedChanges)
	}

	// 5. finish (immediate path calls Finish directly)
	if !rec.HasEvent(trace.SpanSpell, "finish") {
		t.Error("missing finish event")
	}

	// The default EffectStore registers a handler for SpellEffectSchoolDamage,
	// so launch and hit events are expected.
	if !rec.HasEvent(trace.SpanEffectLaunch, "launch") {
		t.Error("expected launch event (default handler registered)")
	}
	if !rec.HasEvent(trace.SpanEffectHit, "hit") {
		t.Error("expected hit event (default handler registered)")
	}
}

// TestFlow_CastFails_Silenced verifies that when the caster has the silenced
// unit state and the spell has PreventionTypeSilence, Prepare() fails and emits
// checkcast.failed(silenced) -> prepare_failed. No launch/hit/finish events.
func TestFlow_CastFails_Silenced(t *testing.T) {
	info := &spelldef.SpellInfo{
		ID:             101,
		Name:           "ArcaneBlast",
		CastTime:       0,
		PreventionType: spelldef.PreventionTypeSilence,
	}
	caster, target := basicCasterAndTarget(500, 5)
	caster.ApplyUnitState(spelldef.UnitStateSilenced)
	targets := []*unit.Unit{target}

	sc, rec := newSpellContextWithRecorder(info, caster, targets)

	result := sc.Prepare()

	if result != spelldef.CastResultFailed {
		t.Fatalf("expected CastResultFailed, got %d", result)
	}
	if sc.LastCastErr != spelldef.CastErrSilenced {
		t.Errorf("expected CastErrSilenced, got %d", sc.LastCastErr)
	}

	// Verify the trace events.
	if !rec.HasEvent(trace.SpanSpell, "prepare") {
		t.Error("missing prepare event")
	}

	// Check that checkcast.failed was emitted with reason=silenced.
	foundSilenced := false
	for _, e := range rec.ByEvent("failed") {
		if reason, ok := e.Fields["reason"]; ok && reason == "silenced" {
			foundSilenced = true
			break
		}
	}
	if !foundSilenced {
		t.Error("missing checkcast.failed(reason=silenced) event")
	}

	if !rec.HasEvent(trace.SpanSpell, "prepare_failed") {
		t.Error("missing prepare_failed event")
	}

	// No launch/hit/finish events should be present.
	if rec.HasEvent(trace.SpanSpell, "cast") {
		t.Error("unexpected cast event")
	}
	if rec.HasEvent(trace.SpanEffectLaunch, "launch") {
		t.Error("unexpected launch event")
	}
	if rec.HasEvent(trace.SpanEffectHit, "hit") {
		t.Error("unexpected hit event")
	}
	if rec.HasEvent(trace.SpanSpell, "finish") {
		t.Error("unexpected finish event")
	}
}

// TestFlow_CastFails_OnCooldown verifies that when a spell is on cooldown,
// Prepare() fails with checkcast.failed(not_ready) -> prepare_failed.
func TestFlow_CastFails_OnCooldown(t *testing.T) {
	spellID := uint32(102)
	info := &spelldef.SpellInfo{
		ID:       spellID,
		Name:     "CooldownSpell",
		CastTime: 0,
	}
	caster, target := basicCasterAndTarget(500, 5)
	targets := []*unit.Unit{target}

	sc, rec := newSpellContextWithRecorder(info, caster, targets)

	// Add a cooldown so the spell is not ready.
	history := cooldown.NewSpellHistory()
	history.AddCooldown(spellID, 5000, 0)
	sc.HistoryProvider = history

	result := sc.Prepare()

	if result != spelldef.CastResultFailed {
		t.Fatalf("expected CastResultFailed, got %d", result)
	}
	if sc.LastCastErr != spelldef.CastErrNotReady {
		t.Errorf("expected CastErrNotReady, got %d", sc.LastCastErr)
	}

	if !rec.HasEvent(trace.SpanSpell, "prepare") {
		t.Error("missing prepare event")
	}

	foundNotReady := false
	for _, e := range rec.ByEvent("failed") {
		if reason, ok := e.Fields["reason"]; ok && reason == "not_ready" {
			foundNotReady = true
			break
		}
	}
	if !foundNotReady {
		t.Error("missing checkcast.failed(reason=not_ready) event")
	}

	if !rec.HasEvent(trace.SpanSpell, "prepare_failed") {
		t.Error("missing prepare_failed event")
	}

	// No launch/hit events.
	if rec.HasEvent(trace.SpanSpell, "cast") {
		t.Error("unexpected cast event")
	}
	if rec.HasEvent(trace.SpanEffectLaunch, "launch") {
		t.Error("unexpected launch event")
	}
	if rec.HasEvent(trace.SpanEffectHit, "hit") {
		t.Error("unexpected hit event")
	}
}

// TestFlow_CastFails_OutOfRange verifies that when the target is beyond
// RangeMax, Prepare() fails with checkcast.failed(out_of_range) -> prepare_failed.
func TestFlow_CastFails_OutOfRange(t *testing.T) {
	info := &spelldef.SpellInfo{
		ID:       103,
		Name:     "ShortRangeSpell",
		CastTime: 0,
		RangeMax: 5,
	}
	caster, target := basicCasterAndTarget(500, 20)
	targets := []*unit.Unit{target}

	sc, rec := newSpellContextWithRecorder(info, caster, targets)

	result := sc.Prepare()

	if result != spelldef.CastResultFailed {
		t.Fatalf("expected CastResultFailed, got %d", result)
	}
	if sc.LastCastErr != spelldef.CastErrOutOfRange {
		t.Errorf("expected CastErrOutOfRange, got %d", sc.LastCastErr)
	}

	if !rec.HasEvent(trace.SpanSpell, "prepare") {
		t.Error("missing prepare event")
	}

	foundOutOfRange := false
	for _, e := range rec.ByEvent("failed") {
		if reason, ok := e.Fields["reason"]; ok && reason == "out_of_range" {
			foundOutOfRange = true
			break
		}
	}
	if !foundOutOfRange {
		t.Error("missing checkcast.failed(reason=out_of_range) event")
	}

	if !rec.HasEvent(trace.SpanSpell, "prepare_failed") {
		t.Error("missing prepare_failed event")
	}

	// No launch/hit events.
	if rec.HasEvent(trace.SpanSpell, "cast") {
		t.Error("unexpected cast event")
	}
	if rec.HasEvent(trace.SpanEffectLaunch, "launch") {
		t.Error("unexpected launch event")
	}
	if rec.HasEvent(trace.SpanEffectHit, "hit") {
		t.Error("unexpected hit event")
	}
}

// TestFlow_CastCancelled verifies that calling Cancel() during a non-instant
// cast produces: prepare -> state_change(Preparing) -> cancel ->
// state_change(Finished, reason=cancelled). No launch/hit events.
func TestFlow_CastCancelled(t *testing.T) {
	info := &spelldef.SpellInfo{
		ID:        104,
		Name:      "LongCastSpell",
		CastTime:  3000,
		PowerCost: 50,
	}
	caster, target := basicCasterAndTarget(200, 5)
	targets := []*unit.Unit{target}

	sc, rec := newSpellContextWithRecorder(info, caster, targets)

	// Prepare() succeeds for non-instant cast (state transitions to Preparing).
	result := sc.Prepare()
	if result != spelldef.CastResultSuccess {
		t.Fatalf("expected Prepare() to succeed for 3000ms cast, got %d (err=%d)", result, sc.LastCastErr)
	}
	if sc.State != StatePreparing {
		t.Fatalf("expected StatePreparing, got %s", sc.State)
	}

	// Cancel the cast.
	sc.Cancel()

	if sc.State != StateFinished {
		t.Errorf("expected StateFinished after cancel, got %s", sc.State)
	}
	if !sc.Cancelled {
		t.Error("expected Cancelled to be true")
	}

	// Verify trace events.
	if !rec.HasEvent(trace.SpanSpell, "prepare") {
		t.Error("missing prepare event")
	}
	if !rec.HasEvent(trace.SpanSpell, "state_change") {
		t.Error("missing state_change event")
	}
	if !rec.HasEvent(trace.SpanSpell, "cancel") {
		t.Error("missing cancel event")
	}

	// Verify the cancel reason in state_change.
	foundCancelledReason := false
	for _, e := range rec.ByEvent("state_change") {
		if reason, ok := e.Fields["reason"]; ok && reason == "cancelled" {
			foundCancelledReason = true
			break
		}
	}
	if !foundCancelledReason {
		t.Error("missing state_change with reason=cancelled")
	}

	// No launch/hit events should be present.
	if rec.HasEvent(trace.SpanSpell, "cast") {
		t.Error("unexpected cast event")
	}
	if rec.HasEvent(trace.SpanEffectLaunch, "launch") {
		t.Error("unexpected launch event")
	}
	if rec.HasEvent(trace.SpanEffectHit, "hit") {
		t.Error("unexpected hit event")
	}
	if rec.HasEvent(trace.SpanSpell, "finish") {
		t.Error("unexpected finish event (cancel produces state_change, not finish)")
	}
}

// TestFlow_DelayedHit verifies the delayed hit path: prepare -> cast ->
// delayed_hit_path -> launch -> delayed_hit_arrived -> hit -> finish.
// Uses a short DelayMs (50ms) and calls WaitDelayedHits() to block until
// the delayed hits complete.
func TestFlow_DelayedHit(t *testing.T) {
	info := &spelldef.SpellInfo{
		ID:       105,
		Name:     "DelayedFireball",
		CastTime: 0,
		DelayMs:  50,
		Effects: []spelldef.SpellEffectInfo{
			{EffectIndex: 0, EffectType: spelldef.SpellEffectSchoolDamage},
		},
	}
	caster, target := basicCasterAndTarget(500, 5)
	targets := []*unit.Unit{target}

	sc, rec := newSpellContextWithRecorder(info, caster, targets)

	// For instant cast (CastTime=0), Prepare() calls Cast() which enters the
	// delayed hit path.
	result := sc.Prepare()
	if result != spelldef.CastResultSuccess {
		t.Fatalf("expected CastResultSuccess, got %d (err=%d)", result, sc.LastCastErr)
	}

	// Process delayed hits (event loop would do this in production).
	for _, sched := range sc.GetDelayedHitSchedules() {
		if sched.Delay > 0 {
			time.Sleep(sched.Delay)
		}
		sc.ExecuteHit(sched.Target, sched.Eff)
		sc.TriggerHitProc(sched.Target)
	}
	sc.Finish()

	// Verify the event sequence.
	if !rec.HasEvent(trace.SpanSpell, "prepare") {
		t.Error("missing prepare event")
	}
	if !rec.HasEvent(trace.SpanSpell, "cast") {
		t.Error("missing cast event")
	}
	if !rec.HasEvent(trace.SpanSpell, "delayed_hit_path") {
		t.Error("missing delayed_hit_path event")
	}

	// state_change to Launched: Cast() emits one, and startDelayedHit() emits
	// another (with pending_hits). Expect at least 1.
	launchedChanges := 0
	for _, e := range rec.ByEvent("state_change") {
		if to, ok := e.Fields["to"]; ok && to == "Launched" {
			launchedChanges++
		}
	}
	if launchedChanges < 1 {
		t.Errorf("expected at least 1 state_change to Launched, got %d", launchedChanges)
	}

	if !rec.HasEvent(trace.SpanSpell, "delayed_hit_arrived") {
		t.Error("missing delayed_hit_arrived event")
	}

	// The default EffectStore registers a handler for SpellEffectSchoolDamage,
	// so hit events are expected from the delayed hit execution.
	if !rec.HasEvent(trace.SpanEffectHit, "hit") {
		t.Error("expected hit event (default handler registered for SpellEffectSchoolDamage)")
	}

	// Finish was called explicitly above.
	if !rec.HasEvent(trace.SpanSpell, "finish") {
		t.Error("missing finish event")
	}
}

// TestFlow_ManaRefund verifies that when a spell has PowerCost > 0, Prepare()
// succeeds and emits a mana_consumed event. It also verifies the mana amount
// was deducted.
func TestFlow_ManaRefund(t *testing.T) {
	const powerCost int32 = 100
	info := &spelldef.SpellInfo{
		ID:        106,
		Name:      "ExpensiveSpell",
		CastTime:  0,
		PowerCost: powerCost,
	}
	caster, target := basicCasterAndTarget(500, 5)
	targets := []*unit.Unit{target}

	sc, rec := newSpellContextWithRecorder(info, caster, targets)

	initialMana := caster.Mana
	result := sc.Prepare()

	if result != spelldef.CastResultSuccess {
		t.Fatalf("expected CastResultSuccess, got %d (err=%d)", result, sc.LastCastErr)
	}

	// Verify mana was consumed.
	if caster.Mana != initialMana-powerCost {
		t.Errorf("mana after cast = %d, want %d", caster.Mana, initialMana-powerCost)
	}

	// Verify mana_consumed event was emitted.
	if !rec.HasEvent(trace.SpanSpell, "mana_consumed") {
		t.Error("missing mana_consumed event")
	}

	// Verify the amount in the event.
	found := false
	for _, e := range rec.ByEvent("mana_consumed") {
		if amount, ok := e.Fields["amount"]; ok && amount == int32(powerCost) {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("mana_consumed event missing amount=%d", powerCost)
	}
}

// TestFlow_ProcTriggered verifies that when a target has a proc aura with 100%
// proc chance and the spell hits, the proc.check event appears in the trace.
func TestFlow_ProcTriggered(t *testing.T) {
	procAuraSpellID := uint32(9001)
	info := &spelldef.SpellInfo{
		ID:       107,
		Name:     "TriggerProcSpell",
		CastTime: 0,
		Effects: []spelldef.SpellEffectInfo{
			{EffectIndex: 0, EffectType: spelldef.SpellEffectSchoolDamage},
		},
	}
	caster, target := basicCasterAndTarget(500, 5)
	targets := []*unit.Unit{target}

	// Set up aura manager on the target with a 100% proc aura.
	targetAuraMgr := aura.NewAuraManager(target)
	procAura := &aura.Aura{
		SpellID:        procAuraSpellID,
		CasterGUID:     caster.GUID,
		Caster:         caster,
		AuraType:       aura.AuraTypeProc,
		Duration:       30000,
		ProcChance:     100.0,
		ProcCharges:    0,
		RemainingProcs: -1, // unlimited
		Effects: []*aura.AuraEffect{
			{
				AuraType:    aura.AuraTypeProc,
				SpellID:     procAuraSpellID,
				EffectIndex: 0,
			},
		},
	}
	applyTrace := trace.NewTrace()
	targetAuraMgr.ApplyAura(procAura, applyTrace, info.ID, info.Name)

	auraProvider := &mockAuraProvider{
		managers: map[uint64]*aura.AuraManager{target.GUID: targetAuraMgr},
	}

	sc, rec := newSpellContextWithRecorder(info, caster, targets)
	sc.AuraProvider = auraProvider

	result := sc.Prepare()

	if result != spelldef.CastResultSuccess {
		t.Fatalf("expected CastResultSuccess, got %d (err=%d)", result, sc.LastCastErr)
	}

	// The proc.check event should have been emitted during triggerHitProc.
	if !rec.HasEvent(trace.SpanProc, "check") {
		t.Error("missing proc.check event")
	}

	// The proc should have triggered (100% chance).
	if !rec.HasEvent(trace.SpanProc, "triggered") {
		t.Error("missing proc.triggered event (expected 100% proc chance to trigger)")
	}
}
