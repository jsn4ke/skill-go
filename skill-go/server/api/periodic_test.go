package api

import (
	"testing"
	"time"

	"skill-go/server/aura"
	"skill-go/server/trace"
	"skill-go/server/unit"
)

func TestPeriodicDamage_TickInterval(t *testing.T) {
	target := unit.NewUnit(99, "DoTTarget", 10000, 0)
	initialHP := target.Health

	auraMgr := aura.NewAuraManager(target)
	a := &aura.Aura{
		SpellID:    38692,
		SourceName: "火球术",
		AuraType:   aura.AuraTypeDebuff,
		Duration:   8000,
		StackAmount: 1,
		Effects: []*aura.AuraEffect{
			{
				AuraType:      aura.AuraTypeDebuff,
				BaseAmount:    21,
				PeriodicTimer: 2000,
			},
		},
	}

	auraMgr.ApplyAura(a, nil, 38692, "火球术")

	// Set timer to 5 seconds ago (should trigger 2 ticks at t=2s and t=4s)
	app := a.Applications[len(a.Applications)-1]
	app.TimerStart = time.Now().Add(-5 * time.Second).UnixMilli()

	gl := &GameLoop{
		auraMgrs: map[uint64]*aura.AuraManager{target.GUID: auraMgr},
		recorder:  trace.NewFlowRecorder(),
		hub:       trace.NewStreamHub(10),
	}

	gl.handleAuraUpdate(Command{})

	eff := a.Effects[0]
	if eff.AppliedTicks != 2 {
		t.Errorf("expected 2 applied ticks, got %d", eff.AppliedTicks)
	}

	damageDealt := initialHP - target.Health
	if damageDealt != 42 {
		t.Errorf("expected 42 damage dealt (2 ticks * 21), got %d", damageDealt)
	}
}

func TestPeriodicDamage_ExpirationBeforeAllTicks(t *testing.T) {
	target := unit.NewUnit(99, "DoTTarget", 10000, 0)
	initialHP := target.Health

	auraMgr := aura.NewAuraManager(target)
	a := &aura.Aura{
		SpellID:    38692,
		SourceName: "火球术",
		AuraType:   aura.AuraTypeDebuff,
		Duration:   8000,
		StackAmount: 1,
		Effects: []*aura.AuraEffect{
			{
				AuraType:      aura.AuraTypeDebuff,
				BaseAmount:    21,
				PeriodicTimer: 2000,
			},
		},
	}

	auraMgr.ApplyAura(a, nil, 38692, "火球术")
	app := a.Applications[len(a.Applications)-1]
	app.TimerStart = time.Now().Add(-9 * time.Second).UnixMilli()

	gl := &GameLoop{
		auraMgrs: map[uint64]*aura.AuraManager{target.GUID: auraMgr},
		recorder:  trace.NewFlowRecorder(),
		hub:       trace.NewStreamHub(10),
	}

	gl.handleAuraUpdate(Command{})

	// 9s / 2s = 4 ticks (at 2,4,6,8), then aura expires at 8s
	eff := a.Effects[0]
	if eff.AppliedTicks != 4 {
		t.Errorf("expected 4 applied ticks before expiration, got %d", eff.AppliedTicks)
	}

	damageDealt := initialHP - target.Health
	if damageDealt != 84 {
		t.Errorf("expected 84 damage (4 ticks * 21), got %d", damageDealt)
	}

	if auraMgr.HasAura(38692) {
		t.Error("aura should have been expired and removed")
	}
}

func TestPeriodicDamage_NoPeriodicEffect(t *testing.T) {
	target := unit.NewUnit(99, "NoDotTarget", 10000, 0)
	initialHP := target.Health

	auraMgr := aura.NewAuraManager(target)
	a := &aura.Aura{
		SpellID:    5001,
		SourceName: "PlainBuff",
		AuraType:   aura.AuraTypeBuff,
		Duration:   5000,
		StackAmount: 1,
		Effects: []*aura.AuraEffect{
			{
				AuraType:      aura.AuraTypeBuff,
				BaseAmount:    40,
				PeriodicTimer: 0,
			},
		},
	}

	auraMgr.ApplyAura(a, nil, 5001, "PlainBuff")
	app := a.Applications[len(a.Applications)-1]
	app.TimerStart = time.Now().Add(-3 * time.Second).UnixMilli()

	gl := &GameLoop{
		auraMgrs: map[uint64]*aura.AuraManager{target.GUID: auraMgr},
		recorder:  trace.NewFlowRecorder(),
		hub:       trace.NewStreamHub(10),
	}

	gl.handleAuraUpdate(Command{})

	if target.Health != initialHP {
		t.Errorf("expected no damage for non-periodic aura, got %d HP change", initialHP-target.Health)
	}

	if !auraMgr.HasAura(5001) {
		t.Error("non-periodic aura should still exist before expiration")
	}
}
