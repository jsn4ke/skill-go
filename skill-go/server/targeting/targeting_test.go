package targeting

import (
	"math"
	"testing"

	"skill-go/server/unit"
)

// mockUnitProvider implements UnitProvider for testing.
type mockUnitProvider struct {
	units []*unit.Unit
}

func (m *mockUnitProvider) GetAllUnits() []*unit.Unit {
	return m.units
}

// --- Area Selection ---

func TestSelectArea(t *testing.T) {
	caster := unit.NewUnit(1, "Caster", 100, 100)
	caster.Position = unit.Position{X: 0, Y: 0, Z: 0}

	units := []*unit.Unit{
		unit.NewUnit(10, "Close", 100, 100),   // at origin
		unit.NewUnit(11, "Medium", 100, 100),  // distance 5
		unit.NewUnit(12, "Far", 100, 100),     // distance 15 (outside radius 10)
		unit.NewUnit(13, "OnEdge", 100, 100),  // distance 10 (on boundary)
	}

	units[0].Position = unit.Position{X: 0, Y: 0, Z: 0}       // dist 0
	units[1].Position = unit.Position{X: 3, Y: 4, Z: 0}       // dist 5
	units[2].Position = unit.Position{X: 12, Y: 9, Z: 0}      // dist 15
	units[3].Position = unit.Position{X: 10, Y: 0, Z: 0}      // dist 10

	provider := &mockUnitProvider{units: units}

	ctx := &SelectionContext{
		Caster:    caster,
		Descriptor: TargetDescriptor{
			Category: SelectArea,
			Dir:      Direction{Radius: 10},
		},
	}

	selected := Select(ctx, provider, nil, 0, "")

	// Should select Close (dist 0), Medium (dist 5), OnEdge (dist 10).
	// Far (dist 15) is outside the radius.
	// Caster is not in the provider's unit list.
	if len(selected) != 3 {
		t.Errorf("SelectArea: expected 3 units selected, got %d", len(selected))
	}

	// Verify that "Far" (dist 15) is not selected.
	for _, u := range selected {
		if u.GUID == 12 {
			t.Error("SelectArea: unit 'Far' at distance 15 should not be selected with radius 10")
		}
	}
}

// --- Cone Selection ---

func TestSelectCone(t *testing.T) {
	caster := unit.NewUnit(1, "Caster", 100, 100)
	caster.Position = unit.Position{X: 0, Y: 0, Z: 0}

	// Caster faces +Y direction.
	// Cone with halfAngle = pi/6 (30 degrees), radius = 10.
	halfAngle := math.Pi / 6 // 30 degrees

	units := []*unit.Unit{
		unit.NewUnit(10, "InFront", 100, 100),   // directly ahead (0, 5, 0) - in cone
		unit.NewUnit(11, "SlightlyOff", 100, 100), // (3, 6, 0) - small angle, in cone
		unit.NewUnit(12, "FarAway", 100, 100),    // (0, 15, 0) - out of radius
		unit.NewUnit(13, "Behind", 100, 100),     // (0, -5, 0) - behind caster
		unit.NewUnit(14, "WideAngle", 100, 100),  // (8, 5, 0) - wide angle, out of cone
	}

	units[0].Position = unit.Position{X: 0, Y: 5, Z: 0}    // angle=0, dist=5
	units[1].Position = unit.Position{X: 3, Y: 6, Z: 0}    // angle ~26deg, dist ~6.7
	units[2].Position = unit.Position{X: 0, Y: 15, Z: 0}   // dist=15 > radius=10
	units[3].Position = unit.Position{X: 0, Y: -5, Z: 0}   // behind
	units[4].Position = unit.Position{X: 8, Y: 5, Z: 0}    // angle ~58deg > 30deg

	provider := &mockUnitProvider{units: units}

	ctx := &SelectionContext{
		Caster:    caster,
		Descriptor: TargetDescriptor{
			Category: SelectCone,
			Dir:      Direction{Radius: 10, ConeAngle: halfAngle},
		},
	}

	selected := Select(ctx, provider, nil, 0, "")

	guids := make(map[uint64]bool)
	for _, u := range selected {
		guids[u.GUID] = true
	}

	if !guids[10] {
		t.Error("SelectCone: 'InFront' should be selected")
	}
	if !guids[11] {
		t.Error("SelectCone: 'SlightlyOff' should be selected")
	}
	if guids[12] {
		t.Error("SelectCone: 'FarAway' should not be selected (out of radius)")
	}
	if guids[13] {
		t.Error("SelectCone: 'Behind' should not be selected")
	}
	if guids[14] {
		t.Error("SelectCone: 'WideAngle' should not be selected (angle too wide)")
	}
}

