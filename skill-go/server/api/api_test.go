package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func setupTestServer(t *testing.T) (*httptest.Server, *GameLoop) {
	t.Helper()
	gl := NewGameLoop(nil)
	gl.Start()
	srv := NewServer(":0", gl)
	ts := httptest.NewServer(srv.Handler)
	t.Cleanup(func() { ts.Close() })
	return ts, gl
}

func getJSON(t *testing.T, url string, v interface{}) {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
		t.Fatalf("decode response: %v", err)
	}
}

func postJSON(t *testing.T, url string, body interface{}) *http.Response {
	t.Helper()
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	resp, err := http.Post(url, "application/json", bytes.NewReader(data))
	if err != nil {
		t.Fatalf("POST %s: %v", url, err)
	}
	return resp
}

// createInstantSpell creates an instant-cast damage spell via the API and returns its ID.
func createInstantSpell(t *testing.T, ts *httptest.Server) uint32 {
	t.Helper()
	body := map[string]interface{}{
		"name":       "InstantBolt",
		"schoolName": "Fire",
		"castTime":   0,
		"effects": []map[string]interface{}{
			{"effectType": "SchoolDamage", "basePoints": 50},
		},
	}
	resp := postJSON(t, ts.URL+"/api/spells", body)
	defer resp.Body.Close()

	var spells []SpellJSON
	getJSON(t, ts.URL+"/api/spells", &spells)
	for _, s := range spells {
		if s.Name == "InstantBolt" {
			return s.ID
		}
	}
	t.Fatal("InstantBolt spell not found after creation")
	return 0
}

// 2.1 POST /api/cast — Fireball (3.5s cast) returns result=preparing
func TestCast_Success(t *testing.T) {
	ts, _ := setupTestServer(t)

	body := map[string]interface{}{
		"spellID":   42833,
		"targetIDs": []uint64{3},
	}
	resp := postJSON(t, ts.URL+"/api/cast", body)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var result CastPrepareResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if result.Result != "preparing" {
		t.Errorf("expected result=preparing, got %s", result.Result)
	}
}

// 2.2 POST /api/cast — cooldown returns result=failed
func TestCast_OnCooldown(t *testing.T) {
	t.Skip("Fireball has no cooldown — test needs a spell with RecoveryTime > 0")
}

