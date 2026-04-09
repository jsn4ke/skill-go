package trace

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
)

// resetNextFlowID stores and restores the global counter so tests are deterministic.
func resetNextFlowID() {
	nextFlowID = 0
}

// captureStdout redirects os.Stdout, runs fn, and returns captured output.
func captureStdout(fn func()) string {
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t := &testing.T{}
		t.Fatalf("failed to create pipe: %v", err)
	}
	os.Stdout = w
	fn()
	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r)
	r.Close()
	return buf.String()
}

func TestFlowIDIncrements(t *testing.T) {
	resetNextFlowID()

	t1 := NewTrace()
	t2 := NewTrace()
	t3 := NewTrace()

	if t1.FlowID != 1 {
		t.Errorf("expected FlowID 1, got %d", t1.FlowID)
	}
	if t2.FlowID != 2 {
		t.Errorf("expected FlowID 2, got %d", t2.FlowID)
	}
	if t3.FlowID != 3 {
		t.Errorf("expected FlowID 3, got %d", t3.FlowID)
	}
}

func TestFlowEventFields(t *testing.T) {
	resetNextFlowID()

	rec := NewFlowRecorder()
	tr := NewTraceWithSinks(rec)

	fields := map[string]interface{}{
		"target":  uint64(12345),
		"damage":  float64(5000.5),
		"crit":    true,
		"school":  "fire",
	}

	e := tr.Event(SpanEffectHit, "deal_damage", uint32(1337), "Fireball", fields)

	if e.FlowID != tr.FlowID {
		t.Errorf("FlowID mismatch: got %d, want %d", e.FlowID, tr.FlowID)
	}
	if e.Span != SpanEffectHit {
		t.Errorf("Span: got %q, want %q", e.Span, SpanEffectHit)
	}
	if e.Event != "deal_damage" {
		t.Errorf("Event: got %q, want %q", e.Event, "deal_damage")
	}
	if e.SpellID != 1337 {
		t.Errorf("SpellID: got %d, want %d", e.SpellID, 1337)
	}
	if e.SpellName != "Fireball" {
		t.Errorf("SpellName: got %q, want %q", e.SpellName, "Fireball")
	}
	if e.Fields == nil {
		t.Fatal("Fields is nil")
	}
	if len(e.Fields) != 4 {
		t.Errorf("Fields length: got %d, want 4", len(e.Fields))
	}
	if e.Timestamp.IsZero() {
		t.Error("Timestamp should not be zero")
	}

	// Verify values in Fields.
	tests := []struct {
		key  string
		want interface{}
	}{
		{"target", uint64(12345)},
		{"damage", float64(5000.5)},
		{"crit", true},
		{"school", "fire"},
	}
	for _, tc := range tests {
		got, ok := e.Fields[tc.key]
		if !ok {
			t.Errorf("missing field %q", tc.key)
			continue
		}
		if got != tc.want {
			t.Errorf("Fields[%q]: got %v (%T), want %v (%T)", tc.key, got, got, tc.want, tc.want)
		}
	}

	// Event was also written to the recorder.
	evts := rec.Events()
	if len(evts) != 1 {
		t.Fatalf("recorder: expected 1 event, got %d", len(evts))
	}
	if evts[0].Event != "deal_damage" {
		t.Errorf("recorder event: got %q, want %q", evts[0].Event, "deal_damage")
	}
}

func TestTraceEventCollectsMultiple(t *testing.T) {
	resetNextFlowID()

	rec := NewFlowRecorder()
	tr := NewTraceWithSinks(rec)

	spans := []string{SpanCheckCast, SpanEffectLaunch, SpanAura}
	events := []string{"check", "launch", "apply"}

	for i := 0; i < 3; i++ {
		tr.Event(spans[i], events[i], uint32(100+i), fmt.Sprintf("Spell%d", i), nil)
	}

	collected := tr.Events()
	if len(collected) != 3 {
		t.Fatalf("expected 3 events, got %d", len(collected))
	}

	for i, e := range collected {
		if e.Span != spans[i] {
			t.Errorf("event %d Span: got %q, want %q", i, e.Span, spans[i])
		}
		if e.Event != events[i] {
			t.Errorf("event %d Event: got %q, want %q", i, e.Event, events[i])
		}
		if e.SpellID != uint32(100+i) {
			t.Errorf("event %d SpellID: got %d, want %d", i, e.SpellID, 100+i)
		}
	}

	// Recorder also collected all 3.
	if got := len(rec.Events()); got != 3 {
		t.Errorf("recorder: expected 3 events, got %d", got)
	}
}

func TestStdoutSinkOutput(t *testing.T) {
	resetNextFlowID()

	rec := NewFlowRecorder()
	tr := NewTraceWithSinks(rec, &StdoutSink{})

	_ = tr.Event(SpanSpell, "cast_start", uint32(42), "Shadowbolt", map[string]interface{}{
		"target": "player1",
	})

	// The StdoutSink already wrote during Event(). We need to capture output.
	// For this test, create a second trace that writes to our capturing sink.
	output := captureStdout(func() {
		tr2 := NewTraceWithSinks(&StdoutSink{})
		_ = tr2.Event(SpanSpell, "cast_start", uint32(42), "Shadowbolt", map[string]interface{}{
			"target": "player1",
		})
	})

	if output == "" {
		t.Fatal("expected non-empty stdout output")
	}

	// Verify the FlowID appears.
	wantPrefix := "[flow-"
	if !strings.Contains(output, wantPrefix) {
		t.Errorf("output missing %q prefix: %s", wantPrefix, output)
	}

	// Verify span and event appear.
	if !strings.Contains(output, "spell.cast_start") {
		t.Errorf("output missing span.event: %s", output)
	}

	// Verify spell info.
	if !strings.Contains(output, "spell=42") {
		t.Errorf("output missing spell ID: %s", output)
	}
	if !strings.Contains(output, "(Shadowbolt)") {
		t.Errorf("output missing spell name: %s", output)
	}

	// Verify field appears.
	if !strings.Contains(output, "target=player1") {
		t.Errorf("output missing field: %s", output)
	}
}

func TestEventsReturnsSnapshot(t *testing.T) {
	resetNextFlowID()

	tr := NewTraceWithSinks() // no sinks
	tr.Event(SpanSpell, "a", 1, "A", nil)
	tr.Event(SpanSpell, "b", 2, "B", nil)

	snap := tr.Events()
	if len(snap) != 2 {
		t.Fatalf("expected 2 events, got %d", len(snap))
	}

	// Mutating the snapshot should not affect the trace.
	snap[0].Event = "MUTATED"
	snap2 := tr.Events()
	if snap2[0].Event == "MUTATED" {
		t.Error("Events() did not return a defensive copy; snapshot mutation affected internal state")
	}
}

func TestAddSink(t *testing.T) {
	resetNextFlowID()

	rec := NewFlowRecorder()
	tr := NewTraceWithSinks() // no sinks initially

	// Event before AddSink -- recorder gets nothing.
	tr.Event(SpanSpell, "before", 1, "A", nil)
	if len(rec.Events()) != 0 {
		t.Fatal("recorder should have no events before AddSink")
	}

	tr.AddSink(rec)
	tr.Event(SpanSpell, "after", 2, "B", nil)

	evts := rec.Events()
	if len(evts) != 1 {
		t.Fatalf("expected 1 event after AddSink, got %d", len(evts))
	}
	if evts[0].Event != "after" {
		t.Errorf("got %q, want %q", evts[0].Event, "after")
	}
}
