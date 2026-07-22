// Package bus implements an in-process publish/subscribe channel for Positions.
//
// Producers (TCP/protocol layer) call Publish. Consumers (store writer,
// WebSocket hub) call Subscribe to get a receive-only channel of Positions.
//
// Concurrency safety proof:
//   - Subscribe/cancel mutate the subscribers map under Lock (exclusive).
//   - Publish reads the map under RLock and sends into channels.
//   - A cancel cannot delete+close ch while Publish holds RLock because
//     Lock waits for all RLocks to release. So Publish is guaranteed not
//     to send on a closed channel.
//   - A slow subscriber's buffered channel may fill; Publish uses select
//     with default to drop FOR THAT SUBSCRIBER ONLY, never blocking other
//     subscribers or the producer.
package bus

import (
	"sync"

	"github.com/rhythm/truck-tracker/internal/metrics"
	"github.com/rhythm/truck-tracker/internal/models"
)

const subscriberBuf = 64

type Bus struct {
	mu      sync.RWMutex
	subs    map[chan models.Position]struct{}
	metrics *metrics.Metrics // optional; nil-safe
}

// New returns a bus with no metrics. Tests use this.
func New() *Bus {
	return &Bus{subs: make(map[chan models.Position]struct{})}
}

// NewWithMetrics returns a bus that increments the supplied counters.
func NewWithMetrics(m *metrics.Metrics) *Bus {
	return &Bus{
		subs:    make(map[chan models.Position]struct{}),
		metrics: m,
	}
}

// Subscribe returns a receive-only channel that delivers every published
// position. The returned cancel function removes the subscriber and closes
// the channel — callers must invoke it (defer cancel()) to avoid leaks.
func (b *Bus) Subscribe() (<-chan models.Position, func()) {
	ch := make(chan models.Position, subscriberBuf)
	b.mu.Lock()
	b.subs[ch] = struct{}{}
	b.mu.Unlock()

	var once sync.Once
	cancel := func() {
		once.Do(func() {
			b.mu.Lock()
			if _, ok := b.subs[ch]; ok {
				delete(b.subs, ch)
				close(ch)
			}
			b.mu.Unlock()
		})
	}
	return ch, cancel
}

// Publish broadcasts a position to all current subscribers. Never blocks:
// a slow subscriber's message is dropped (per-subscriber).
func (b *Bus) Publish(p models.Position) {
	if b.metrics != nil {
		b.metrics.BusPublishTotal.WithLabelValues("position").Inc()
	}
	b.mu.RLock()
	defer b.mu.RUnlock()
	for ch := range b.subs {
		select {
		case ch <- p:
		default:
			if b.metrics != nil {
				b.metrics.BusDropTotal.Inc()
			}
		}
	}
}

// Len returns the current subscriber count — useful for /healthz.
func (b *Bus) Len() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.subs)
}
