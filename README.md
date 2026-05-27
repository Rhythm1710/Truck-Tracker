# Truck Tracker

A senior-friendly fleet-tracking app for a father with a 1–5 truck operation
running Pictor PS10a / PT06 GPS trackers.

- **mobile/** — React Native + TypeScript app (iOS + Android)
- **backend/** — Go server (TCP listener + REST + WebSocket + PostgreSQL)

## Architecture

See `ADR-001-truck-tracker-architecture.md` (one level up) for the full
architecture decision record. Short version:

```
Trackers → Cellular → Go server (single binary, multi-goroutine)
                      ├─ TCP :5023  (tracker frames in)
                      ├─ HTTP :8080 (REST + WebSocket for the app)
                      ├─ Position Bus (channels)
                      └─ PostgreSQL
                              ↓
                        React Native app
                        (Firebase Phone Auth + Mappls maps)
```

## Getting started

1. **Backend:** `cd backend && cp .env.example .env && make migrate-up && make sqlc && make run`
2. **Mobile:**  `cd mobile && npm install && npm run pods && npm run ios`

Each has its own README with the full setup.

## Status

- [x] Figma designs (Login / Home / Truck Detail)
- [x] ADR-001 architecture
- [x] Backend scaffold (chi, pgx/sqlc, gorilla/websocket, firebase-admin)
- [x] Mobile scaffold (RN + TS, react-query, zustand, Firebase Phone Auth)
- [ ] Pictor PS10a/PT06 protocol body parsers (blocked on docs)
- [ ] `sqlc generate` (run after first `go mod tidy`)
- [ ] VPS deployment
- [ ] Mappls API keys wired in
- [ ] Firebase project created + keys placed in `ios/` and `android/app/`
