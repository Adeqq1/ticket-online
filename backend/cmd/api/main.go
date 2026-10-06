package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Adeqq1/ticket-online/backend/internal/email"
	"github.com/Adeqq1/ticket-online/backend/internal/payment"
	"github.com/Adeqq1/ticket-online/backend/internal/platform"
	"github.com/Adeqq1/ticket-online/backend/internal/reservation"
	"github.com/Adeqq1/ticket-online/backend/migrations"
)

func main() {
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

	serverCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	server := platform.NewHTTPServer(cfg.HTTPAddr, platform.NewHandlerWithPaymentConfig(db, logger, cfg.ReservationTTL, cfg.StaticDir, cfg.AppEnv == "development", cfg.OrderAccessSecret, cfg.MidtransServerKey, cfg.FrontendURL, cfg.TrustedProxyCIDRs...))
	worker := reservation.NewWorker(reservation.NewRepository(db, cfg.ReservationTTL), cfg.ExpiryInterval, logger)
	go worker.Run(serverCtx)
	go payment.NewRepository(db).RunExpiryWorker(serverCtx, cfg.ExpiryInterval, cfg.MidtransServerKey, logger)
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
