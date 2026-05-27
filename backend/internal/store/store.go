// Package store owns the Postgres connection pool and the data access
// methods used by the API + position bus subscriber.
//
// Layering:
//   - sqlc/  : generated types  (do not edit; produced by `make sqlc`)
//   - store.go : pool + handwritten queries that translate rows to models
//
// We keep handwritten queries here for two reasons:
//  1. Until `make sqlc` is run for the first time, the generated `Queries`
//     type doesn't exist, so the server still has to build.
//  2. The TruckSummary / history queries use LATERAL joins / dynamic limits
//     that are awkward to express in sqlc.
package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rhythm/truck-tracker/internal/models"
)

// Sentinel errors. Wrap pgx.ErrNoRows so handlers can map 404 cleanly.
var (
	ErrNotFound  = errors.New("store: not found")
	ErrForbidden = errors.New("store: forbidden")
)

type Store struct {
	pool *pgxpool.Pool
}

// New connects to Postgres and returns a Store. Caller must Close.
func New(ctx context.Context, dsn string) (*Store, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("pgxpool parse: %w", err)
	}
	cfg.MaxConns = 10
	cfg.MaxConnLifetime = 30 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("pgxpool connect: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("pgxpool ping: %w", err)
	}

	return &Store{pool: pool}, nil
}

func (s *Store) Close() { s.pool.Close() }

// Pool exposes the raw pool to callers that need transactions etc.
// Avoid in handlers; prefer adding a method here.
func (s *Store) Pool() *pgxpool.Pool { return s.pool }

// ---------------------------------------------------------------------------
// Users
// ---------------------------------------------------------------------------

// EnsureUserByPhone returns the user_id for `phone`, creating the row if
// missing. Used by the auth/exchange handler so first-time logins work.
func (s *Store) EnsureUserByPhone(ctx context.Context, phone string) (string, error) {
	var id string
	err := s.pool.QueryRow(ctx, `SELECT id FROM users WHERE phone = $1`, phone).Scan(&id)
	if err == nil {
		return id, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return "", err
	}
	err = s.pool.QueryRow(ctx,
		`INSERT INTO users (phone, name) VALUES ($1, '') RETURNING id`,
		phone,
	).Scan(&id)
	return id, err
}

// ---------------------------------------------------------------------------
// Positions (writer — called from the bus subscriber)
// ---------------------------------------------------------------------------

// SavePosition writes one GPS fix. If DeviceID isn't set on the position,
// we resolve it via IMEI.
func (s *Store) SavePosition(ctx context.Context, p models.Position) error {
	if p.DeviceID == "" {
		if p.IMEI == "" {
			return errors.New("position has neither DeviceID nor IMEI")
		}
		var devID string
		err := s.pool.QueryRow(ctx,
			`SELECT id FROM devices WHERE imei = $1`, p.IMEI,
		).Scan(&devID)
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("device with imei=%s not registered", p.IMEI)
		}
		if err != nil {
			return err
		}
		p.DeviceID = devID
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO positions
		  (device_id, lat, lng, speed_kmh, heading, ignition, device_at, recv_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		p.DeviceID, p.Lat, p.Lng, p.SpeedKmh, p.Heading, p.Ignition, p.DeviceAt, p.RecvAt,
	)
	return err
}

// ---------------------------------------------------------------------------
// Trucks (read — called from API handlers)
// ---------------------------------------------------------------------------

// ListTrucks returns every device owned by user + the latest known position.
// One query with LATERAL join — single round trip.
func (s *Store) ListTrucks(ctx context.Context, ownerID string) ([]models.TruckSummary, error) {
	rows, err := s.pool.Query(ctx, listTrucksSQL, ownerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []models.TruckSummary{} // never nil — JSON returns [] not null
	for rows.Next() {
		t, err := scanTruckRow(rows, ownerID)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// GetTruck returns one truck by device id, scoped to its owner.
// Returns ErrNotFound if the device doesn't exist OR isn't owned by ownerID
// (we don't disambiguate to avoid leaking IDs).
func (s *Store) GetTruck(ctx context.Context, ownerID, deviceID string) (models.TruckSummary, error) {
	rows, err := s.pool.Query(ctx, getTruckSQL, ownerID, deviceID)
	if err != nil {
		return models.TruckSummary{}, err
	}
	defer rows.Close()
	if !rows.Next() {
		return models.TruckSummary{}, ErrNotFound
	}
	return scanTruckRow(rows, ownerID)
}

// ListPositionHistory returns positions for a device in [from, to], capped at
// `limit`. Caller must ownership-check via GetTruck first (or we re-check here).
func (s *Store) ListPositionHistory(
	ctx context.Context, ownerID, deviceID string,
	from, to time.Time, limit int,
) ([]models.Position, error) {
	if limit <= 0 || limit > 5000 {
		limit = 1000
	}
	// Ownership check inline — one query, no extra round trip.
	rows, err := s.pool.Query(ctx, `
		SELECT p.lat, p.lng, p.speed_kmh, p.heading, p.ignition, p.device_at, p.recv_at
		FROM positions p
		JOIN devices d ON d.id = p.device_id
		WHERE d.id = $1 AND d.owner_id = $2
		  AND p.recv_at BETWEEN $3 AND $4
		ORDER BY p.recv_at ASC
		LIMIT $5`,
		deviceID, ownerID, from, to, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []models.Position{}
	for rows.Next() {
		var p models.Position
		if err := rows.Scan(
			&p.Lat, &p.Lng, &p.SpeedKmh, &p.Heading, &p.Ignition, &p.DeviceAt, &p.RecvAt,
		); err != nil {
			return nil, err
		}
		p.DeviceID = deviceID
		out = append(out, p)
	}
	return out, rows.Err()
}

// ---------------------------------------------------------------------------
// SQL constants
// ---------------------------------------------------------------------------

const truckCols = `
	d.id, d.imei, d.plate, d.label,
	p.lat, p.lng, p.speed_kmh, p.heading, p.ignition, p.device_at, p.recv_at`

var listTrucksSQL = `
	SELECT ` + truckCols + `
	FROM devices d
	LEFT JOIN LATERAL (
	    SELECT * FROM positions
	    WHERE device_id = d.id
	    ORDER BY recv_at DESC
	    LIMIT 1
	) p ON true
	WHERE d.owner_id = $1
	ORDER BY d.plate`

var getTruckSQL = `
	SELECT ` + truckCols + `
	FROM devices d
	LEFT JOIN LATERAL (
	    SELECT * FROM positions
	    WHERE device_id = d.id
	    ORDER BY recv_at DESC
	    LIMIT 1
	) p ON true
	WHERE d.owner_id = $1 AND d.id = $2`

// scanTruckRow shares the row-scan logic between list & get.
func scanTruckRow(rows pgx.Rows, ownerID string) (models.TruckSummary, error) {
	var t models.TruckSummary
	var lat, lng *float64
	var spd, hdg *float64
	var ign *bool
	var dAt, rAt *time.Time
	if err := rows.Scan(
		&t.Device.ID, &t.Device.IMEI, &t.Device.Plate, &t.Device.Label,
		&lat, &lng, &spd, &hdg, &ign, &dAt, &rAt,
	); err != nil {
		return models.TruckSummary{}, err
	}
	t.Device.OwnerID = ownerID
	if lat != nil {
		t.Latest = &models.Position{
			DeviceID: t.Device.ID, IMEI: t.Device.IMEI,
			Lat: *lat, Lng: *lng,
			SpeedKmh: *spd, Heading: *hdg, Ignition: *ign,
			DeviceAt: *dAt, RecvAt: *rAt,
		}
	}
	return t, nil
}
