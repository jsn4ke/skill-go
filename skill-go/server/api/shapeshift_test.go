package api

import (
	"encoding/json"
	"testing"
)

// --- helpers ---

func findUnitByGUID(units []UnitJSON, guid uint64) *UnitJSON {
	for i := range units {
		if units[i].GUID == guid {
			return &units[i]
		}
	}
	return nil
}

func unitHasAura(u *UnitJSON, spellID uint32) bool {
	if u == nil {
		return false
	}
	for _, a := range u.Auras {
		if a.SpellID == spellID {
			return true
		}
	}
	return false
}

// TestShapeshift_OnOff tests that casting a shapeshift spell twice activates then deactivates it.
func TestShapeshift_OnOff(t *testing.T) {
	ts, _ := setupTestServer(t)

	var spells []SpellJSON
	getJSON(t, ts.URL+"/api/spells", &spells)

	var stealthSpell *SpellJSON
	for i := range spells {
		if spells[i].ShapeshiftForm > 0 {
			stealthSpell = &spells[i]
			break
		}
	}
	if stealthSpell == nil {
		t.Fatal("no shapeshift spell found in spell book")
	}

	// Cast 1: toggle ON
	body := map[string]interface{}{
		"casterGuid": 1,
		"spellID":    stealthSpell.ID,
		"targetIDs":  []uint64{},
	}
	resp := postJSON(t, ts.URL+"/api/cast", body)
	defer resp.Body.Close()

	var castResp CastResponse
	if err := json.NewDecoder(resp.Body).Decode(&castResp); err != nil {
		t.Fatalf("decode cast response: %v", err)
	}
	if castResp.Result != "toggle_on" {
		t.Fatalf("expected toggle_on, got %q (error=%q)", castResp.Result, castResp.Error)
	}

	// Verify the caster has the shapeshift aura
	var units []UnitJSON
	getJSON(t, ts.URL+"/api/units", &units)
	caster := findUnitByGUID(units, 1)
	if caster == nil {
		t.Fatal("caster not found")
	}
	if !unitHasAura(caster, stealthSpell.ID) {
		t.Error("expected caster to have shapeshift aura after toggle_on")
	}
	if caster.Form != stealthSpell.ShapeshiftForm {
		t.Errorf("expected caster form=%d, got %d", stealthSpell.ShapeshiftForm, caster.Form)
	}

	// Cast 2: toggle OFF
	resp2 := postJSON(t, ts.URL+"/api/cast", body)
	defer resp2.Body.Close()

	var castResp2 CastResponse
	if err := json.NewDecoder(resp2.Body).Decode(&castResp2); err != nil {
		t.Fatalf("decode cast response 2: %v", err)
	}
	if castResp2.Result != "toggle_off" {
		t.Errorf("expected toggle_off, got %s", castResp2.Result)
	}

	// Verify the aura is removed
	getJSON(t, ts.URL+"/api/units", &units)
	caster = findUnitByGUID(units, 1)
	if unitHasAura(caster, stealthSpell.ID) {
		t.Error("expected shapeshift aura to be removed after toggle_off")
	}
	if caster.Form != 0 {
		t.Errorf("expected caster form=0 after toggle_off, got %d", caster.Form)
	}
}

// TestShapeshift_MutualExclusion tests that activating a form removes the previous form aura.
func TestShapeshift_MutualExclusion(t *testing.T) {
	ts, _ := setupTestServer(t)

	var spells []SpellJSON
	getJSON(t, ts.URL+"/api/spells", &spells)

	var battleStance, defStance *SpellJSON
	for i := range spells {
		if spells[i].ShapeshiftForm == 17 {
			battleStance = &spells[i]
		}
		if spells[i].ShapeshiftForm == 18 {
			defStance = &spells[i]
		}
	}
	if battleStance == nil || defStance == nil {
		t.Fatal("need battle stance (17) and defense stance (18) for mutual exclusion test")
	}

	// Activate battle stance
	body := map[string]interface{}{
		"casterGuid": 1,
		"spellID":    battleStance.ID,
		"targetIDs":  []uint64{},
	}
	resp := postJSON(t, ts.URL+"/api/cast", body)
	defer resp.Body.Close()
	var castResp CastResponse
	json.NewDecoder(resp.Body).Decode(&castResp)
	if castResp.Result != "toggle_on" {
		t.Fatalf("expected toggle_on for battle stance, got %q", castResp.Result)
	}

	// Activate defense stance (should remove battle stance)
	body2 := map[string]interface{}{
		"casterGuid": 1,
		"spellID":    defStance.ID,
		"targetIDs":  []uint64{},
	}
	resp2 := postJSON(t, ts.URL+"/api/cast", body2)
	defer resp2.Body.Close()
	var castResp2 CastResponse
	json.NewDecoder(resp2.Body).Decode(&castResp2)
	if castResp2.Result != "toggle_on" {
		t.Fatalf("expected toggle_on for defense stance, got %q", castResp2.Result)
	}

	// Verify: defense stance aura exists, battle stance aura removed
	var units []UnitJSON
	getJSON(t, ts.URL+"/api/units", &units)
	caster := findUnitByGUID(units, 1)
	if !unitHasAura(caster, defStance.ID) {
		t.Error("expected defense stance aura after activation")
	}
	if unitHasAura(caster, battleStance.ID) {
		t.Error("expected battle stance aura to be removed (mutual exclusion)")
	}
	if caster.Form != 18 {
		t.Errorf("expected caster form=18 (defense), got %d", caster.Form)
	}
}

