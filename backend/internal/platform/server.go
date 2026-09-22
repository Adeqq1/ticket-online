package platform

import (
	"context"
	"database/sql"
	"log/slog"
	"net/http"
	"time"
)

func NewHandler(db *sql.DB, logger *slog.Logger) http.Handler {
	mux := http.NewServeMux()
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
		started := time.Now()
		next.ServeHTTP(w, r)
		logger.InfoContext(context.Background(), "http request", "method", r.Method, "path", r.URL.Path, "duration", time.Since(started).String())
	})
}
