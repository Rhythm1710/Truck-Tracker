// Package bus implements an in-process publish/subscribe channel for Positions.
//
// Producers (TCP/protocol layer) call Publish. Consumers (store writer,
// WebSocket hub) call Subscribe to get a receive-only channel of Positions.
//
// Design notes:
//   - Per-subscriber buffered channels (cap 64). If a subscriber stalls, its
//     channel fills and the producer drops the message FOR THAT SUBSCRIBER ONLY.
//     This guarantees one slow consumer can't block others.
//   - Subscribers receive a copy by value (Position is small, copy is cheap).
package bus

import (
	"sync"

	"github.com/rhythm/truck-tracker/internal/models"
)

const subscriberBuf = 64

type Bus struct {
	mu   sync.RWMutex
	subs map[chan models.Position]struct{}
}

func New() *Bus {
	return &Bus{subs: make(map[chan models.Position]struct{})}
}

// Subscribe returns a receive-only channel that delivers every published
// position. The returned cancel function removes the subscriber and closes
// the channel — callers must invoke it to avoid leaks.
func (b *Bus) Subscribe() (<-chan models.Position, func()) {
	ch := make(chan models.Position, subscriberBuf)
	b.mu.Lock()
	b.subs[ch] = struct{}{}
	b.mu.Unlock()

	cancel := func() {
		b.mu.Lock()
		if _, ok := b.subs[ch]; ok {
			delete(b.subs, ch)
			close(ch)
		}
		b.mu.Unlock()
	}
	return ch, cancel
}

// Publish broadcasts a position to all current subscribers. If a subscriber's
// buffer is full, the message is dropped for that subscriber only (never blocks).
func (b *Bus) Publish(p models.Position) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for ch := range b.subs {
		select {
		case ch <- p:
		default:
			// Subscriber is slow — drop for this one and move on.
			// TODO: emit a metric (bus_drop_total{subscriber=...}).
		}
	}
}

// Len returns the current subscriber count — useful for /healthz.
func (b *Bus) Len() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.subs)
}