// --- Chain Selection ---

func TestSelectChain(t *testing.T) {
	caster := unit.NewUnit(1, "Caster", 100, 100)
	caster.Position = unit.Position{X: 0, Y: 0, Z: 0}

	// Three targets close together. Chain should bounce to closest damaged target.
	target1 := unit.NewUnit(10, "Target1", 50, 100)  // damaged (50/100 HP)
	target1.Position = unit.Position{X: 5, Y: 0, Z: 0}

	target2 := unit.NewUnit(11, "Target2", 30, 100)  // most damaged (30/100 HP)
	target2.Position = unit.Position{X: 10, Y: 0, Z: 0}

	target3 := unit.NewUnit(12, "Target3", 80, 100)  // less damaged (80/100 HP)
	target3.Position = unit.Position{X: 8, Y: 3, Z: 0}

	provider := &mockUnitProvider{
		units: []*unit.Unit{caster, target1, target2, target3},
	}

	ctx := &SelectionContext{
		Caster:          caster,
		ExplicitTargets: []*unit.Unit{target1},
		Descriptor: TargetDescriptor{
			Category: SelectChain,
			Dir:      Direction{Radius: 15},
			Validation: ValidationRule{
				MaxTargets: 3,
			},
		},
	}

	selected := Select(ctx, provider, nil, 0, "")

	if len(selected) < 2 {
		t.Fatalf("SelectChain: expected at least 2 targets (primary + bounces), got %d", len(selected))
	}

	// First target should be the explicit target.
	if selected[0].GUID != target1.GUID {
		t.Errorf("SelectChain: first target should be Target1 (GUID=10), got GUID=%d", selected[0].GUID)
	}

	// Chain prefers most damaged target within range. Target2 has lowest HP.
	// Target3 is farther from Target1 but closer if considering damage preference.
	// The chain from Target1 should go to the closest unit with lowest HP within bounceRange.
	// Target1 is at (5,0), Target2 at (10,0) dist=5, Target3 at (8,3) dist=sqrt(9+9)=4.24
	// Target2 has lower HP (30 vs 80), so Target2 should be preferred despite distance.
	// But distance matters first: only units within bounceRange (15) are considered,
	// then among those, lowest HP wins.
	if len(selected) >= 2 {
		if selected[1].GUID != target2.GUID {
			// target2 has HP=30 (most damaged), should be preferred over target3 (HP=80)
			// Both are in range. Chain prefers most damaged.
			t.Errorf("SelectChain: second bounce should prefer most damaged target (Target2, HP=30), got %s (HP=%d)",
				selected[1].Name, selected[1].Health)
		}
	}
}

// --- Line Selection ---

func TestSelectLine(t *testing.T) {
	caster := unit.NewUnit(1, "Caster", 100, 100)
	caster.Position = unit.Position{X: 0, Y: 0, Z: 0}

	// Caster faces +Y. Line: length=20, width=4 (halfWidth=2).
	units := []*unit.Unit{
		unit.NewUnit(10, "OnLine", 100, 100),      // (0, 10, 0) - on the line
		unit.NewUnit(11, "OnLineFar", 100, 100),   // (0, 18, 0) - on the line near end
		unit.NewUnit(12, "OffWidth", 100, 100),    // (3, 10, 0) - outside width
		unit.NewUnit(13, "BeyondLength", 100, 100), // (0, 25, 0) - beyond length
		unit.NewUnit(14, "Behind", 100, 100),      // (0, -5, 0) - behind caster
		unit.NewUnit(15, "OnEdge", 100, 100),      // (1.9, 10, 0) - within width
	}

	units[0].Position = unit.Position{X: 0, Y: 10, Z: 0}   // on line, within width
	units[1].Position = unit.Position{X: 0, Y: 18, Z: 0}   // on line, within length
	units[2].Position = unit.Position{X: 3, Y: 10, Z: 0}   // perp dist = 3 > halfWidth(2)
	units[3].Position = unit.Position{X: 0, Y: 25, Z: 0}   // along = 25 > length(20)
	units[4].Position = unit.Position{X: 0, Y: -5, Z: 0}   // along = -5 < 0
	units[5].Position = unit.Position{X: 1.9, Y: 10, Z: 0}  // perp dist = 1.9 <= halfWidth(2)

	provider := &mockUnitProvider{units: units}

	ctx := &SelectionContext{
		Caster:    caster,
		Descriptor: TargetDescriptor{
			Category: SelectLine,
			Dir:      Direction{Length: 20, Width: 4},
		},
	}

	selected := Select(ctx, provider, nil, 0, "")

	guids := make(map[uint64]bool)
	for _, u := range selected {
		guids[u.GUID] = true
	}

	if !guids[10] {
		t.Error("SelectLine: 'OnLine' should be selected")
	}
	if !guids[11] {
		t.Error("SelectLine: 'OnLineFar' should be selected")
	}
	if guids[12] {
		t.Error("SelectLine: 'OffWidth' (perp=3) should not be selected with width=4")
	}
	if guids[13] {
		t.Error("SelectLine: 'BeyondLength' should not be selected")
	}
	if guids[14] {
		t.Error("SelectLine: 'Behind' should not be selected")
	}
	if !guids[15] {
		t.Error("SelectLine: 'OnEdge' should be selected (within width boundary)")
	}
}

