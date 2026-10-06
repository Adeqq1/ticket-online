package recovery

import (
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/Adeqq1/ticket-online/backend/internal/orderaccess"
)

func TestInputNormalization(t *testing.T) {
	if got := NormalizeReference(" to-ABCDEF0123456789abcd "); got != "TO-abcdef0123456789abcd" {
		t.Fatalf("reference = %q", got)
	}
	for _, bad := range []string{"", "TO-123", "XX-abcdef0123456789abcd", "TO-abcdef0123456789abcdz"} {
		if NormalizeReference(bad) != "" {
			t.Fatalf("reference %q was accepted", bad)
		}
	}
	if got := NormalizeEmail(" Buyer@Example.com "); got != "buyer@example.com" {
		t.Fatalf("email = %q", got)
	}
	for _, bad := range []string{"", "buyer", "Buyer <buyer@example.com>", "a@b.c\r\nBcc: x@y.z", strings.Repeat("a", 250) + "@b.co"} {
		if NormalizeEmail(bad) != "" {
			t.Fatalf("email %q was accepted", bad)
		}
	}
	access := orderaccess.New(nil, make([]byte, 32))
	if PairKey(access, "Buyer@Example.com", "TO-a") != PairKey(access, "buyer@example.com", "TO-a") || PairKey(access, "buyer@example.com", "TO-a") == PairKey(access, "buyer@example.com", "TO-b") {
		t.Fatal("pair key must ignore email case and separate references")
	}
}

func TestTokenIsStoredOnlyAsHash(t *testing.T) {
	raw, hash, err := NewToken()
	if err != nil {
		t.Fatal(err)
	}
	if !validToken(raw) || hash != HashToken(raw) || len(hash) != 64 || strings.Contains(hash, raw) {
		t.Fatalf("token %q hash %q", raw, hash)
	}
	other, _, _ := NewToken()
	if raw == other || validToken(raw[:42]) || validToken(raw+"A") || validToken(strings.Repeat("*", 43)) {
		t.Fatal("token validation accepted a malformed or repeated token")
	}
}

func TestEligible(t *testing.T) {
	now := time.Now()
	order := Order{Status: "PAID", Complete: true, StartsAt: now.Add(48 * time.Hour)}
	if !order.Eligible(now) {
		t.Fatal("paid complete order with live access was rejected")
	}
	for _, changed := range []Order{{Status: "PENDING", Complete: true, StartsAt: order.StartsAt}, {Status: "PAID", StartsAt: order.StartsAt}, {Status: "PAID", Complete: true, StartsAt: now.AddDate(0, 0, -2)}} {
		if changed.Eligible(now) {
			t.Fatalf("ineligible order accepted: %+v", changed)
		}
	}
}

func TestClientIP(t *testing.T) {
	trusted := []netip.Prefix{netip.MustParsePrefix("172.20.0.3/32"), netip.MustParsePrefix("2001:db8::3/128")}
	for _, tc := range []struct{ name, remote, forwarded, want string }{
		{"client A", "172.20.0.3:41000", "192.0.2.1", "192.0.2.1"},
		{"client B", "172.20.0.3:42000", "192.0.2.2", "192.0.2.2"},
		{"direct spoof", "192.0.2.1:41000", "192.0.2.2", "192.0.2.1"},
		{"trusted chain", "172.20.0.3:41000", "192.0.2.1, 2001:db8::3", "192.0.2.1"},
		{"untrusted intermediate", "172.20.0.3:41000", "192.0.2.1, 192.0.2.2", "192.0.2.2"},
		{"spoofed prefix", "172.20.0.3:41000", "attacker, 192.0.2.1", "192.0.2.1"},
		{"malformed", "172.20.0.3:41000", "192.0.2.1, attacker", "172.20.0.3"},
		{"empty", "172.20.0.3:41000", "", "172.20.0.3"},
		{"IPv6", "[2001:db8::3]:41000", "2001:db8::1", "2001:db8::1"},
		{"mapped IPv4", "[::ffff:172.20.0.3]:41000", "::ffff:192.0.2.1", "192.0.2.1"},
		{"invalid peer", "invalid", "192.0.2.1", "unknown"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest("POST", "/", nil)
			r.RemoteAddr = tc.remote
			r.Header.Set("X-Forwarded-For", tc.forwarded)
			if got := clientIP(r, trusted); got != tc.want {
				t.Fatalf("clientIP = %q, want %q", got, tc.want)
			}
		})
	}
	r := httptest.NewRequest("POST", "/", nil)
	r.RemoteAddr = "172.20.0.3:41000"
	r.Header.Add("X-Forwarded-For", "192.0.2.1")
	r.Header.Add("X-Forwarded-For", "192.0.2.2")
	if clientIP(r, nil) != "172.20.0.3" || clientIP(r, trusted) != "192.0.2.2" {
		t.Fatal("default trust or multiple forwarding headers handled incorrectly")
	}
}
