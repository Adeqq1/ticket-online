package staffauth

import (
	"errors"
	"testing"
	"time"
)

func TestStaffPasswordHash(t *testing.T) {
	hash, err := hashPassword("correct horse battery")
	if err != nil {
		t.Fatal(err)
	}
	if !verifyPassword("correct horse battery", hash) || verifyPassword("wrong password here", hash) || verifyPassword("correct horse battery", "invalid") {
		t.Fatal("password hash did not verify correctly")
	}
}

func TestStaffInputValidation(t *testing.T) {
	name, email, err := ValidateIdentity("  Gate Lead ", " STAFF@Example.com ")
	if err != nil || name != "Gate Lead" || email != "staff@example.com" {
		t.Fatalf("normalized identity = (%q, %q, %v)", name, email, err)
	}
	for _, password := range []string{"short", string(make([]byte, 129))} {
		if !errors.Is(ValidatePassword(password), ErrInvalidInput) {
			t.Fatalf("password length %d should fail", len(password))
		}
	}
	if err := ValidatePassword("a sufficiently long password"); err != nil {
		t.Fatalf("valid password rejected: %v", err)
	}
	if _, _, err := ValidateIdentity("A", "bad-email"); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("invalid email error = %v", err)
	}
}

func TestLoginLimiterByAccountAndSource(t *testing.T) {
	limiter := newLoginLimiter()
	now := time.Date(2026, 10, 5, 8, 0, 0, 0, time.UTC)
	for range 5 {
		if !limiter.Allow("192.0.2.4:1000", "staff@example.com", now) {
			t.Fatal("account unexpectedly limited early")
		}
		limiter.Failure("192.0.2.4:1000", "staff@example.com", now, ErrCredentials)
	}
	if limiter.Allow("192.0.2.4:2000", "staff@example.com", now) {
		t.Fatal("same account from another source port was not limited")
	}
	if err := limiter.Failure("192.0.2.4:1000", "staff@example.com", now, ErrCredentials); !errors.Is(err, ErrRateLimited) {
		t.Fatalf("fifth failure = %v", err)
	}
	if !limiter.Allow("192.0.2.4:1000", "staff@example.com", now.Add(16*time.Minute)) {
		t.Fatal("account was still limited after the failure window")
	}

	ipLimiter := newLoginLimiter()
	for i := range 30 {
		if !ipLimiter.Allow("192.0.2.4:1000", string(rune('a'+i)), now) {
			t.Fatalf("source limited after %d attempts", i)
		}
	}
	if ipLimiter.Allow("192.0.2.4:2000", "another@example.com", now) {
		t.Fatal("source was not limited at 30 attempts")
	}
}

func TestSessionTokenIsUnpredictableAndScopedByHash(t *testing.T) {
	first, err := newSessionToken()
	if err != nil {
		t.Fatal(err)
	}
	second, err := newSessionToken()
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 43 || first == second || string(tokenHash(first)) == first {
		t.Fatal("session token was not generated or hashed as expected")
	}
}