// --- Trajectory Selection ---

func TestSelectTrajectory(t *testing.T) {
	caster := unit.NewUnit(1, "Caster", 100, 100)
	caster.Position = unit.Position{X: 0, Y: 0, Z: 0}

	// Use a simple axis-aligned trajectory to avoid floating point issues at boundaries.
	// Trajectory from (0,0) to (0,20), width=2.
	target := unit.NewUnit(2, "Target", 100, 100)
	target.Position = unit.Position{X: 0, Y: 20, Z: 0}

	onPath := unit.NewUnit(10, "OnPath", 100, 100)
	onPath.Position = unit.Position{X: 0, Y: 10, Z: 0}  // directly on the line

	offPath := unit.NewUnit(11, "OffPath", 100, 100)
	offPath.Position = unit.Position{X: 5, Y: 10, Z: 0} // perp dist = 5, outside width

	beyond := unit.NewUnit(12, "Beyond", 100, 100)
	beyond.Position = unit.Position{X: 0, Y: 25, Z: 0}   // beyond endpoint

	slightlyOff := unit.NewUnit(13, "SlightlyOff", 100, 100)
	slightlyOff.Position = unit.Position{X: 0.8, Y: 10, Z: 0} // perp = 0.8 <= halfWidth(1)

	provider := &mockUnitProvider{
		units: []*unit.Unit{caster, target, onPath, offPath, beyond, slightlyOff},
	}

	ctx := &SelectionContext{
		Caster:          caster,
		ExplicitTargets: []*unit.Unit{target},
		Descriptor: TargetDescriptor{
			Category:  SelectTrajectory,
			Reference: RefTarget,
			Dir:       Direction{Width: 2},
		},
	}

	selected := Select(ctx, provider, nil, 0, "")

	guids := make(map[uint64]bool)
	for _, u := range selected {
		guids[u.GUID] = true
	}

	// Caster should not be selected (excluded by GUID check).
	if guids[1] {
		t.Error("SelectTrajectory: caster should not be in results")
	}

	// Target at endpoint should be selected (on the trajectory line, within width).
	if !guids[2] {
		t.Error("SelectTrajectory: endpoint Target should be selected")
	}

	// Unit on the path should be selected.
	if !guids[10] {
		t.Error("SelectTrajectory: 'OnPath' should be selected")
	}

	// Unit far off the path should not be selected.
	if guids[11] {
		t.Error("SelectTrajectory: 'OffPath' should not be selected")
	}

	// Unit beyond the endpoint should not be selected.
	if guids[12] {
		t.Error("SelectTrajectory: 'Beyond' should not be selected")
	}

	// Unit slightly off the path but within width should be selected.
	if !guids[13] {
		t.Error("SelectTrajectory: 'SlightlyOff' should be selected (within width)")
	}
}

// --- FilterFunc ---

