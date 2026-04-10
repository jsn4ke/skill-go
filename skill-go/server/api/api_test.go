package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func setupTestServer(t *testing.T) (*httptest.Server, *GameState) {
	t.Helper()
	gs := NewGameState()
	srv := NewServer(":0", gs)
	ts := httptest.NewServer(srv.Handler)
	t.Cleanup(func() { ts.Close() })
	return ts, gs
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

// 2.1 POST /api/cast — successful cast returns result=success
func TestCast_Success(t *testing.T) {
	ts, _ := setupTestServer(t)

	body := map[string]interface{}{
		"spellID":   1001,
		"targetIDs": []uint64{3},
	}
	resp := postJSON(t, ts.URL+"/api/cast", body)
	defer resp.Body.Close()

	var result CastResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if result.Result != "success" {
		t.Errorf("expected result=success, got %s", result.Result)
	}
	if len(result.Events) == 0 {
		t.Error("expected trace events from cast")
	}
}

// 2.2 POST /api/cast — cooldown returns result=failed
func TestCast_OnCooldown(t *testing.T) {
	ts, _ := setupTestServer(t)

	body := map[string]interface{}{
		"spellID":   1001,
		"targetIDs": []uint64{3},
	}

	// First cast succeeds
	resp1 := postJSON(t, ts.URL+"/api/cast", body)
	resp1.Body.Close()

	// Second cast should fail (cooldown)
	resp2 := postJSON(t, ts.URL+"/api/cast", body)
	defer resp2.Body.Close()

	var result CastResponse
	if err := json.NewDecoder(resp2.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if result.Result != "failed" {
		t.Errorf("expected result=failed, got %s", result.Result)
	}
	if result.Error == "" {
		t.Error("expected error field to be set")
	}
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
func TestTrace_EventsAfterCast(t *testing.T) {
	ts, _ := setupTestServer(t)

	// Cast a spell to generate events
	body := map[string]interface{}{
		"spellID":   1001,
		"targetIDs": []uint64{3},
	}
	resp := postJSON(t, ts.URL+"/api/cast", body)
	resp.Body.Close()

	var events []TraceEventJSON
	getJSON(t, ts.URL+"/api/trace", &events)

	if len(events) == 0 {
		t.Fatal("expected trace events after cast")
	}

	// Verify at least a "prepare" event exists
	found := false
	for _, e := range events {
		if e.Span == "spell" && e.Event == "prepare" {
			found = true
			break
		}
	}
	if !found {
		t.Error("missing spell.prepare event in trace")
	}
}

// 2.6 GET /api/trace?clear=true — clears and returns empty
func TestTrace_Clear(t *testing.T) {
	ts, _ := setupTestServer(t)

	// Generate some events
	body := map[string]interface{}{
		"spellID":   1001,
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

	if len(spells) < 8 {
		t.Errorf("expected >= 8 spells, got %d", len(spells))
	}

	// Check Fireball
	found := false
	for _, s := range spells {
		if s.ID == 1001 && s.Name == "Fireball" {
			found = true
			if s.SchoolName != "Fire" {
				t.Errorf("expected school Fire, got %s", s.SchoolName)
			}
			if s.CD != 6000 {
				t.Errorf("expected CD 6000, got %d", s.CD)
			}
		}
	}
	if !found {
		t.Error("Fireball (1001) not found in spell list")
	}
}

// 2.8 POST /api/reset — restores HP after damage
func TestReset_RestoresHP(t *testing.T) {
	ts, _ := setupTestServer(t)

	// Cast a damaging spell
	body := map[string]interface{}{
		"spellID":   1001,
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
