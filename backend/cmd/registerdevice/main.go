// Command registerdevice inserts (or finds) a user and registers a device
// against them. Idempotent — re-running with the same phone+IMEI prints
// the existing IDs instead of erroring.
//
// Usage:
//
//	go run ./cmd/registerdevice \
//	    -phone +919876543210 \
//	    -imei  123456789012345 \
//	    -plate 'HR 55 AB 1234' \
//	    -name  'Papa' \
//	    -label 'Karnal route'
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	var (
		phone = flag.String("phone", "", "user phone in E.164 (e.g., +919876543210)")
		imei  = flag.String("imei", "", "tracker IMEI (15 digits)")
		plate = flag.String("plate", "", "vehicle plate (e.g., HR 55 AB 1234)")
		name  = flag.String("name", "", "optional user-friendly name")
		label = flag.String("label", "", "optional device label")
		dsn   = flag.String("dsn", envOr("DATABASE_URL", ""), "postgres DSN (defaults to $DATABASE_URL)")
	)
	flag.Parse()

	if err := validate(*phone, *imei, *plate, *dsn); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		flag.Usage()
		os.Exit(2)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, *dsn)
	if err != nil {
		die("connect: %v", err)
	}
	defer pool.Close()

	userID, created, err := upsertUser(ctx, pool, *phone, *name)
	if err != nil {
		die("upsert user: %v", err)
	}
	if created {
		fmt.Printf("✓ user created   id=%s phone=%s name=%q\n", userID, *phone, *name)
	} else {
		fmt.Printf("· user exists    id=%s phone=%s\n", userID, *phone)
	}

	devID, created, err := upsertDevice(ctx, pool, *imei, *plate, *label, userID)
	if err != nil {
		die("upsert device: %v", err)
	}
	if created {
		fmt.Printf("✓ device created id=%s imei=%s plate=%q\n", devID, *imei, *plate)
	} else {
		fmt.Printf("· device exists  id=%s imei=%s plate=%q (owner unchanged)\n", devID, *imei, *plate)
	}

	fmt.Println()
	fmt.Println("Next: point the tracker at your server (SMS APN + server IP).")
	fmt.Println("      The server's TCP listener is on :5023.")
}

// --- helpers ---------------------------------------------------------------

func validate(phone, imei, plate, dsn string) error {
	if dsn == "" {
		return errors.New("DATABASE_URL or -dsn is required")
	}
	if !strings.HasPrefix(phone, "+") || len(phone) < 8 {
		return errors.New("phone must be E.164 (e.g., +919876543210)")
	}
	if len(imei) < 14 || len(imei) > 17 {
		return errors.New("imei should be ~15 digits")
	}
	if strings.TrimSpace(plate) == "" {
		return errors.New("plate is required")
	}
	return nil
}

func upsertUser(ctx context.Context, pool *pgxpool.Pool, phone, name string) (id string, created bool, err error) {
	// Try to fetch first to know whether we created or not.
	err = pool.QueryRow(ctx, `SELECT id FROM users WHERE phone = $1`, phone).Scan(&id)
	if err == nil {
		// Optionally update name if user passed one and existing is blank.
		if name != "" {
			_, _ = pool.Exec(ctx, `UPDATE users SET name = $1 WHERE id = $2 AND name = ''`, name, id)
		}
		return id, false, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return "", false, err
	}
	err = pool.QueryRow(ctx,
		`INSERT INTO users (phone, name) VALUES ($1, $2) RETURNING id`,
		phone, name,
	).Scan(&id)
	return id, true, err
}

func upsertDevice(ctx context.Context, pool *pgxpool.Pool, imei, plate, label, ownerID string) (id string, created bool, err error) {
	err = pool.QueryRow(ctx, `SELECT id FROM devices WHERE imei = $1`, imei).Scan(&id)
	if err == nil {
		// Refresh plate/label if changed.
		_, _ = pool.Exec(ctx,
			`UPDATE devices SET plate = $1, label = COALESCE(NULLIF($2, ''), label) WHERE id = $3`,
			plate, label, id,
		)
		return id, false, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return "", false, err
	}
	err = pool.QueryRow(ctx,
		`INSERT INTO devices (imei, plate, label, owner_id) VALUES ($1, $2, $3, $4) RETURNING id`,
		imei, plate, label, ownerID,
	).Scan(&id)
	return id, true, err
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func die(format string, args ...any) {
	fmt.Fprintln(os.Stderr, fmt.Sprintf(format, args...))
	os.Exit(1)
}
