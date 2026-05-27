// Package ws is the WebSocket layer that pushes live Positions to mobile
// clients.
//
// Design:
//   - Hub holds one record per connected client (userID + send channel).
//   - A single fan-out goroutine reads from the Position Bus and writes the
//     position to every client whose user owns that device. (For v1, with one
//     user, every connected client gets every position — we'll add per-user
//     filtering when we get multi-user.)
//   - Each client also runs a read goroutine to detect disconnects.
package ws

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/gorilla/websocket"

	"github.com/rhythm/truck-tracker/internal/auth"
	"github.com/rhythm/truck-tracker/internal/bus"
	"github.com/rhythm/truck-tracker/internal/models"
)

type Hub struct {
	bus    *bus.Bus
	secret string
	log    *slog.Logger
	up     websocket.Upgrader
}

func NewHub(b *bus.Bus, secret string, log *slog.Logger) *Hub {
	return &Hub{
		bus:    b,
		secret: secret,
		log:    log,
		up: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			// TODO: lock down origins in production.
			CheckOrigin: func(*http.Request) bool { return true },
		},
	}
}

type outgoing struct {
	Type string          `json:"type"`
	Data models.Position `json:"data"`
}

// ServeHTTP handles GET /v1/live. Authenticates via Bearer token in the
// `Sec-WebSocket-Protocol` header (since browsers can't set arbitrary headers).
func (h *Hub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Mobile passes the app token as a query param `?token=...`.
	token := r.URL.Query().Get("token")
	claims, err := auth.Verify(h.secret, token)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	c, err := h.up.Upgrade(w, r, nil)
	if err != nil {
		h.log.Error("ws: upgrade", "err", err)
		return
	}
	defer c.Close()

	h.log.Info("ws: client connected", "user", claims.UserID)
	defer h.log.Info("ws: client disconnected", "user", claims.UserID)

	// Subscribe to the position bus.
	sub, cancel := h.bus.Subscribe()
	defer cancel()

	// Read loop — detect disconnect / pong messages.
	closed := make(chan struct{})
	go func() {
		defer close(closed)
		for {
			if _, _, err := c.NextReader(); err != nil {
				return
			}
		}
	}()

	// Ping every 30s.
	pingT := time.NewTicker(30 * time.Second)
	defer pingT.Stop()

	for {
		select {
		case <-closed:
			return
		case <-r.Context().Done():
			return
		case <-pingT.C:
			_ = c.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		case p, ok := <-sub:
			if !ok {
				return
			}
			// TODO: filter — only send positions for trucks this user owns.
			payload, _ := json.Marshal(outgoing{Type: "position", Data: p})
			_ = c.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.WriteMessage(websocket.TextMessage, payload); err != nil {
				return
			}
		}
	}
}

// SubscribeBus starts the store-side subscriber: reads from the bus and
// returns a function to call inside the main goroutine for store writes.
// Wired up in main.go.
func SubscribeBus(ctx context.Context, b *bus.Bus, onPos func(models.Position)) {
	sub, cancel := b.Subscribe()
	defer cancel()
	for {
		select {
		case <-ctx.Done():
			return
		case p, ok := <-sub:
			if !ok {
				return
			}
			onPos(p)
		}
	}
}
