# Truck Tracker — Optimization & UX Plan

Status: Draft proposal, no code changed yet.
Scope: **No new features.** Everything below sharpens what already exists.
Audit basis: full read of all 17 Go files + 13 TS/TSX files in `~/Truck-Tracker/`.

Each item is labeled:

- **Effort** — S (≤30 min), M (1–2 hr), L (½ day+).
- **Impact** — 🟥 high (correctness/UX-blocking), 🟧 medium (clear win, not urgent), 🟨 low (polish).
- **File** — primary file to touch.

Recommended sequencing is at the end.

---

## 1. Correctness bugs to fix first

### 1.1 🟥 `scanTruckRow` scans `REAL` columns into `*float64` — likely pgx type mismatch
- **File:** `backend/internal/store/store.go` lines 229–246
- **Detail:** `speed_kmh` and `heading` are `REAL` (float32) in the schema, but we scan into `*float64`. pgx v5 is strict about numeric widths and will return a "cannot scan REAL into *float64" error at first use. Same risk on `lat`/`lng` (those are DOUBLE PRECISION → `*float64` is correct), but for the REALs change to `*float32` then assign with explicit conversion to `float64` when populating `models.Position`.
- **Effort:** S

### 1.2 🟥 Bus drops positions if the store-writer falls behind, but TCP path keeps acking
- **File:** `backend/internal/bus/bus.go` + `backend/cmd/server/main.go`
- **Detail:** When the store-writer goroutine stalls (e.g., Postgres pause), the bus drops the message *for the store subscriber* — we lose that position permanently. The tracker has already received an ACK and won't retransmit. Trade-off was intentional ("never block producer"), but the store path deserves a small bounded retry queue with WAL-on-disk semantics, or at minimum a dropped-position metric and a louder log line.
- **Effort:** M (queue) or S (just metric + log)

### 1.3 🟥 WebSocket broadcasts ignore ownership
- **File:** `backend/internal/ws/hub.go` lines 102–113 (`TODO: filter`)
- **Detail:** Every connected client receives every position regardless of who owns the device. With one user it's invisible; the day you onboard a second user it becomes a data leak. Fix now by checking `claims.UserID == device.OwnerID` for each broadcast (cache device→owner in memory; invalidate on device CRUD).
- **Effort:** M

### 1.4 🟧 `cancel` in `bus.Subscribe` closes channels — but `Publish` may have a buffered send pending
- **File:** `backend/internal/bus/bus.go`
- **Detail:** Sequence is safe due to RWLock pairing, but the implicit guarantee is non-obvious. Add a comment block explicitly proving the race-freedom, or switch to "drain then close" semantics where `cancel` marks subscriber dead and a periodic janitor closes channels.
- **Effort:** S

### 1.5 🟧 Login frame's IMEI never tags the TCP-server logger
- **File:** `backend/internal/tcp/server.go` line 91
- **Detail:** We only set `imeiTagged = true` on `FramePosition`. But IMEI is known on `FrameLogin` first. Move the tagging block to run whenever `frame.IMEI` is set and not yet tagged. (Improves debuggability — current logs show "anon" connections until the first position lands.)
- **Effort:** S

### 1.6 🟧 `useEffect` in `<Map>` re-runs on every render
- **File:** `mobile/src/screens/TruckDetail.tsx` line 110
- **Detail:** `coords` is a fresh array (`[truck.latest.lng, truck.latest.lat]`) on every parent render, so `useEffect([coords, followLive])` fires even when lat/lng haven't changed. Pass `lat` and `lng` as separate primitive deps, OR memoize the tuple with `useMemo`.
- **Effort:** S

### 1.7 🟧 Map follow-mode never disengages on user pan
- **File:** `mobile/src/screens/TruckDetail.tsx` lines 107–122
- **Detail:** `followLive` starts `true` and stays `true`. When the user pans/zooms, the next position update snaps the camera back to the truck, which feels broken. Wire `MapView.onRegionDidChange` (or the Mappls equivalent) → on user-gesture changes, `setFollowLive(false)`. The existing recenter FAB already re-engages it.
- **Effort:** S

### 1.8 🟨 `useMemo` imported but unused in `TruckDetail.tsx`
- **File:** `mobile/src/screens/TruckDetail.tsx` line 8
- **Detail:** Strict TS / lint flag would catch this. Remove or use.
- **Effort:** S

