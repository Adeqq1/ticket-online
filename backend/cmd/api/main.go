package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Adeqq1/ticket-online/backend/internal/email"
	"github.com/Adeqq1/ticket-online/backend/internal/operations"
	"github.com/Adeqq1/ticket-online/backend/internal/payment"
	"github.com/Adeqq1/ticket-online/backend/internal/platform"
	"github.com/Adeqq1/ticket-online/backend/internal/refund"
	"github.com/Adeqq1/ticket-online/backend/internal/reservation"
	"github.com/Adeqq1/ticket-online/backend/internal/staffauth"
	"github.com/Adeqq1/ticket-online/backend/migrations"
)

func main() {
	if len(os.Args) == 2 && os.Args[1] == "healthcheck" {
		healthcheck()
		return
	}
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	cfg, err := platform.LoadConfig()
	if err != nil {
		logger.Error("invalid configuration", "error", err)
		os.Exit(1)
	}

	startupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	db, err := platform.OpenDatabase(startupCtx, cfg.MySQLDSN)
	cancel()
	if err != nil {
		logger.Error("database unavailable", "error", err)
		os.Exit(1)
	}
	defer db.Close()
	migrationCtx, cancelMigration := context.WithTimeout(context.Background(), 30*time.Second)
	if err := migrations.Run(migrationCtx, db); err != nil {
		cancelMigration()
		logger.Error("migration failed", "error", err)
		os.Exit(1)
	}
	cancelMigration()
	bindingCtx, cancelBinding := context.WithTimeout(context.Background(), 10*time.Second)
	if err := platform.BindPaymentEnvironment(bindingCtx, db, cfg.MidtransEnvironment); err != nil {
		cancelBinding()
		logger.Error("database payment environment mismatch", "error", err)
		os.Exit(1)
	}
	cancelBinding()

	serverCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	slog.SetDefault(logger)
	metrics := operations.NewService(db, staffauth.New(db), operations.Process)
	server := platform.NewHTTPServer(cfg.HTTPAddr, platform.NewHandlerWithRefundConfig(db, logger, cfg.ReservationTTL, cfg.StaticDir, cfg.AppEnv == "development", cfg.OrderAccessSecret, cfg.MidtransServerKey, cfg.FrontendURL, cfg.MidtransEnvironment, cfg.TransactionsEnabled, metrics, cfg.MidtransRefundMethods, cfg.TrustedProxyCIDRs...))
	worker := reservation.NewWorker(reservation.NewRepository(db, cfg.ReservationTTL), cfg.ExpiryInterval, logger)
	operations.Process.Register("reservation", time.Now())
	operations.Process.Register("payment_expiry", time.Now())
	operations.Process.Register("email", time.Now())
	go worker.Run(serverCtx)
	go payment.NewRepositoryWithMidtransEnvironment(db, cfg.MidtransEnvironment).RunExpiryWorker(serverCtx, cfg.ExpiryInterval, cfg.MidtransServerKey, logger)
	go refund.New(db, staffauth.New(db), cfg.MidtransServerKey, cfg.MidtransEnvironment, cfg.MidtransRefundMethods, logger).Run(serverCtx, cfg.ExpiryInterval)
	go metrics.Run(serverCtx, logger)
	emailWorker := email.NewService(db, email.Config{
		Host: cfg.SMTPHost, Port: cfg.SMTPPort, Username: cfg.SMTPUsername, Password: cfg.SMTPPassword,
		From: cfg.SMTPFrom, TLSMode: cfg.SMTPTLSMode, FrontendURL: cfg.FrontendURL, AccessSecret: cfg.OrderAccessSecret,
	}, logger)
	emailWorkerDone := make(chan struct{})
	go func() { defer close(emailWorkerDone); emailWorker.Run(serverCtx, cfg.ExpiryInterval) }()
	go func() {
		logger.Info("http server listening", "addr", cfg.HTTPAddr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("http server stopped unexpectedly", "error", err)
			stop()
		}
	}()

	<-serverCtx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown failed", "error", err)
		os.Exit(1)
	}
	select {
	case <-emailWorkerDone:
	case <-shutdownCtx.Done():
		logger.Warn("email worker did not stop before shutdown deadline")
	}
	logger.Info("http server stopped")
}

func healthcheck() {
	address := os.Getenv("HTTP_ADDR")
	if address == "" {
		address = ":8080"
	}
	_, port, err := net.SplitHostPort(address)
	if err != nil || port == "" {
		fmt.Fprintln(os.Stderr, "invalid HTTP_ADDR")
		os.Exit(1)
	}
	client := http.Client{Timeout: 3 * time.Second}
	response, err := client.Get("http://" + net.JoinHostPort("127.0.0.1", port) + "/api/v1/ready")
	if err != nil {
		fmt.Fprintln(os.Stderr, "API readiness check failed")
		os.Exit(1)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		fmt.Fprintln(os.Stderr, "API is not ready")
		os.Exit(1)
	}
}
