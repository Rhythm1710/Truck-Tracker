package ws

import (
	"encoding/json"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/rhythm/truck-tracker/internal/bus"
	"github.com/rhythm/truck-tracker/internal/models"
)

func testHub(t *testing.T) (*Hub, *bus.Bus) {
	t.Helper()
	b := bus.New()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewHub(b, "test-secret", log, nil, nil), b
}

func newClient(devices ...string) *client {
	set := make(map[string]struct{}, len(devices))
	for _, id := range devices {
		set[id] = struct{}{}
	}
	return &client{
		send:      make(chan []byte, clientSendBuf),
		deviceIDs: set,
	}
}

func drain(ch <-chan []byte) ([]byte, bool) {
	select {
	case payload, ok := <-ch:
		return payload, ok
	default:
		return nil, false
	}
}

func TestFanoutOnlyOwnedTrucks(t *testing.T) {
	h, _ := testHub(t)
	alice := newClient("truck-a")
	bob := newClient("truck-b")
	h.register(alice)
	h.register(bob)
	t.Cleanup(func() {
		h.unregister(alice)
		h.unregister(bob)
	})

	const payload = `{"type":"position","device":"a"}`
	h.fanout("truck-a", []byte(payload))

	got, ok := drain(alice.send)
	if !ok || string(got) != payload {
		t.Fatalf("alice got %q ok=%v", got, ok)
	}
	if extra, ok := drain(bob.send); ok {
		t.Fatalf("bob received a truck he does not own: %s", extra)
	}
}

func TestFanoutSharedTruckReachesEveryOwner(t *testing.T) {
	h, _ := testHub(t)
	a := newClient("shared", "other")
	b := newClient("shared")
	stranger := newClient()
	h.register(a)
	h.register(b)
	h.register(stranger)
	t.Cleanup(func() {
		h.unregister(a)
		h.unregister(b)
		h.unregister(stranger)
	})

	h.fanout("shared", []byte("p"))

	if _, ok := drain(a.send); !ok {
		t.Fatal("owner a missed the shared truck")
	}
	if _, ok := drain(b.send); !ok {
		t.Fatal("owner b missed the shared truck")
	}
	if extra, ok := drain(stranger.send); ok {
		t.Fatalf("client with no trucks received %s", extra)
	}
}

func TestFanoutEmptyDeviceIDMatchesNobody(t *testing.T) {
	h, _ := testHub(t)
	// A blank key must not become a wildcard.
	blank := &client{send: make(chan []byte, 1), deviceIDs: map[string]struct{}{"": {}}}
	owner := newClient("truck-a")
	nilSet := &client{send: make(chan []byte, 1)}
	h.register(blank)
	h.register(owner)
	h.register(nilSet)
	t.Cleanup(func() {
		h.unregister(blank)
		h.unregister(owner)
		h.unregister(nilSet)
	})

	h.fanout("", []byte("orphan"))
	h.fanout("truck-a", []byte("owned"))

	if extra, ok := drain(blank.send); ok {
		t.Fatalf("blank device id was delivered: %s", extra)
	}
	if extra, ok := drain(nilSet.send); ok {
		t.Fatalf("nil ownership set received %s", extra)
	}
	got, ok := drain(owner.send)
	if !ok || string(got) != "owned" {
		t.Fatalf("owner got %q ok=%v", got, ok)
	}
}

func TestFanoutFullBufferDoesNotBlockOtherOwners(t *testing.T) {
	h, _ := testHub(t)
	slow := &client{send: make(chan []byte, 1), deviceIDs: map[string]struct{}{"truck-a": {}}}
	fast := newClient("truck-a")
	slow.send <- []byte("already-full")
	h.register(slow)
	h.register(fast)
	t.Cleanup(func() {
		h.unregister(slow)
		h.unregister(fast)
	})

	done := make(chan struct{})
	go func() {
		h.fanout("truck-a", []byte("next"))
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("fanout blocked on a full client buffer")
	}
	got, ok := drain(fast.send)
	if !ok || string(got) != "next" {
		t.Fatalf("fast owner got %q ok=%v", got, ok)
	}
}

func TestBusLoopFiltersByDeviceID(t *testing.T) {
	h, b := testHub(t)
	alice := newClient("dev-alice")
	bob := newClient("dev-bob")
	h.register(alice)
	h.register(bob)
	t.Cleanup(func() {
		h.unregister(alice)
		h.unregister(bob)
	})

	h.startBusReader()
	deadline := time.Now().Add(2 * time.Second)
	for b.Len() < 1 {
		if time.Now().After(deadline) {
			t.Fatal("bus reader did not subscribe")
		}
		time.Sleep(5 * time.Millisecond)
	}

	b.Publish(models.Position{
		DeviceID: "dev-alice",
		IMEI:     "not-the-routing-key",
		Lat:      12.9,
		Lng:      77.6,
	})

	var raw []byte
	select {
	case raw = <-alice.send:
	case <-time.After(2 * time.Second):
		t.Fatal("alice did not receive her truck")
	}

	var msg struct {
		Type string `json:"type"`
		Data struct {
			DeviceID string  `json:"deviceId"`
			Lat      float64 `json:"lat"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &msg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if msg.Type != "position" || msg.Data.DeviceID != "dev-alice" || msg.Data.Lat != 12.9 {
		t.Fatalf("payload %+v", msg)
	}

	time.Sleep(30 * time.Millisecond)
	if extra, ok := drain(bob.send); ok {
		t.Fatalf("bob received %s", extra)
	}

	b.Publish(models.Position{DeviceID: "dev-nobody", Lat: 1})
	select {
	case extra := <-alice.send:
		t.Fatalf("alice received an unowned position %s", extra)
	case extra := <-bob.send:
		t.Fatalf("bob received an unowned position %s", extra)
	case <-time.After(150 * time.Millisecond):
	}
}
