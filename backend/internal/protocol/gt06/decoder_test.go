package gt06

import (
	"bytes"
	"context"
	"encoding/hex"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/rhythm/truck-tracker/internal/protocol"
)

// hexBytes is a tiny helper for embedding spec frames as `78 78 0D 01 ...`.
func hexBytes(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(strings.ReplaceAll(s, " ", ""))
	if err != nil {
		t.Fatalf("bad hex: %v", err)
	}
	return b
}

func TestDecoder_Login_PS10A_SpecExample(t *testing.T) {
	// From spec §5.1.3 — IMEI 123456789012345
	frame := hexBytes(t, "78 78 0D 01 01 23 45 67 89 01 23 45 00 01 8C DD 0D 0A")
	d := New()
	got, err := d.ReadFrame(context.Background(), bytes.NewReader(frame))
	if err != nil {
		t.Fatalf("ReadFrame: %v", err)
	}
	if got.Kind != protocol.FrameLogin {
		t.Fatalf("kind = %v, want Login", got.Kind)
	}
	if got.IMEI != "123456789012345" {
		t.Fatalf("imei = %q, want 123456789012345", got.IMEI)
	}
	// Expect ACK = 78 78 05 01 00 01 D9 DC 0D 0A
	wantAck := hexBytes(t, "78 78 05 01 00 01 D9 DC 0D 0A")
	if !bytes.Equal(got.Ack, wantAck) {
		t.Fatalf("ack = % X, want % X", got.Ack, wantAck)
	}
}

func TestDecoder_Login_AppendixB(t *testing.T) {
	// Appendix B login: 78 78 0D 01 03 53 41 35 32 15 03 62 00 02 2D 06 0D 0A
	frame := hexBytes(t, "78 78 0D 01 03 53 41 35 32 15 03 62 00 02 2D 06 0D 0A")
	d := New()
	got, err := d.ReadFrame(context.Background(), bytes.NewReader(frame))
	if err != nil {
		t.Fatalf("ReadFrame: %v", err)
	}
	// IMEI is BCD: 03 53 41 35 32 15 03 62 → 0353413532150362
	// Leading 0 stripped → 353413532150362 (15 digits)
	if got.IMEI != "353413532150362" {
		t.Fatalf("imei = %q, want 353413532150362", got.IMEI)
	}
}

func TestDecoder_Location_PS10A_AppendixB(t *testing.T) {
	// Appendix B GPS+LBS: 78 78 1F 12 0B 08 1D 11 2E 10 CF 02 7A C7 EB 0C 46 58 49 00 14 8F 01 CC 00 28 7D 00 1F B8 00 03 80 81 0D 0A
	frame := hexBytes(t, "78 78 1F 12 0B 08 1D 11 2E 10 CF 02 7A C7 EB 0C 46 58 49 00 14 8F 01 CC 00 28 7D 00 1F B8 00 03 80 81 0D 0A")
	d := New()
	got, err := d.ReadFrame(context.Background(), bytes.NewReader(frame))
	if err != nil {
		t.Fatalf("ReadFrame: %v", err)
	}
	if got.Kind != protocol.FramePosition {
		t.Fatalf("kind = %v, want Position", got.Kind)
	}
	if got.Position == nil {
		t.Fatal("Position is nil")
	}
	p := got.Position

	// Spec example values:
	//   Date: 2011-08-29 17:46:16 UTC
	wantTime := time.Date(2011, time.August, 29, 17, 46, 16, 0, time.UTC)
	if !p.DeviceAt.Equal(wantTime) {
		t.Errorf("deviceAt = %v, want %v", p.DeviceAt, wantTime)
	}

	// Lat raw 0x027AC7EB = 41,601,003 → /30000 = 1386.7001 min → /60 = 23.11167°
	// Sign: BYTE_1 bit2=1 → North (positive)
	wantLat := float64(41601003) / 30000.0 / 60.0
	if math.Abs(p.Lat-wantLat) > 1e-6 {
		t.Errorf("lat = %v, want %v", p.Lat, wantLat)
	}
	// Lng raw 0x0C465849 = 205,936,713 → /30000 = 6864.5571 min → /60 = 114.40929°
	// Sign: BYTE_1 bit3=0 → East (positive)
	wantLng := float64(205936713) / 30000.0 / 60.0
	if math.Abs(p.Lng-wantLng) > 1e-6 {
		t.Errorf("lng = %v, want %v", p.Lng, wantLng)
	}
	// Speed = 0x00 = 0
	if p.SpeedKmh != 0 {
		t.Errorf("speed = %v, want 0", p.SpeedKmh)
	}
	// Course: BYTE_1=0x14 BYTE_2=0x8F → ((0x14 & 0x03) << 8) | 0x8F = 0x008F = 143
	if p.Heading != 143 {
		t.Errorf("course = %v, want 143", p.Heading)
	}
	// Ignition: BYTE_1 bit6=0 → must be false
	if p.Ignition {
		t.Errorf("ignition = true, want false (bit6 off)")
	}
}

