package platform

import (
	"encoding/base64"
	"testing"
)

func TestLoadConfigAppEnv(t *testing.T) {
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
