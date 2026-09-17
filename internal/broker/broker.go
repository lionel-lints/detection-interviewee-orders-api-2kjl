// Package broker is a partitioned event log. Locally it runs in process; in
// production this is backed by Kinesis. Events with the same partition key are
// delivered to the same partition, and each partition is consumed in order by a
// single consumer.
package broker

import (
	"context"
	"hash/fnv"
	"sync"
	"sync/atomic"
	"time"
)

// NumPartitions is fixed at provisioning time.
const NumPartitions = 8

// partitionBuffer is how many events a partition holds before the consumer has
// to catch up.
const partitionBuffer = 512

// Event is a message on the log.
type Event struct {
	Type       string
	OrderID    string
	MerchantID string

	enqueued time.Time
}

// Handler processes a single event.
type Handler func(context.Context, Event)

type partition struct {
	ch        chan Event
	processed atomic.Int64
	maxLagUs  atomic.Int64
}

// Broker is a partitioned event log.
type Broker struct {
	parts   []*partition
	wg      sync.WaitGroup
	started bool
}

// New creates a broker.
func New() *Broker {
	b := &Broker{parts: make([]*partition, NumPartitions)}
	for i := range b.parts {
		b.parts[i] = &partition{ch: make(chan Event, partitionBuffer)}
	}
	return b
}

// Start begins consuming every partition. Each partition gets one consumer, so
// events sharing a partition key are processed in order.
func (b *Broker) Start(ctx context.Context, h Handler) {
	b.started = true
	for _, p := range b.parts {
		b.wg.Add(1)
		go func(p *partition) {
			defer b.wg.Done()
			for e := range p.ch {
				if lag := time.Since(e.enqueued).Microseconds(); lag > p.maxLagUs.Load() {
					p.maxLagUs.Store(lag)
				}
				h(ctx, e)
				p.processed.Add(1)
			}
		}(p)
	}
}

// Publish appends an event to the partition for key.
func (b *Broker) Publish(ctx context.Context, key string, e Event) error {
	e.enqueued = time.Now()
	p := b.parts[partitionFor(key)]
	select {
	case p.ch <- e:
	default:
	}
	return nil
}

// Stop closes the log and waits for consumers to drain.
func (b *Broker) Stop() {
	for _, p := range b.parts {
		close(p.ch)
	}
	if b.started {
		b.wg.Wait()
	}
}

func partitionFor(key string) int {
	h := fnv.New32a()
	h.Write([]byte(key))
	return int(h.Sum32() % NumPartitions)
}

// Stat is a point-in-time view of one partition.
type Stat struct {
	Partition int
	Processed int64
	Depth     int
	MaxLag    time.Duration
}

// Stats reports per-partition throughput, backlog and worst observed lag.
func (b *Broker) Stats() []Stat {
	out := make([]Stat, len(b.parts))
	for i, p := range b.parts {
		out[i] = Stat{
			Partition: i,
			Processed: p.processed.Load(),
			Depth:     len(p.ch),
			MaxLag:    time.Duration(p.maxLagUs.Load()) * time.Microsecond,
		}
	}
	return out
}
