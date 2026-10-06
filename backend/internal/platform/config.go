package platform

import (
	"fmt"
	"net"
	"net/mail"
	"net/netip"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/Adeqq1/ticket-online/backend/internal/orderaccess"
)

type Config struct {
	AppEnv              string
	HTTPAddr            string
	MySQLDSN            string
	StaticDir           string
	ReservationTTL      time.Duration
	ExpiryInterval      time.Duration
	OrderAccessSecret   []byte
	MidtransServerKey   string
	MidtransEnvironment string
	TransactionsEnabled bool
	FrontendURL         string
	SMTPHost            string
	SMTPPort            int
	SMTPUsername        string
	SMTPPassword        string
	SMTPFrom            string
	SMTPTLSMode         string
	TrustedProxyCIDRs   []netip.Prefix
}

func LoadConfig() (Config, error) {
	cfg := Config{
		AppEnv:              envOr("APP_ENV", "production"),
		HTTPAddr:            envOr("HTTP_ADDR", ":8080"),
		MySQLDSN:            os.Getenv("MYSQL_DSN"),
		StaticDir:           os.Getenv("STATIC_DIR"),
		MidtransServerKey:   strings.TrimSpace(os.Getenv("MIDTRANS_SERVER_KEY")),
		MidtransEnvironment: envOr("MIDTRANS_ENV", "sandbox"),
		FrontendURL:         strings.TrimRight(envOr("FRONTEND_URL", "http://localhost:5173"), "/"),
		SMTPHost:            strings.TrimSpace(os.Getenv("SMTP_HOST")),
		SMTPPort:            587,
		SMTPUsername:        os.Getenv("SMTP_USERNAME"),
		SMTPPassword:        os.Getenv("SMTP_PASSWORD"),
		SMTPFrom:            strings.TrimSpace(os.Getenv("SMTP_FROM")),
		SMTPTLSMode:         envOr("SMTP_TLS_MODE", "starttls"),
		ReservationTTL:      10 * time.Minute,
		ExpiryInterval:      15 * time.Second,
	}
	var err error
	if cfg.AppEnv != "development" && cfg.AppEnv != "production" {
		return Config{}, fmt.Errorf("invalid APP_ENV")
	}
	if cfg.MidtransEnvironment != "sandbox" && cfg.MidtransEnvironment != "production" {
		return Config{}, fmt.Errorf("MIDTRANS_ENV must be sandbox or production")
	}
	if cfg.MidtransEnvironment == "production" && cfg.AppEnv != "production" {
		return Config{}, fmt.Errorf("MIDTRANS_ENV=production requires APP_ENV=production")
	}
	transactionsEnabled := os.Getenv("TRANSACTIONS_ENABLED")
	if transactionsEnabled == "" {
		cfg.TransactionsEnabled = cfg.MidtransEnvironment == "sandbox"
	} else if transactionsEnabled == "true" {
		cfg.TransactionsEnabled = true
	} else if transactionsEnabled == "false" {
		cfg.TransactionsEnabled = false
	} else {
		return Config{}, fmt.Errorf("TRANSACTIONS_ENABLED must be true or false")
	}
	if cfg.MidtransEnvironment == "production" {
		if cfg.MidtransServerKey == "" || !strings.HasPrefix(cfg.MidtransServerKey, "Mid-server-") {
			return Config{}, fmt.Errorf("production MIDTRANS_SERVER_KEY is required and must be a production key")
		}
		frontend, err := url.Parse(cfg.FrontendURL)
		if err != nil || frontend.Host == "" || frontend.Scheme != "https" || frontend.Path != "" || frontend.User != nil || frontend.RawQuery != "" || frontend.Fragment != "" {
			return Config{}, fmt.Errorf("production FRONTEND_URL must be an HTTPS origin")
		}
	} else if cfg.MidtransServerKey != "" && !strings.HasPrefix(cfg.MidtransServerKey, "SB-Mid-server-") {
		return Config{}, fmt.Errorf("sandbox MIDTRANS_SERVER_KEY must be a sandbox key")
	}
	if cfg.MySQLDSN == "" {
		return Config{}, fmt.Errorf("MYSQL_DSN is required")
	}
	if value := strings.TrimSpace(os.Getenv("TRUSTED_PROXY_CIDRS")); value != "" {
		for _, raw := range strings.Split(value, ",") {
			prefix, err := netip.ParsePrefix(strings.TrimSpace(raw))
			if err != nil {
				return Config{}, fmt.Errorf("invalid TRUSTED_PROXY_CIDRS: %w", err)
			}
			cfg.TrustedProxyCIDRs = append(cfg.TrustedProxyCIDRs, prefix.Masked())
		}
	}
	if value := os.Getenv("SMTP_PORT"); value != "" {
		var port int
		if _, err := fmt.Sscan(value, &port); err != nil || port < 1 || port > 65535 || fmt.Sprint(port) != value {
			return Config{}, fmt.Errorf("invalid SMTP_PORT")
		}
		cfg.SMTPPort = port
	}
	if (cfg.SMTPUsername == "") != (cfg.SMTPPassword == "") {
		return Config{}, fmt.Errorf("SMTP_USERNAME and SMTP_PASSWORD must be set together")
	}
	if cfg.SMTPTLSMode != "starttls" && cfg.SMTPTLSMode != "tls" && cfg.SMTPTLSMode != "none" {
		return Config{}, fmt.Errorf("invalid SMTP_TLS_MODE")
	}
	if cfg.SMTPTLSMode == "none" && cfg.AppEnv != "development" {
		return Config{}, fmt.Errorf("SMTP_TLS_MODE=none is allowed only in development")
	}
	if cfg.SMTPTLSMode == "none" && cfg.SMTPUsername != "" {
		return Config{}, fmt.Errorf("SMTP credentials require TLS")
	}
	if cfg.SMTPHost != "" {
		if strings.ContainsAny(cfg.SMTPHost, " \t\r\n/\\") || (strings.Contains(cfg.SMTPHost, ":") && net.ParseIP(strings.Trim(cfg.SMTPHost, "[]")) == nil) {
			return Config{}, fmt.Errorf("invalid SMTP_HOST")
		}
		if address := net.ParseIP(strings.Trim(cfg.SMTPHost, "[]")); address != nil {
			cfg.SMTPHost = address.String()
		}
		if cfg.SMTPFrom == "" {
			return Config{}, fmt.Errorf("SMTP_FROM is required when SMTP_HOST is set")
		}
		if _, err := mail.ParseAddress(cfg.SMTPFrom); err != nil {
			return Config{}, fmt.Errorf("invalid SMTP_FROM")
		}
		frontend, err := url.Parse(cfg.FrontendURL)
		if err != nil || frontend.Host == "" || frontend.Path != "" || (frontend.Scheme != "http" && frontend.Scheme != "https") || frontend.User != nil || frontend.RawQuery != "" || frontend.Fragment != "" || (cfg.AppEnv == "production" && frontend.Scheme != "https") {
			return Config{}, fmt.Errorf("FRONTEND_URL must be an origin and use HTTPS in production when email is enabled")
		}
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
