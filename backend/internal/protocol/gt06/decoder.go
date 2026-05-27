// Package gt06 implements Pictor's PS10A and PT06 GT06-family protocols.
//
// Frame envelope (identical across PS10A and PT06):
//
//   short packets:  78 78  LEN(1)   PROTO  BODY...  SERIAL(2)  CRC(2)  0D 0A
//   long  packets:  79 79  LEN(2)   PROTO  BODY...  SERIAL(2)  CRC(2)  0D 0A
//
//   LEN counts: PROTO + BODY + SERIAL + CRC  (i.e., everything between LEN
//   and the trailing 0D 0A, excluding LEN itself).
//
//   CRC-ITU is computed over [LEN .. SERIAL] inclusive.
//
// Protocol numbers we recognize (* = PT06-specific, ° = PS10A-specific):
//
//   0x01  Login
//   0x12  Location (PS10A)°
//   0x22  Location with UTC + ACC + reporting mode (PT06)*
//   0x13  Status / Heartbeat
//   0x16  Alarm (PS10A)°
//   0x26  Alarm with UTC (PT06)*
//
// Unknown but well-framed packets are ACK'd and returned as FrameUnknown so
// the connection stays open.
package gt06

import (
	"bufio"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"time"

	"github.com/rhythm/truck-tracker/internal/models"
	"github.com/rhythm/truck-tracker/internal/protocol"
	"github.com/rhythm/truck-tracker/internal/protocol/crc"
)

// Protocol numbers.
const (
	protoLogin       byte = 0x01
	protoLocationA   byte = 0x12 // PS10A
	protoLocationP   byte = 0x22 // PT06 (UTC)
	protoStatus      byte = 0x13
	protoAlarmA      byte = 0x16 // PS10A
	protoAlarmP      byte = 0x26 // PT06 (UTC)
	protoServerCmd   byte = 0x80
)

type Decoder struct {
	br   *bufio.Reader
	imei string // captured on login, reused on subsequent frames
}

func New() *Decoder { return &Decoder{} }

// ReadFrame finds the next valid frame on r and decodes it.
//
// Robustness rules:
//   - Up to 4096 bytes of pre-start garbage are tolerated before giving up.
//   - CRC errors return ErrCRC. The caller logs and continues — trackers
//     send a fresh frame every few seconds, so dropping one is benign.
//   - Unknown protocol numbers return FrameUnknown with an ACK so the
//     tracker doesn't disconnect.
func (d *Decoder) ReadFrame(ctx context.Context, r io.Reader) (protocol.Frame, error) {
	if d.br == nil {
		d.br = bufio.NewReader(r)
	}
	if err := ctx.Err(); err != nil {
		return protocol.Frame{}, err
	}

	// Find start marker. 0x78 0x78 or 0x79 0x79.
	var longLen bool
	for i := 0; i < 4096; i++ {
		b, err := d.br.ReadByte()
		if err != nil {
			return protocol.Frame{}, err
		}
		if b != 0x78 && b != 0x79 {
			continue
		}
		b2, err := d.br.ReadByte()
		if err != nil {
			return protocol.Frame{}, err
		}
		if b2 == b { // matched pair
			longLen = (b == 0x79)
			return d.readPacket(longLen)
		}
	}
	return protocol.Frame{}, fmt.Errorf("gt06: no start marker in 4096 bytes: %w", protocol.ErrFraming)
}

