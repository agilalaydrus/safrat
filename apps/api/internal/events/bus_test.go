package events

import (
	"os"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

// TestRedisBusCrossInstance proves the whole point of the Redis backend: a
// Publish on one *Bus (simulating API replica A) reaches a subscriber on a
// separate *Bus with its own client (replica B) via shared Redis. Skipped
// unless REDIS_TEST_URL is set — never point it at a Redis you care about.
func TestRedisBusCrossInstance(t *testing.T) {
	url := os.Getenv("REDIS_TEST_URL")
	if url == "" {
		t.Skip("REDIS_TEST_URL not set")
	}
	optA, err := redis.ParseURL(url)
	if err != nil {
		t.Fatalf("parse url: %v", err)
	}
	optB, _ := redis.ParseURL(url)
	rdbA := redis.NewClient(optA)
	rdbB := redis.NewClient(optB)
	defer rdbA.Close()
	defer rdbB.Close()

	publisher := NewRedisBus(rdbA)  // replica A
	subscriber := NewRedisBus(rdbB) // replica B

	op := "op-test-" + time.Now().Format("150405.000000")
	ch, unsub, ok := subscriber.Subscribe(op)
	if !ok {
		t.Fatal("subscribe returned ok=false")
	}
	defer unsub()

	// Redis SUBSCRIBE is asynchronous — give it a moment to register before
	// publishing, or the message races ahead of the subscription.
	time.Sleep(250 * time.Millisecond)

	publisher.Publish(op, "health", "entity-123")

	select {
	case ev := <-ch:
		if ev.Type != "health" || ev.EntityID != "entity-123" {
			t.Fatalf("unexpected event: %+v", ev)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for cross-instance event")
	}
}
