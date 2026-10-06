package platform

import (
	"encoding/base64"
	"fmt"
	"net/netip"
	"strings"
	"testing"
)

func clearSMTPEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{"SMTP_HOST", "SMTP_PORT", "SMTP_USERNAME", "SMTP_PASSWORD", "SMTP_FROM", "SMTP_TLS_MODE"} {
		t.Setenv(key, "")
	}
}

func TestLoadConfigAppEnv(t *testing.T) {
	clearSMTPEnv(t)
	t.Setenv("MYSQL_DSN", "ticket:ticket@tcp(localhost:3306)/ticket_online?parseTime=true")
	t.Setenv("ORDER_ACCESS_SECRET", base64.StdEncoding.EncodeToString(make([]byte, 32)))
	t.Setenv("APP_ENV", "")
	cfg, err := LoadConfig()
	if err != nil || cfg.AppEnv != "production" {
		t.Fatalf("default APP_ENV = %q, %v; want production", cfg.AppEnv, err)
	}
	t.Setenv("APP_ENV", "development")
	cfg, err = LoadConfig()
	if err != nil || cfg.AppEnv != "development" {
		t.Fatalf("development APP_ENV = %q, %v", cfg.AppEnv, err)
	}
	t.Setenv("APP_ENV", "staging")
	if _, err := LoadConfig(); err == nil {
		t.Fatal("expected invalid APP_ENV to fail")
	}
}

func TestLoadConfigRejectsMissingOrInvalidOrderSecret(t *testing.T) {
	clearSMTPEnv(t)
	t.Setenv("MYSQL_DSN", "ticket:ticket@tcp(localhost:3306)/ticket_online?parseTime=true")
	t.Setenv("APP_ENV", "production")
	for _, secret := range []string{"", "invalid", base64.StdEncoding.EncodeToString(make([]byte, 31))} {
		t.Setenv("ORDER_ACCESS_SECRET", secret)
		if _, err := LoadConfig(); err == nil {
			t.Fatal("missing or invalid order secret was accepted")
		}
	}
}

func TestLoadConfigSMTP(t *testing.T) {
	t.Setenv("MYSQL_DSN", "ticket:ticket@tcp(localhost:3306)/ticket_online?parseTime=true")
	t.Setenv("ORDER_ACCESS_SECRET", base64.StdEncoding.EncodeToString(make([]byte, 32)))
	t.Setenv("APP_ENV", "development")
	clearSMTPEnv(t)
	cfg, err := LoadConfig()
	if err != nil || cfg.SMTPPort != 587 || cfg.SMTPHost != "" {
		t.Fatalf("default SMTP config = %+v, %v", cfg, err)
	}
	t.Setenv("SMTP_HOST", "smtp.example.com")
	t.Setenv("SMTP_FROM", "tickets@example.com")
	t.Setenv("SMTP_USERNAME", "mailer")
	t.Setenv("SMTP_PASSWORD", "secret")
	cfg, err = LoadConfig()
	if err != nil || cfg.SMTPPort != 587 || cfg.SMTPTLSMode != "starttls" {
		t.Fatalf("configured SMTP = %+v, %v", cfg, err)
	}
	for _, invalid := range []struct{ key, value string }{
		{"SMTP_PORT", "70000"}, {"SMTP_FROM", "bad"}, {"SMTP_TLS_MODE", "plain"}, {"SMTP_HOST", "smtp.example.com:2525"},
		{"SMTP_PASSWORD", ""},
	} {
		t.Run(fmt.Sprintf("reject_%s", invalid.key), func(t *testing.T) {
			t.Setenv(invalid.key, invalid.value)
			if _, err := LoadConfig(); err == nil {
				t.Fatalf("invalid %s was accepted", invalid.key)
			}
		})
	}
	for _, tc := range []struct {
		env, url string
		valid    bool
	}{
		{"production", "https://tickets.example.com", true},
		{"production", "https://tickets.example.com/", true},
		{"production", "http://tickets.example.com", false},
		{"development", "http://localhost:5173", true},
		{"development", "https://tickets.example.com", true},
	} {
		t.Run(tc.env+tc.url, func(t *testing.T) {
			t.Setenv("APP_ENV", tc.env)
			t.Setenv("FRONTEND_URL", tc.url)
			cfg, err := LoadConfig()
			if (err == nil) != tc.valid {
				t.Fatalf("URL %q: error = %v", tc.url, err)
			}
			if tc.valid && cfg.FrontendURL != strings.TrimRight(tc.url, "/") {
				t.Fatalf("normalized URL = %q", cfg.FrontendURL)
			}
		})
	}
	for _, env := range []string{"production", "development"} {
		for _, url := range []string{"https://tickets.example.com/app", "ftp://tickets.example.com", "https://user@tickets.example.com", "https://tickets.example.com?x=1", "https://tickets.example.com#fragment", "https:///"} {
			t.Run(env+url, func(t *testing.T) {
				t.Setenv("APP_ENV", env)
				t.Setenv("FRONTEND_URL", url)
				if _, err := LoadConfig(); err == nil {
					t.Fatalf("invalid URL %q accepted", url)
				}
			})
		}
	}
	t.Setenv("APP_ENV", "production")
	t.Setenv("SMTP_TLS_MODE", "none")
	if _, err := LoadConfig(); err == nil {
		t.Fatal("production SMTP without TLS was accepted")
	}
}

func TestLoadConfigTrustedProxies(t *testing.T) {
	clearSMTPEnv(t)
	t.Setenv("MYSQL_DSN", "test")
	t.Setenv("ORDER_ACCESS_SECRET", base64.StdEncoding.EncodeToString(make([]byte, 32)))
	t.Setenv("APP_ENV", "development")
	for _, value := range []string{"", "127.0.0.1/32, ::1/128", "invalid", "127.0.0.1", "10.0.0.1/33", "127.0.0.1/32,"} {
		t.Run(value, func(t *testing.T) {
			t.Setenv("TRUSTED_PROXY_CIDRS", value)
			cfg, err := LoadConfig()
			valid := value == "" || value == "127.0.0.1/32, ::1/128"
			if (err == nil) != valid {
				t.Fatalf("CIDRs %q: error = %v", value, err)
			}
			if valid && value != "" && (len(cfg.TrustedProxyCIDRs) != 2 || cfg.TrustedProxyCIDRs[1] != netip.MustParsePrefix("::1/128")) {
				t.Fatalf("parsed proxies = %v", cfg.TrustedProxyCIDRs)
			}
			if value == "" && len(cfg.TrustedProxyCIDRs) != 0 {
				t.Fatal("default must not trust proxies")
			}
		})
	}
}
