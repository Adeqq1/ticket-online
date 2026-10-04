package platform

import (
	"fmt"
	"os"
	"time"

	"github.com/Adeqq1/ticket-online/backend/internal/orderaccess"
)

type Config struct {
	AppEnv            string
	HTTPAddr          string
	MySQLDSN          string
	StaticDir         string
	ReservationTTL    time.Duration
	ExpiryInterval    time.Duration
	OrderAccessSecret []byte
}

func LoadConfig() (Config, error) {
	cfg := Config{
		AppEnv:         envOr("APP_ENV", "production"),
		HTTPAddr:       envOr("HTTP_ADDR", ":8080"),
		MySQLDSN:       os.Getenv("MYSQL_DSN"),
		StaticDir:      os.Getenv("STATIC_DIR"),
		ReservationTTL: 10 * time.Minute,
		ExpiryInterval: 15 * time.Second,
	}
	var err error
	if cfg.AppEnv != "development" && cfg.AppEnv != "production" {
		return Config{}, fmt.Errorf("invalid APP_ENV")
	}
	if cfg.MySQLDSN == "" {
		return Config{}, fmt.Errorf("MYSQL_DSN is required")
	}
	cfg.OrderAccessSecret, err = orderaccess.ParseSecret(os.Getenv("ORDER_ACCESS_SECRET"))
	if err != nil {
		return Config{}, err
	}
	if value := os.Getenv("RESERVATION_TTL"); value != "" {
		cfg.ReservationTTL, err = time.ParseDuration(value)
		if err != nil || cfg.ReservationTTL <= 0 {
			return Config{}, fmt.Errorf("invalid RESERVATION_TTL")
		}
	}
	if value := os.Getenv("EXPIRY_INTERVAL"); value != "" {
		cfg.ExpiryInterval, err = time.ParseDuration(value)
		if err != nil || cfg.ExpiryInterval <= 0 {
			return Config{}, fmt.Errorf("invalid EXPIRY_INTERVAL")
		}
	}
	return cfg, nil
}

func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
