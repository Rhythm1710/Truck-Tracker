-- Initial schema: users, devices, positions, device_sessions.
-- Apply with golang-migrate:
--   migrate -database "$DATABASE_URL" -path migrations up

CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- ---------------------------------------------------------------------------
-- users
-- ---------------------------------------------------------------------------
CREATE TABLE users (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    phone       TEXT NOT NULL UNIQUE,
    name        TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ---------------------------------------------------------------------------
-- devices
-- ---------------------------------------------------------------------------
CREATE TABLE devices (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    imei        TEXT NOT NULL UNIQUE,
    plate       TEXT NOT NULL,
    label       TEXT NOT NULL DEFAULT '',
    owner_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_devices_owner_id ON devices(owner_id);

-- ---------------------------------------------------------------------------
-- positions
-- Time-series. BRIN on recv_at keeps the index small as the table grows;
-- B-tree on (device_id, recv_at DESC) makes "latest by truck" queries O(log n).
-- ---------------------------------------------------------------------------
CREATE TABLE positions (
    id          BIGSERIAL PRIMARY KEY,
    device_id   UUID NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    lat         DOUBLE PRECISION NOT NULL,
    lng         DOUBLE PRECISION NOT NULL,
    speed_kmh   REAL NOT NULL DEFAULT 0,
    heading     REAL NOT NULL DEFAULT 0,
    ignition    BOOLEAN NOT NULL DEFAULT FALSE,
    device_at   TIMESTAMPTZ NOT NULL,
    recv_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_positions_device_recv ON positions(device_id, recv_at DESC);
CREATE INDEX idx_positions_recv_at_brin ON positions USING BRIN (recv_at);

-- ---------------------------------------------------------------------------
-- device_sessions
-- One row per TCP connection lifecycle. Useful for debugging "why isn't this
-- truck reporting" — see when it last connected and from which IP.
-- ---------------------------------------------------------------------------
CREATE TABLE device_sessions (
    id              BIGSERIAL PRIMARY KEY,
    device_id       UUID NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    connected_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    disconnected_at TIMESTAMPTZ,
    last_ip         INET
);
CREATE INDEX idx_device_sessions_device_id ON device_sessions(device_id, connected_at DESC);
