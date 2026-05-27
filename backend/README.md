# Truck Tracker — Backend

Single-binary Go server that:

- Listens on TCP `:5023` for Pictor GPS trackers (GT06-family protocol)
- Exposes REST + WebSocket on HTTP `:8080` for the mobile app
- Persists everything to PostgreSQL
- Verifies Firebase Phone Auth ID tokens and issues short-lived app tokens

## Layout

```
cmd/
  server/main.go          # wires everything together
  faketracker/main.go     # simulates a PS10A device for E2E testing
internal/
  config/                 # envconfig
  models/                 # Position, Device, User
  bus/                    # in-process pub/sub on channels
  store/                  # pgx pool + sqlc-generated queries
  auth/                   # Firebase verify + HMAC app tokens
  protocol/               # tracker protocol interface
    crc/                  # CRC-ITU lookup-table implementation (verified)
    gt06/                 # PS10A + PT06 frame + body decoders
  tcp/                    # tracker listener
  api/                    # chi router + REST handlers
    middleware/auth.go    # bearer-token middleware
  ws/                     # WebSocket hub
migrations/               # golang-migrate SQL files
sqlc.yaml                 # sqlc codegen config
```

## Quick start (local)

```bash
# 1. dependencies
brew install go postgresql sqlc golang-migrate    # macOS

# 2. database
createdb trucktracker
make migrate-up

# 3. sqlc code-gen
make sqlc

# 4. env
cp .env.example .env
# edit .env — paste DATABASE_URL, FIREBASE_CREDENTIALS_PATH, APP_JWT_SECRET

# 5. run
make run
```

The server binds:

- `http://localhost:8080` — REST + WebSocket
- `tcp://localhost:5023`  — trackers

## Firebase setup

1. Create a Firebase project, enable **Phone** sign-in method.
2. Project Settings → Service Accounts → "Generate new private key".
3. Save the JSON file and point `FIREBASE_CREDENTIALS_PATH` at it.

## Auth flow

```
Mobile  ──Firebase Phone Auth──▶  Firebase
Mobile  ──POST /v1/auth/exchange { firebaseIdToken }──▶  Server
Server  ──VerifyIDToken──▶  Firebase, returns phone
Server  ──Issue(app token, exp=24h)──▶  Mobile
Mobile  ──Authorization: Bearer <app token>──▶  Server (all REST + WS)
```

## Pictor protocol implementation

`internal/protocol/gt06` is a real decoder for both **PS10A** (`0x12` location,
`0x16` alarm) and **PT06** (`0x22` location, `0x26` alarm) — the two protocol
families share the same frame envelope (`78 78 LEN PROTO BODY SERIAL CRC 0D 0A`)
and only differ in body layouts, so one decoder handles both.

CRC-ITU is implemented in `internal/protocol/crc` from the vendor's Appendix-A
lookup table. The implementation is verified against **all 7 vendor golden
frames** in the protocol PDFs.

### End-to-end testing without hardware

```bash
# terminal 1
make run

# terminal 2
make faketracker   # replays login → heartbeat → location frames in a loop
```

The fake tracker walks a small circle so you'll see live position updates
on any WebSocket client subscribed to `/v1/live`.

## What's still stubbed (deliberately)

- **`internal/api/router.go`** — `getTruckHandler` and `historyHandler` are
  stubbed pending sqlc generation. Run `make sqlc` then implement them
  against the generated `Queries` type.

## Open items

| # | Item                                                          |
|---|---------------------------------------------------------------|
| 1 | Run `sqlc generate` after first `go mod tidy`                 |
| 2 | Decide VPS provider, configure SIM APN, point trackers at IP  |
| 3 | Wire `getTruckHandler` / `historyHandler` against sqlc types  |

## Run tests

```bash
make test
```
