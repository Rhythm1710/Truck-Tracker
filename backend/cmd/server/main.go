// Command server is the entry point for the truck-tracker backend.
//
// It wires together:
//   - PostgreSQL store
//   - Firebase admin client
//   - Position bus
//   - TCP listener (trackers in) → protocol decoder → bus
//   - HTTP API + WebSocket (mobile app)
//   - A store-writer goroutine subscribed to the bus
//
// All goroutines share a single context cancelled on SIGINT/SIGTERM for
// graceful shutdown.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/rhythm/truck-tracker/internal/api"
	"github.com/rhythm/truck-tracker/internal/auth"
	"github.com/rhythm/truck-tracker/internal/bus"
	"github.com/rhythm/truck-tracker/internal/config"
	"github.com/rhythm/truck-tracker/internal/models"
	"github.com/rhythm/truck-tracker/internal/protocol"
	"github.com/rhythm/truck-tracker/internal/protocol/gt06"
	"github.com/rhythm/truck-tracker/internal/store"
	"github.com/rhythm/truck-tracker/internal/tcp"
	"github.com/rhythm/truck-tracker/internal/ws"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	cfg, err := config.Load()
	if err != nil {
		log.Error("config", "err", err)
		os.Exit(1)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// --- Store ---
	st, err := store.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Error("store", "err", err)
		os.Exit(1)
	}
	defer st.Close()

	// --- Firebase ---
	fb, err := auth.NewFirebase(ctx, cfg.FirebaseCredentialsPath)
	if err != nil {
		log.Error("firebase", "err", err)
		os.Exit(1)
	}

	// --- Position Bus ---
	b := bus.New()

	// Subscribe a store-writer to persist every position.
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		ws.SubscribeBus(ctx, b, func(p models.Position) {
			if err := st.SavePosition(ctx, p); err != nil {
				log.Error("store: save position", "imei", p.IMEI, "err", err)
			}
		})
	}()

	// --- WebSocket hub ---
	hub := ws.NewHub(b, cfg.AppJWTSecret, log)

	// --- HTTP API ---
	router := api.NewRouter(api.Deps{
		Store:        st,
		Firebase:     fb,
		AppJWTSecret: cfg.AppJWTSecret,
		AppJWTTTL:    cfg.AppJWTTTL,
		WSHandler:    hub,
		Log:          log,
	})
	httpSrv := &http.Server{
		Addr:         cfg.HTTPAddr,
		Handler:      router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		log.Info("http: listening", "addr", cfg.HTTPAddr)
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("http", "err", err)
			cancel()
		}
	}()

	// --- TCP listener ---
	tcpSrv := tcp.NewServer(
		cfg.TCPAddr,
		func() protocol.Decoder { return gt06.New() },
		b, log,
	)
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := tcpSrv.Run(ctx); err != nil {
			log.Error("tcp", "err", err)
			cancel()
		}
	}()

	// --- Wait for shutdown ---
	<-ctx.Done()
	log.Info("shutting down")

	shutdownCtx, sCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer sCancel()
	_ = httpSrv.Shutdown(shutdownCtx)

	wg.Wait()
	log.Info("bye")
}