// readPacket reads everything between the start marker and 0x0D 0x0A.
func (d *Decoder) readPacket(longLen bool) (protocol.Frame, error) {
	// Read LEN (1 or 2 bytes).
	var length int
	var lenBytes []byte
	if longLen {
		var hb [2]byte
		if _, err := io.ReadFull(d.br, hb[:]); err != nil {
			return protocol.Frame{}, err
		}
		length = int(binary.BigEndian.Uint16(hb[:]))
		lenBytes = hb[:]
	} else {
		lb, err := d.br.ReadByte()
		if err != nil {
			return protocol.Frame{}, err
		}
		length = int(lb)
		lenBytes = []byte{lb}
	}
	if length < 5 { // minimum: PROTO(1) + SERIAL(2) + CRC(2)
		return protocol.Frame{}, fmt.Errorf("gt06: length %d too small: %w", length, protocol.ErrFraming)
	}

	// Read LEN bytes: PROTO + BODY + SERIAL + CRC.
	rest := make([]byte, length)
	if _, err := io.ReadFull(d.br, rest); err != nil {
		return protocol.Frame{}, err
	}

	// Trailing 0x0D 0x0A.
	var tail [2]byte
	if _, err := io.ReadFull(d.br, tail[:]); err != nil {
		return protocol.Frame{}, err
	}
	if tail != [2]byte{0x0D, 0x0A} {
		return protocol.Frame{}, fmt.Errorf("gt06: bad tail %02X %02X: %w", tail[0], tail[1], protocol.ErrFraming)
	}

	// Verify CRC over [LEN .. SERIAL].
	crcStart := append([]byte{}, lenBytes...)
	crcStart = append(crcStart, rest[:length-2]...) // exclude CRC bytes
	wantCRC := binary.BigEndian.Uint16(rest[length-2:])
	if got := crc.Sum(crcStart); got != wantCRC {
		return protocol.Frame{}, fmt.Errorf("gt06: want %04X got %04X: %w", wantCRC, got, protocol.ErrCRC)
	}

	proto := rest[0]
	body := rest[1 : length-4] // strip serial(2) + crc(2)
	serial := binary.BigEndian.Uint16(rest[length-4 : length-2])

	// Dispatch.
	switch proto {
	case protoLogin:
		return d.parseLogin(body, serial)
	case protoLocationA, protoLocationP:
		return d.parseLocation(proto, body, serial)
	case protoStatus:
		return d.parseStatus(body, serial)
	case protoAlarmA, protoAlarmP:
		return d.parseAlarm(proto, body, serial)
	default:
		return protocol.Frame{
			Kind: protocol.FrameUnknown,
			IMEI: d.imei,
			Ack:  buildAck(proto, serial),
		}, nil
	}
}

// ---------------------------------------------------------------------------
// Body parsers
// ---------------------------------------------------------------------------

// parseLogin handles both the PS10A (8-byte IMEI) and the PT06 (8-byte IMEI
// + 2-byte model id + 2-byte timezone/language) variants.
func (d *Decoder) parseLogin(body []byte, serial uint16) (protocol.Frame, error) {
	if len(body) < 8 {
		return protocol.Frame{}, fmt.Errorf("gt06: login body %d bytes < 8", len(body))
	}
	d.imei = decodeBCDIMEI(body[:8])
	// We deliberately ignore PT06's extra model/timezone bytes for now —
	// they're not part of our domain model and we can revisit if needed.
	return protocol.Frame{
		Kind: protocol.FrameLogin,
		IMEI: d.imei,
		Ack:  buildAck(protoLogin, serial),
	}, nil
}

// parseLocation handles both 0x12 (PS10A) and 0x22 (PT06).
//
// PS10A body layout (verified against spec App-B example):
//   GPS core (18 bytes): date(6) gps_info(1) lat(4) lng(4) speed(1) course_status(2)
//   LBS      (8 bytes):  mcc(2) mnc(1) lac(2) cell_id(3)
//   = 26 bytes; +4 if mileage is enabled.
//
// PT06 body adds 3 bytes after LBS: acc(1) report_mode(1) gps_retx(1).
// Mileage (if enabled) is the last 4 bytes.
const gpsCoreLen = 18
const lbsLen = 8