func TestFilterFunc_RemovesLowHPUnits(t *testing.T) {
	caster := unit.NewUnit(1, "Caster", 100, 100)
	caster.Position = unit.Position{X: 0, Y: 0, Z: 0}

	highHP := unit.NewUnit(10, "HighHP", 90, 100)
	highHP.Position = unit.Position{X: 1, Y: 0, Z: 0}

	medHP := unit.NewUnit(11, "MedHP", 50, 100)
	medHP.Position = unit.Position{X: 2, Y: 0, Z: 0}

	lowHP := unit.NewUnit(12, "LowHP", 10, 100)
	lowHP.Position = unit.Position{X: 3, Y: 0, Z: 0}

	deadUnit := unit.NewUnit(13, "Dead", 0, 100)
	deadUnit.Position = unit.Position{X: 4, Y: 0, Z: 0}
	deadUnit.TakeDamage(100)

	provider := &mockUnitProvider{
		units: []*unit.Unit{caster, highHP, medHP, lowHP, deadUnit},
	}

	// Filter that removes units below 50 HP.
	minHPFilter := func(targets []*unit.Unit) []*unit.Unit {
		var filtered []*unit.Unit
		for _, u := range targets {
			if u.Health >= 50 {
				filtered = append(filtered, u)
			}
		}
		return filtered
	}

	ctx := &SelectionContext{
		Caster:    caster,
		Descriptor: TargetDescriptor{
			Category: SelectArea,
			Dir:      Direction{Radius: 20},
		},
		Filters: map[FilterPoint][]FilterFunc{
			FilterUnit: {minHPFilter},
		},
	}

	selected := Select(ctx, provider, nil, 0, "")

	for _, u := range selected {
		if u.Health < 50 {
			t.Errorf("FilterFunc: unit %s with HP=%d should have been filtered out", u.Name, u.Health)
		}
	}

	// Verify highHP and medHP are kept.
	guids := make(map[uint64]bool)
	for _, u := range selected {
		guids[u.GUID] = true
	}

	if !guids[10] {
		t.Error("FilterFunc: 'HighHP' (90HP) should be in results")
	}
	if !guids[11] {
		t.Error("FilterFunc: 'MedHP' (50HP) should be in results")
	}
	if guids[12] {
		t.Error("FilterFunc: 'LowHP' (10HP) should be filtered out")
	}
	if guids[13] {
		t.Error("FilterFunc: 'Dead' (0HP) should be filtered out")
	}
}

// --- Validation Rules ---

func TestValidation_AliveOnly(t *testing.T) {
	caster := unit.NewUnit(1, "Caster", 100, 100)
	caster.Position = unit.Position{X: 0, Y: 0, Z: 0}

	alive := unit.NewUnit(10, "Alive", 100, 100)
	alive.Position = unit.Position{X: 3, Y: 0, Z: 0}

	dead := unit.NewUnit(11, "Dead", 0, 100)
	dead.Position = unit.Position{X: 2, Y: 0, Z: 0}
	dead.TakeDamage(100)

	provider := &mockUnitProvider{units: []*unit.Unit{caster, alive, dead}}

	ctx := &SelectionContext{
		Caster:    caster,
		Descriptor: TargetDescriptor{
			Category: SelectArea,
			Dir:      Direction{Radius: 10},
			Validation: ValidationRule{
				AliveOnly: true,
			},
		},
	}

	selected := Select(ctx, provider, nil, 0, "")

	for _, u := range selected {
		if !u.IsAlive() {
			t.Errorf("AliveOnly validation: dead unit %s should not be selected", u.Name)
		}
	}

	if len(selected) != 2 {
		// Caster (alive, dist 0) + alive unit (dist 3)
		t.Errorf("AliveOnly: expected 2 units (caster + alive), got %d", len(selected))
	}
}

func TestValidation_MaxTargets(t *testing.T) {
	caster := unit.NewUnit(1, "Caster", 100, 100)
	caster.Position = unit.Position{X: 0, Y: 0, Z: 0}

	units := []*unit.Unit{caster}
	for i := 0; i < 5; i++ {
		u := unit.NewUnit(uint64(10+i), "Unit", 100, 100)
		u.Position = unit.Position{X: float64(i), Y: 0, Z: 0}
		units = append(units, u)
	}

	provider := &mockUnitProvider{units: units}

	ctx := &SelectionContext{
		Caster:    caster,
		Descriptor: TargetDescriptor{
			Category: SelectArea,
			Dir:      Direction{Radius: 20},
			Validation: ValidationRule{
				MaxTargets: 2,
			},
		},
	}

	selected := Select(ctx, provider, nil, 0, "")

	if len(selected) > 2 {
		t.Errorf("MaxTargets: expected at most 2 units, got %d", len(selected))
	}
}

