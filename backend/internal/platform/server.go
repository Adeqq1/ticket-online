package platform

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"log/slog"
	"net/http"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Adeqq1/ticket-online/backend/internal/adminissues"
	"github.com/Adeqq1/ticket-online/backend/internal/adminorders"
	"github.com/Adeqq1/ticket-online/backend/internal/catalog"
	"github.com/Adeqq1/ticket-online/backend/internal/checkin"
	"github.com/Adeqq1/ticket-online/backend/internal/checkout"
	"github.com/Adeqq1/ticket-online/backend/internal/operations"
	"github.com/Adeqq1/ticket-online/backend/internal/orderaccess"
	"github.com/Adeqq1/ticket-online/backend/internal/payment"
	"github.com/Adeqq1/ticket-online/backend/internal/recovery"
	"github.com/Adeqq1/ticket-online/backend/internal/refund"
	"github.com/Adeqq1/ticket-online/backend/internal/reservation"
	"github.com/Adeqq1/ticket-online/backend/internal/staffauth"
)

func NewHandlerWithOrderAccess(db *sql.DB, logger *slog.Logger, reservationTTL time.Duration, staticDir string, development bool, secret []byte) http.Handler {
	return newHandler(db, logger, reservationTTL, staticDir, development, secret, "", "", "sandbox", true, nil, nil)
}

func NewHandlerWithPaymentConfig(db *sql.DB, logger *slog.Logger, reservationTTL time.Duration, staticDir string, development bool, secret []byte, midtransKey, frontendURL string, trustedProxies ...netip.Prefix) http.Handler {
	return newHandler(db, logger, reservationTTL, staticDir, development, secret, midtransKey, frontendURL, "sandbox", true, nil, nil, trustedProxies...)
}

func NewHandlerWithOperations(db *sql.DB, logger *slog.Logger, reservationTTL time.Duration, staticDir string, development bool, secret []byte, midtransKey, frontendURL string, metrics *operations.Service, trustedProxies ...netip.Prefix) http.Handler {
	return newHandler(db, logger, reservationTTL, staticDir, development, secret, midtransKey, frontendURL, "sandbox", true, metrics, nil, trustedProxies...)
}

func NewHandlerWithPaymentRuntime(db *sql.DB, logger *slog.Logger, reservationTTL time.Duration, staticDir string, development bool, secret []byte, midtransKey, frontendURL, environment string, transactionsEnabled bool, metrics *operations.Service, trustedProxies ...netip.Prefix) http.Handler {
	return newHandler(db, logger, reservationTTL, staticDir, development, secret, midtransKey, frontendURL, environment, transactionsEnabled, metrics, nil, trustedProxies...)
}

func NewHandlerWithRefundConfig(db *sql.DB, logger *slog.Logger, reservationTTL time.Duration, staticDir string, development bool, secret []byte, midtransKey, frontendURL, environment string, transactionsEnabled bool, metrics *operations.Service, refundMethods []string, trustedProxies ...netip.Prefix) http.Handler {
	return newHandler(db, logger, reservationTTL, staticDir, development, secret, midtransKey, frontendURL, environment, transactionsEnabled, metrics, refundMethods, trustedProxies...)
}