func (d *Decoder) parseLocation(proto byte, body []byte, serial uint16) (protocol.Frame, error) {
	want := gpsCoreLen + lbsLen
	if len(body) < want {
		return protocol.Frame{}, fmt.Errorf("gt06: location body %d bytes < %d", len(body), want)
	}
	pos, err := decodeGPSCore(body[:gpsCoreLen])
	if err != nil {
		return protocol.Frame{}, err
	}
	pos.IMEI = d.imei
	pos.RecvAt = time.Now().UTC()

	// PT06-specific tail right after LBS: ACC + report_mode + GPS retx.
	if proto == protoLocationP && len(body) >= want+3 {
		acc := body[want] == 0x01
		pos.Ignition = acc
		// body[want+1] = data reporting mode (ignored)
		// body[want+2] = GPS re-transmission flag (ignored for now)
	}

	return protocol.Frame{
		Kind:     protocol.FramePosition,
		IMEI:     d.imei,
		Position: &pos,
		Ack:      buildAck(proto, serial),
	}, nil
}

// parseStatus handles 0x13 heartbeat:
//   body = terminal_info(1) voltage(1) gsm_signal(1) alarm_lang(2)
// = 5 bytes.
func (d *Decoder) parseStatus(body []byte, serial uint16) (protocol.Frame, error) {
	if len(body) < 5 {
		return protocol.Frame{}, fmt.Errorf("gt06: status body %d bytes < 5", len(body))
	}
	st := decodeStatus(body[0], body[1], body[2], body[3], body[4])
	return protocol.Frame{
		Kind:   protocol.FrameHeartbeat,
		IMEI:   d.imei,
		Status: &st,
		Ack:    buildAck(protoStatus, serial),
	}, nil
}

// parseAlarm handles 0x16 (PS10A) and 0x26 (PT06).
//
// Verified body layout (App-B PS10A alarm = 32 bytes):
//   GPS core (18) + LBS Length (1) + LBS (8) + status block (5)
//
// Spec §5.3.1.14–18: status block = terminal_info(1) voltage(1) gsm(1) alarm_lang(2)
//
// Both PS10A and PT06 include the LBS Length byte in alarm frames (the
// vendor's PS10A example shows 0x09 there; PT06 spec also has it).
func (d *Decoder) parseAlarm(proto byte, body []byte, serial uint16) (protocol.Frame, error) {
	const want = gpsCoreLen + 1 /*lbsLen byte*/ + lbsLen + 5 /*status*/
	if len(body) < want {
		return protocol.Frame{}, fmt.Errorf("gt06: alarm body %d bytes < %d", len(body), want)
	}
	pos, err := decodeGPSCore(body[:gpsCoreLen])
	if err != nil {
		return protocol.Frame{}, err
	}
	pos.IMEI = d.imei
	pos.RecvAt = time.Now().UTC()

	statusOff := gpsCoreLen + 1 + lbsLen
	st := decodeStatus(body[statusOff], body[statusOff+1], body[statusOff+2], body[statusOff+3], body[statusOff+4])

	// Use the terminal info's ACC bit to set ignition on the position.
	pos.Ignition = st.ACCHigh

	return protocol.Frame{
		Kind:     protocol.FrameAlarm,
		IMEI:     d.imei,
		Position: &pos,
		Status:   &st,
		Ack:      buildAck(proto, serial),
	}, nil
}

// ---------------------------------------------------------------------------
// Field decoders
// ---------------------------------------------------------------------------

