// Package events is a pub/sub broker for the operator monitoring dashboard's
// real-time stream (MonitoringService.StreamEvents).
//
// Two interchangeable backends behind the same Publish/Subscribe interface:
//
//   - NewBus() — in-memory, single-process. Fine for a one-replica deployment;
//     a subscriber on instance A never sees events published on instance B.
//   - NewRedisBus(rdb) — Redis pub/sub. Publish on any replica reaches
//     subscribers on every replica, so the API can scale horizontally. The
//     server picks this automatically when REDIS_URL is set.
//
// Callers hold a *Bus and don't care which backend is active — exactly the
// "keep the same interface so callers don't change" migration the in-memory
// version was written to allow.
package events

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
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
	mu     sync.Mutex
	subs   map[string]map[chan Event]struct{} // operatorID -> set of subscriber channels (in-memory backend)
	counts map[string]int                     // operatorID -> subscriber count (redis backend cap)
	rdb    *redis.Client                      // nil => in-memory backend
}

func NewBus() *Bus {
	return &Bus{subs: make(map[string]map[chan Event]struct{})}
}

// NewRedisBus returns a Redis-pub/sub-backed bus — cross-replica delivery,
// same interface as NewBus.
func NewRedisBus(rdb *redis.Client) *Bus {
	return &Bus{subs: make(map[string]map[chan Event]struct{}), counts: make(map[string]int), rdb: rdb}
}

func channelKey(operatorID string) string { return "events:" + operatorID }

// Subscribe returns a channel of events for this operator and an
// unsubscribe func the caller must defer — ok is false (channel is nil)
// if this operator already has maxSubscribersPerOperator open streams.
func (b *Bus) Subscribe(operatorID string) (ch chan Event, unsubscribe func(), ok bool) {
	if b.rdb != nil {
		return b.subscribeRedis(operatorID)
	}
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

// subscribeRedis backs Subscribe when a Redis client is configured. The reader
// goroutine is the sole owner of ch — the only sender and the only closer — so
// unsubscribe (which just cancels the context) can never race a send onto a
// closed channel.
func (b *Bus) subscribeRedis(operatorID string) (chan Event, func(), bool) {
	b.mu.Lock()
	if b.counts[operatorID] >= maxSubscribersPerOperator {
		b.mu.Unlock()
		return nil, nil, false
	}
	b.counts[operatorID]++
	b.mu.Unlock()

	ctx, cancel := context.WithCancel(context.Background())
	pubsub := b.rdb.Subscribe(ctx, channelKey(operatorID))
	ch := make(chan Event, eventBufferSize)

	go func() {
		defer close(ch)
		defer func() { _ = pubsub.Close() }()
		redisCh := pubsub.Channel()
		for {
			select {
			case <-ctx.Done():
				return
			case msg, open := <-redisCh:
				if !open {
					return
				}
				var ev Event
				if err := json.Unmarshal([]byte(msg.Payload), &ev); err != nil {
					continue
				}
				select {
				case ch <- ev:
				case <-ctx.Done():
					return
				default:
					// Buffer full — drop, same loss-tolerant semantics as the
					// in-memory path (events are just refetch signals).
				}
			}
		}
	}()

	var once sync.Once
	unsubscribe := func() {
		once.Do(func() {
			cancel()
			b.mu.Lock()
			b.counts[operatorID]--
			if b.counts[operatorID] <= 0 {
				delete(b.counts, operatorID)
			}
			b.mu.Unlock()
		})
	}
	return ch, unsubscribe, true
}

// Publish is non-blocking — a slow/stalled subscriber (in-memory) or a slow
// Redis (redis backend) never backs up the service call that triggered the
// event. A nil *Bus is a valid no-op, matching the notifier pattern — callers
// never need to nil-check before calling.
func (b *Bus) Publish(operatorID, eventType, entityID string) {
	if b == nil {
		return
	}
	if b.rdb != nil {
		payload, err := json.Marshal(Event{Type: eventType, EntityID: entityID, CreatedAt: time.Now()})
		if err != nil {
			return
		}
		// Short timeout so a slow/unreachable Redis can't stall the request.
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = b.rdb.Publish(ctx, channelKey(operatorID), payload).Err()
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
