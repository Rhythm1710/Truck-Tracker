// Command server is the entry point for the truck-tracker backend.
//
// It wires together:
//   - PostgreSQL store
//   - Firebase admin client
//   - Position bus
//   - TCP listener (trackers in) → protocol decoder → bus
//   - HTTP API + WebSocket (mobile app)
//   - A store-writer goroutine subscribed to the bus
//   - Optional Prometheus /metrics endpoint
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

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/rhythm/truck-tracker/internal/api"
	"github.com/rhythm/truck-tracker/internal/auth"
	"github.com/rhythm/truck-tracker/internal/bus"
	"github.com/rhythm/truck-tracker/internal/config"
	"github.com/rhythm/truck-tracker/internal/metrics"
	"github.com/rhythm/truck-tracker/internal/models"
	"github.com/rhythm/truck-tracker/internal/protocol"
	"github.com/rhythm/truck-tracker/internal/protocol/gt06"
	"github.com/rhythm/truck-tracker/internal/store"
	"github.com/rhythm/truck-tracker/internal/tcp"
	"github.com/rhythm/truck-tracker/internal/ws"
)

func main() {
	bootstrap := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	cfg, err := config.Load()
	if err != nil {
		bootstrap.Error("config", "err", err)
		os.Exit(1)
	}

	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.SlogLevel()}))

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// --- Metrics registry (custom — not the global one) ---
	reg := prometheus.NewRegistry()
	m := metrics.New(reg)

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
	b := bus.NewWithMetrics(m)

	// Subscribe a store-writer to persist every position.
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		ws.SubscribeBus(ctx, b, func(p models.Position) {
			start := time.Now()
			if err := st.SavePosition(ctx, p); err != nil {
				log.Error("store: save position", "imei", p.IMEI, "err", err)
				m.PositionSaveErrors.Inc()
				return
			}
			m.PositionSaveDuration.Observe(time.Since(start).Seconds())
		})
	}()

	// --- WebSocket hub ---
	hub := ws.NewHub(b, cfg.AppJWTSecret, log, st, m)

	// --- HTTP API ---
	var metricsHandler http.Handler
	if cfg.MetricsEnabled {
		metricsHandler = promhttp.HandlerFor(reg, promhttp.HandlerOpts{Registry: reg})
	}
	router := api.NewRouter(api.Deps{
		Store:          st,
		Firebase:       fb,
		AppJWTSecret:   cfg.AppJWTSecret,
		AppJWTTTL:      cfg.AppJWTTTL,
		WSHandler:      hub,
		Log:            log,
		MetricsHandler: metricsHandler,
	})
	httpSrv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second, // slowloris defense
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second, // close zombie keep-alives
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		log.Info("http: listening", "addr", cfg.HTTPAddr, "metrics", cfg.MetricsEnabled)
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("http", "err", err)
			cancel()
		}
	}()

	// --- TCP listener ---
	tcpSrv := tcp.NewServer(tcp.Config{
		Addr:    cfg.TCPAddr,
		Decoder: func() protocol.Decoder { return gt06.New() },
		Bus:     b,
		Store:   st,
		Log:     log,
		Metrics: m,
	})
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
