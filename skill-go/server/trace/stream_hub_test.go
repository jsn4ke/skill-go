package trace

import (
	"sync"
	"testing"
	"time"
)

func TestStreamHub_SubscriberReceivesEvents(t *testing.T) {
	hub := NewStreamHub(100)
	sub := hub.Subscribe()

	e := FlowEvent{FlowID: 1, Span: "spell", Event: "prepare", SpellID: 1001}
	hub.Publish(e)

	select {
	case got := <-sub.Events():
		if got.Span != "spell" || got.Event != "prepare" {
			t.Errorf("unexpected event: %s.%s", got.Span, got.Event)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for event")
	}

	hub.Unsubscribe(sub)
}

func TestStreamHub_MultipleSubscribers(t *testing.T) {
	hub := NewStreamHub(100)
	sub1 := hub.Subscribe()
	sub2 := hub.Subscribe()

	e := FlowEvent{FlowID: 1, Span: "spell", Event: "prepare"}
	hub.Publish(e)

	for i, sub := range []*Subscriber{sub1, sub2} {
		select {
		case <-sub.Events():
			// ok
		case <-time.After(time.Second):
			t.Fatalf("subscriber %d did not receive event", i)
		}
	}

	hub.Unsubscribe(sub1)
	hub.Unsubscribe(sub2)
}

func TestStreamHub_UnsubscribeStopsEvents(t *testing.T) {
	hub := NewStreamHub(100)
	sub := hub.Subscribe()

	hub.Publish(FlowEvent{FlowID: 1, Span: "spell", Event: "prepare"})
	// Drain the first event
	<-sub.Events()

	hub.Unsubscribe(sub)

	// Channel should be closed
	_, ok := <-sub.Events()
	if ok {
		t.Error("expected channel to be closed after unsubscribe")
	}
}

func TestStreamHub_RingBufferOverflow(t *testing.T) {
	capacity := 10
	hub := NewStreamHub(capacity)

	for i := 0; i < 15; i++ {
		hub.Publish(FlowEvent{FlowID: uint64(i + 1), Span: "spell", Event: "test"})
	}

	events := hub.Query(0, "", 100)
	if len(events) != capacity {
		t.Fatalf("expected %d events, got %d", capacity, len(events))
	}
	// Should have events 6-15 (the last 10)
	if events[0].FlowID != 6 {
		t.Errorf("expected first event FlowID=6, got %d", events[0].FlowID)
	}
}

func TestStreamHub_QueryByFlowID(t *testing.T) {
	hub := NewStreamHub(100)

	hub.Publish(FlowEvent{FlowID: 1, Span: "spell", Event: "prepare"})
	hub.Publish(FlowEvent{FlowID: 1, Span: "effect_hit", Event: "damage"})
	hub.Publish(FlowEvent{FlowID: 2, Span: "spell", Event: "prepare"})

	events := hub.Query(1, "", 100)
	if len(events) != 2 {
		t.Fatalf("expected 2 events for flow_id=1, got %d", len(events))
	}
	for _, e := range events {
		if e.FlowID != 1 {
			t.Errorf("unexpected FlowID: %d", e.FlowID)
		}
	}
}

func TestStreamHub_QueryBySpan(t *testing.T) {
	hub := NewStreamHub(100)

	hub.Publish(FlowEvent{FlowID: 1, Span: "spell", Event: "prepare"})
	hub.Publish(FlowEvent{FlowID: 1, Span: "effect_hit", Event: "damage"})
	hub.Publish(FlowEvent{FlowID: 2, Span: "cooldown", Event: "add_cooldown"})

	events := hub.Query(0, "effect_hit", 100)
	if len(events) != 1 {
		t.Fatalf("expected 1 event for span=effect_hit, got %d", len(events))
	}
	if events[0].Span != "effect_hit" {
		t.Errorf("unexpected span: %s", events[0].Span)
	}
}

func TestStreamHub_QueryLimit(t *testing.T) {
	hub := NewStreamHub(100)

	for i := 0; i < 20; i++ {
		hub.Publish(FlowEvent{FlowID: uint64(i + 1), Span: "spell", Event: "test"})
	}

	events := hub.Query(0, "", 5)
	if len(events) != 5 {
		t.Fatalf("expected 5 events with limit=5, got %d", len(events))
	}
	// Should be the last 5
	if events[0].FlowID != 16 {
		t.Errorf("expected first event FlowID=16, got %d", events[0].FlowID)
	}
}

func TestStreamHub_Clear(t *testing.T) {
	hub := NewStreamHub(100)

	hub.Publish(FlowEvent{FlowID: 1, Span: "spell", Event: "prepare"})
	hub.Publish(FlowEvent{FlowID: 2, Span: "spell", Event: "hit"})

	hub.Clear()

	events := hub.Query(0, "", 100)
	if len(events) != 0 {
		t.Fatalf("expected 0 events after clear, got %d", len(events))
	}
}

func TestStreamHub_ConcurrentPublish(t *testing.T) {
	hub := NewStreamHub(1000)
	sub := hub.Subscribe()

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id uint64) {
			defer wg.Done()
			hub.Publish(FlowEvent{FlowID: id, Span: "spell", Event: "test"})
		}(uint64(i + 1))
	}
	wg.Wait()

	// Give subscriber time to receive
	time.Sleep(50 * time.Millisecond)

	// Drain subscriber
	count := 0
	for {
		select {
		case <-sub.Events():
			count++
		default:
			goto done
		}
	}
done:

	if count == 0 {
		t.Error("subscriber received no events from concurrent publishes")
	}

	hub.Unsubscribe(sub)
}
