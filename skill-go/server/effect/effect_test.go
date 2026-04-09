package effect

import (
	"testing"

	"skill-go/server/spelldef"
	"skill-go/server/trace"
	"skill-go/server/unit"
)

// mockCasterInfo implements CasterInfo for testing.
type mockCasterInfo struct {
	caster    *unit.Unit
	targets   []*unit.Unit
	trace     *trace.Trace
	spellID   uint32
	spellName string
}

func (m *mockCasterInfo) Caster() *unit.Unit       { return m.caster }
func (m *mockCasterInfo) Targets() []*unit.Unit     { return m.targets }
func (m *mockCasterInfo) GetTrace() *trace.Trace    { return m.trace }
func (m *mockCasterInfo) GetSpellID() uint32        { return m.spellID }
func (m *mockCasterInfo) GetSpellName() string      { return m.spellName }

func newCtx(caster *unit.Unit, targets []*unit.Unit) *mockCasterInfo {
	return &mockCasterInfo{
		caster:    caster,
		targets:   targets,
		trace:     trace.NewTrace(),
		spellID:   1,
		spellName: "TestSpell",
	}
}

func TestNewStore_DefaultHandlers(t *testing.T) {
	tests := []struct {
		name       string
		effectType spelldef.SpellEffectType
		wantLaunch bool
		wantHit    bool
	}{
		{
			name:       "SchoolDamage has launch handler by default",
			effectType: spelldef.SpellEffectSchoolDamage,
			wantLaunch: true,
			wantHit:    true,
		},
		{
			name:       "Heal has launch handler by default",
			effectType: spelldef.SpellEffectHeal,
			wantLaunch: true,
			wantHit:    true,
		},
		{
			name:       "Energize has no handler without RegisterExtended",
			effectType: spelldef.SpellEffectEnergize,
			wantLaunch: false,
			wantHit:    false,
		},
		{
			name:       "WeaponDamage has no handler without RegisterExtended",
			effectType: spelldef.SpellEffectWeaponDamage,
			wantLaunch: false,
			wantHit:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := NewStore()

			gotLaunch := store.GetLaunchHandler(tt.effectType) != nil
			gotHit := store.GetHitHandler(tt.effectType) != nil

			if gotLaunch != tt.wantLaunch {
				t.Errorf("GetLaunchHandler(%v) nil = %v, want %v", tt.effectType, gotLaunch, tt.wantLaunch)
			}
			if gotHit != tt.wantHit {
				t.Errorf("GetHitHandler(%v) nil = %v, want %v", tt.effectType, gotHit, tt.wantHit)
			}
		})
	}
}

func TestStore_CustomLaunchHandler(t *testing.T) {
	store := NewStore()

	var called bool
	var receivedEff spelldef.SpellEffectInfo

	store.RegisterLaunch(spelldef.SpellEffectSchoolDamage, func(ctx CasterInfo, eff spelldef.SpellEffectInfo) {
		called = true
		receivedEff = eff
	})

	caster := unit.NewUnit(1, "caster", 100, 200)
	ctx := newCtx(caster, nil)
	eff := spelldef.SpellEffectInfo{
		EffectIndex: 0,
		EffectType:  spelldef.SpellEffectSchoolDamage,
		BasePoints:  50,
	}

	handler := store.GetLaunchHandler(spelldef.SpellEffectSchoolDamage)
	handler(ctx, eff)

	if !called {
		t.Error("custom launch handler was not called")
	}
	if receivedEff.BasePoints != 50 {
		t.Errorf("launch handler received BasePoints = %d, want 50", receivedEff.BasePoints)
	}
}

