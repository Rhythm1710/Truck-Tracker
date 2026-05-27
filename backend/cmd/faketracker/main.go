// Command faketracker simulates a Pictor PS10A device for end-to-end testing.
//
// It opens a TCP connection to our server, sends a Login packet with a
// configurable IMEI, then a Heartbeat, then a stream of Location packets
// that walk in a small circle around a chosen lat/lng.
//
// Run alongside the real server:
//
//	# terminal 1
//	cd backend && make run
//
//	# terminal 2
//	go run ./cmd/faketracker \
//	    -addr 127.0.0.1:5023 \
//	    -imei 123456789012345 \
//	    -lat 29.6857 -lng 76.9905 \
//	    -interval 5s
//
// You should see the server log "tcp: connection opened", login + position
// frames decoded, and live updates flowing out to any connected WebSocket
// client.
package main

import (
	"context"
	"encoding/binary"
	"flag"
	"io"
	"log"
	"math"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rhythm/truck-tracker/internal/protocol/crc"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:5023", "TCP address of the truck-tracker server")
	imei := flag.String("imei", "123456789012345", "15-digit IMEI to send in the login packet")
	lat := flag.Float64("lat", 29.6857, "Starting latitude (default Karnal, India)")
	lng := flag.Float64("lng", 76.9905, "Starting longitude")
	speed := flag.Int("speed", 45, "Reported speed in km/h (0-255)")
	interval := flag.Duration("interval", 5*time.Second, "Position-report interval")
	flag.Parse()

	if len(*imei) != 15 {
		log.Fatalf("imei must be exactly 15 digits, got %q (%d)", *imei, len(*imei))
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	conn, err := net.Dial("tcp", *addr)
	if err != nil {
		log.Fatalf("dial %s: %v", *addr, err)
	}
	defer conn.Close()
	log.Printf("connected to %s as IMEI %s", *addr, *imei)

	// Server replies arrive on this goroutine — we just log them.
	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := conn.Read(buf)
			if err != nil {
				if err != io.EOF {
					log.Printf("read: %v", err)
				}
				return
			}
			log.Printf("← server: % X", buf[:n])
		}
	}()

	var serial uint16 = 1

	// 1) Login.
	if _, err := conn.Write(buildLogin(*imei, serial)); err != nil {
		log.Fatalf("write login: %v", err)
	}
	log.Printf("→ login (serial=%d)", serial)
	serial++

	// Give the server a beat to ACK before we start chattering.
	time.Sleep(500 * time.Millisecond)

	// 2) One heartbeat.
	if _, err := conn.Write(buildHeartbeat(serial)); err != nil {
		log.Fatalf("write heartbeat: %v", err)
	}
	log.Printf("→ heartbeat (serial=%d)", serial)
	serial++

	// 3) Location updates in a small clockwise circle until ctx is cancelled.
	tick := time.NewTicker(*interval)
	defer tick.Stop()
	angle := 0.0
	for {
		select {
		case <-ctx.Done():
			log.Printf("shutting down")
			return
		case <-tick.C:
			// Walk a ~30 m circle so the marker visibly moves on the map.
			const radiusDeg = 0.0003
			curLat := *lat + radiusDeg*math.Cos(angle)
			curLng := *lng + radiusDeg*math.Sin(angle)
			angle += math.Pi / 12

			frame := buildLocation(curLat, curLng, *speed, int(angle*180/math.Pi)%360, time.Now().UTC(), serial)
			if _, err := conn.Write(frame); err != nil {
				log.Fatalf("write location: %v", err)
			}
			log.Printf("→ location lat=%.6f lng=%.6f speed=%d (serial=%d)", curLat, curLng, *speed, serial)
			serial++
		}
	}
}

// ---------------------------------------------------------------------------
// Frame builders — minimal but spec-compliant
// ---------------------------------------------------------------------------