### 1.9 🟨 `var _ = models.Position{}` dead-import sentinel
- **File:** `backend/internal/api/router.go` last line
- **Detail:** Leftover from when handlers were stubs. Models is referenced via `store.GetTruck`'s return type — the import isn't actually unused. Delete the sentinel.
- **Effort:** S

---

## 2. Performance — backend

### 2.1 🟧 JSON-marshal once per Publish, not per WebSocket client
- **File:** `backend/internal/ws/hub.go` lines 109
- **Detail:** With N clients, current code does N marshals of the same position. Move marshaling into a single fan-out goroutine in the Hub that subscribes to the bus, marshals once, then writes the same `[]byte` to each connection via per-connection write queues.
- **Effort:** M
- **Wins:** ~70% CPU saved on broadcast for ≥3 connected clients.

### 2.2 🟧 Position writes go one-at-a-time
- **File:** `backend/internal/store/store.go` `SavePosition`
- **Detail:** Replace with a batch writer: collect positions in a 100-msg buffer with a 500ms timer, flush via `pgx.Batch` or COPY. For 5 trucks @ 5s cadence this is unnecessary; for any growth it matters and the change is small.
- **Effort:** M
- **Wins:** Reduces Postgres round-trips ~10× under load.

### 2.3 🟧 IMEI→DeviceID lookup runs on every position
- **File:** `backend/internal/store/store.go` `SavePosition`, `backend/internal/tcp/server.go` `handle`
- **Detail:** The TCP handler knows the IMEI on login. Resolve to `device_id` once per connection and stamp every subsequent `Position.DeviceID` before publishing onto the bus. Store can then skip the SELECT.
- **Effort:** S

### 2.4 🟧 Bus iteration under RLock
- **File:** `backend/internal/bus/bus.go`
- **Detail:** `Publish` walks `subs` map while holding the read lock. Switch to copy-on-write semantics: keep `subs` as an `atomic.Pointer[[]chan Position]`. Subscribe/cancel build a new slice + CAS. Publish reads atomically with zero locking. Worth it only if QPS climbs.
- **Effort:** M (defer until scale demands it).

### 2.5 🟨 Prepared statements for hot queries
- **File:** `backend/internal/store/store.go`
- **Detail:** pgx caches prepared statements per-connection; verify it's enabled (`pgxpool.Config.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol` *disables* it). We don't set it, so default mode preps automatically. Just confirm — and if we move to sqlc, this is moot.

### 2.6 🟨 HTTP server lacks `IdleTimeout` and `ReadHeaderTimeout`
- **File:** `backend/cmd/server/main.go` lines 94–99
- **Detail:** Without `ReadHeaderTimeout` (slowloris) and `IdleTimeout` (zombie keep-alives), an attacker can park sockets. Add both at 30s / 120s.
- **Effort:** S

### 2.7 🟨 `http.MaxBytesReader` on `/auth/exchange`
- **File:** `backend/internal/api/router.go`
- **Detail:** A 10GB POST body would chew memory. Cap at 8KB.
- **Effort:** S

---

## 3. Performance — mobile

