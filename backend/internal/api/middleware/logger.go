package middleware

import (
	"bufio"
	"context"
	"log/slog"
	"net"
	"net/http"
	"time"

	chim "github.com/go-chi/chi/v5/middleware"
)

type logCtxKey struct{}

// WithLogger stores log on ctx for handler retrieval.
func WithLogger(ctx context.Context, log *slog.Logger) context.Context {
	return context.WithValue(ctx, logCtxKey{}, log)
}

// LogFrom retrieves the request-scoped logger. Falls back to slog.Default().
func LogFrom(ctx context.Context) *slog.Logger {
	if l, ok := ctx.Value(logCtxKey{}).(*slog.Logger); ok && l != nil {
		return l
	}
	return slog.Default()
}

// responseWriter wraps http.ResponseWriter to capture the written status code.
type responseWriter struct {
	http.ResponseWriter
	status  int
	written bool
}

func (rw *responseWriter) WriteHeader(code int) {
	if !rw.written {
		rw.status = code
		rw.written = true
		rw.ResponseWriter.WriteHeader(code)
	}
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	if !rw.written {
		rw.WriteHeader(http.StatusOK)
	}
	return rw.ResponseWriter.Write(b)
}

// Hijack forwards to the underlying ResponseWriter so WebSocket upgrades work.
func (rw *responseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h, ok := rw.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, http.ErrNotSupported
	}
	return h.Hijack()
}

// RequestLogger logs one structured line per request: method, path, status,
// latency_ms, req_id. It also puts a req_id-tagged logger on the context so
// handlers can call LogFrom(r.Context()) for correlated log lines.
// Must run after chi's RequestID middleware.
func RequestLogger(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			reqID := chim.GetReqID(r.Context())

			reqLog := log.With("req_id", reqID)
			r = r.WithContext(WithLogger(r.Context(), reqLog))

			// Echo the request id in the response so clients can quote it in
			// bug reports / support tickets.
			if reqID != "" {
				w.Header().Set("X-Request-Id", reqID)
			}

			rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(rw, r)

			// LogFrom(r.Context()) here picks up any enrichment that downstream
			// middlewares (like RequireAuth → user_id) added.
			LogFrom(r.Context()).Info("http",
				"method", r.Method,
				"path", r.URL.Path,
				"status", rw.status,
				"latency_ms", time.Since(start).Milliseconds(),
			)
		})
	}
}
