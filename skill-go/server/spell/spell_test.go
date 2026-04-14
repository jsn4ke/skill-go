package spell

import (
	"testing"
	"time"

	"skill-go/server/spelldef"
	"skill-go/server/unit"
)

// --- State Machine Transitions ---

func TestNewSpellContext_InitialStateIsNone(t *testing.T) {
	info := &spelldef.SpellInfo{ID: 1, Name: "TestSpell"}
	caster := unit.NewUnit(1, "Caster", 100, 100)
	targets := []*unit.Unit{unit.NewUnit(2, "Target", 100, 100)}

	sc := New(1, info, caster, targets)

	if sc.State != StateNone {
		t.Errorf("expected StateNone, got %s", sc.State)
	}
}

func TestInstantCastTransition_NoneToLaunchedToFinished(t *testing.T) {
	// Instant cast (CastTime=0) should skip Preparing and go directly to
	// Launched then Finished in a single Prepare() call.
	info := &spelldef.SpellInfo{
		ID:       1,
		Name:     "InstantSpell",
		CastTime: 0,
	}
	caster := unit.NewUnit(1, "Caster", 100, 100)
	caster.Position = unit.Position{X: 0, Y: 0, Z: 0}
	target := unit.NewUnit(2, "Target", 100, 100)
	target.Position = unit.Position{X: 0, Y: 5, Z: 0}
	targets := []*unit.Unit{target}

	sc := New(1, info, caster, targets)

	if sc.State != StateNone {
		t.Fatalf("initial state: expected StateNone, got %s", sc.State)
	}

	// Prepare() for instant cast calls Cast() internally which calls Finish().
	result := sc.Prepare()

	if result != spelldef.CastResultSuccess {
		t.Errorf("Prepare() result: expected CastResultSuccess, got %d", result)
	}

	// Instant cast: Prepare -> Cast -> Finish, so final state is Finished.
	if sc.State != StateFinished {
		t.Errorf("final state: expected StateFinished, got %s", sc.State)
	}
}

func TestCastTimeSpellTransition_PreparingState(t *testing.T) {
	// A spell with CastTime > 0 should transition to Preparing.
	info := &spelldef.SpellInfo{
		ID:       2,
		Name:     "Fireball",
		CastTime: 3000,
	}
	caster := unit.NewUnit(1, "Caster", 100, 100)
	caster.Position = unit.Position{X: 0, Y: 0, Z: 0}
	target := unit.NewUnit(2, "Target", 100, 100)
	target.Position = unit.Position{X: 0, Y: 5, Z: 0}
	targets := []*unit.Unit{target}

	sc := New(2, info, caster, targets)

	result := sc.Prepare()

	if result != spelldef.CastResultSuccess {
		t.Errorf("Prepare() result: expected success, got %d (err=%d)", result, sc.LastCastErr)
	}

	// With cast time > 0, Prepare should leave us in Preparing state.
	if sc.State != StatePreparing {
		t.Errorf("state after Prepare: expected StatePreparing, got %s", sc.State)
	}

	if sc.CastDuration == 0 {
		t.Error("expected non-zero CastDuration for a 3000ms spell")
	}
}

func TestPrepare_DeadCasterFails(t *testing.T) {
	info := &spelldef.SpellInfo{ID: 3, Name: "TestSpell", CastTime: 0}
	caster := unit.NewUnit(1, "Caster", 100, 100)
	caster.TakeDamage(100) // kill the caster
	targets := []*unit.Unit{unit.NewUnit(2, "Target", 100, 100)}

	sc := New(3, info, caster, targets)
	result := sc.Prepare()

	if result != spelldef.CastResultFailed {
		t.Errorf("expected CastResultFailed for dead caster, got %d", result)
	}
	if sc.LastCastErr != spelldef.CastErrDead {
		t.Errorf("expected CastErrDead, got %d", sc.LastCastErr)
	}
	if sc.State != StateFinished {
		t.Errorf("expected StateFinished, got %s", sc.State)
	}
}

func TestCancel_FromPreparingState(t *testing.T) {
	info := &spelldef.SpellInfo{
		ID:        4,
		Name:      "LongCast",
		CastTime:  5000,
		PowerCost: 50,
	}
	caster := unit.NewUnit(1, "Caster", 100, 200)
	caster.Position = unit.Position{X: 0, Y: 0, Z: 0}
	target := unit.NewUnit(2, "Target", 100, 100)
	target.Position = unit.Position{X: 0, Y: 5, Z: 0}
	targets := []*unit.Unit{target}

	sc := New(4, info, caster, targets)
	sc.Prepare()

	if sc.State != StatePreparing {
		t.Fatalf("expected StatePreparing before cancel, got %s", sc.State)
	}

	manaBefore := caster.Mana
	sc.Cancel()

	if sc.State != StateFinished {
		t.Errorf("expected StateFinished after cancel, got %s", sc.State)
	}
	if !sc.Cancelled {
		t.Error("expected Cancelled to be true")
	}
	// Mana should be refunded on cancel.
	if caster.Mana <= manaBefore {
		t.Errorf("expected mana refund after cancel, manaBefore=%d, manaAfter=%d", manaBefore, caster.Mana)
	}
}