func TestDecoder_Status_AppendixB(t *testing.T) {
	// Appendix B status: 78 78 0A 13 44 01 04 00 01 00 05 08 45 0D 0A
	frame := hexBytes(t, "78 78 0A 13 44 01 04 00 01 00 05 08 45 0D 0A")
	d := New()
	got, err := d.ReadFrame(context.Background(), bytes.NewReader(frame))
	if err != nil {
		t.Fatalf("ReadFrame: %v", err)
	}
	if got.Kind != protocol.FrameHeartbeat {
		t.Fatalf("kind = %v, want Heartbeat", got.Kind)
	}
	if got.Status == nil {
		t.Fatal("Status is nil")
	}
	s := got.Status
	// terminal_info = 0x44 = 01000100 → GPS tracking on, charge on
	if !s.GPSTracking {
		t.Errorf("GPSTracking = false, want true")
	}
	if !s.ChargeOn {
		t.Errorf("ChargeOn = false, want true")
	}
	if s.ACCHigh {
		t.Errorf("ACCHigh = true, want false")
	}
	if s.Alarm != "Normal" {
		t.Errorf("Alarm = %q, want Normal", s.Alarm)
	}
	if s.VoltageLevel != 1 {
		t.Errorf("VoltageLevel = %d, want 1", s.VoltageLevel)
	}
	if s.GSMSignal != 4 {
		t.Errorf("GSMSignal = %d, want 4", s.GSMSignal)
	}
}

func TestDecoder_BadCRC_Errors(t *testing.T) {
	// Login frame with mangled CRC.
	frame := hexBytes(t, "78 78 0D 01 01 23 45 67 89 01 23 45 00 01 FF FF 0D 0A")
	d := New()
	_, err := d.ReadFrame(context.Background(), bytes.NewReader(frame))
	if err == nil {
		t.Fatal("want CRC error, got nil")
	}
}

func TestDecoder_GarbageBeforeStart_IsSkipped(t *testing.T) {
	// Some junk, then a real frame.
	junk := hexBytes(t, "DE AD BE EF CA FE 78 78 0D 01 01 23 45 67 89 01 23 45 00 01 8C DD 0D 0A")
	d := New()
	got, err := d.ReadFrame(context.Background(), bytes.NewReader(junk))
	if err != nil {
		t.Fatalf("ReadFrame: %v", err)
	}
	if got.Kind != protocol.FrameLogin {
		t.Fatalf("kind = %v, want Login", got.Kind)
	}
}

func TestDecoder_Sequential_LoginThenStatus(t *testing.T) {
	// Two back-to-back frames on one connection.
	stream := bytes.Join([][]byte{
		hexBytes(t, "78 78 0D 01 01 23 45 67 89 01 23 45 00 01 8C DD 0D 0A"),
		hexBytes(t, "78 78 0A 13 44 01 04 00 01 00 05 08 45 0D 0A"),
	}, nil)

	d := New()
	r := bytes.NewReader(stream)

	f1, err := d.ReadFrame(context.Background(), r)
	if err != nil {
		t.Fatalf("frame 1: %v", err)
	}
	if f1.Kind != protocol.FrameLogin {
		t.Fatalf("frame 1 kind = %v", f1.Kind)
	}

	f2, err := d.ReadFrame(context.Background(), r)
	if err != nil {
		t.Fatalf("frame 2: %v", err)
	}
	if f2.Kind != protocol.FrameHeartbeat {
		t.Fatalf("frame 2 kind = %v", f2.Kind)
	}
	// IMEI sticky from login.
	if f2.IMEI != "123456789012345" {
		t.Fatalf("frame 2 imei = %q, want sticky from login", f2.IMEI)
	}
}
