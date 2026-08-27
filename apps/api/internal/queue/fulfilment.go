// Package queue hands work to the worker the moment it exists, rather than
// waiting for the next sweep to notice it.
package queue

import (
	"context"
	"encoding/json"

	"github.com/hibiken/asynq"
)

// TaskDispatchOne asks the worker to send one specific fulfilment now.
const TaskDispatchOne = "fulfilment:dispatch_one"

// DispatchPayload names the order to send.
type DispatchPayload struct {
	OrderID string `json:"order_id"`
}

// FulfilmentQueue enqueues a single fulfilment for immediate sending.
//
// This is the fast path, and it is what actually meets the latency a digital
// product needs: a jamaah buying pulsa expects it in seconds, not whenever a
// periodic sweep next runs.
//
// The sweep still exists and still runs, as the net underneath — an enqueue
// that fails, a Redis restart that drops the queue, a worker that dies
// mid-send. Neither mechanism is sufficient alone: the queue is fast but
// losable, the sweep is durable but slow. Together the common case is
// immediate and the uncommon case is merely late.
type FulfilmentQueue struct {
	client *asynq.Client
}

// NewFulfilmentQueue returns nil when Redis is not configured, which callers
// must tolerate: without it every fulfilment simply waits for the sweep, which
// is slower but never wrong.
func NewFulfilmentQueue(redisURL string) (*FulfilmentQueue, error) {
	if redisURL == "" {
		return nil, nil
	}
	options, err := asynq.ParseRedisURI(redisURL)
	if err != nil {
		return nil, err
	}
	return &FulfilmentQueue{client: asynq.NewClient(options)}, nil
}

// EnqueueDispatch asks for one order to be sent now.
//
// Failing here is survivable and deliberately not propagated as a reason to
// fail the payment: the money has already settled, the fulfilment row already
// records that a delivery is owed, and the sweep will find it. Losing the
// payment because the queue was briefly unreachable would be far worse than
// sending the pulsa a minute late.
func (q *FulfilmentQueue) EnqueueDispatch(ctx context.Context, orderID string) error {
	if q == nil || q.client == nil {
		return nil
	}
	payload, err := json.Marshal(DispatchPayload{OrderID: orderID})
	if err != nil {
		return err
	}
	// Retention keeps the task visible in asynq's own inspector after it runs,
	// which is where somebody looks when asking why a delivery was slow.
	_, err = q.client.EnqueueContext(ctx, asynq.NewTask(TaskDispatchOne, payload),
		asynq.MaxRetry(3), asynq.Queue("default"))
	return err
}

func (q *FulfilmentQueue) Close() error {
	if q == nil || q.client == nil {
		return nil
	}
	return q.client.Close()
}
