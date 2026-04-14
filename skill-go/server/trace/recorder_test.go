package trace

import (
	"testing"
)

func makeEvent(span, event string, spellID uint32) FlowEvent {
	return FlowEvent{
		FlowID:    1,
		Span:      span,
		Event:     event,
		SpellID:   spellID,
		SpellName: "TestSpell",
		Fields:    nil,
	}
}

func TestBySpan(t *testing.T) {
	rec := NewFlowRecorder()

	rec.Write(makeEvent(SpanSpell, "cast_start", 1))
	rec.Write(makeEvent(SpanCheckCast, "ok", 1))
	rec.Write(makeEvent(SpanSpell, "cast_end", 1))
	rec.Write(makeEvent(SpanAura, "apply", 2))

	tests := []struct {
		name string
		span string
		want int
	}{
		{"spell events", SpanSpell, 2},
		{"checkcast events", SpanCheckCast, 1},
		{"aura events", SpanAura, 1},
		{"non-existent span", "nonexistent", 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := rec.BySpan(tc.span)
			if len(got) != tc.want {
				t.Errorf("BySpan(%q): got %d events, want %d", tc.span, len(got), tc.want)
			}
			// Verify all returned events match the span.
			for _, e := range got {
				if e.Span != tc.span {
					t.Errorf("BySpan(%q): returned event with span %q", tc.span, e.Span)
				}
			}
		})
	}
}

func TestByEvent(t *testing.T) {
	rec := NewFlowRecorder()

	rec.Write(makeEvent(SpanSpell, "cast_start", 1))
	rec.Write(makeEvent(SpanCheckCast, "ok", 1))
	rec.Write(makeEvent(SpanSpell, "cast_start", 2))
	rec.Write(makeEvent(SpanAura, "apply", 3))

	tests := []struct {
		name    string
		event   string
		want    int
		wantIDs []uint32
	}{
		{"cast_start", "cast_start", 2, []uint32{1, 2}},
		{"ok", "ok", 1, []uint32{1}},
		{"apply", "apply", 1, []uint32{3}},
		{"non-existent event", "nonexistent", 0, nil},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := rec.ByEvent(tc.event)
			if len(got) != tc.want {
				t.Errorf("ByEvent(%q): got %d events, want %d", tc.event, len(got), tc.want)
				return
			}
			for i, e := range got {
				if i < len(tc.wantIDs) && e.SpellID != tc.wantIDs[i] {
					t.Errorf("ByEvent(%q)[%d].SpellID: got %d, want %d",
						tc.event, i, e.SpellID, tc.wantIDs[i])
				}
				if e.Event != tc.event {
					t.Errorf("ByEvent(%q): returned event with Event %q", tc.event, e.Event)
				}
			}
		})
	}
}

func TestHasEvent(t *testing.T) {
	rec := NewFlowRecorder()

	rec.Write(makeEvent(SpanSpell, "cast_start", 1))
	rec.Write(makeEvent(SpanCheckCast, "ok", 1))
	rec.Write(makeEvent(SpanAura, "apply", 2))

	tests := []struct {
		name  string
		span  string
		event string
		want  bool
	}{
		{"existing spell.cast_start", SpanSpell, "cast_start", true},
		{"existing checkcast.ok", SpanCheckCast, "ok", true},
		{"existing aura.apply", SpanAura, "apply", true},
		{"wrong span", SpanAura, "cast_start", false},
		{"wrong event", SpanSpell, "apply", false},
		{"both wrong", "nonexistent", "nonexistent", false},
		{"empty span", "", "cast_start", false},
		{"empty event", SpanSpell, "", false},
		{"both empty", "", "", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := rec.HasEvent(tc.span, tc.event)
			if got != tc.want {
				t.Errorf("HasEvent(%q, %q): got %v, want %v",
					tc.span, tc.event, got, tc.want)
			}
		})
	}
}

func TestCount(t *testing.T) {
	rec := NewFlowRecorder()

	rec.Write(makeEvent(SpanSpell, "cast_start", 1))
	rec.Write(makeEvent(SpanCheckCast, "ok", 1))
	rec.Write(makeEvent(SpanSpell, "cast_start", 2))
	rec.Write(makeEvent(SpanAura, "apply", 3))
	rec.Write(makeEvent(SpanAura, "remove", 3))

	tests := []struct {
		name  string
		span  string
		event string
		want  int
	}{
		{"span only - spell", SpanSpell, "", 2},
		{"span only - aura", SpanAura, "", 2},
		{"span only - nonexistent", "nonexistent", "", 0},
		{"event only - cast_start", "", "cast_start", 2},
		{"event only - ok", "", "ok", 1},
		{"event only - nonexistent", "", "nonexistent", 0},
		{"both - spell+cast_start", SpanSpell, "cast_start", 2},
		{"both - aura+apply", SpanAura, "apply", 1},
		{"both - mismatch", SpanSpell, "apply", 0},
		{"both empty - total", "", "", 5},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := rec.Count(tc.span, tc.event)
			if got != tc.want {
				t.Errorf("Count(%q, %q): got %d, want %d",
					tc.span, tc.event, got, tc.want)
			}
		})
	}
}

func TestCountEmpty(t *testing.T) {
	rec := NewFlowRecorder()

	tests := []struct {
		name  string
		span  string
		event string
		want  int
	}{
		{"total", "", "", 0},
		{"span only", SpanSpell, "", 0},
		{"event only", "", "cast_start", 0},
		{"both", SpanSpell, "cast_start", 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := rec.Count(tc.span, tc.event)
			if got != tc.want {
				t.Errorf("Count(%q, %q): got %d, want %d",
					tc.span, tc.event, got, tc.want)
			}
		})
	}
}

func TestReset(t *testing.T) {
	rec := NewFlowRecorder()

	rec.Write(makeEvent(SpanSpell, "cast_start", 1))
	rec.Write(makeEvent(SpanAura, "apply", 2))

	if got := len(rec.Events()); got != 2 {
		t.Fatalf("before reset: expected 2 events, got %d", got)
	}

	rec.Reset()

	if got := len(rec.Events()); got != 0 {
		t.Errorf("after reset: expected 0 events, got %d", got)
	}

	// Verify recorder is still usable after reset.
	rec.Write(makeEvent(SpanSpell, "new_event", 3))
	if got := len(rec.Events()); got != 1 {
		t.Errorf("after reset+write: expected 1 event, got %d", got)
	}
	if got := rec.Events()[0].Event; got != "new_event" {
		t.Errorf("after reset+write: got event %q, want %q", got, "new_event")
	}
}

func TestRecorderEventsReturnsSnapshot(t *testing.T) {
	rec := NewFlowRecorder()

	rec.Write(makeEvent(SpanSpell, "a", 1))
	rec.Write(makeEvent(SpanSpell, "b", 2))

	snap := rec.Events()
	if len(snap) != 2 {
		t.Fatalf("expected 2 events, got %d", len(snap))
	}

	// Mutating the snapshot must not affect the recorder.
	snap[0].Event = "MUTATED"
	snap2 := rec.Events()
	if snap2[0].Event == "MUTATED" {
		t.Error("Events() did not return a defensive copy")
	}
}
