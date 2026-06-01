// Package api wires the chi router with all REST handlers.
package api

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	chim "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"github.com/rhythm/truck-tracker/internal/api/middleware"
	"github.com/rhythm/truck-tracker/internal/auth"
	"github.com/rhythm/truck-tracker/internal/models"
	"github.com/rhythm/truck-tracker/internal/store"
)

type Deps struct {
	Store        *store.Store
	Firebase     *auth.Firebase
	AppJWTSecret string
	AppJWTTTL    time.Duration
	WSHandler    http.Handler // ws.Hub
	Log          *slog.Logger
}

func NewRouter(d Deps) http.Handler {
	r := chi.NewRouter()

	r.Use(chim.RequestID)
	r.Use(chim.RealIP)
	r.Use(middleware.RequestLogger(d.Log))
	r.Use(chim.Recoverer)
	r.Use(chim.Timeout(30 * time.Second))
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "OPTIONS"},
		AllowedHeaders:   []string{"Authorization", "Content-Type"},
		AllowCredentials: false,
		MaxAge:           300,
	}))

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	r.Route("/v1", func(r chi.Router) {
		// public
		r.Post("/auth/exchange", d.exchangeHandler())

		// websocket — does its own auth (token query param)
		r.Handle("/live", d.WSHandler)

		// authenticated
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireAuth(d.AppJWTSecret))
			r.Get("/trucks", d.listTrucksHandler())
			r.Get("/trucks/{id}", d.getTruckHandler())
			r.Get("/trucks/{id}/history", d.historyHandler())
		})
	})

	return r
}

// ---------------------------------------------------------------------------
// Handlers
// ---------------------------------------------------------------------------

type exchangeReq struct {
	FirebaseIDToken string `json:"firebaseIdToken"`
}
type exchangeResp struct {
	APIToken  string    `json:"apiToken"`
	ExpiresAt time.Time `json:"expiresAt"`
}

func (d Deps) exchangeHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log := middleware.LogFrom(r.Context())
		var body exchangeReq
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.FirebaseIDToken == "" {
			writeErr(w, http.StatusBadRequest, "missing firebaseIdToken")
			return
		}

		phone, err := d.Firebase.VerifyIDToken(r.Context(), body.FirebaseIDToken)
		if err != nil {
			log.Warn("auth: firebase verify failed", "err", err)
			writeErr(w, http.StatusUnauthorized, "invalid firebase token")
			return
		}

		uid, err := d.Store.EnsureUserByPhone(r.Context(), phone)
		if err != nil {
			log.Error("auth: ensure user", "err", err)
			writeErr(w, http.StatusInternalServerError, "internal")
			return
		}

		exp := time.Now().Add(d.AppJWTTTL)
		tok, err := auth.Issue(d.AppJWTSecret, auth.Claims{
			UserID:    uid,
			Phone:     phone,
			ExpiresAt: exp.Unix(),
		})
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "issue token")
			return
		}
		writeJSON(w, http.StatusOK, exchangeResp{APIToken: tok, ExpiresAt: exp})
	}
}

func (d Deps) listTrucksHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log := middleware.LogFrom(r.Context())
		claims, _ := middleware.ClaimsFrom(r.Context())
		trucks, err := d.Store.ListTrucks(r.Context(), claims.UserID)
		if err != nil {
			log.Error("api: list trucks", "err", err)
			writeErr(w, http.StatusInternalServerError, "internal")
			return
		}
		writeJSON(w, http.StatusOK, trucks)
	}
}

func (d Deps) getTruckHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log := middleware.LogFrom(r.Context())
		claims, _ := middleware.ClaimsFrom(r.Context())
		id := chi.URLParam(r, "id")
		if id == "" {
			writeErr(w, http.StatusBadRequest, "missing id")
			return
		}
		truck, err := d.Store.GetTruck(r.Context(), claims.UserID, id)
		if errors.Is(err, store.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "not found")
			return
		}
		if err != nil {
			log.Error("api: get truck", "id", id, "err", err)
			writeErr(w, http.StatusInternalServerError, "internal")
			return
		}
		writeJSON(w, http.StatusOK, truck)
	}
}

// historyHandler returns positions for a given device in a time window.
// Defaults: from = now-24h, to = now, limit = 1000 (capped at 5000 in store).
func (d Deps) historyHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log := middleware.LogFrom(r.Context())
		claims, _ := middleware.ClaimsFrom(r.Context())
		id := chi.URLParam(r, "id")
		if id == "" {
			writeErr(w, http.StatusBadRequest, "missing id")
			return
		}

		now := time.Now().UTC()
		from := parseTime(r.URL.Query().Get("from"), now.Add(-24*time.Hour))
		to := parseTime(r.URL.Query().Get("to"), now)
		if !from.Before(to) {
			writeErr(w, http.StatusBadRequest, "from must be before to")
			return
		}
		limit := parseInt(r.URL.Query().Get("limit"), 1000)

		positions, err := d.Store.ListPositionHistory(r.Context(), claims.UserID, id, from, to, limit)
		if err != nil {
			log.Error("api: history", "id", id, "err", err)
			writeErr(w, http.StatusInternalServerError, "internal")
			return
		}
		writeJSON(w, http.StatusOK, positions)
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

type errBody struct {
	Error string `json:"error"`
}

func writeJSON(w http.ResponseWriter, code int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(body)
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, errBody{Error: msg})
}

func parseTime(s string, fallback time.Time) time.Time {
	if s == "" {
		return fallback
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return fallback
	}
	return t
}

func parseInt(s string, fallback int) int {
	if s == "" {
		return fallback
	}
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return fallback
	}
	return n
}

// silence the linter when models is referenced only via store-returned types
var _ = models.Position{}