func newHandler(db *sql.DB, logger *slog.Logger, reservationTTL time.Duration, staticDir string, development bool, secret []byte, midtransKey, frontendURL, environment string, transactionsEnabled bool, metrics *operations.Service, refundMethods []string, trustedProxies ...netip.Prefix) http.Handler {
	mux := http.NewServeMux()
	access := orderaccess.New(db, secret)
	catalogHandler := catalog.NewHandler(catalog.NewService(catalog.NewRepository(db)), logger)
	reservationHandler := reservation.NewHandler(reservation.NewRepository(db, reservationTTL), logger)
	checkoutHandler := checkout.NewHandlerWithAccess(checkout.NewRepository(db), logger, access)
	paymentRepository := payment.NewRepositoryWithMidtransEnvironment(db, environment)
	paymentHandler := payment.NewHandlerWithMidtrans(paymentRepository, logger, access, midtransKey, frontendURL, environment)
	staffService := staffauth.New(db)
	staffauth.NewHandler(staffService, logger).Register(mux)
	catalog.NewAdminHandler(catalog.NewRepository(db), staffService, logger).Register(mux)
	adminorders.NewHandler(adminorders.NewService(db, staffService), logger).Register(mux)
	refundService := refund.New(db, staffService, midtransKey, environment, refundMethods, logger)
	refund.NewHandler(refundService, logger).Register(mux)
	adminissues.NewHandler(adminissues.NewService(db, staffService, access, paymentRepository, midtransKey), logger).Register(mux)
	if metrics == nil {
		metrics = operations.NewService(db, staffService, operations.Process)
	}
	metrics.Register(mux)
	recovery.NewHandler(db, access, logger, trustedProxies...).Register(mux)
	checkinHandler := checkin.NewHandler(checkin.NewService(db, staffService), logger)
	mux.HandleFunc("POST /api/v1/staff/check-ins", checkinHandler.CheckIn)
	mux.HandleFunc("GET /api/v1/staff/ticket-status", checkinHandler.TicketStatus)
	mux.HandleFunc("GET /api/v1/admin/check-ins", checkinHandler.History)
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
	mux.HandleFunc("GET /api/v1/reservations/{reservationID}/event", catalogHandler.ReservationDetail)
	mux.HandleFunc("POST /api/v1/reservations", reservationHandler.Create)
	mux.HandleFunc("GET /api/v1/reservations/{reservationID}", reservationHandler.Get)
	mux.HandleFunc("DELETE /api/v1/reservations/{reservationID}", reservationHandler.Cancel)
	mux.HandleFunc("POST /api/v1/reservations/{reservationID}/convert", reservationHandler.Convert)
	mux.HandleFunc("POST /api/v1/reservations/{reservationID}/checkout", checkoutHandler.Create)
	mux.HandleFunc("GET /api/v1/orders/{orderID}", checkoutHandler.Get)
	mux.HandleFunc("GET /api/v1/orders/{orderID}/tickets", paymentHandler.TicketsForOrder)
	mux.HandleFunc("GET /api/v1/tickets/{ticketID}", paymentHandler.GetTicket)
	if development {
		mux.HandleFunc("POST /api/v1/orders/{orderID}/simulate-payment", paymentHandler.Simulate)
	}
	if midtransKey != "" {
		mux.HandleFunc("POST /api/v1/orders/{orderID}/payments", paymentHandler.CreateSnap)
		mux.HandleFunc("POST /api/v1/payments/midtrans/notification", paymentHandler.MidtransNotification)
	}
	mux.HandleFunc("/", staticHandler(staticDir))
	return loggingMiddleware(logger, transactionGate(transactionsEnabled, mux), operations.Process)
}

func transactionGate(enabled bool, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !enabled && createsTransaction(r.Method, r.URL.Path) {
			w.Header().Set("Cache-Control", "no-store")
			Error(w, http.StatusServiceUnavailable, "TRANSACTIONS_PAUSED", "Transaksi baru sedang dihentikan sementara")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func createsTransaction(method, path string) bool {
	if method != http.MethodPost {
		return false
	}
	if path == "/api/v1/reservations" || strings.HasPrefix(path, "/api/v1/orders/") && (strings.HasSuffix(path, "/payments") || strings.HasSuffix(path, "/simulate-payment")) {
		return true
	}
	if strings.HasPrefix(path, "/api/v1/reservations/") {
		return strings.HasSuffix(path, "/convert") || strings.HasSuffix(path, "/checkout")
	}
	return false
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

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (w *statusRecorder) WriteHeader(status int) {
	if w.status != 0 {
		return
	}
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}
func (w *statusRecorder) Write(value []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(value)
}
func (w *statusRecorder) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func loggingMiddleware(logger *slog.Logger, next http.Handler, tracker *operations.Tracker) http.Handler {
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
		recorder := &statusRecorder{ResponseWriter: w}
		next.ServeHTTP(recorder, r)
		status := recorder.status
		if status == 0 {
			status = http.StatusOK
		}
		tracker.Request(status, time.Now().UTC())
		logger.InfoContext(context.Background(), "http request", "request_id", requestID, "method", r.Method, "path", r.URL.Path, "status", status, "duration", time.Since(started).String())
	})
}