func TestStore_SchoolDamageHit(t *testing.T) {
	store := NewStore()

	caster := unit.NewUnit(1, "caster", 100, 200)
	caster.HitSpell = 100.0 // ensure no miss
	target := unit.NewUnit(2, "target", 100, 200)
	ctx := newCtx(caster, []*unit.Unit{target})

	eff := spelldef.SpellEffectInfo{
		EffectIndex: 0,
		EffectType:  spelldef.SpellEffectSchoolDamage,
		BasePoints:  30,
	}

	handler := store.GetHitHandler(spelldef.SpellEffectSchoolDamage)
	if handler == nil {
		t.Fatal("GetHitHandler(SpellEffectSchoolDamage) = nil")
	}

	handler(ctx, eff, target)

	// With ±4% variance: expected ~30 damage, HP around 70
	if target.Health >= 100 {
		t.Error("target should have taken damage")
	}
	if target.Health < 50 || target.Health > 75 {
		t.Errorf("target Health = %d, expected ~70 (100 - 30 ±4%%)", target.Health)
	}
}

func TestStore_HealHit(t *testing.T) {
	store := NewStore()

	caster := unit.NewUnit(1, "caster", 100, 200)
	target := unit.NewUnit(2, "target", 50, 100)
	target.MaxHealth = 100
	target.Health = 50
	ctx := newCtx(caster, []*unit.Unit{target})

	eff := spelldef.SpellEffectInfo{
		EffectIndex: 0,
		EffectType:  spelldef.SpellEffectHeal,
		BasePoints:  30,
	}

	handler := store.GetHitHandler(spelldef.SpellEffectHeal)
	if handler == nil {
		t.Fatal("GetHitHandler(SpellEffectHeal) = nil")
	}

	handler(ctx, eff, target)

	if target.Health != 80 {
		t.Errorf("target Health = %d, want 80 (50 + 30)", target.Health)
	}
}

func TestStore_EnergizeHit(t *testing.T) {
	store := NewStore()
	RegisterExtended(store, nil, nil)

	caster := unit.NewUnit(1, "caster", 100, 100)
	target := unit.NewUnit(2, "target", 100, 50)
	target.MaxMana = 100
	target.Mana = 50
	ctx := newCtx(caster, []*unit.Unit{target})

	eff := spelldef.SpellEffectInfo{
		EffectIndex:    0,
		EffectType:     spelldef.SpellEffectEnergize,
		EnergizeType:   spelldef.PowerTypeMana,
		EnergizeAmount: 30,
	}

	handler := store.GetHitHandler(spelldef.SpellEffectEnergize)
	if handler == nil {
		t.Fatal("GetHitHandler(SpellEffectEnergize) = nil after RegisterExtended")
	}

	handler(ctx, eff, target)

	if target.Mana != 80 {
		t.Errorf("target Mana = %d, want 80 (50 + 30)", target.Mana)
	}
}

func TestStore_EnergizeHit_CappedAtMax(t *testing.T) {
	store := NewStore()
	RegisterExtended(store, nil, nil)

	caster := unit.NewUnit(1, "caster", 100, 100)
	target := unit.NewUnit(2, "target", 100, 100)
	ctx := newCtx(caster, []*unit.Unit{target})

	eff := spelldef.SpellEffectInfo{
		EffectIndex:    0,
		EffectType:     spelldef.SpellEffectEnergize,
		EnergizeType:   spelldef.PowerTypeMana,
		EnergizeAmount: 50,
	}

	handler := store.GetHitHandler(spelldef.SpellEffectEnergize)
	handler(ctx, eff, target)

	if target.Mana != 100 {
		t.Errorf("target Mana = %d, want 100 (capped at max)", target.Mana)
	}
}

func TestStore_WeaponDamageHit(t *testing.T) {
	store := NewStore()
	RegisterExtended(store, nil, nil)

	caster := unit.NewUnit(1, "caster", 100, 200)
	caster.SetWeaponDamage(100, 100) // fixed weapon damage of 100
	caster.HitMelee = 100.0          // cap hit to ensure no miss
	target := unit.NewUnit(2, "target", 200, 200)
	ctx := newCtx(caster, []*unit.Unit{target})

	eff := spelldef.SpellEffectInfo{
		EffectIndex:   0,
		EffectType:    spelldef.SpellEffectWeaponDamage,
		BasePoints:    10,
		WeaponPercent: 1.0,
	}

	handler := store.GetHitHandler(spelldef.SpellEffectWeaponDamage)
	if handler == nil {
		t.Fatal("GetHitHandler(SpellEffectWeaponDamage) = nil after RegisterExtended")
	}

	handler(ctx, eff, target)

	// weaponDamage=100 + basePoints=10 = 110 (with ±4% variance)
	// Health should be around 90
	if target.Health > 200 || target.Health <= 0 {
		t.Errorf("target Health = %d, want ~90 (200 - 110 ±4%%)", target.Health)
	}
	if target.Health >= 200 {
		t.Error("target should have taken damage")
	}
}