### 3.1 🟧 Two WebSocket connections open simultaneously when on TruckDetail
- **File:** `mobile/src/hooks/useLivePositions.ts` + `mobile/src/screens/{Home,TruckDetail}.tsx`
- **Detail:** Home calls `useLivePositions()`. When you navigate to TruckDetail, *that* screen also calls `useLivePositions()`. Stack navigator keeps Home mounted underneath, so we now have **two open WebSockets pumping identical data**. Double bandwidth + battery.
- **Fix:** Hoist the WebSocket into a `LivePositionsProvider` mounted once at the navigator root. Screens become passive consumers of the react-query cache (they're already that today, the hook is just plumbing).
- **Effort:** M

### 3.2 🟧 WebSocket stays open while app is backgrounded
- **File:** `mobile/src/hooks/useLivePositions.ts`
- **Detail:** Subscribe to `AppState`: on `'background'`/`'inactive'` close the socket and stop reconnect attempts; on `'active'` reopen. Material battery + data savings, especially on Android where the OS won't always tear down sockets aggressively.
- **Effort:** M

### 3.3 🟧 Home uses `ScrollView` + `.map`, not `FlatList`
- **File:** `mobile/src/screens/Home.tsx` line 88
- **Detail:** For 1–5 trucks, ScrollView is fine. For any forward-thinking (and to get pull-to-refresh + recycling for free), swap to `FlatList`. Wrap `TruckCard` in `React.memo` so unchanged cards skip re-render when one card's position updates.
- **Effort:** S

### 3.4 🟨 `getMappls()` re-evaluated each render
- **File:** `mobile/src/screens/TruckDetail.tsx` line 105
- **Detail:** Hoist the SDK reference to module scope (or `useMemo([], ...)`) — `getMappls()` already memoizes internally, but the import resolution still happens each render. Tiny.
- **Effort:** S

### 3.5 🟨 `statusFromPosition` recomputed per render in every TruckCard
- **File:** `mobile/src/components/TruckCard.tsx`
- **Detail:** Wrap card body in `React.memo` with a custom comparator on `truck.device.id` + `truck.latest?.at`. Skips re-rendering 4 stable cards when only 1 truck just moved.
- **Effort:** S

---

## 4. UX — senior-friendliness

The whole product is for a father who is "less knowledgeable about tech." These are the items that compound that comfort.

### 4.1 🟥 OTP autofill / SMS retriever not wired
- **File:** `mobile/src/screens/Login.tsx`
- **Detail:** On iOS, `TextInput` with `textContentType="oneTimeCode"` enables the QuickType bar to surface the code from Messages — one tap. On Android, integrate `react-native-otp-verify` (uses Google's SMS Retriever API; doesn't need READ_SMS permission). Without these, your father is manually copying digits.
- **Effort:** M

### 4.2 🟥 OTP doesn't auto-submit when the 6th digit is typed
- **File:** `mobile/src/screens/Login.tsx` `handleVerify`
- **Detail:** Once `otp.length === 6`, call `handleVerify()` automatically. One less button press.
- **Effort:** S

### 4.3 🟥 Phone input doesn't auto-focus
- **File:** `mobile/src/screens/Login.tsx`
- **Detail:** Add `autoFocus` to the phone TextInput. Keyboard appears immediately. Saves a tap and feels responsive.
- **Effort:** S

### 4.4 🟥 No connectivity indicator on Home
- **File:** `mobile/src/screens/Home.tsx`
- **Detail:** If the WebSocket is disconnected (no internet, server down), cards silently freeze. Add a slim banner: "🟢 Live" when connected, "🟠 Reconnecting…" otherwise. Track WS state in the provider from 3.1.
- **Effort:** S

### 4.5 🟧 OTP screen uses one long text input — use a 6-cell visual
- **File:** `mobile/src/screens/Login.tsx`
- **Detail:** Six bordered boxes that auto-advance feel familiar (banks, payments) and reduce confusion. There are good RN libs (`react-native-confirmation-code-field`) — or roll it yourself, ~50 LOC.
- **Effort:** M

### 4.6 🟧 Avatar circle = logout is undiscoverable
- **File:** `mobile/src/screens/Home.tsx`
- **Detail:** "Tap your initial to log out" is invisible. Either (a) add a tiny `Logout` label under the avatar, (b) show a confirmation menu on tap, or (c) move logout to a real "Settings" button. For one user, (a) is simplest.
- **Effort:** S

### 4.7 🟧 Empty state is just text
- **File:** `mobile/src/screens/Home.tsx` lines 96–99
- **Detail:** Add a soft truck illustration above the "No trucks yet" copy. SVG or PNG, ~80kb. Warmth matters when there's nothing on the screen.
- **Effort:** S

### 4.8 🟧 Splash is a bare spinner
- **File:** `mobile/App.tsx`
- **Detail:** During the hydrate window (~50ms typically) the user sees `ActivityIndicator`. Add the truck icon + "Truck Tracker" wordmark so it feels intentional, not loading.
- **Effort:** S

### 4.9 🟧 Map marker doesn't rotate with heading
- **File:** `mobile/src/screens/TruckDetail.tsx` `markerWrap`
- **Detail:** A truck pointing the direction it's moving is a small touch with big perceived intelligence. Apply `transform: [{ rotate: ${heading}deg }]` to the marker bubble. Suppress rotation when speed < 5 km/h to avoid jitter at standstill.
- **Effort:** S

### 4.10 🟧 Pressable feedback is just opacity
- **File:** Login button, TruckCard, all Pressables
- **Detail:** Add `android_ripple={{ color }}` and a subtle scale on iOS (use `react-native-reanimated` or just `transform:[{scale: pressed ? 0.97 : 1}]`). Tactile + native-feeling.
- **Effort:** S

### 4.11 🟧 No haptic feedback
- **File:** Login (OTP success), Home (refresh), TruckDetail (recenter)
- **Detail:** `react-native-haptic-feedback`: a `light` impact on successful OTP, `success` on login, `selection` on pull-to-refresh. Adds confidence.
- **Effort:** S

### 4.12 🟧 Phone number not formatted as you type
- **File:** `mobile/src/screens/Login.tsx`
- **Detail:** Display as `98765 43210` (space after 5 digits) as the user types. Keep the underlying value digits-only. ~10 lines.
- **Effort:** S

### 4.13 🟨 `formatRelativeTime` returns `—` for missing time
- **File:** `mobile/src/lib/types.ts` line 56
- **Detail:** "Waiting for device" reads more humanly. Also: "5 sec ago" instead of "just now" gives a sense of freshness when polling is fast.
- **Effort:** S

### 4.14 🟨 Status text inside pill is 13pt
- **File:** `mobile/src/components/StatusPill.tsx`
- **Detail:** For senior eyes, 15pt is the new 13. Match the rest of the body type scale.
- **Effort:** S

### 4.15 🟨 Recenter FAB is at fixed `bottom: 300`
- **File:** `mobile/src/screens/TruckDetail.tsx` line 222
- **Detail:** Anchor to sheet top, not absolute. If we ever change sheet height, the FAB position drifts. Use `onLayout` on the sheet to compute.
- **Effort:** S

### 4.16 🟨 Status pill green-on-green has marginal contrast
- **File:** `mobile/src/theme.ts` `statusRunning` / `statusRunningBg`
- **Detail:** `#16A34A` on `#DEF6E3` is ≈ 3.4:1 — under WCAG AA for normal text. Darken text to `#0E7A37` (≈ 4.8:1) or darken the pill background. Same check for amber and red pills.
- **Effort:** S

---

## 5. Accessibility (WCAG 2.1 AA)

### 5.1 🟧 No `accessibilityLabel` / `accessibilityRole` on Pressables
- **Files:** all screens
- **Detail:** A blind user navigating with VoiceOver hears "image" instead of "Get OTP" because the button content includes an emoji. Add semantic labels everywhere:
  ```tsx
  <Pressable accessibilityRole="button" accessibilityLabel="Get OTP" ...>
  ```
- **Effort:** S (10 files, ~30 props)

### 5.2 🟧 Touch targets some smaller than 44pt
- **Files:** `Home.avatar` (48pt ✓), `TruckDetail.backBtn` (40pt ✗), small "Change number" link
- **Detail:** Bump to 48pt min. Use `hitSlop` for inline text links.
- **Effort:** S

### 5.3 🟨 No `maxFontSizeMultiplier` cap
- **Files:** all screens
- **Detail:** When the OS Dynamic Type is set huge, our fixed layouts will overflow. Cap fonts at `maxFontSizeMultiplier={1.5}` on titles, but leave bodies free to scale.
- **Effort:** S

### 5.4 🟨 OTP `letterSpacing: 4` clashes with large-text mode
- **File:** `mobile/src/screens/Login.tsx`
- **Detail:** When text scales, letter spacing scales too and starts breaking lines. Use a fixed-cell OTP input (see 4.5) which sidesteps this entirely.

---

## 6. Reliability & security

### 6.1 🟥 No rate limiting on `/auth/exchange`
- **File:** `backend/internal/api/router.go`
- **Detail:** An attacker can call this endpoint at 1000 RPS, each call hitting Firebase. Wrap in `chi/middleware`'s `Throttle` or a per-IP token bucket (`golang.org/x/time/rate`). 10 req/min/IP is plenty.
- **Effort:** S

### 6.2 🟥 No min-length validation on `APP_JWT_SECRET`
- **File:** `backend/internal/config/config.go`
- **Detail:** If env is set to `x`, the server starts and silently issues weak tokens. Require ≥ 32 raw bytes (or base64 ≥ 44).
- **Effort:** S

### 6.3 🟧 No TLS in front of the server
- **Detail:** Mobile app currently talks plain HTTP. Document a deploy path that puts Caddy or Nginx in front for `Let's Encrypt` certs; or ship `autocert` directly in the Go binary if the VPS has port 80/443 free. Either is fine; pick one and write the playbook.
- **Effort:** M (deploy doc, not code)

### 6.4 🟧 TCP graceful shutdown doesn't drain in-flight connections
- **File:** `backend/internal/tcp/server.go`
- **Detail:** On SIGTERM, listener closes but live `handle()` goroutines aren't told to exit. They'll exit when their 5-minute read deadline fires. Wire a `<-ctx.Done()` check in the connection loop so they exit cleanly within a few seconds.
- **Effort:** S

### 6.5 🟧 Error response shape inconsistent
- **File:** `backend/internal/api/middleware/auth.go`
- **Detail:** `http.Error(w, "missing bearer token", 401)` returns `text/plain`; other handlers return JSON `{"error":"..."}`. Pick JSON for everything so mobile only needs one parser.
- **Effort:** S

### 6.6 🟧 WebSocket connection lifetime can exceed token expiry
- **File:** `backend/internal/ws/hub.go`
- **Detail:** Token verified at upgrade, not after. If we ever shorten the TTL or rotate the HMAC secret, sockets keep working until disconnect. Add a `time.AfterFunc(expiresAt-now, ws.Close)` so the connection terminates exactly when the token would.
- **Effort:** S

### 6.7 🟨 PII in logs
- **File:** `backend/internal/api/router.go` `exchangeHandler`
- **Detail:** We log `firebase verify failed` with `err` that may contain the phone. Mask phone in logs (`+9198****210`) for any growth path.
- **Effort:** S

### 6.8 🟨 `device_sessions` table not used
- **File:** `backend/migrations/000001_init.up.sql` + `backend/internal/tcp/server.go`
- **Detail:** We created the table but nothing writes to it. Either wire it (open row on connect, close row on disconnect) or drop the table from the migration. Wiring is useful: "why isn't this truck reporting" becomes a one-query answer.
- **Effort:** S

---

## 7. Observability

### 7.1 🟧 No `/metrics` Prometheus endpoint
- **Detail:** Wire `prometheus/client_golang` and expose `bus_publish_total`, `bus_drop_total`, `ws_clients`, `tcp_connections`, `position_save_duration_seconds`, `http_request_duration_seconds`. Even on a single VPS you'll want these the moment something feels off.
- **Effort:** M

### 7.2 🟨 `req_id` not echoed in error responses
- **File:** `backend/internal/api/router.go` `writeErr`
- **Detail:** Add `X-Request-Id` header on responses so the user can paste it back when something goes wrong, and you can grep logs.
- **Effort:** S

### 7.3 🟨 No structured log of `user_id` on authed requests
- **File:** `backend/internal/api/middleware/auth.go`
- **Detail:** After verifying claims, attach `user_id` to the request-scoped logger.
- **Effort:** S

---

## 8. Code quality / hygiene

### 8.1 🟧 Replace handwritten queries with `sqlc`-generated once available
- **File:** `backend/internal/store/store.go`
- **Detail:** `make sqlc` will produce `internal/store/sqlc/`. Then re-write `EnsureUserByPhone`, `SavePosition`, `GetTruck`, `ListPositionHistory` to call generated typed methods. Compile-time SQL correctness.
- **Effort:** M

### 8.2 🟨 `models.Position.Status()` exists on the server but mobile re-implements it
- **File:** `backend/internal/models/models.go` + `mobile/src/lib/types.ts`
- **Detail:** Server doesn't ship the status string in the JSON — mobile derives it. If we ever change thresholds (e.g., "moving" cutoff goes from 5 km/h to 3), both sides must change in lockstep. Two options: (a) drop the server method (it's unused) and keep the rule client-side only, or (b) ship `status` in the position JSON and remove the client compute. (a) is lower-risk.
- **Effort:** S

