// Package protocol defines the contract between the TCP layer and concrete
// GPS protocol implementations (PS10A, PT06, future variants).
//
// The TCP listener doesn't know which protocol a connection speaks — it
// reads bytes and asks a Decoder to identify and parse them. Adding support
// for a new tracker family means implementing Decoder and wiring a Factory
// in main.go.
package protocol

import (
	"context"
	"errors"
	"io"

	"github.com/rhythm/truck-tracker/internal/models"
)

// Frame is a single decoded message from a tracker.
type Frame struct {
	// Kind indicates what the tracker sent: login, heartbeat, gps, alarm, ...
	Kind FrameKind

	// IMEI is set on login frames; subsequent frames use the connection's
	// already-known IMEI (the decoder remembers it internally and stamps
	// it on outgoing Frames).
	IMEI string

	// Position is set when Kind == FramePosition or FrameAlarm.
	Position *models.Position

	// Status is set when Kind == FrameHeartbeat or FrameAlarm.
	Status *DeviceStatus

	// Ack is the bytes we should write back to the tracker to acknowledge.
	// Many protocols REQUIRE an ACK or they'll disconnect & reconnect.
	Ack []byte
}

type FrameKind int

const (
	FrameUnknown FrameKind = iota
	FrameLogin
	FrameHeartbeat
	FramePosition
	FrameAlarm
)

// DeviceStatus is the snapshot of tracker health pulled from a heartbeat
// or alarm packet (terminal info byte + voltage + GSM signal + alarm/lang).
type DeviceStatus struct {
	OilCut       bool   // bit7 of terminal info
	GPSTracking  bool   // bit6
	Alarm        string // SOS / LowBattery / PowerCut / Shock / Normal
	ChargeOn     bool
	ACCHigh      bool
	Defense      bool
	VoltageLevel uint8 // 0..6
	GSMSignal    uint8 // 0..4
	AlarmCode    uint8 // 0=normal, 1=SOS, 2=power cut, 3=shock, ...
	Language     uint8 // 1=Chinese, 2=English
}

// Decoder reads tracker frames from a connection.
//
// Implementations should buffer bytes internally and return one Frame per
// successful parse, or io.EOF if the connection is cleanly closed.
type Decoder interface {
	// ReadFrame blocks until a frame is parsed or the context is cancelled.
	ReadFrame(ctx context.Context, r io.Reader) (Frame, error)
}

// Factory makes a fresh Decoder for a new connection.
type Factory func() Decoder

// Sentinel errors.
var (
	ErrUnsupportedProtocol = errors.New("protocol: unsupported")
	ErrCRC                 = errors.New("protocol: CRC mismatch")
	ErrFraming             = errors.New("protocol: bad framing")
	_                      = io.EOF
)