// --- CheckCast Validation ---

func TestCheckCast(t *testing.T) {
	tests := []struct {
		name    string
		info    *spelldef.SpellInfo
		setup   func(caster *unit.Unit, targets []*unit.Unit)
		wantErr spelldef.CastError
	}{
		{
			name: "pass with no restrictions",
			info: &spelldef.SpellInfo{
				ID:             10,
				Name:           "SimpleSpell",
				PreventionType: spelldef.PreventionTypeNone,
				RangeMax:       40,
			},
			setup: func(caster *unit.Unit, targets []*unit.Unit) {
				caster.Position = unit.Position{X: 0, Y: 0, Z: 0}
				targets[0].Position = unit.Position{X: 0, Y: 20, Z: 0}
			},
			wantErr: spelldef.CastErrNone,
		},
		{
			name: "silenced caster fails for silence-prevented spell",
			info: &spelldef.SpellInfo{
				ID:             11,
				Name:           "ArcaneBlast",
				PreventionType: spelldef.PreventionTypeSilence,
			},
			setup: func(caster *unit.Unit, targets []*unit.Unit) {
				caster.ApplyUnitState(spelldef.UnitStateSilenced)
			},
			wantErr: spelldef.CastErrSilenced,
		},
		{
			name: "silenced caster passes for non-silence spell",
			info: &spelldef.SpellInfo{
				ID:             12,
				Name:           "PhysicalStrike",
				PreventionType: spelldef.PreventionTypeNone,
			},
			setup: func(caster *unit.Unit, targets []*unit.Unit) {
				caster.ApplyUnitState(spelldef.UnitStateSilenced)
			},
			wantErr: spelldef.CastErrNone,
		},
		{
			name: "target out of range fails",
			info: &spelldef.SpellInfo{
				ID:       13,
				Name:     "ShortRangeSpell",
				RangeMax: 10,
			},
			setup: func(caster *unit.Unit, targets []*unit.Unit) {
				caster.Position = unit.Position{X: 0, Y: 0, Z: 0}
				targets[0].Position = unit.Position{X: 0, Y: 50, Z: 0}
			},
			wantErr: spelldef.CastErrOutOfRange,
		},
		{
			name: "target within range passes",
			info: &spelldef.SpellInfo{
				ID:       14,
				Name:     "MediumRangeSpell",
				RangeMax: 30,
			},
			setup: func(caster *unit.Unit, targets []*unit.Unit) {
				caster.Position = unit.Position{X: 0, Y: 0, Z: 0}
				targets[0].Position = unit.Position{X: 0, Y: 15, Z: 0}
			},
			wantErr: spelldef.CastErrNone,
		},
		{
			name: "no range restriction passes at any distance",
			info: &spelldef.SpellInfo{
				ID:       15,
				Name:     "NoRangeSpell",
				RangeMax: 0,
			},
			setup: func(caster *unit.Unit, targets []*unit.Unit) {
				caster.Position = unit.Position{X: 0, Y: 0, Z: 0}
				targets[0].Position = unit.Position{X: 0, Y: 1000, Z: 0}
			},
			wantErr: spelldef.CastErrNone,
		},
		{
			name: "disarmed caster fails for pacify-prevented spell",
			info: &spelldef.SpellInfo{
				ID:             16,
				Name:           "MeleeAttack",
				PreventionType: spelldef.PreventionTypePacify,
			},
			setup: func(caster *unit.Unit, targets []*unit.Unit) {
				caster.ApplyUnitState(spelldef.UnitStateDisarmed)
			},
			wantErr: spelldef.CastErrDisarmed,
		},
		{
			name: "wrong shapeshift form fails",
			info: &spelldef.SpellInfo{
				ID:                     17,
				Name:                   "BearFormSpell",
				RequiresShapeshiftMask: 1, // form 1 required
			},
			setup: func(caster *unit.Unit, targets []*unit.Unit) {
				caster.CurrentForm = 2 // wrong form
			},
			wantErr: spelldef.CastErrShapeshifted,
		},
		{
			name: "correct shapeshift form passes",
			info: &spelldef.SpellInfo{
				ID:                     18,
				Name:                   "BearFormSpell",
				RequiresShapeshiftMask: 1,
			},
			setup: func(caster *unit.Unit, targets []*unit.Unit) {
				caster.CurrentForm = 1
			},
			wantErr: spelldef.CastErrNone,
		},
		{
			name: "area restriction fails when area does not match",
			info: &spelldef.SpellInfo{
				ID:             19,
				Name:           "ZoneSpecificSpell",
				RequiredAreaID: 123,
			},
			setup: func(caster *unit.Unit, targets []*unit.Unit) {
				// No area setup needed - RequiredAreaID != 0 always fails in current impl.
			},
			wantErr: spelldef.CastErrWrongArea,
		},
		{
			name: "mounted caster fails for spells with cast time",
			info: &spelldef.SpellInfo{
				ID:       20,
				Name:     "CastWhileMounted",
				CastTime: 2000,
			},
			setup: func(caster *unit.Unit, targets []*unit.Unit) {
				caster.MountID = 1
			},
			wantErr: spelldef.CastErrMounted,
		},
		{
			name: "mounted caster passes for instant cast",
			info: &spelldef.SpellInfo{
				ID:       21,
				Name:     "InstantWhileMounted",
				CastTime: 0,
			},
			setup: func(caster *unit.Unit, targets []*unit.Unit) {
				caster.MountID = 1
			},
			wantErr: spelldef.CastErrNone,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			caster := unit.NewUnit(1, "Caster", 100, 100)
			target := unit.NewUnit(2, "Target", 100, 100)
			targets := []*unit.Unit{target}

			if tt.setup != nil {
				tt.setup(caster, targets)
			}

			err := CheckCast(tt.info, caster, targets, nil, nil)
			if err != tt.wantErr {
				t.Errorf("CheckCast() error = %d, want %d", err, tt.wantErr)
			}
		})
	}
}

