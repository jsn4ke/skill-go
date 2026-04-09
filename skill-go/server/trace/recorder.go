package trace

import "sync"

// FlowRecorder captures FlowEvents for test assertions.
type FlowRecorder struct {
	events []FlowEvent
	mu     sync.Mutex
}

// NewFlowRecorder creates a FlowRecorder.
func NewFlowRecorder() *FlowRecorder {
	return &FlowRecorder{}
}

// Write appends a FlowEvent (implements TraceSink).
func (r *FlowRecorder) Write(e FlowEvent) {
	r.mu.Lock()
	r.events = append(r.events, e)
	r.mu.Unlock()
}

// Events returns a snapshot of all captured events.
func (r *FlowRecorder) Events() []FlowEvent {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]FlowEvent, len(r.events))
	copy(out, r.events)
	return out
}

// BySpan returns events matching the given span.
func (r *FlowRecorder) BySpan(span string) []FlowEvent {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []FlowEvent
	for _, e := range r.events {
		if e.Span == span {
			out = append(out, e)
		}
	}
	return out
}

// ByEvent returns events matching the given event name.
func (r *FlowRecorder) ByEvent(event string) []FlowEvent {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []FlowEvent
	for _, e := range r.events {
		if e.Event == event {
			out = append(out, e)
		}
	}
	return out
}

// HasEvent returns true if any event matches the given span and event.
func (r *FlowRecorder) HasEvent(span, event string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, e := range r.events {
		if e.Span == span && e.Event == event {
			return true
		}
	}
	return false
}

// Count returns the number of events matching the given span and event.
// Empty strings match all.
func (r *FlowRecorder) Count(span, event string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := 0
	for _, e := range r.events {
		if (span == "" || e.Span == span) && (event == "" || e.Event == event) {
			n++
		}
	}
	return n
}

// Reset clears all captured events.
func (r *FlowRecorder) Reset() {
	r.mu.Lock()
	r.events = nil
	r.mu.Unlock()
}
