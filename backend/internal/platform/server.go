package platform

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Adeqq1/ticket-online/backend/internal/catalog"
	"github.com/Adeqq1/ticket-online/backend/internal/reservation"
)

func NewHandler(db *sql.DB, logger *slog.Logger) http.Handler {
	return NewHandlerWithConfig(db, logger, 10*time.Minute, "")
}

func NewHandlerWithTTL(db *sql.DB, logger *slog.Logger, reservationTTL time.Duration) http.Handler {
	return NewHandlerWithConfig(db, logger, reservationTTL, "")
}

func NewHandlerWithConfig(db *sql.DB, logger *slog.Logger, reservationTTL time.Duration, staticDir string) http.Handler {
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
	mux.HandleFunc("/", staticHandler(staticDir))
	return loggingMiddleware(logger, mux)
}

func staticHandler(staticDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if staticDir == "" || strings.HasPrefix(r.URL.Path, "/api/") || r.URL.Path == "/api" {
			Error(w, http.StatusNotFound, "NOT_FOUND", "Route not found")
			return
		}
		requested := filepath.Join(staticDir, filepath.Clean("/"+r.URL.Path))
		if info, err := os.Stat(requested); err == nil && !info.IsDir() {
			http.ServeFile(w, r, requested)
			return
		}
		index := filepath.Join(staticDir, "index.html")
		if _, err := os.Stat(index); err != nil {
			Error(w, http.StatusNotFound, "NOT_FOUND", "Route not found")
			return
		}
		http.ServeFile(w, r, index)
	}
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