// --- ModifierChain ---

func TestHasteModifier(t *testing.T) {
	tests := []struct {
		name         string
		hastePercent float64
		baseMs       int32
		wantMs       int32
	}{
		{
			name:         "50% haste on 3000ms gives 2000ms",
			hastePercent: 50,
			baseMs:       3000,
			wantMs:       2000,
		},
		{
			name:         "100% haste on 3000ms gives 1500ms",
			hastePercent: 100,
			baseMs:       3000,
			wantMs:       1500,
		},
		{
			name:         "0% haste returns base unchanged",
			hastePercent: 0,
			baseMs:       3000,
			wantMs:       3000,
		},
		{
			name:         "negative haste returns base unchanged",
			hastePercent: -10,
			baseMs:       3000,
			wantMs:       3000,
		},
		{
			name:         "25% haste on 2000ms gives 1600ms",
			hastePercent: 25,
			baseMs:       2000,
			wantMs:       1600,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := HasteModifier{HastePercent: tt.hastePercent}
			got := m.Modify(tt.baseMs)
			if got != tt.wantMs {
				t.Errorf("HasteModifier{%.0f}.Modify(%d) = %d, want %d", tt.hastePercent, tt.baseMs, got, tt.wantMs)
			}
		})
	}
}

func TestFlatModifier(t *testing.T) {
	tests := []struct {
		name   string
		flatMs int32
		baseMs int32
		wantMs int32
	}{
		{
			name:   "subtract 1000ms from 3000ms gives 2000ms",
			flatMs: -1000,
			baseMs: 3000,
			wantMs: 2000,
		},
		{
			name:   "add 500ms to 1000ms gives 1500ms",
			flatMs: 500,
			baseMs: 1000,
			wantMs: 1500,
		},
		{
			name:   "zero flat modifier returns base unchanged",
			flatMs: 0,
			baseMs: 3000,
			wantMs: 3000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := FlatModifier{FlatMs: tt.flatMs}
			got := m.Modify(tt.baseMs)
			if got != tt.wantMs {
				t.Errorf("FlatModifier{%d}.Modify(%d) = %d, want %d", tt.flatMs, tt.baseMs, got, tt.wantMs)
			}
		})
	}
}

