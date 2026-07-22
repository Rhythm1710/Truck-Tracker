package bus

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rhythm/truck-tracker/internal/models"
)

func TestBus_SingleSubscriberReceives(t *testing.T) {
	b := New()
	sub, cancel := b.Subscribe()
	defer cancel()

	go b.Publish(models.Position{IMEI: "imei-a"})

	select {
	case p := <-sub:
		if p.IMEI != "imei-a" {
			t.Fatalf("got imei=%q, want imei-a", p.IMEI)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for position")
	}
}

func TestBus_MultipleSubscribersAllReceive(t *testing.T) {
	b := New()
	sub1, cancel1 := b.Subscribe()
	defer cancel1()
	sub2, cancel2 := b.Subscribe()
	defer cancel2()

	go b.Publish(models.Position{IMEI: "imei-b"})

	for i, ch := range []<-chan models.Position{sub1, sub2} {
		select {
		case p := <-ch:
			if p.IMEI != "imei-b" {
				t.Errorf("sub%d: got %q", i, p.IMEI)
			}
		case <-time.After(time.Second):
			t.Errorf("sub%d: timed out", i)
		}
	}
}

func TestBus_SlowSubscriberDoesNotBlockOthers(t *testing.T) {
	b := New()
	// One slow subscriber that never reads.
	_, cancelSlow := b.Subscribe()
	defer cancelSlow()
	// One fast subscriber.
	fast, cancelFast := b.Subscribe()
	defer cancelFast()

	// Publish more than the buffer's capacity (64). The slow subscriber's
	// channel will fill; Publish must still deliver to the fast one.
	const N = subscriberBuf + 10
	done := make(chan struct{})
	go func() {
		for i := 0; i < N; i++ {
			b.Publish(models.Position{IMEI: "x"})
		}
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Publish blocked — slow subscriber was not isolated")
	}

	// Fast subscriber should have received exactly subscriberBuf or more
	// messages — drain it to verify we got at least some.
	count := 0
loop:
	for {
		select {
		case <-fast:
			count++
		default:
			break loop
		}
	}
	if count == 0 {
		t.Fatal("fast subscriber got nothing")
	}
}

func TestBus_CancelRemovesSubscriber(t *testing.T) {
	b := New()
	sub, cancel := b.Subscribe()
	if b.Len() != 1 {
		t.Fatalf("Len after Subscribe = %d, want 1", b.Len())
	}
	cancel()
	if b.Len() != 0 {
		t.Fatalf("Len after cancel = %d, want 0", b.Len())
	}
	// Reading from closed channel returns zero value with ok=false.
	if p, ok := <-sub; ok {
		t.Fatalf("expected closed channel, got %+v", p)
	}
}

func TestBus_DoubleCancelIsSafe(t *testing.T) {
	b := New()
	_, cancel := b.Subscribe()
	cancel()
	cancel() // should not panic
}

func TestBus_PublishAfterCancelDoesNotPanic(t *testing.T) {
	b := New()
	_, cancel := b.Subscribe()
	cancel()
	// Should not panic; just drops on the floor.
	b.Publish(models.Position{IMEI: "ghost"})
}

func TestBus_ConcurrentPublishSubscribe(t *testing.T) {
	b := New()
	var wg sync.WaitGroup
	var delivered atomic.Int64

	// 4 subscribers that drain.
	for i := 0; i < 4; i++ {
		sub, cancel := b.Subscribe()
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer cancel()
			for range sub {
				delivered.Add(1)
			}
		}()
	}

	// 8 publishers.
	var pubWG sync.WaitGroup
	for i := 0; i < 8; i++ {
		pubWG.Add(1)
		go func() {
			defer pubWG.Done()
			for j := 0; j < 100; j++ {
				b.Publish(models.Position{IMEI: "x"})
			}
		}()
	}
	pubWG.Wait()

	// Cancel subscribers so they return.
	b.mu.Lock()
	for ch := range b.subs {
		delete(b.subs, ch)
		close(ch)
	}
	b.mu.Unlock()
	wg.Wait()

	if delivered.Load() == 0 {
		t.Fatal("nothing delivered under concurrent load")
	}
}
