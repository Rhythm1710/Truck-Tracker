# ADR-001 — Truck Tracker End-to-End Architecture

**Status:** Accepted
**Owner:** Rhythm Arora
**Context:** Simple fleet-tracking app for a father with limited tech literacy. Fleet size 1–5 trucks. Hardware already purchased: Pictor PS10a and PT06 GPS trackers (no Pictor SIM, no Pictor API access).

---

## 1. Goals & Non-Goals

**Goals**
- One person (the father) can see live location & status of every truck on a phone.
- Both Android and iOS supported from one codebase.
- Senior-friendly UI: big text, high contrast, English, minimal screens.
- Real-time updates (sub-10-second latency) when a truck moves.
- Cost cap: under ₹1,000/month operating budget for a 1–5 truck fleet.

**Non-Goals (v1)**
- Multi-tenant / multi-customer SaaS.
- Driver app, dispatcher console, web dashboard.
- Trip history analytics, fuel tracking, geofencing — deferred to v2.
- High availability / multi-region failover.

---

## 2. High-Level Architecture

```
Trackers → Cellular → Go server (single binary) → Mobile app
                          │
                          ├── TCP listener (:5023)        ──┐
                          ├── HTTP API (:8080)              │ in-process
                          ├── WebSocket hub (:8080/v1/live) │ channels
                          ├── Position Bus                  ─┘
                          └── PostgreSQL store
```

Three goroutine-based services share one process, one Postgres database, and an in-process pub/sub bus implemented with Go channels.

---

## 3. Decisions

### 3.1 Mobile — React Native + TypeScript
**Decision:** RN + TS, Mappls SDK for maps, Firebase Phone Auth for OTP, Zustand for app state, React Query for server state.
**Why:** Single codebase ships iOS + Android. Engineer-friendly stack for a coding engineer. Mappls has an official RN SDK with Indian map tiles + place data.
**Trade-offs:** Slightly less polished than fully native; native modules sometimes need patching on RN upgrades.

### 3.2 Backend — Custom Go server
**Decision:** Single Go binary, multi-goroutine, no microservices.
**Why:** Go's concurrency model (goroutine per device connection) maps perfectly onto a TCP fan-in workload. Single binary deployment is operationally trivial. Binary protocol parsing is native and fast.
**Alternative rejected:** Traccar (open-source GPS platform) — would have saved 3–4 weeks but the engineer wanted to own the stack and learn the protocol layer.

### 3.3 HTTP router — chi
**Decision:** `github.com/go-chi/chi/v5`.
**Why:** Stdlib-compatible, tiny, idiomatic Go middleware story, no `Context` lock-in. Lets us swap to stdlib net/http later with near-zero changes.

### 3.4 Database — PostgreSQL + pgx + sqlc
**Decision:** Postgres for storage, `pgx` as the driver, `sqlc` to generate type-safe Go from SQL.
**Why:** sqlc gives us compile-time-checked queries without an ORM's runtime overhead. pgx is the fastest Postgres driver for Go and supports native Postgres types. Postgres handles the time-series workload fine for a small fleet; we'll add a BRIN index on `positions.recv_at` and partition monthly only if data volume warrants.
**Migrations:** `golang-migrate` with numbered SQL files in `/migrations`.

### 3.5 WebSocket — `gorilla/websocket`
**Decision:** Use `github.com/gorilla/websocket`.
**Why:** Most-used Go WebSocket lib, stable, simple API. The hub pattern with per-user fan-out is well-documented.

### 3.6 Auth — Firebase Phone Auth + server-side ID-token verification
**Decision:** Mobile uses Firebase Phone Auth for OTP. Client sends the Firebase ID token to `POST /v1/auth/exchange`. Server verifies the token using `firebase.google.com/go/v4` Admin SDK, then issues its own short-lived API token (JWT) that the client uses for all subsequent calls.
**Why:** Free tier (10K verifications/mo) covers a single-fleet app many times over. Server-issued token avoids verifying Firebase JWTs on every request and gives us tenant-aware claims.

