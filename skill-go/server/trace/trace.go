package trace

import (
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

// Span constants identify subsystems in the spell flow.
const (
	SpanSpell        = "spell"
	SpanCheckCast    = "checkcast"
	SpanCooldown     = "cooldown"
	SpanEffectLaunch = "effect_launch"
	SpanEffectHit    = "effect_hit"
	SpanAura         = "aura"
	SpanProc         = "proc"
	SpanTargeting    = "targeting"
	SpanScript       = "script"
	SpanCombat       = "combat"
)

// FlowEvent represents a single event in the spell flow trace.
type FlowEvent struct {
	FlowID    uint64
	Timestamp time.Time
	Span      string
	Event     string
	SpellID   uint32
	SpellName string
	Fields    map[string]interface{}
}

// TraceSink receives FlowEvents.
type TraceSink interface {
	Write(event FlowEvent)
}

// Trace holds the event log for a single spell cast.
type Trace struct {
	FlowID uint64
	events []FlowEvent
	mu     sync.Mutex
	sinks  []TraceSink
}

var nextFlowID uint64

// NewTrace creates a new Trace with an auto-incremented FlowID and a StdoutSink.
func NewTrace() *Trace {
	id := atomic.AddUint64(&nextFlowID, 1)
	return &Trace{
		FlowID: id,
		sinks:  []TraceSink{&StdoutSink{}},
	}
}

// NewTraceWithSinks creates a Trace with custom sinks (no stdout by default).
func NewTraceWithSinks(sinks ...TraceSink) *Trace {
	id := atomic.AddUint64(&nextFlowID, 1)
	return &Trace{
		FlowID: id,
		sinks:  sinks,
	}
}

// Event records a flow event and writes it to all sinks.
// If t is nil, this is a no-op.
func (t *Trace) Event(span, event string, spellID uint32, spellName string, fields map[string]interface{}) FlowEvent {
	if t == nil {
		return FlowEvent{}
	}
	e := FlowEvent{
		FlowID:    t.FlowID,
		Timestamp: time.Now(),
		Span:      span,
		Event:     event,
		SpellID:   spellID,
		SpellName: spellName,
		Fields:    fields,
	}

	t.mu.Lock()
	t.events = append(t.events, e)
	t.mu.Unlock()

	for _, sink := range t.sinks {
		sink.Write(e)
	}

	return e
}

// Events returns a snapshot of all recorded events.
func (t *Trace) Events() []FlowEvent {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := make([]FlowEvent, len(t.events))
	copy(out, t.events)
	return out
}

// AddSink appends an additional sink to the trace.
func (t *Trace) AddSink(sink TraceSink) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.sinks = append(t.sinks, sink)
}

// StdoutSink formats FlowEvents to stdout.
type StdoutSink struct{}

func (s *StdoutSink) Write(e FlowEvent) {
	ts := e.Timestamp.Format("15:04:05.000")
	fmt.Fprintf(os.Stdout, "[flow-%05d] %s %s.%s | spell=%d(%s)",
		e.FlowID, ts, e.Span, e.Event, e.SpellID, e.SpellName)
	for k, v := range e.Fields {
		fmt.Fprintf(os.Stdout, " %s=%v", k, v)
	}
	fmt.Fprintln(os.Stdout)
}
