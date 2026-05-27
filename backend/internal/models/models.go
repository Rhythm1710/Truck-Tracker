// Package models holds shared domain structs used across packages.
//
// These are the "wire" shapes — they're what flows through the Position Bus,
// what handlers serialize to JSON, and what sqlc-generated types are mapped to.
package models

import "time"

// Position is a single GPS fix from a tracker, after protocol decoding.
type Position struct {
	DeviceID  string    `json:"deviceId"`
	IMEI      string    `json:"imei"`
	Lat       float64   `json:"lat"`
	Lng       float64   `json:"lng"`
	SpeedKmh  float64   `json:"speed"`
	Heading   float64   `json:"heading"`  // degrees, 0 = north
	Ignition  bool      `json:"ignition"` // true = engine on
	DeviceAt  time.Time `json:"deviceAt"` // timestamp from the tracker itself
	RecvAt    time.Time `json:"at"`       // timestamp when our server received it
}

// Device is a registered tracker tied to a user.
type Device struct {
	ID      string `json:"id"`
	IMEI    string `json:"imei"`
	Plate   string `json:"plate"`
	Label   string `json:"label,omitempty"`
	OwnerID string `json:"ownerId"`
}

// User is the human who logs in via OTP.
type User struct {
	ID    string `json:"id"`
	Phone string `json:"phone"`
	Name  string `json:"name,omitempty"`
}

// TruckSummary is what GET /v1/trucks returns — device + its latest position.
type TruckSummary struct {
	Device Device   `json:"device"`
	Latest *Position `json:"latest,omitempty"` // nil if device never reported
}

// Status derives a human label from a Position. Stays in models so mobile
// can reuse the same thresholds via API responses.
func (p *Position) Status() string {
	if p == nil {
		return "Offline"
	}
	if time.Since(p.RecvAt) > 15*time.Minute {
		return "Offline"
	}
	switch {
	case p.Ignition && p.SpeedKmh > 5:
		return "Running"
	case p.Ignition:
		return "Idle"
	default:
		return "Stopped"
	}
}
