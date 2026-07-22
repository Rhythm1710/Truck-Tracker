// Per-IP token-bucket rate limiter for hot/public endpoints (auth/exchange).
//
// Trade-offs:
//   - In-memory map keyed by IP; one limiter per IP, GC'd after idle window.
//   - No distributed coordination — fine for a single-instance deploy.
//     For multi-instance scale-out, swap to Redis-backed.
package middleware

import (
	"encoding/json"
	"net"
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type clientLimiter struct {
	lim  *rate.Limiter
	seen time.Time
}

type RateLimiter struct {
	mu       sync.Mutex
	clients  map[string]*clientLimiter
	rps      rate.Limit
	burst    int
	idleTTL  time.Duration
	stopOnce sync.Once
	stop     chan struct{}
}

// NewRateLimiter creates a per-IP limiter that allows `rps` requests per
// second with a burst of `burst`. Stale entries are GC'd after `idleTTL`
// of inactivity.
func NewRateLimiter(rps float64, burst int, idleTTL time.Duration) *RateLimiter {
	rl := &RateLimiter{
		clients: make(map[string]*clientLimiter),
		rps:     rate.Limit(rps),
		burst:   burst,
		idleTTL: idleTTL,
		stop:    make(chan struct{}),
	}
	go rl.gcLoop()
	return rl
}

// Stop ends the background GC goroutine.
func (rl *RateLimiter) Stop() {
	rl.stopOnce.Do(func() { close(rl.stop) })
}

func (rl *RateLimiter) gcLoop() {
	t := time.NewTicker(rl.idleTTL)
	defer t.Stop()
	for {
		select {
		case <-rl.stop:
			return
		case <-t.C:
			cutoff := time.Now().Add(-rl.idleTTL)
			rl.mu.Lock()
			for ip, c := range rl.clients {
				if c.seen.Before(cutoff) {
					delete(rl.clients, ip)
				}
			}
			rl.mu.Unlock()
		}
	}
}

func (rl *RateLimiter) limiterFor(ip string) *rate.Limiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	c, ok := rl.clients[ip]
	if !ok {
		c = &clientLimiter{lim: rate.NewLimiter(rl.rps, rl.burst)}
		rl.clients[ip] = c
	}
	c.seen = time.Now()
	return c.lim
}

// Middleware returns a net/http middleware that 429s when the per-IP bucket
// is exhausted.
func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := clientIP(r)
		if !rl.limiterFor(ip).Allow() {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Retry-After", "5")
			w.WriteHeader(http.StatusTooManyRequests)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "rate limited"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

// clientIP extracts the best-guess client IP. chi's RealIP middleware
// already updates r.RemoteAddr from X-Forwarded-For/X-Real-IP when present.
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
