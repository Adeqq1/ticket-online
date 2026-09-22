package platform

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"log/slog"
	"net/http"
	"time"

	"github.com/Adeqq1/ticket-online/backend/internal/catalog"
	"github.com/Adeqq1/ticket-online/backend/internal/reservation"
)

func NewHandler(db *sql.DB, logger *slog.Logger) http.Handler {
	return NewHandlerWithTTL(db, logger, 10*time.Minute)
}

func NewHandlerWithTTL(db *sql.DB, logger *slog.Logger, reservationTTL time.Duration) http.Handler {
	mux := http.NewServeMux()
	catalogHandler := catalog.NewHandler(catalog.NewService(catalog.NewRepository(db)), logger)
	reservationHandler := reservation.NewHandler(reservation.NewRepository(db, reservationTTL), logger)
	mux.HandleFunc("GET /api/v1/health", func(w http.ResponseWriter, _ *http.Request) {
		JSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /api/v1/ready", func(w http.ResponseWriter, r *http.Request) {
		if err := db.PingContext(r.Context()); err != nil {
			Error(w, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "Service is not ready")
			return
		}
		JSON(w, http.StatusOK, map[string]string{"status": "ready"})
	})
	mux.HandleFunc("GET /api/v1/events", catalogHandler.List)
	mux.HandleFunc("GET /api/v1/events/{eventID}", catalogHandler.Detail)
	mux.HandleFunc("POST /api/v1/reservations", reservationHandler.Create)
	mux.HandleFunc("GET /api/v1/reservations/{reservationID}", reservationHandler.Get)
	mux.HandleFunc("DELETE /api/v1/reservations/{reservationID}", reservationHandler.Cancel)
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		Error(w, http.StatusNotFound, "NOT_FOUND", "Route not found")
	})
	return loggingMiddleware(logger, mux)
}

func NewHTTPServer(addr string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
}

func loggingMiddleware(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Header.Get("X-Request-ID")
		if requestID == "" {
			var bytes [16]byte
			if _, err := rand.Read(bytes[:]); err == nil {
				requestID = hex.EncodeToString(bytes[:])
			} else {
				requestID = "unknown"
			}
		}
		w.Header().Set("X-Request-ID", requestID)
		started := time.Now()
		next.ServeHTTP(w, r)
		logger.InfoContext(context.Background(), "http request", "request_id", requestID, "method", r.Method, "path", r.URL.Path, "duration", time.Since(started).String())
	})
}
