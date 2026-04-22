package api

import (
	"encoding/json"
	"testing"
)

// TestToggle_SpellOnOff tests that casting a toggle spell twice activates then deactivates it.
func TestToggle_SpellOnOff(t *testing.T) {
	ts, _ := setupTestServer(t)

	var spells []SpellJSON
	getJSON(t, ts.URL+"/api/spells", &spells)

	var stealthSpell *SpellJSON
	for i := range spells {
		if spells[i].IsToggle {
			stealthSpell = &spells[i]
			break
		}
	}
	if stealthSpell == nil {
		t.Fatal("no toggle spell found in spell book")
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

	// Verify the caster has the toggle aura
	var units []UnitJSON
	getJSON(t, ts.URL+"/api/units", &units)
	caster := findUnitByGUID(units, 1)
	if caster == nil {
		t.Fatal("caster not found")
	}
	if !unitHasAura(caster, stealthSpell.ID) {
		t.Error("expected caster to have toggle aura after toggle_on")
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
		t.Error("expected toggle aura to be removed after toggle_off")
	}
}

// TestToggle_MutualExclusion tests that activating a same-group toggle removes the previous one.
func TestToggle_MutualExclusion(t *testing.T) {
	ts, _ := setupTestServer(t)

	var spells []SpellJSON
	getJSON(t, ts.URL+"/api/spells", &spells)

	var battleStance, defStance *SpellJSON
	for i := range spells {
		if spells[i].ToggleGroup == "warrior_stance" {
			if battleStance == nil {
				battleStance = &spells[i]
			} else {
				defStance = &spells[i]
				break
			}
		}
	}
	if battleStance == nil || defStance == nil {
		t.Fatal("need at least 2 warrior stance toggles for mutual exclusion test")
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

	// Activate defense stance (same toggle group)
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
}

// TestToggle_BreakOnDamage tests that taking damage removes a BreakOnDamage toggle aura.
func TestToggle_BreakOnDamage(t *testing.T) {
	ts, _ := setupTestServer(t)

	var spells []SpellJSON
	getJSON(t, ts.URL+"/api/spells", &spells)

	var stealthSpell *SpellJSON
	for i := range spells {
		if spells[i].ID == 1784 {
			stealthSpell = &spells[i]
			break
		}
	}
	if stealthSpell == nil {
		t.Fatal("stealth spell (1784) not found")
	}

	// Activate stealth on caster
	body := map[string]interface{}{
		"casterGuid": 1,
		"spellID":    stealthSpell.ID,
		"targetIDs":  []uint64{},
	}
	resp := postJSON(t, ts.URL+"/api/cast", body)
	defer resp.Body.Close()
	var castResp CastResponse
	json.NewDecoder(resp.Body).Decode(&castResp)
	if castResp.Result != "toggle_on" {
		t.Fatalf("expected toggle_on, got %q", castResp.Result)
	}

	// Cast Charge (instant, no resource cost) from Warrior targeting Mage
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
	if unitHasAura(caster, stealthSpell.ID) {
		t.Error("expected stealth aura to be removed after taking damage (BreakOnDamage)")
	}
}

// TestToggle_NoGCDOrCooldown verifies that toggle spells can be cast repeatedly.
func TestToggle_NoGCDOrCooldown(t *testing.T) {
	ts, _ := setupTestServer(t)

	var spells []SpellJSON
	getJSON(t, ts.URL+"/api/spells", &spells)

	var toggleSpell *SpellJSON
	for i := range spells {
		if spells[i].IsToggle {
			toggleSpell = &spells[i]
			break
		}
	}
	if toggleSpell == nil {
		t.Fatal("no toggle spell found")
	}

	body := map[string]interface{}{
		"casterGuid": 1,
		"spellID":    toggleSpell.ID,
		"targetIDs":  []uint64{},
	}

	// Cast toggle ON
	resp := postJSON(t, ts.URL+"/api/cast", body)
	defer resp.Body.Close()
	var castResp CastResponse
	json.NewDecoder(resp.Body).Decode(&castResp)
	if castResp.Result != "toggle_on" {
		t.Fatalf("expected toggle_on, got %q", castResp.Result)
	}

	// Immediately cast toggle OFF
	resp2 := postJSON(t, ts.URL+"/api/cast", body)
	defer resp2.Body.Close()
	var castResp2 CastResponse
	json.NewDecoder(resp2.Body).Decode(&castResp2)
	if castResp2.Result != "toggle_off" {
		t.Errorf("expected toggle_off (no GCD/CD), got %q", castResp2.Result)
	}

	// Toggle back ON immediately
	resp3 := postJSON(t, ts.URL+"/api/cast", body)
	defer resp3.Body.Close()
	var castResp3 CastResponse
	json.NewDecoder(resp3.Body).Decode(&castResp3)
	if castResp3.Result != "toggle_on" {
		t.Errorf("expected toggle_on again (no GCD/CD), got %q", castResp3.Result)
	}
}

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