### 8.3 🟨 `protoServerCmd` defined but unused
- **File:** `backend/internal/protocol/gt06/decoder.go` line 47
- **Detail:** No code path emits server commands yet. Either remove or implement (e.g., send `LOCATION,1#` on demand).
- **Effort:** S

### 8.4 🟨 `mobile/src/lib/env.ts` will be edited in place
- **Detail:** The `.gitignore` already excludes `env.ts.local` (anticipated pattern). Better: rename `env.ts` → `env.ts.example`, ship that, and require the user to copy to `env.ts` (which `.gitignore` excludes outright). Removes the risk of accidental commit of real keys.
- **Effort:** S

### 8.5 🟨 No ESLint config in `mobile/`
- **Detail:** `package.json` has the `lint` script but no `.eslintrc.js`. Add the standard `@react-native/eslint-config` one-liner config.
- **Effort:** S

### 8.6 🟨 No `.dockerignore`
- **File:** `backend/`
- **Detail:** Without it, `docker build` ships `*.log`, local `.env`, and a `.git/` copy into the build context. Add a one-liner ignore.
- **Effort:** S

---

## 9. Testing gaps

### 9.1 🟧 No tests for `bus`
- **File:** `backend/internal/bus/`
- **Detail:** Test (a) one subscriber receives; (b) two subscribers both receive; (c) slow subscriber doesn't block fast subscriber; (d) cancel removes; (e) publish after cancel doesn't panic.
- **Effort:** M

