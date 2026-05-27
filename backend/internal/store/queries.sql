-- ===========================================================================
-- sqlc input. Run `sqlc generate` to produce type-safe Go in internal/store/sqlc.
-- Conventions: queries return whole rows; mappers in store/ convert to models.
-- ===========================================================================

-- ----- users ---------------------------------------------------------------

-- name: GetUserByPhone :one
SELECT * FROM users WHERE phone = $1;

-- name: GetUserByID :one
SELECT * FROM users WHERE id = $1;

-- name: CreateUser :one
INSERT INTO users (phone, name)
VALUES ($1, $2)
RETURNING *;

-- name: UpsertUserByPhone :one
INSERT INTO users (phone, name)
VALUES ($1, $2)
ON CONFLICT (phone) DO UPDATE
   SET name = COALESCE(NULLIF(EXCLUDED.name, ''), users.name)
RETURNING *;

-- ----- devices -------------------------------------------------------------

-- name: ListDevicesByOwner :many
SELECT * FROM devices WHERE owner_id = $1 ORDER BY plate;

-- name: GetDeviceByID :one
SELECT * FROM devices WHERE id = $1;

-- name: GetDeviceByIMEI :one
SELECT * FROM devices WHERE imei = $1;

-- name: CreateDevice :one
INSERT INTO devices (imei, plate, label, owner_id)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- ----- positions -----------------------------------------------------------

-- name: InsertPosition :exec
INSERT INTO positions
  (device_id, lat, lng, speed_kmh, heading, ignition, device_at, recv_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8);

-- name: GetLatestPosition :one
SELECT * FROM positions
WHERE device_id = $1
ORDER BY recv_at DESC
LIMIT 1;

-- name: ListPositionsRange :many
SELECT * FROM positions
WHERE device_id = $1 AND recv_at BETWEEN $2 AND $3
ORDER BY recv_at ASC
LIMIT $4;

-- ----- device_sessions -----------------------------------------------------

-- name: OpenDeviceSession :one
INSERT INTO device_sessions (device_id, last_ip)
VALUES ($1, $2)
RETURNING *;

-- name: CloseDeviceSession :exec
UPDATE device_sessions
SET disconnected_at = NOW()
WHERE id = $1;