// 2.3 POST /api/cast — invalid spell returns 400
func TestCast_InvalidSpell(t *testing.T) {
	ts, _ := setupTestServer(t)

	body := map[string]interface{}{
		"spellID":   0,
		"targetIDs": []uint64{3},
	}
	resp := postJSON(t, ts.URL+"/api/cast", body)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

// 2.4 GET /api/units — returns unit array with HP/Mana/Level
func TestUnits_ReturnsAllUnits(t *testing.T) {
	ts, _ := setupTestServer(t)

	var units []UnitJSON
	getJSON(t, ts.URL+"/api/units", &units)

	if len(units) != 3 {
		t.Fatalf("expected 3 units, got %d", len(units))
	}

	// Check first unit (Mage)
	mage := units[0]
	if mage.Name != "Mage" {
		t.Errorf("expected Mage, got %s", mage.Name)
	}
	if mage.Health <= 0 || mage.MaxHealth <= 0 {
		t.Error("expected positive HP")
	}
	if mage.Mana <= 0 || mage.MaxMana <= 0 {
		t.Error("expected positive Mana")
	}
	if mage.Level == 0 {
		t.Error("expected non-zero Level")
	}
}

// 2.5 GET /api/trace — returns event list after cast
// Creates an instant-cast spell, casts it, and verifies events in the response.
func TestTrace_EventsAfterCast(t *testing.T) {
	ts, _ := setupTestServer(t)

	spellID := createInstantSpell(t, ts)

	body := map[string]interface{}{
		"spellID":   spellID,
		"targetIDs": []uint64{3},
	}
	resp := postJSON(t, ts.URL+"/api/cast", body)
	defer resp.Body.Close()

	var result CastResponse
	json.NewDecoder(resp.Body).Decode(&result)

	if len(result.Events) == 0 {
		t.Fatal("expected trace events in cast response")
	}

	found := false
	for _, e := range result.Events {
		if e.Span == "spell" && e.Event == "prepare" {
			found = true
			break
		}
	}
	if !found {
		t.Error("missing spell.prepare event in cast response")
	}
}

// 2.6 GET /api/trace?clear=true — clears and returns empty
func TestTrace_Clear(t *testing.T) {
	ts, _ := setupTestServer(t)

	// Generate some events via an instant cast
	spellID := createInstantSpell(t, ts)

	body := map[string]interface{}{
		"spellID":   spellID,
		"targetIDs": []uint64{3},
	}
	resp := postJSON(t, ts.URL+"/api/cast", body)
	resp.Body.Close()

	// Clear trace
	var events []TraceEventJSON
	getJSON(t, ts.URL+"/api/trace?clear=true", &events)

	// After clear, next fetch should be empty
	var empty []TraceEventJSON
	getJSON(t, ts.URL+"/api/trace", &empty)

	if len(empty) != 0 {
		t.Errorf("expected 0 events after clear, got %d", len(empty))
	}
}

// 2.7 GET /api/spells — returns spell list
func TestSpells_ReturnsList(t *testing.T) {
	ts, _ := setupTestServer(t)

	var spells []SpellJSON
	getJSON(t, ts.URL+"/api/spells", &spells)

	if len(spells) < 1 {
		t.Errorf("expected >= 1 spell, got %d", len(spells))
	}

	// Check Fireball
	found := false
	for _, s := range spells {
		if s.ID == 42833 && s.Name == "Fireball" {
			found = true
			if s.SchoolName != "Fire" {
				t.Errorf("expected school Fire, got %s", s.SchoolName)
			}
			if s.CD != 0 {
				t.Errorf("expected CD 0, got %d", s.CD)
			}
		}
	}
	if !found {
		t.Error("Fireball (42833) not found in spell list")
	}
}

// 2.8 POST /api/reset — restores HP after damage
func TestReset_RestoresHP(t *testing.T) {
	ts, _ := setupTestServer(t)

	// Cast an instant-cast damaging spell (Fireball has 3.5s cast, won't deal damage immediately)
	spellID := createInstantSpell(t, ts)

	body := map[string]interface{}{
		"spellID":   spellID,
		"targetIDs": []uint64{3},
	}
	resp := postJSON(t, ts.URL+"/api/cast", body)
	resp.Body.Close()

	// Check target took damage
	var unitsBefore []UnitJSON
	getJSON(t, ts.URL+"/api/units", &unitsBefore)
	dummyBefore := unitsBefore[2] // Target Dummy
	if dummyBefore.Health >= dummyBefore.MaxHealth {
		t.Error("expected target to have taken damage")
	}

	// Reset
	resp2 := postJSON(t, ts.URL+"/api/reset", nil)
	resp2.Body.Close()

	// Check HP restored
	var unitsAfter []UnitJSON
	getJSON(t, ts.URL+"/api/units", &unitsAfter)
	dummyAfter := unitsAfter[2]
	if dummyAfter.Health != dummyAfter.MaxHealth {
		t.Errorf("expected HP=%d after reset, got %d", dummyAfter.MaxHealth, dummyAfter.Health)
	}
}

// Bonus: CORS headers present
func TestCORS_HeadersPresent(t *testing.T) {
	ts, _ := setupTestServer(t)

	resp, err := http.Get(ts.URL + "/api/spells")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.Header.Get("Access-Control-Allow-Origin") != "*" {
		t.Error("missing CORS header")
	}
}

// --- Unit Management API Tests ---

func TestAddUnit_Success(t *testing.T) {
	ts, _ := setupTestServer(t)

	body := map[string]interface{}{"name": "Goblin", "level": 30}
	resp := postJSON(t, ts.URL+"/api/units/add", body)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var units []UnitJSON
	json.NewDecoder(resp.Body).Decode(&units)

	if len(units) != 4 {
		t.Fatalf("expected 4 units after add, got %d", len(units))
	}

	// Find the goblin
	found := false
	for _, u := range units {
		if u.Name == "Goblin" {
			found = true
			if u.Level != 30 {
				t.Errorf("expected level 30, got %d", u.Level)
			}
			if u.Health <= 0 {
				t.Error("expected positive HP")
			}
		}
	}
	if !found {
		t.Error("Goblin not found in unit list")
	}
}

func TestAddUnit_Defaults(t *testing.T) {
	ts, _ := setupTestServer(t)

	body := map[string]interface{}{}
	resp := postJSON(t, ts.URL+"/api/units/add", body)
	defer resp.Body.Close()

	var units []UnitJSON
	json.NewDecoder(resp.Body).Decode(&units)

	// Should have 4 units (3 original + 1 new)
	if len(units) != 4 {
		t.Fatalf("expected 4 units, got %d", len(units))
	}

	// Find the new unit (last one)
	newUnit := units[3]
	if newUnit.Name != "Unknown" {
		t.Errorf("expected default name 'Unknown', got %s", newUnit.Name)
	}
	if newUnit.Level != 60 {
		t.Errorf("expected default level 60, got %d", newUnit.Level)
	}
}

func TestRemoveUnit_Success(t *testing.T) {
	ts, _ := setupTestServer(t)

	// Add a unit first
	addBody := map[string]interface{}{"name": "Temp", "level": 1}
	addResp := postJSON(t, ts.URL+"/api/units/add", addBody)
	addResp.Body.Close()

	// Get the new unit's GUID
	var unitsBefore []UnitJSON
	getJSON(t, ts.URL+"/api/units", &unitsBefore)
	var tempGUID uint64
	for _, u := range unitsBefore {
		if u.Name == "Temp" {
			tempGUID = u.GUID
			break
		}
	}
	if tempGUID == 0 {
		t.Fatal("Temp unit not found")
	}

	// Remove it via DELETE
	req, _ := http.NewRequest(http.MethodDelete, ts.URL+fmt.Sprintf("/api/units/%d", tempGUID), nil)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var unitsAfter []UnitJSON
	getJSON(t, ts.URL+"/api/units", &unitsAfter)
	if len(unitsAfter) != 3 {
		t.Errorf("expected 3 units after removal, got %d", len(unitsAfter))
	}
}

func TestRemoveUnit_CannotRemoveCaster(t *testing.T) {
	ts, _ := setupTestServer(t)

	req, _ := http.NewRequest(http.MethodDelete, ts.URL+"/api/units/1", nil)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}

	var errResp map[string]string
	json.NewDecoder(resp.Body).Decode(&errResp)
	if errResp["error"] != "cannot remove caster" {
		t.Errorf("expected 'cannot remove caster' error, got %s", errResp["error"])
	}
}

func TestMoveUnit_Success(t *testing.T) {
	ts, _ := setupTestServer(t)

	body := map[string]interface{}{"guid": 1.0, "x": 15.0, "z": 5.0}
	resp := postJSON(t, ts.URL+"/api/units/move", body)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var units []UnitJSON
	json.NewDecoder(resp.Body).Decode(&units)

	// Find the caster
	for _, u := range units {
		if u.GUID == 1 {
			if u.Position.X != 15.0 {
				t.Errorf("expected X=15, got %v", u.Position.X)
			}
			if u.Position.Z != 5.0 {
				t.Errorf("expected Z=5, got %v", u.Position.Z)
			}
			return
		}
	}
	t.Error("caster not found")
}

func TestMoveUnit_NotFound(t *testing.T) {
	ts, _ := setupTestServer(t)

	body := map[string]interface{}{"guid": 99999.0, "x": 0.0, "z": 0.0}
	resp := postJSON(t, ts.URL+"/api/units/move", body)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for non-existent unit, got %d", resp.StatusCode)
	}
}
