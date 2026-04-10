package trace

import (
	"sync"
)

// Subscriber receives FlowEvents from the StreamHub.
type Subscriber struct {
	ch chan FlowEvent
}

// Events returns the channel for receiving events.
func (s *Subscriber) Events() <-chan FlowEvent {
	return s.ch
}

// StreamHub fans out FlowEvents to multiple subscribers and stores them in a ring buffer.
type StreamHub struct {
	mu          sync.RWMutex
	subscribers map[*Subscriber]struct{}

	bufMu    sync.Mutex
	buf      []FlowEvent
	bufStart int
	bufLen   int
	bufCap   int
}

// NewStreamHub creates a StreamHub with a ring buffer of the given capacity.
func NewStreamHub(bufferCap int) *StreamHub {
	return &StreamHub{
		subscribers: make(map[*Subscriber]struct{}),
		bufCap:      bufferCap,
		buf:         make([]FlowEvent, bufferCap),
	}
}

// Subscribe registers a new subscriber and returns it.
func (h *StreamHub) Subscribe() *Subscriber {
	s := &Subscriber{ch: make(chan FlowEvent, 256)}
	h.mu.Lock()
	h.subscribers[s] = struct{}{}
	h.mu.Unlock()
	return s
}

// Unsubscribe removes a subscriber and closes its channel.
func (h *StreamHub) Unsubscribe(s *Subscriber) {
	h.mu.Lock()
	delete(h.subscribers, s)
	h.mu.Unlock()
	close(s.ch)
}

// Publish sends an event to all subscribers and adds it to the ring buffer.
func (h *StreamHub) Publish(e FlowEvent) {
	// Ring buffer
	h.bufMu.Lock()
	h.buf[(h.bufStart+h.bufLen)%h.bufCap] = e
	if h.bufLen < h.bufCap {
		h.bufLen++
	} else {
		h.bufStart = (h.bufStart + 1) % h.bufCap
	}
	h.bufMu.Unlock()

	// Fan out to subscribers (non-blocking)
	h.mu.RLock()
	defer h.mu.RUnlock()
	for s := range h.subscribers {
		select {
		case s.ch <- e:
		default:
			// drop if subscriber is slow
		}
	}
}

// Query returns events from the ring buffer, optionally filtered by flowID and/or span.
// limit controls the maximum number of results (0 = use default 100).
func (h *StreamHub) Query(flowID uint64, span string, limit int) []FlowEvent {
	if limit <= 0 {
		limit = 100
	}

	h.bufMu.Lock()
	events := make([]FlowEvent, h.bufLen)
	for i := 0; i < h.bufLen; i++ {
		events[i] = h.buf[(h.bufStart+i)%h.bufCap]
	}
	h.bufMu.Unlock()

	// Filter
	filtered := events[:0]
	for _, e := range events {
		if flowID != 0 && e.FlowID != flowID {
			continue
		}
		if span != "" && e.Span != span {
			continue
		}
		filtered = append(filtered, e)
	}

	// Return most recent `limit` events
	if len(filtered) > limit {
		filtered = filtered[len(filtered)-limit:]
	}
	return filtered
}

// Clear empties the ring buffer.
func (h *StreamHub) Clear() {
	h.bufMu.Lock()
	h.bufStart = 0
	h.bufLen = 0
	h.bufMu.Unlock()
}

// ClearSubscribers removes all subscribers and closes their channels.
func (h *StreamHub) ClearSubscribers() {
	h.mu.Lock()
	for s := range h.subscribers {
		close(s.ch)
		delete(h.subscribers, s)
	}
	h.mu.Unlock()
}

// StreamSink is a TraceSink that publishes events to a StreamHub.
type StreamSink struct {
	hub *StreamHub
}

// NewStreamSink creates a StreamSink that publishes to the given hub.
func NewStreamSink(hub *StreamHub) *StreamSink {
	return &StreamSink{hub: hub}
}

// Write publishes the event to the StreamHub.
func (s *StreamSink) Write(e FlowEvent) {
	s.hub.Publish(e)
}
