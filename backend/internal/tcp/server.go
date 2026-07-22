// Package tcp owns the listener that trackers connect to.
//
// One goroutine accepts connections. For each accepted connection, a
// "handle" goroutine reads frames using a Decoder and publishes decoded
// Positions onto the Bus. Goroutines exit on context cancel (graceful
// shutdown) or on read error.
//
// On login, IMEI is resolved to a device row once and pinned on the
// connection — every subsequent position is stamped with that device_id,
// avoiding an IMEI→UUID lookup per packet downstream.
package tcp

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"sync"
	"time"

	"github.com/rhythm/truck-tracker/internal/bus"
	"github.com/rhythm/truck-tracker/internal/metrics"
	"github.com/rhythm/truck-tracker/internal/protocol"
	"github.com/rhythm/truck-tracker/internal/store"
)

type Config struct {
	Addr    string
	Decoder protocol.Factory
	Bus     *bus.Bus
	Store   *store.Store // resolves IMEI→device_id + records sessions
	Log     *slog.Logger
	Metrics *metrics.Metrics
}

type Server struct {
	cfg Config

	wg sync.WaitGroup // tracks active connection goroutines
}

func NewServer(cfg Config) *Server {
	return &Server{cfg: cfg}
}

// Run starts accepting connections. Blocks until ctx is cancelled, then
// waits up to gracePeriod for in-flight handlers to finish.
func (s *Server) Run(ctx context.Context) error {
	lc := net.ListenConfig{KeepAlive: 60 * time.Second}
	ln, err := lc.Listen(ctx, "tcp", s.cfg.Addr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", s.cfg.Addr, err)
	}
	s.cfg.Log.Info("tcp: listening", "addr", s.cfg.Addr)

	// On shutdown, close the listener so Accept() returns.
	go func() {
		<-ctx.Done()
		_ = ln.Close()
	}()

	for {
		conn, err := ln.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				s.cfg.Log.Info("tcp: shutting down, draining connections")
				s.wg.Wait()
				s.cfg.Log.Info("tcp: drained")
				return nil
			}
			s.cfg.Log.Error("tcp: accept", "err", err)
			continue
		}
		s.wg.Add(1)
		go func(c net.Conn) {
			defer s.wg.Done()
			s.handle(ctx, c)
		}(conn)
	}
}

func (s *Server) handle(ctx context.Context, c net.Conn) {
	defer c.Close()

	remote := c.RemoteAddr().String()
	log := s.cfg.Log.With("remote", remote)
	log.Info("tcp: connection opened")
	defer log.Info("tcp: connection closed")

	if s.cfg.Metrics != nil {
		s.cfg.Metrics.TCPConnectionsTotal.Inc()
		s.cfg.Metrics.TCPConnectionsOpen.Inc()
		defer s.cfg.Metrics.TCPConnectionsOpen.Dec()
	}

	dec := s.cfg.Decoder()

	// Per-connection state, set on login.
	var (
		imei      string
		deviceID  string
		sessionID int64
		sessionOK bool
	)
	defer func() {
		if sessionOK {
			closeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := s.cfg.Store.CloseDeviceSession(closeCtx, sessionID); err != nil {
				log.Warn("tcp: close session", "err", err)
			}
		}
	}()

	for {
		// 5-minute idle timeout — trackers heartbeat every ~2 min by default.
		_ = c.SetReadDeadline(time.Now().Add(5 * time.Minute))

		frame, err := dec.ReadFrame(ctx, c)
		if err != nil {
			classifyDecodeErr(err, s.cfg.Metrics)
			log.Info("tcp: read ended", "err", err)
			return
		}

		// Frame metric.
		if s.cfg.Metrics != nil {
			s.cfg.Metrics.TCPFramesTotal.WithLabelValues(frameKindName(frame.Kind)).Inc()
		}

		// On login, resolve IMEI→device_id once and open a session row.
		if frame.Kind == protocol.FrameLogin && frame.IMEI != "" {
			imei = frame.IMEI
			log = log.With("imei", imei)
			devID, derr := s.cfg.Store.ResolveDeviceIDByIMEI(ctx, imei)
			if derr != nil {
				log.Warn("tcp: imei not registered — closing", "err", derr)
				// Still ACK the login so we don't get pounded with retries,
				// but tear down — no point persisting orphan positions.
				_ = writeAck(c, frame.Ack)
				return
			}
			deviceID = devID
			log = log.With("device_id", deviceID)

			ip := ipOnly(remote)
			if sid, oerr := s.cfg.Store.OpenDeviceSession(ctx, deviceID, ip); oerr != nil {
				log.Warn("tcp: open session", "err", oerr)
			} else {
				sessionID = sid
				sessionOK = true
			}
		}

		// Write ACK (with deadline).
		if err := writeAck(c, frame.Ack); err != nil {
			log.Error("tcp: write ack", "err", err)
			return
		}

		// Publish positions onto the bus for downstream consumers.
		if frame.Kind == protocol.FramePosition && frame.Position != nil {
			frame.Position.IMEI = imei
			frame.Position.DeviceID = deviceID // pinned; store skips IMEI lookup
			s.cfg.Bus.Publish(*frame.Position)
		}

		if ctx.Err() != nil {
			return
		}
	}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func writeAck(c net.Conn, ack []byte) error {
	if len(ack) == 0 {
		return nil
	}
	_ = c.SetWriteDeadline(time.Now().Add(10 * time.Second))
	_, err := c.Write(ack)
	return err
}

func ipOnly(remote string) string {
	host, _, err := net.SplitHostPort(remote)
	if err != nil {
		return remote
	}
	return host
}

func frameKindName(k protocol.FrameKind) string {
	switch k {
	case protocol.FrameLogin:
		return "login"
	case protocol.FrameHeartbeat:
		return "heartbeat"
	case protocol.FramePosition:
		return "position"
	case protocol.FrameAlarm:
		return "alarm"
	default:
		return "unknown"
	}
}

func classifyDecodeErr(err error, m *metrics.Metrics) {
	if m == nil || err == nil {
		return
	}
	switch {
	case errors.Is(err, protocol.ErrCRC):
		m.TCPDecodeErrorsTotal.WithLabelValues("crc").Inc()
	case errors.Is(err, protocol.ErrFraming):
		m.TCPDecodeErrorsTotal.WithLabelValues("framing").Inc()
	case errors.Is(err, protocol.ErrUnsupportedProtocol):
		m.TCPDecodeErrorsTotal.WithLabelValues("unsupported").Inc()
	}
}
