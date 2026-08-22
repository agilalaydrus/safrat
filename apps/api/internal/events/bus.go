// Package events is an in-process pub/sub broker for the operator
// monitoring dashboard's real-time stream (MonitoringService.StreamEvents).
//
// Deliberately in-memory, not Redis — same call as the rate limiter in
// internal/middleware/ratelimit.go ("fine for the single-API-instance
// deployment in DEPLOY.md; move to Redis if the API ever runs more than
// one replica"). If this API is ever horizontally scaled, a subscriber
// connected to instance A will miss events published on instance B —
// switch Bus to a Redis pub/sub-backed implementation at that point,
// keeping the same Publish/Subscribe interface so callers don't change.
package events

import (
	"sync"
	"time"
)

type Event struct {
	Type      string    `json:"type"`
	EntityID  string    `json:"entity_id"`
	CreatedAt time.Time `json:"created_at"`
}

// maxSubscribersPerOperator caps concurrent monitoring-dashboard viewers per
// operator — a resource-exhaustion guard (each subscriber holds a goroutine
// + buffered channel open for the lifetime of the connection), not a
// realistic usage ceiling for how many staff would watch this dashboard at
// once.
const maxSubscribersPerOperator = 20

// eventBufferSize is how many events queue per subscriber before Publish
// starts dropping — the frontend only uses events as a signal to refetch a
// snapshot, so losing one under a burst is harmless (the next event, or the
// periodic heartbeat, triggers the same refetch).
const eventBufferSize = 8

type Bus struct {
	mu   sync.Mutex
	subs map[string]map[chan Event]struct{} // operatorID -> set of subscriber channels
}

func NewBus() *Bus {
	return &Bus{subs: make(map[string]map[chan Event]struct{})}
}

// Subscribe returns a channel of events for this operator and an
// unsubscribe func the caller must defer — ok is false (channel is nil)
// if this operator already has maxSubscribersPerOperator open streams.
func (b *Bus) Subscribe(operatorID string) (ch chan Event, unsubscribe func(), ok bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	set, exists := b.subs[operatorID]
	if !exists {
		set = make(map[chan Event]struct{})
		b.subs[operatorID] = set
	}
	if len(set) >= maxSubscribersPerOperator {
		return nil, nil, false
	}
	ch = make(chan Event, eventBufferSize)
	set[ch] = struct{}{}
	unsubscribe = func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		if s, ok := b.subs[operatorID]; ok {
			delete(s, ch)
			if len(s) == 0 {
				delete(b.subs, operatorID)
			}
		}
		close(ch)
	}
	return ch, unsubscribe, true
}

// Publish is non-blocking — a slow/stalled subscriber never backs up the
// service call that triggered the event (see eventBufferSize: a full
// channel just drops the event for that one subscriber). A nil *Bus is a
// valid no-op, same defensive-nil pattern as PushNotifier/SOSNotifier —
// callers never need to nil-check before calling.
func (b *Bus) Publish(operatorID, eventType, entityID string) {
	if b == nil {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	for ch := range b.subs[operatorID] {
		select {
		case ch <- Event{Type: eventType, EntityID: entityID, CreatedAt: time.Now()}:
		default:
		}
	}
}
