// Package fulfilment stands in for the fulfilment service. It consumes events
// off the broker and calls the (simulated) downstream fulfilment API.
package fulfilment

import (
	"context"
	"sync"
	"time"

	"orders-api/internal/broker"
)

// downstreamCall is how long the real fulfilment API takes to accept one event.
const downstreamCall = 4 * time.Millisecond

// Consumer applies events to the fulfilment system.
type Consumer struct {
	mu     sync.Mutex
	seen   map[string]struct{}
	counts map[string]int

	processed int
}

// New creates a consumer.
func New() *Consumer {
	return &Consumer{
		seen:   make(map[string]struct{}),
		counts: make(map[string]int),
	}
}

// Handle applies one event. Redelivery is expected, so already-applied events
// are skipped.
func (c *Consumer) Handle(ctx context.Context, e broker.Event) {
	key := e.Type + ":" + e.OrderID

	c.mu.Lock()
	if _, dup := c.seen[key]; dup {
		c.mu.Unlock()
		return
	}
	c.seen[key] = struct{}{}
	c.counts[e.Type]++
	c.mu.Unlock()

	time.Sleep(downstreamCall)
	c.processed++
}

// Counts returns how many events of each type have been applied.
func (c *Consumer) Counts() map[string]int {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make(map[string]int, len(c.counts))
	for k, v := range c.counts {
		out[k] = v
	}
	return out
}

// Processed returns the total number of events applied.
func (c *Consumer) Processed() int { return c.processed }