### 3.7 GPS protocol — TBD (likely GT06 / Concox family)
**Decision:** Awaiting docs from Pictor support for PS10a and PT06. Most Chinese-origin trackers in this product class speak the GT06 family, which has well-documented public specs.
**Approach:** Implement `internal/protocol/gt06` first with a clean `Decoder`/`Encoder` interface. If Pictor uses a variant, we add `internal/protocol/pictor` behind the same interface and pick based on the device's login packet signature.

### 3.8 In-process Position Bus over external queues
**Decision:** Implement pub/sub as a Go channel-based hub inside the same binary.
**Why:** A single-fleet workload doesn't justify Redis/NATS/Kafka. One channel, two subscribers (store + WebSocket). If we later split into multiple binaries, we swap the bus implementation behind an interface.

### 3.9 Hosting — TBD
**Decision:** Deferred. Constraints recorded: must have a static public IP, must allow inbound TCP on a custom port (e.g., 5023), India-region preferred for low latency to trackers.
**Candidates:** DigitalOcean Bangalore, Hetzner Mumbai, Hostinger India.

---

## 4. Data Model

```sql
users         (id, phone, name, created_at)
devices       (id, imei, plate, label, owner_id → users.id, created_at)
positions     (id, device_id → devices.id, lat, lng, speed_kmh, heading,
               ignition, recv_at, device_at)
device_sessions (id, device_id, connected_at, disconnected_at, last_ip)
```

Indexes: `positions(device_id, recv_at DESC)` for "latest by truck", BRIN on `positions.recv_at` for history queries, unique on `devices.imei`.

---

## 5. API Contract

```
POST  /v1/auth/exchange       Body {firebaseIdToken} → {apiToken, expiresAt}
GET   /v1/trucks              → [{id, plate, label, latest: Position}]
GET   /v1/trucks/:id          → {device, latest: Position}
GET   /v1/trucks/:id/history  ?from=&to=&limit=  → [Position]
WS    /v1/live                Auth via apiToken; messages:
                              { type: "position", deviceId, lat, lng,
                                speed, heading, ignition, at }
```

All bodies are JSON. All timestamps are RFC 3339 UTC. WebSocket reconnects with exponential backoff client-side.

---

## 6. Concurrency & Failure Model

- **Goroutine per tracker connection.** TCP listener accepts, spawns a handler goroutine that owns the connection until disconnect.
- **Position Bus is bounded.** Channel with capacity 256. If WebSocket fan-out blocks, store still writes; if store blocks, WebSocket still delivers. Slow consumer drops oldest, never the producer.
- **Graceful shutdown.** `context.Context` is plumbed from `main` through every goroutine. SIGTERM cancels the context; listeners stop accepting new work and existing goroutines finish their current message.
- **DB outage.** Position writes go to a small bounded in-memory queue; WebSocket continues. Once Postgres recovers, queue is drained. After overflow, oldest positions are dropped with a metric incremented.

---

## 7. Open Items

| # | Item | Owner | Blocker |
|---|------|-------|---------|
| 1 | Get Pictor PS10a / PT06 protocol spec | Rhythm | Pictor support |
| 2 | Choose VPS provider | Rhythm | Decision deferred |
| 3 | Configure data SIM in each tracker, point it at server IP via SMS commands | Rhythm | Has the VPS first |
| 4 | Firebase project setup (Phone Auth enabled, India region) | Rhythm | Quick task |
| 5 | Mappls developer account + API key for RN SDK | Rhythm | Quick task |

---

## 8. Future Changes Anticipated

- **v2 — Trip history & daily summaries.** Already shaped in the schema (`positions` table is the source of truth).
- **v2 — Push notifications** (ignition, overspeed, geofence). Will add `events` table and a notifier goroutine subscribed to the Position Bus.
- **v2 — Call Driver button** on Truck Detail screen. Pure mobile change; backend adds `drivers` table with phone numbers per device.
- **Scale out (>50 trucks).** Replace in-process bus with Redis pub/sub; split TCP listener into its own binary.