func TestValidation_ConditionFunc(t *testing.T) {
	caster := unit.NewUnit(1, "Caster", 100, 100)
	caster.Position = unit.Position{X: 0, Y: 0, Z: 0}

	warrior := unit.NewUnit(10, "Warrior", 100, 0)  // no mana
	warrior.Position = unit.Position{X: 3, Y: 0, Z: 0}

	mage := unit.NewUnit(11, "Mage", 100, 500)  // has mana
	mage.Position = unit.Position{X: 5, Y: 0, Z: 0}

	provider := &mockUnitProvider{units: []*unit.Unit{caster, warrior, mage}}

	// Condition: unit must have mana > 0.
	hasManaCondition := func(u *unit.Unit) bool {
		return u.Mana > 0
	}

	ctx := &SelectionContext{
		Caster:    caster,
		Descriptor: TargetDescriptor{
			Category: SelectArea,
			Dir:      Direction{Radius: 20},
			Validation: ValidationRule{
				Conditions: []ConditionFunc{hasManaCondition},
			},
		},
	}

	selected := Select(ctx, provider, nil, 0, "")

	for _, u := range selected {
		if u.Mana <= 0 {
			t.Errorf("ConditionFunc: unit %s with mana=%d should be filtered out", u.Name, u.Mana)
		}
	}

	// Only units with mana > 0: caster (100 mana) + mage (500 mana).
	if len(selected) != 2 {
		t.Errorf("ConditionFunc: expected 2 units, got %d", len(selected))
	}
}

// --- SelectSelf ---

func TestSelectSelf(t *testing.T) {
	caster := unit.NewUnit(1, "Caster", 100, 100)

	provider := &mockUnitProvider{units: []*unit.Unit{caster}}

	ctx := &SelectionContext{
		Caster:    caster,
		Descriptor: TargetDescriptor{
			Category: SelectSelf,
		},
	}

	selected := Select(ctx, provider, nil, 0, "")

	if len(selected) != 1 {
		t.Fatalf("SelectSelf: expected 1 target, got %d", len(selected))
	}
	if selected[0].GUID != caster.GUID {
		t.Error("SelectSelf: selected unit should be the caster")
	}
}

// --- SelectSingle ---

func TestSelectSingle(t *testing.T) {
	caster := unit.NewUnit(1, "Caster", 100, 100)
	target := unit.NewUnit(2, "Target", 100, 100)
	other := unit.NewUnit(3, "Other", 100, 100)

	provider := &mockUnitProvider{units: []*unit.Unit{caster, target, other}}

	ctx := &SelectionContext{
		Caster:          caster,
		ExplicitTargets: []*unit.Unit{target, other},
		Descriptor: TargetDescriptor{
			Category: SelectSingle,
		},
	}

	selected := Select(ctx, provider, nil, 0, "")

	if len(selected) != 1 {
		t.Fatalf("SelectSingle: expected 1 target, got %d", len(selected))
	}
	if selected[0].GUID != target.GUID {
		t.Error("SelectSingle: should select the first explicit target")
	}
}

// --- Area Filter ---

func TestAreaFilter(t *testing.T) {
	caster := unit.NewUnit(1, "Caster", 100, 100)
	caster.Position = unit.Position{X: 0, Y: 0, Z: 0}

	u1 := unit.NewUnit(10, "Unit1", 100, 100)
	u1.Position = unit.Position{X: 1, Y: 0, Z: 0}

	u2 := unit.NewUnit(11, "Unit2", 100, 100)
	u2.Position = unit.Position{X: 2, Y: 0, Z: 0}

	u3 := unit.NewUnit(12, "Unit3", 100, 100)
	u3.Position = unit.Position{X: 3, Y: 0, Z: 0}

	provider := &mockUnitProvider{units: []*unit.Unit{caster, u1, u2, u3}}

	// FilterArea filter: keep only the first 2 units from the area set.
	areaFilter := func(targets []*unit.Unit) []*unit.Unit {
		if len(targets) > 2 {
			return targets[:2]
		}
		return targets
	}

	ctx := &SelectionContext{
		Caster:    caster,
		Descriptor: TargetDescriptor{
			Category: SelectArea,
			Dir:      Direction{Radius: 10},
		},
		Filters: map[FilterPoint][]FilterFunc{
			FilterArea: {areaFilter},
		},
	}

	selected := Select(ctx, provider, nil, 0, "")

	if len(selected) != 2 {
		t.Errorf("AreaFilter: expected 2 units after area filter, got %d", len(selected))
	}
}
