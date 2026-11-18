// Package ws is the WebSocket layer that pushes live Positions to mobile
// clients.
//
// Design:
//   - One hub-side goroutine reads the Position Bus.
//   - It marshals each Position once and fans the bytes out to every
//     client whose owned-device set contains the position's DeviceID.
//   - Each client has its own buffered send chan (capacity 64). A slow
//     client drops messages for itself without blocking the hub.
//   - Each client also has a read goroutine that detects disconnects.
//   - The client's API token expiry is enforced server-side: when the
//     token would expire, we close the socket — mobile reconnects with
//     a refreshed token via /v1/auth/exchange.
package ws

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"github.com/rhythm/truck-tracker/internal/auth"
	"github.com/rhythm/truck-tracker/internal/bus"
	"github.com/rhythm/truck-tracker/internal/metrics"
	"github.com/rhythm/truck-tracker/internal/models"
	"github.com/rhythm/truck-tracker/internal/store"
)

const (
	clientSendBuf   = 64
	writeDeadline   = 10 * time.Second
	pingInterval    = 30 * time.Second
	pongWait        = 60 * time.Second
	defaultMaxMsgSz = 4 * 1024
	// How often a live socket reloads the trucks its user owns.
	deviceRefreshInterval = 60 * time.Second
)

// ---------------------------------------------------------------------------
// Hub
// ---------------------------------------------------------------------------

type Hub struct {
	bus     *bus.Bus
	store   *store.Store
	secret  string
	log     *slog.Logger
	metrics *metrics.Metrics

	up websocket.Upgrader

	clientsMu sync.RWMutex
	clients   map[*client]struct{}

	started sync.Once

	// listDeviceIDs reloads ownership. Nil in tests that do not refresh.
	// Production sets it from the store.
	listDeviceIDs deviceIDLister
	// deviceRefresh is how often refreshDevicesLoop reloads ownership.
	// Zero means deviceRefreshInterval.
	deviceRefresh time.Duration
}

// deviceIDLister returns the device ids owned by a user.
type deviceIDLister func(ctx context.Context, ownerID string) ([]string, error)

func NewHub(b *bus.Bus, secret string, log *slog.Logger, st *store.Store, m *metrics.Metrics) *Hub {
	h := &Hub{
		bus:           b,
		store:         st,
		secret:        secret,
		log:           log,
		metrics:       m,
		clients:       make(map[*client]struct{}),
		deviceRefresh: deviceRefreshInterval,
		up: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			// TODO: lock down origins in production.
			CheckOrigin: func(*http.Request) bool { return true },
		},
	}
	if st != nil {
		h.listDeviceIDs = st.ListUserDeviceIDs
	}
	return h
}

// startBusReader spins up the single fan-out goroutine, lazily on first
// connect so empty deployments don't keep a goroutine around.
func (h *Hub) startBusReader() {
	h.started.Do(func() {
		go h.busLoop()
	})
}

func (h *Hub) busLoop() {
	sub, cancel := h.bus.Subscribe()
	defer cancel()
	for p := range sub {
		payload, err := json.Marshal(outgoing{Type: "position", Data: p})
		if err != nil {
			h.log.Error("ws: marshal", "err", err)
			continue
		}
		h.fanout(p.DeviceID, payload)
	}
}

// fanout delivers payload to every client that owns the given deviceID.
// Read-locked iteration; per-client send uses a non-blocking select.
func (h *Hub) fanout(deviceID string, payload []byte) {
	h.clientsMu.RLock()
	defer h.clientsMu.RUnlock()
	for c := range h.clients {
		if !c.ownsDevice(deviceID) {
			continue
		}
		select {
		case c.send <- payload:
		default:
			// Client is slow. Don't block.
			if h.metrics != nil {
				h.metrics.WSDropTotal.Inc()
			}
		}
	}
}

func (h *Hub) register(c *client) {
	h.clientsMu.Lock()
	h.clients[c] = struct{}{}
	h.clientsMu.Unlock()
	if h.metrics != nil {
		h.metrics.WSClients.Inc()
	}
}

func (h *Hub) unregister(c *client) {
	h.clientsMu.Lock()
	if _, ok := h.clients[c]; ok {
		delete(h.clients, c)
		close(c.send) // safe — only hub writes to c.send
	}
	h.clientsMu.Unlock()
	if h.metrics != nil {
		h.metrics.WSClients.Dec()
	}
}