func TestModifierChain(t *testing.T) {
	tests := []struct {
		name   string
		chain  ModifierChain
		baseMs int32
		wantMs int32
	}{
		{
			name:   "empty chain returns base unchanged",
			chain:  ModifierChain{},
			baseMs: 3000,
			wantMs: 3000,
		},
		{
			name:   "single HasteModifier(50) on 3000ms gives 2000ms",
			chain:  ModifierChain{HasteModifier{HastePercent: 50}},
			baseMs: 3000,
			wantMs: 2000,
		},
		{
			name:   "single FlatModifier(-1000) on 3000ms gives 2000ms",
			chain:  ModifierChain{FlatModifier{FlatMs: -1000}},
			baseMs: 3000,
			wantMs: 2000,
		},
		{
			name: "Haste(50) then Flat(-500) on 3000ms: 3000/1.5=2000, 2000-500=1500",
			chain: ModifierChain{
				HasteModifier{HastePercent: 50},
				FlatModifier{FlatMs: -500},
			},
			baseMs: 3000,
			wantMs: 1500,
		},
		{
			name: "Flat(-500) then Haste(50) on 3000ms: 3000-500=2500, 2500/1.5=1666",
			chain: ModifierChain{
				FlatModifier{FlatMs: -500},
				HasteModifier{HastePercent: 50},
			},
			baseMs: 3000,
			wantMs: 1666,
		},
		{
			name: "modifiers clamped to minimum 0: Flat(-5000) on 3000ms gives 0",
			chain: ModifierChain{
				FlatModifier{FlatMs: -5000},
			},
			baseMs: 3000,
			wantMs: 0,
		},
		{
			name: "large negative combined gives 0",
			chain: ModifierChain{
				FlatModifier{FlatMs: -4000},
				HasteModifier{HastePercent: 50},
				FlatModifier{FlatMs: -1000},
			},
			baseMs: 3000,
			wantMs: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.chain.Apply(tt.baseMs)
			if got != tt.wantMs {
				t.Errorf("ModifierChain.Apply(%d) = %d, want %d", tt.baseMs, got, tt.wantMs)
			}
		})
	}
}

func TestModifierType(t *testing.T) {
	h := HasteModifier{HastePercent: 50}
	if h.Type() != "haste" {
		t.Errorf("HasteModifier.Type() = %q, want %q", h.Type(), "haste")
	}

	f := FlatModifier{FlatMs: -1000}
	if f.Type() != "flat" {
		t.Errorf("FlatModifier.Type() = %q, want %q", f.Type(), "flat")
	}
}

// --- ReCheckRange ---

func TestReCheckRange(t *testing.T) {
	tests := []struct {
		name      string
		rangeMax  float64
		casterPos unit.Position
		targetPos unit.Position
		want      bool
	}{
		{
			name:      "target within range returns true",
			rangeMax:  40,
			casterPos: unit.Position{X: 0, Y: 0, Z: 0},
			targetPos: unit.Position{X: 0, Y: 30, Z: 0},
			want:      true,
		},
		{
			name:      "target beyond range returns false",
			rangeMax:  20,
			casterPos: unit.Position{X: 0, Y: 0, Z: 0},
			targetPos: unit.Position{X: 0, Y: 30, Z: 0},
			want:      false,
		},
		{
			name:      "target at exact max range returns true",
			rangeMax:  25,
			casterPos: unit.Position{X: 0, Y: 0, Z: 0},
			targetPos: unit.Position{X: 0, Y: 25, Z: 0},
			want:      true,
		},
		{
			name:      "zero range max always returns true",
			rangeMax:  0,
			casterPos: unit.Position{X: 0, Y: 0, Z: 0},
			targetPos: unit.Position{X: 0, Y: 1000, Z: 0},
			want:      true,
		},
		{
			name:      "3D distance check includes Z axis",
			rangeMax:  10,
			casterPos: unit.Position{X: 0, Y: 0, Z: 0},
			targetPos: unit.Position{X: 0, Y: 8, Z: 8},
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := &spelldef.SpellInfo{RangeMax: tt.rangeMax}
			caster := unit.NewUnit(1, "Caster", 100, 100)
			caster.Position = tt.casterPos
			target := unit.NewUnit(2, "Target", 100, 100)
			target.Position = tt.targetPos
			targets := []*unit.Unit{target}

			got := ReCheckRange(info, caster, targets, nil)
			if got != tt.want {
				t.Errorf("ReCheckRange() = %v, want %v", got, tt.want)
			}
		})
	}
}

// --- Cast with ModifierChain ---

func TestPrepare_AppliesModifierChain(t *testing.T) {
	info := &spelldef.SpellInfo{
		ID:       30,
		Name:     "ModifiedFireball",
		CastTime: 3000,
	}
	caster := unit.NewUnit(1, "Caster", 100, 100)
	caster.Position = unit.Position{X: 0, Y: 0, Z: 0}
	target := unit.NewUnit(2, "Target", 100, 100)
	target.Position = unit.Position{X: 0, Y: 5, Z: 0}
	targets := []*unit.Unit{target}

	sc := New(30, info, caster, targets)
	sc.CastModifiers = ModifierChain{
		HasteModifier{HastePercent: 50},
		FlatModifier{FlatMs: -500},
	}

	result := sc.Prepare()

	if result != spelldef.CastResultSuccess {
		t.Fatalf("Prepare() failed: err=%d", sc.LastCastErr)
	}

	// 3000 / 1.5 = 2000, then 2000 - 500 = 1500ms
	expectedDuration := 1500 * time.Millisecond
	if sc.CastDuration != expectedDuration {
		t.Errorf("CastDuration = %v, want %v", sc.CastDuration, expectedDuration)
	}
}