### 9.2 🟧 No tests for `auth.Issue` / `auth.Verify`
- **File:** `backend/internal/auth/firebase.go`
- **Detail:** Test signing roundtrip, tamper detection (flipped byte), expiry, malformed input.
- **Effort:** S

### 9.3 🟨 No store tests against a real Postgres
- **Detail:** Add `testcontainers-go` to spin up Postgres in a Docker container during `go test`. Test `ListTrucks`, `GetTruck` (ownership check), `ListPositionHistory` (window).
- **Effort:** L

### 9.4 🟨 No integration test: fake tracker → server → REST round-trip
- **Detail:** Use the existing `cmd/faketracker` in a `go test` that starts the server in-process and asserts on `/v1/trucks/:id`.
- **Effort:** L

---

## 10. Build / CI / deployment

### 10.1 🟧 No CI config
- **Detail:** Minimal GitHub Actions: backend `go vet`, `go test ./...`, `go build`. Mobile `npm ci && npm run tsc && npm run lint`.
- **Effort:** M

### 10.2 🟨 `Dockerfile` could add `HEALTHCHECK`
- **Detail:** `HEALTHCHECK --interval=30s --timeout=3s CMD wget -qO- http://localhost:8080/healthz || exit 1`. Lets orchestrators restart sick containers.
- **Effort:** S