// ServeHTTP handles GET /v1/live.
//
// Auth: mobile passes the app token as a query param `?token=...` because
// browsers can't set arbitrary headers on a WebSocket upgrade.
func (h *Hub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	claims, err := auth.Verify(h.secret, token)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Look up the devices this user owns so we can filter broadcasts.
	deviceIDList, err := h.store.ListUserDeviceIDs(r.Context(), claims.UserID)
	if err != nil {
		h.log.Error("ws: list devices", "user", claims.UserID, "err", err)
		http.Error(w, "internal", http.StatusInternalServerError)
		return
	}
	deviceIDs := make(map[string]struct{}, len(deviceIDList))
	for _, id := range deviceIDList {
		deviceIDs[id] = struct{}{}
	}

	conn, err := h.up.Upgrade(w, r, nil)
	if err != nil {
		h.log.Error("ws: upgrade", "err", err)
		return
	}

	h.startBusReader()

	c := &client{
		conn:      conn,
		send:      make(chan []byte, clientSendBuf),
		deviceIDs: deviceIDs,
		userID:    claims.UserID,
	}
	h.register(c)
	defer h.unregister(c)
	defer conn.Close()

	refreshCtx, stopRefresh := context.WithCancel(context.Background())
	defer stopRefresh()
	go h.refreshDevicesLoop(refreshCtx, c)

	h.log.Info("ws: client connected", "user", claims.UserID, "devices", len(deviceIDs))
	defer h.log.Info("ws: client disconnected", "user", claims.UserID)

	// Pong handler keeps the read loop alive.
	conn.SetReadLimit(defaultMaxMsgSz)
	_ = conn.SetReadDeadline(time.Now().Add(pongWait))
	conn.SetPongHandler(func(string) error {
		_ = conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	// Enforce token expiry — when the token would expire, close the socket
	// so the mobile client refreshes and reconnects.
	ttl := time.Until(time.Unix(claims.ExpiresAt, 0))
	if ttl > 0 {
		t := time.AfterFunc(ttl, func() {
			h.log.Info("ws: token expired, closing", "user", claims.UserID)
			_ = conn.Close()
		})
		defer t.Stop()
	}

	go c.readLoop()
	c.writeLoop(h.metrics)
}

// ---------------------------------------------------------------------------
// client
// ---------------------------------------------------------------------------

type outgoing struct {
	Type string          `json:"type"`
	Data models.Position `json:"data"`
}

type client struct {
	conn      *websocket.Conn
	send      chan []byte
	mu        sync.RWMutex
	deviceIDs map[string]struct{}
	userID    string
}

// ownsDevice reports whether this client should receive a position for
// deviceID. An empty id matches nobody — there is no truck to attribute
// it to — and a nil or empty ownership set matches nobody either.
func (c *client) ownsDevice(deviceID string) bool {
	if c == nil || deviceID == "" {
		return false
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	if len(c.deviceIDs) == 0 {
		return false
	}
	_, ok := c.deviceIDs[deviceID]
	return ok
}

// setDeviceIDs replaces the ownership set. Blank ids are ignored so they
// cannot become a wildcard.
func (c *client) setDeviceIDs(ids []string) {
	next := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		if id == "" {
			continue
		}
		next[id] = struct{}{}
	}
	c.mu.Lock()
	c.deviceIDs = next
	c.mu.Unlock()
}

// refreshClientDevices reloads this client's trucks from the store.
// A lookup error leaves the previous set in place.
func (h *Hub) refreshClientDevices(ctx context.Context, c *client) error {
	if h == nil || h.listDeviceIDs == nil || c == nil {
		return nil
	}
	ids, err := h.listDeviceIDs(ctx, c.userID)
	if err != nil {
		return err
	}
	c.setDeviceIDs(ids)
	return nil
}

// refreshDevicesLoop reloads ownership until ctx is cancelled.
func (h *Hub) refreshDevicesLoop(ctx context.Context, c *client) {
	if h == nil || h.listDeviceIDs == nil || c == nil {
		return
	}
	interval := h.deviceRefresh
	if interval <= 0 {
		interval = deviceRefreshInterval
	}
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if err := h.refreshClientDevices(ctx, c); err != nil && h.log != nil {
				h.log.Error("ws: refresh devices", "user", c.userID, "err", err)
			}
		}
	}
}

// readLoop discards inbound frames (we don't expect any) but maintains
// the read deadline via PongHandler. Returns on any error → triggers
// writeLoop teardown via send chan close.
func (c *client) readLoop() {
	for {
		if _, _, err := c.conn.NextReader(); err != nil {
			return
		}
	}
}

func (c *client) writeLoop(m *metrics.Metrics) {
	pingT := time.NewTicker(pingInterval)
	defer pingT.Stop()

	for {
		select {
		case payload, ok := <-c.send:
			if !ok {
				return
			}
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeDeadline))
			if err := c.conn.WriteMessage(websocket.TextMessage, payload); err != nil {
				return
			}
			if m != nil {
				m.WSMessages.Inc()
			}
		case <-pingT.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeDeadline))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// SubscribeBus starts the store-side subscriber: reads from the bus and
// calls onPos for every position. Wired up in main.go.
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