// decodeGPSCore decodes the first 18 bytes of a location/alarm body — just
// the GPS portion (no LBS). Caller is responsible for the LBS and tail bytes.
//
// Layout: date(6) gps_info(1) lat(4) lng(4) speed(1) course_status(2)
//
// Caller must set IMEI and RecvAt after this returns.
func decodeGPSCore(b []byte) (models.Position, error) {
	if len(b) < gpsCoreLen {
		return models.Position{}, fmt.Errorf("gt06: gps core %d bytes < %d", len(b), gpsCoreLen)
	}

	yr := 2000 + int(b[0])
	mo := int(b[1])
	dy := int(b[2])
	hr := int(b[3])
	mn := int(b[4])
	sc := int(b[5])
	deviceAt := time.Date(yr, time.Month(mo), dy, hr, mn, sc, 0, time.UTC)

	// b[6] = gps_info_length(high nibble) | satellites(low nibble) — we don't model it.

	latRaw := binary.BigEndian.Uint32(b[7:11])
	lngRaw := binary.BigEndian.Uint32(b[11:15])
	speed := float64(b[15])

	csByte1 := b[16]
	csByte2 := b[17]
	course := float64(((uint16(csByte1)&0x03)<<8)|uint16(csByte2)) // 0..1023, practical 0..360

	// Sign bits per spec §5.2.1.9:
	//   csByte1 bit2 (0x04): 1 = North, 0 = South
	//   csByte1 bit3 (0x08): 0 = East,  1 = West
	lat := float64(latRaw) / 30000.0 / 60.0
	if csByte1&0x04 == 0 {
		lat = -lat
	}
	lng := float64(lngRaw) / 30000.0 / 60.0
	if csByte1&0x08 != 0 {
		lng = -lng
	}

	// Ignition from BYTE_1 bit7 (only valid when bit6 = 1, per spec note).
	ignition := (csByte1&0x40 != 0) && (csByte1&0x80 != 0)

	// (We discard MCC/MNC/LAC/CellID — could be useful later for LBS fallback.)

	return models.Position{
		Lat:      lat,
		Lng:      lng,
		SpeedKmh: speed,
		Heading:  course,
		Ignition: ignition,
		DeviceAt: deviceAt,
	}, nil
}

// decodeStatus interprets the 5-byte status block:
//   terminal_info(1) voltage(1) gsm_signal(1) alarm_lang(2)
func decodeStatus(termInfo, voltage, gsm, alarmHi, alarmLo byte) protocol.DeviceStatus {
	st := protocol.DeviceStatus{
		OilCut:       termInfo&0x80 != 0,
		GPSTracking:  termInfo&0x40 != 0,
		ChargeOn:     termInfo&0x04 != 0,
		ACCHigh:      termInfo&0x02 != 0,
		Defense:      termInfo&0x01 != 0,
		VoltageLevel: voltage,
		GSMSignal:    gsm,
		AlarmCode:    alarmHi,
		Language:     alarmLo,
	}
	// bits 5-3 = alarm code (per spec §5.4.1.4)
	switch (termInfo >> 3) & 0x07 {
	case 0b100:
		st.Alarm = "SOS"
	case 0b011:
		st.Alarm = "LowBattery"
	case 0b010:
		st.Alarm = "PowerCut"
	case 0b001:
		st.Alarm = "Shock"
	default:
		st.Alarm = "Normal"
	}
	return st
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// buildAck constructs a server→tracker ACK: 78 78 05 PROTO SER_H SER_L CRC_H CRC_L 0D 0A.
// LEN=5 covers: PROTO(1) + SERIAL(2) + CRC(2).
func buildAck(proto byte, serial uint16) []byte {
	body := []byte{0x05, proto}
	body = binary.BigEndian.AppendUint16(body, serial)
	c := crc.Sum(body)
	out := []byte{0x78, 0x78}
	out = append(out, body...)
	out = binary.BigEndian.AppendUint16(out, c)
	out = append(out, 0x0D, 0x0A)
	return out
}

// decodeBCDIMEI converts 8 BCD-encoded bytes (16 nibbles) into a 15-digit
// IMEI string. The leading nibble is a zero pad per spec.
func decodeBCDIMEI(b []byte) string {
	digits := make([]byte, 16)
	for i, c := range b {
		digits[i*2] = '0' + (c >> 4)
		digits[i*2+1] = '0' + (c & 0x0F)
	}
	// Strip the leading pad nibble to get exactly 15 digits.
	for len(digits) > 15 && digits[0] == '0' {
		digits = digits[1:]
	}
	return string(digits)
}
