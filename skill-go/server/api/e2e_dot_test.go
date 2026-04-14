package api

import (
	"testing"
	"time"

	"skill-go/server/aura"
	"skill-go/server/spelldef"
	"skill-go/server/trace"
	"skill-go/server/unit"
)

// TestE2E_FireballDoT verifies the full cast → aura → periodic damage pipeline
// by constructing the same objects as NewGameLoop + makeAuraHandler.
func TestE2E_FireballDoT(t *testing.T) {
	// Setup units
	caster := unit.NewUnit(1, "Mage", 5000, 20000)
	caster.SetLevel(60)
	target := unit.NewUnit(3, "Target Dummy", 15000, 0)
	target.SetLevel(63)
	target.Position = unit.Position{X: 30, Y: 0, Z: 0}

	targetInitialHP := target.Health

	// Setup aura manager (same as NewGameLoop)
	targetAuraMgr := aura.NewAuraManager(target)
	auraMgrs := map[uint64]*aura.AuraManager{target.GUID: targetAuraMgr}
	auraProvider := &simpleAuraProvider{managers: auraMgrs}

	// Build the aura exactly as makeAuraHandler would for Fireball effect[1]
	// spellId=38692, effectIndex=1, apply_aura, fire, value=21, tickInterval=2000, duration=8000
	fireballEffect := spelldef.SpellEffectInfo{
		EffectIndex:         1,
		EffectType:          spelldef.SpellEffectApplyAura,
		SchoolMask:          spelldef.SchoolMaskFire,
		BasePoints:          21,
	PeriodicTickInterval: 2000,
		AuraDuration:        8000,
	}

	// Apply aura through makeAuraHandler (same code path as real cast)
	auraHandler := makeAuraHandler(auraProvider)
	recorder := trace.NewFlowRecorder()
	fakeCtx := &testCasterInfo{
		spellID:   38692,
		spellName: "火球术",
		caster:    caster,
		targets:   []*unit.Unit{target},
		trace:     trace.NewTraceWithSinks(recorder),
	}
	auraHandler(fakeCtx, fireballEffect, target)

	// Verify aura was applied to target's aura manager
	a, ok := targetAuraMgr.Auras[9001] // SpellID = EffectIndex + 9000 = 9001
	if !ok {
		t.Fatal("aura not found in target's aura manager (key 9001)")
	}

	// Verify aura has correct fields
	if len(a.Effects) != 1 {
		t.Fatalf("expected 1 effect, got %d", len(a.Effects))
	}
	eff := a.Effects[0]
	t.Logf("Aura applied: spellID=%d, duration=%d, periodicTimer=%d, baseAmount=%d",
		a.SpellID, a.Duration, eff.PeriodicTimer, eff.BaseAmount)

	if eff.PeriodicTimer != 2000 {
		t.Errorf("expected PeriodicTimer=2000, got %d", eff.PeriodicTimer)
	}
	if eff.BaseAmount != 21 {
		t.Errorf("expected BaseAmount=21, got %d", eff.BaseAmount)
	}
	if eff.AppliedTicks != 0 {
		t.Errorf("expected AppliedTicks=0, got %d", eff.AppliedTicks)
	}

	// Now simulate time passing by manipulating TimerStart
	app := a.Applications[len(a.Applications)-1]
	if app == nil {
		t.Fatal("aura has no applications")
	}

	// Simulate 5 seconds passing
	app.TimerStart = time.Now().Add(-5 * time.Second).UnixMilli()

	// Run handleAuraUpdate (same as auraTicker would trigger)
	gl := &GameLoop{
		auraMgrs: auraMgrs,
		recorder:  trace.NewFlowRecorder(),
		hub:       trace.NewStreamHub(10),
	}
	gl.handleAuraUpdate(Command{})

	// Verify 2 ticks applied (at 2s and 4s)
	if eff.AppliedTicks != 2 {
		t.Errorf("expected 2 applied ticks after 5s, got %d", eff.AppliedTicks)
	}

	// Verify target took damage
	damageDealt := targetInitialHP - target.Health
	if damageDealt != 42 {
		t.Errorf("expected 42 damage (2 ticks * 21), got %d (HP: %d → %d)",
			damageDealt, targetInitialHP, target.Health)
	}
}

// testCasterInfo implements effect.CasterInfo for testing
type testCasterInfo struct {
	spellID   uint32
	spellName string
	caster    *unit.Unit
	targets   []*unit.Unit
	trace     *trace.Trace
}

func (c *testCasterInfo) GetSpellID() uint32      { return c.spellID }
func (c *testCasterInfo) GetSpellName() string    { return c.spellName }
func (c *testCasterInfo) Caster() *unit.Unit      { return c.caster }
func (c *testCasterInfo) Targets() []*unit.Unit    { return c.targets }
func (c *testCasterInfo) GetTrace() *trace.Trace  { return c.trace }