### 10.3 🟨 No `pre-commit` config
- **Detail:** Set up `gofmt`, `goimports`, `golangci-lint`, `prettier` for `.tsx`. Cheap insurance.
- **Effort:** S

---

## Recommended sequencing

Three sprints, in priority order.

### Sprint A (≈ ½ day): Correctness first
Items: 1.1, 1.3, 1.6, 1.7, 1.8, 6.1, 6.2, 6.5
Why: bug fixes that prevent failures, plus the small security gates (rate limit + secret length) — never ship without these.

### Sprint B (≈ 1 day): UX polish & senior-friendliness
Items: 4.1, 4.2, 4.3, 4.4, 4.5, 4.6, 4.8, 4.9, 4.10, 4.11, 4.12, 4.14, 5.1, 5.2
Why: The product's success metric is "does my father feel comfortable using this?" These items are exactly that.

### Sprint C (≈ 1 day): Performance & ops
Items: 2.1, 2.3, 3.1, 3.2, 3.3, 6.4, 6.8, 7.1, 7.2, 8.1, 8.6, 10.1, 10.2
Why: With one truck on a VPS none of these matter; they're the work that pays off as soon as you add the second truck or want to debug a production issue.

Everything else (🟨 items) is "do it when you touch the file anyway."

---

## What we explicitly did NOT add to the list

- **New features.** No history-on-map, no driver calling, no geofences, no notifications.
- **Multi-user / multi-tenant infrastructure.** Single-fleet today.
- **Protocol additions.** PS10A + PT06 cover the hardware in hand.
- **Switching backends.** Custom Go server is the chosen path.

Tell me which sprint to start with and I'll begin coding through it.
