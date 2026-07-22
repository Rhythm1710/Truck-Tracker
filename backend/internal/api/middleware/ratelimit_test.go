package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRateLimiter_AllowsUpToBurst_ThenDenies(t *testing.T) {
	// 0.01 rps = 1 token per 100s — effectively only the burst.
	rl := NewRateLimiter(0.01, 3, time.Minute)
	defer rl.Stop()

	handler := rl.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	doReq := func() int {
		r := httptest.NewRequest("GET", "/", nil)
		r.RemoteAddr = "1.2.3.4:5555"
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w.Code
	}

	// First 3 should pass (burst).
	for i := 0; i < 3; i++ {
		if code := doReq(); code != http.StatusOK {
			t.Errorf("req %d: code=%d want 200", i+1, code)
		}
	}
	// 4th should be limited.
	if code := doReq(); code != http.StatusTooManyRequests {
		t.Errorf("4th req: code=%d want 429", code)
	}
}

func TestRateLimiter_PerIP_Isolated(t *testing.T) {
	rl := NewRateLimiter(0.01, 1, time.Minute)
	defer rl.Stop()
	handler := rl.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	for _, ip := range []string{"1.1.1.1:1", "2.2.2.2:2", "3.3.3.3:3"} {
		r := httptest.NewRequest("GET", "/", nil)
		r.RemoteAddr = ip
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != http.StatusOK {
			t.Errorf("ip %s: first req should pass, got %d", ip, w.Code)
		}
	}
}