// buildLogin packs a 15-digit IMEI as 8 bytes BCD with a leading 0 pad.
func buildLogin(imei string, serial uint16) []byte {
	padded := "0" + imei // 16 nibbles
	body := make([]byte, 1+8)
	body[0] = 0x01
	for i := 0; i < 8; i++ {
		hi := padded[i*2] - '0'
		lo := padded[i*2+1] - '0'
		body[1+i] = (hi << 4) | lo
	}
	return wrap(body, serial)
}

// buildHeartbeat: terminal_info=0x44 (GPS on, charge on), voltage=4, gsm=4,
// alarm/lang=0x00 0x02 (normal, English).
func buildHeartbeat(serial uint16) []byte {
	body := []byte{0x13, 0x44, 0x04, 0x04, 0x00, 0x02}
	return wrap(body, serial)
}

// buildLocation: PS10A 0x12 frame for one GPS fix. Always reports
// north-positive lat and east-positive lng — adjust signs in the caller if
// you need a different hemisphere.
func buildLocation(lat, lng float64, speedKmh, course int, t time.Time, serial uint16) []byte {
	body := []byte{0x12}

	// Date (UTC, 6 bytes: y-2000, m, d, h, m, s).
	body = append(body,
		byte(t.Year()-2000), byte(t.Month()), byte(t.Day()),
		byte(t.Hour()), byte(t.Minute()), byte(t.Second()),
	)

	// GPS info byte: length=12, sats=10 (cosmetic).
	body = append(body, 0xCA)

	// Lat/lng to raw int = degrees * 60 * 30000, big-endian uint32.
	body = binary.BigEndian.AppendUint32(body, uint32(math.Abs(lat)*60*30000))
	body = binary.BigEndian.AppendUint32(body, uint32(math.Abs(lng)*60*30000))

	// Speed.
	if speedKmh < 0 {
		speedKmh = 0
	} else if speedKmh > 255 {
		speedKmh = 255
	}
	body = append(body, byte(speedKmh))

	// Course (10 bits) + status flags in BYTE_1.
	// BYTE_1 bits: bit7=ACC ON, bit6=GPS tracking on, bit5=0(realtime),
	// bit4=1(positioned), bit3=0(East), bit2=1(North), bits1-0 = course high bits.
	if course < 0 {
		course = 0
	} else if course > 1023 {
		course = course % 1024
	}
	statusFlags := byte(0x80 | 0x40 | 0x10 | 0x04) // ACC+GPS+positioned+North, East implicit (bit3=0)
	byte1 := statusFlags | byte((course>>8)&0x03)
	byte2 := byte(course & 0xFF)
	body = append(body, byte1, byte2)

	// LBS info — fake values that are valid but irrelevant for our app.
	body = binary.BigEndian.AppendUint16(body, 404)  // MCC (India)
	body = append(body, 10)                          // MNC
	body = binary.BigEndian.AppendUint16(body, 5000) // LAC
	body = append(body, 0x00, 0x1F, 0xB8)            // Cell ID (3 bytes)

	return wrap(body, serial)
}

// wrap takes [PROTO+BODY] bytes and emits the full GT06 frame:
// 78 78 LEN PROTO BODY SERIAL CRC 0D 0A.
func wrap(protoAndBody []byte, serial uint16) []byte {
	// LEN counts: PROTO + BODY + SERIAL(2) + CRC(2) = len(protoAndBody) + 4.
	length := byte(len(protoAndBody) + 4)
	covered := []byte{length}
	covered = append(covered, protoAndBody...)
	covered = binary.BigEndian.AppendUint16(covered, serial)
	c := crc.Sum(covered)

	out := []byte{0x78, 0x78}
	out = append(out, covered...)
	out = binary.BigEndian.AppendUint16(out, c)
	out = append(out, 0x0D, 0x0A)
	return out
}

var _ = os.Args // touch os to satisfy unused-import linters if log gets stripped
