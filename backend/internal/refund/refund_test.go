package refund

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestRefundRequestUsesStableKeyAndServerAuth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/v2/order-1/refund" {
			t.Errorf("request = %s %s", r.Method, r.URL.Path)
		}
		user, _, ok := r.BasicAuth()
		if !ok || user != "server-key" {
			t.Errorf("Basic Auth user = %q", user)
		}
		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		if body["refund_key"] != "refund-stable-key" {
			t.Errorf("refund key = %q", body["refund_key"])
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	s := New(nil, nil, "server-key", "sandbox", nil, nil)
	s.base = server.URL
	if err := s.send(context.Background(), "order-1", "refund-stable-key"); err != nil {
		t.Fatal(err)
	}
}

func TestRefundRequestHonorsTimeoutAndDoesNotFollowRedirects(t *testing.T) {
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(80 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer slow.Close()
	s := New(nil, nil, "key", "sandbox", nil, nil)
	s.base = slow.URL
	s.client = &http.Client{Timeout: 5 * time.Millisecond}
	if err := s.send(context.Background(), "order", "key"); err == nil {
		t.Fatal("provider timeout was treated as success")
	}

	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "https://example.invalid/refund", http.StatusFound)
	}))
	defer redirect.Close()
	s.base = redirect.URL
	s.client = &http.Client{Timeout: time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	if err := s.send(context.Background(), "order", "key"); err == nil || !strings.Contains(err.Error(), "302") {
		t.Fatalf("redirect result = %v", err)
	}
}

func TestRefundWindowIsConservative(t *testing.T) {
	if refundWindow("QRIS") != 7*24*time.Hour || refundWindow("GOPAY") != 45*24*time.Hour || refundWindow("VIRTUAL_ACCOUNT") != 0 {
		t.Fatal("refund method window changed")
	}
}

func TestRefundWindowAllowsRefundBeforeEventStarts(t *testing.T) {
	now := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)
	paidAt := now.Add(-time.Hour)
	eventStarts := now.Add(24 * time.Hour)
	if !now.Before(eventStarts) || !withinRefundWindow("QRIS", paidAt, now) {
		t.Fatal("eligible paid order was rejected before event start")
	}
	if withinRefundWindow("QRIS", now.Add(2*time.Minute), now) || withinRefundWindow("QRIS", now.Add(-8*24*time.Hour), now) {
		t.Fatal("refund outside safe time window was accepted")
	}
}
