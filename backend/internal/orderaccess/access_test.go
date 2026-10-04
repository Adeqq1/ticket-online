package orderaccess

import (
	"encoding/base64"
	"testing"
	"time"
)

func TestTokenAndWIBExpiry(t *testing.T) {
	secret := make([]byte, 32)
	first := New(nil, secret)
	second := New(nil, append([]byte(nil), secret...))
	if first.Token("order-one") != second.Token("order-one") || first.Token("order-one") == first.Token("order-two") {
		t.Fatal("order token must be stable and scoped to one order")
	}
	startsAt := time.Date(2026, time.December, 31, 17, 30, 0, 0, time.UTC) // 00.30 WIB on Jan 1
	want := time.Date(2027, time.January, 1, 17, 0, 0, 0, time.UTC)
	if got := Expiry(startsAt); !got.Equal(want) {
		t.Fatalf("expiry = %s, want %s", got, want)
	}
	misconfiguredLocation := time.Date(2026, time.December, 31, 17, 30, 0, 0, time.FixedZone("dsn-location", 10*60*60))
	if got := Expiry(misconfiguredLocation); !got.Equal(want) {
		t.Fatalf("expiry with non-UTC DSN location = %s, want %s", got, want)
	}
	if _, err := first.authorize("order-one", time.Now().AddDate(0, 0, -2), first.Token("order-one")); err != ErrExpired {
		t.Fatalf("expired order access error = %v, want ErrExpired", err)
	}
	if _, err := first.authorize("order-one", time.Now().Add(time.Hour), first.Token("order-two")); err != ErrUnauthorized {
		t.Fatalf("wrong order access error = %v, want ErrUnauthorized", err)
	}
}

func TestParseSecretRequires32Bytes(t *testing.T) {
	if _, err := ParseSecret(base64.StdEncoding.EncodeToString(make([]byte, 31))); err == nil {
		t.Fatal("expected short secret to fail")
	}
	if _, err := ParseSecret(base64.StdEncoding.EncodeToString(make([]byte, 32))); err != nil {
		t.Fatalf("valid secret rejected: %v", err)
	}
}