// TestShapeshift_BreakOnDamage tests that taking damage removes a BreakOnDamage form aura (Stealth).
func TestShapeshift_BreakOnDamage(t *testing.T) {
	ts, _ := setupTestServer(t)

	// Activate stealth on caster (Mage, guid=1)
	body := map[string]interface{}{
		"casterGuid": 1,
		"spellID":    1784,
		"targetIDs":  []uint64{},
	}
	resp := postJSON(t, ts.URL+"/api/cast", body)
	defer resp.Body.Close()
	var castResp CastResponse
	json.NewDecoder(resp.Body).Decode(&castResp)
	if castResp.Result != "toggle_on" {
		t.Fatalf("expected toggle_on, got %q", castResp.Result)
	}

	// First put Warrior (guid=2) into Battle Stance so Charge can be cast
	var spells []SpellJSON
	getJSON(t, ts.URL+"/api/spells", &spells)
	var battleStance *SpellJSON
	for i := range spells {
		if spells[i].ShapeshiftForm == 17 {
			battleStance = &spells[i]
			break
		}
	}
	if battleStance != nil {
		stanceBody := map[string]interface{}{
			"casterGuid": 2,
			"spellID":    battleStance.ID,
			"targetIDs":  []uint64{},
		}
		resp0 := postJSON(t, ts.URL+"/api/cast", stanceBody)
		resp0.Body.Close()
	}

	// Cast Charge (instant, requires Battle Stance) from Warrior targeting Mage
	damageBody := map[string]interface{}{
		"casterGuid": 2,
		"spellID":    100,
		"targetIDs":  []uint64{1},
	}
	resp2 := postJSON(t, ts.URL+"/api/cast", damageBody)
	defer resp2.Body.Close()

	// Verify stealth aura was removed by BreakOnDamage
	var units []UnitJSON
	getJSON(t, ts.URL+"/api/units", &units)
	caster := findUnitByGUID(units, 1)
	if unitHasAura(caster, 1784) {
		t.Error("expected stealth aura to be removed after taking damage (BreakOnDamage)")
	}
	if caster.Form != 0 {
		t.Errorf("expected form=0 after stealth break, got %d", caster.Form)
	}
}

// TestShapeshift_StancesCheck tests that casting a stance-restricted spell fails when not in the required form.
func TestShapeshift_StancesCheck(t *testing.T) {
	ts, _ := setupTestServer(t)

	var spells []SpellJSON
	getJSON(t, ts.URL+"/api/spells", &spells)

	// Find Charge (spell 100) — requires Battle Stance (stances=0x10000)
	var chargeSpell *SpellJSON
	for i := range spells {
		if spells[i].ID == 100 {
			chargeSpell = &spells[i]
			break
		}
	}
	if chargeSpell == nil {
		t.Fatal("charge spell (100) not found")
	}
	if chargeSpell.Stances == 0 {
		t.Fatal("charge spell should have stances restriction")
	}

	// Find Defense Stance (form=18)
	var defStance *SpellJSON
	for i := range spells {
		if spells[i].ShapeshiftForm == 18 {
			defStance = &spells[i]
			break
		}
	}
	if defStance == nil {
		t.Fatal("defense stance spell not found")
	}

	// Activate Defense Stance on Warrior (guid=2)
	stanceBody := map[string]interface{}{
		"casterGuid": 2,
		"spellID":    defStance.ID,
		"targetIDs":  []uint64{},
	}
	resp := postJSON(t, ts.URL+"/api/cast", stanceBody)
	defer resp.Body.Close()
	var stanceResp CastResponse
	json.NewDecoder(resp.Body).Decode(&stanceResp)
	if stanceResp.Result != "toggle_on" {
		t.Fatalf("expected toggle_on for defense stance, got %q", stanceResp.Result)
	}

	// Try to cast Charge while in Defense Stance — should fail
	chargeBody := map[string]interface{}{
		"casterGuid": 2,
		"spellID":    chargeSpell.ID,
		"targetIDs":  []uint64{1},
	}
	resp2 := postJSON(t, ts.URL+"/api/cast", chargeBody)
	defer resp2.Body.Close()
	var chargeResp CastResponse
	json.NewDecoder(resp2.Body).Decode(&chargeResp)
	if chargeResp.Result != "failed" {
		t.Fatalf("expected 'failed' for wrong stance, got %q (error=%q)", chargeResp.Result, chargeResp.Error)
	}
	if chargeResp.Error == "" {
		t.Error("expected non-empty error for wrong stance")
	}
}
