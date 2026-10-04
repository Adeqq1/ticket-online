package staffauth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
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

func TestInvalidStaffInputRejectedBeforeDatabaseAccess(t *testing.T) {
	service := New(nil)
	if err := service.ReplaceAssignments(context.Background(), "staff-id", nil); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("nil assignments error = %v", err)
	}
	for _, name := range []string{"A", "李", strings.Repeat("李", 81)} {
		if _, _, err := ValidateIdentity(name, "staff@example.com"); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("invalid name %q error = %v", name, err)
		}
		if err := service.UpdateStaff(context.Background(), "staff-id", &name, nil); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("invalid update name error = %v", err)
		}
	}
	for _, name := range []string{" 李明 ", strings.Repeat("李", 80)} {
		if _, _, err := ValidateIdentity(name, "staff@example.com"); err != nil {
			t.Fatalf("valid Unicode name rejected: %v", err)
		}
	}
}

func TestLoginLimiterCapacityOnConcurrentFailures(t *testing.T) {
	limiter := newLoginLimiter()
	now := time.Now()
	for i := range 9999 {
		limiter.windows[fmt.Sprintf("email:existing-%d@example.com", i)] = limitWindow{until: now.Add(time.Minute)}
	}
	if !limiter.Allow("192.0.2.4:1000", "new@example.com", now) {
		t.Fatal("existing source should fill the final slot")
	}
	var wg sync.WaitGroup
	for i := range 20 {
		wg.Go(func() {
			if err := limiter.Failure("192.0.2.4:1000", fmt.Sprintf("new-%d@example.com", i), now, ErrCredentials); !errors.Is(err, ErrRateLimited) {
				t.Errorf("new key at capacity error = %v", err)
			}
		})
	}
	wg.Wait()
	if len(limiter.windows) != 10000 {
		t.Fatalf("limiter has %d keys; want 10000", len(limiter.windows))
	}
	if err := limiter.Failure("192.0.2.4:1000", "existing-0@example.com", now, ErrCredentials); !errors.Is(err, ErrCredentials) || limiter.windows["email:existing-0@example.com"].count != 1 {
		t.Fatalf("existing key was not updated: %v", err)
	}
	if !limiter.Allow("192.0.2.4:2000", "new@example.com", now) || limiter.Allow("192.0.2.5:1000", "new@example.com", now) {
		t.Fatal("Allow should admit an existing source and reject a new source at capacity")
	}
	if err := limiter.Failure("192.0.2.4:1000", "new@example.com", now.Add(2*time.Minute), ErrCredentials); !errors.Is(err, ErrCredentials) || len(limiter.windows) != 1 {
		t.Fatalf("expired slots were not reclaimed: len=%d error=%v", len(limiter.windows), err)
	}
}