func TestStore_WeaponDamageHit_ZeroWeaponPercent(t *testing.T) {
	store := NewStore()
	RegisterExtended(store, nil, nil)

	caster := unit.NewUnit(1, "caster", 100, 200)
	caster.SetWeaponDamage(100, 100) // fixed weapon damage
	caster.HitMelee = 100.0
	target := unit.NewUnit(2, "target", 200, 200)
	ctx := newCtx(caster, []*unit.Unit{target})

	eff := spelldef.SpellEffectInfo{
		EffectIndex:   0,
		EffectType:    spelldef.SpellEffectWeaponDamage,
		BasePoints:    25,
		WeaponPercent: 0.0,
	}

	handler := store.GetHitHandler(spelldef.SpellEffectWeaponDamage)
	handler(ctx, eff, target)

	// weaponDamage=100 + basePoints=25 = 125 (±4% variance)
	if target.Health >= 200 {
		t.Error("target should have taken damage")
	}
	// Should be around 75
	if target.Health > 100 {
		t.Errorf("target Health = %d, expected ~75 (200 - 125 ±4%%)", target.Health)
	}
}

func TestStore_EnergizeLaunch(t *testing.T) {
	store := NewStore()
	RegisterExtended(store, nil, nil)

	handler := store.GetLaunchHandler(spelldef.SpellEffectEnergize)
	if handler == nil {
		t.Fatal("GetLaunchHandler(SpellEffectEnergize) = nil after RegisterExtended")
	}

	// Verify the handler does not panic.
	caster := unit.NewUnit(1, "caster", 100, 100)
	ctx := newCtx(caster, nil)
	eff := spelldef.SpellEffectInfo{
		EffectIndex:    0,
		EffectType:     spelldef.SpellEffectEnergize,
		EnergizeAmount: 20,
	}

	handler(ctx, eff)
	// If we get here without panic, the launch handler executed successfully.
}

func TestStore_WeaponDamageLaunch(t *testing.T) {
	store := NewStore()
	RegisterExtended(store, nil, nil)

	handler := store.GetLaunchHandler(spelldef.SpellEffectWeaponDamage)
	if handler == nil {
		t.Fatal("GetLaunchHandler(SpellEffectWeaponDamage) = nil after RegisterExtended")
	}

	// Verify the handler does not panic.
	caster := unit.NewUnit(1, "caster", 100, 100)
	ctx := newCtx(caster, nil)
	eff := spelldef.SpellEffectInfo{
		EffectIndex:   0,
		EffectType:    spelldef.SpellEffectWeaponDamage,
		BasePoints:    50,
		WeaponPercent: 1.5,
	}

	handler(ctx, eff)
}

func TestStore_SchoolDamageKillTarget(t *testing.T) {
	store := NewStore()

	caster := unit.NewUnit(1, "caster", 100, 200)
	target := unit.NewUnit(2, "target", 10, 100)
	ctx := newCtx(caster, []*unit.Unit{target})

	eff := spelldef.SpellEffectInfo{
		EffectIndex: 0,
		EffectType:  spelldef.SpellEffectSchoolDamage,
		BasePoints:  100,
	}

	handler := store.GetHitHandler(spelldef.SpellEffectSchoolDamage)
	handler(ctx, eff, target)

	if target.Alive {
		t.Error("target should be dead after taking lethal damage")
	}
	if target.Health != 0 {
		t.Errorf("target Health = %d, want 0", target.Health)
	}
}
