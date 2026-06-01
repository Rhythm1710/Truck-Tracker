// Package tcp owns the listener that trackers connect to.
//
// One goroutine accepts connections. For each accepted connection, one
// "connection goroutine" reads frames using a Decoder and publishes
// decoded Positions onto the Bus. Goroutines exit on context cancel
// (graceful shutdown) or on read error.
package tcp

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"time"

	"github.com/rhythm/truck-tracker/internal/bus"
	"github.com/rhythm/truck-tracker/internal/protocol"
)

type Server struct {
	addr    string
	decoder protocol.Factory
	bus     *bus.Bus
	log     *slog.Logger
}

func NewServer(addr string, decoder protocol.Factory, b *bus.Bus, log *slog.Logger) *Server {
	return &Server{addr: addr, decoder: decoder, bus: b, log: log}
}

// Run starts accepting connections. It blocks until ctx is cancelled.
func (s *Server) Run(ctx context.Context) error {
	lc := net.ListenConfig{KeepAlive: 60 * time.Second}
	ln, err := lc.Listen(ctx, "tcp", s.addr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", s.addr, err)
	}
	s.log.Info("tcp: listening", "addr", s.addr)

	// Close the listener on shutdown so Accept() returns.
	go func() {
		<-ctx.Done()
		_ = ln.Close()
	}()

	for {
		conn, err := ln.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				s.log.Info("tcp: shutting down")
				return nil
			}
			s.log.Error("tcp: accept", "err", err)
			continue
		}
		go s.handle(ctx, conn)
	}
}

func (s *Server) handle(ctx context.Context, c net.Conn) {
	defer c.Close()
	log := s.log.With("remote", c.RemoteAddr().String())
	log.Info("tcp: connection opened")
	defer log.Info("tcp: connection closed")

	dec := s.decoder()
	imeiTagged := false
	for {
		// 5-minute idle timeout — trackers heartbeat every ~2 mins by default.
		_ = c.SetReadDeadline(time.Now().Add(5 * time.Minute))

		frame, err := dec.ReadFrame(ctx, c)
		if err != nil {
			log.Info("tcp: read ended", "err", err)
			return
		}

		// Send ACK back if the protocol wants one.
		if len(frame.Ack) > 0 {
			if _, werr := c.Write(frame.Ack); werr != nil {
				log.Error("tcp: write ack", "err", werr)
				return
			}
		}

		// Publish positions onto the bus for downstream consumers.
		if frame.Kind == protocol.FramePosition && frame.Position != nil {
			frame.Position.IMEI = frame.IMEI
			if !imeiTagged && frame.IMEI != "" {
				log = log.With("imei", frame.IMEI)
				imeiTagged = true
			}
			s.bus.Publish(*frame.Position)
		}

		if ctx.Err() != nil {
			return
		}
	}
}
