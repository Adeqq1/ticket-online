package payment

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestValidateNormalizesPaymentRequest(t *testing.T) {
	got, err := Validate(Request{Method: "  virtual_account ", Result: " succeeded "})
	if err != nil || got != (Request{Method: "VIRTUAL_ACCOUNT", Result: "SUCCEEDED"}) {
		t.Fatalf("Validate() = %+v, %v", got, err)
	}
}

func TestValidateRejectsUnsupportedPaymentValues(t *testing.T) {
	for _, request := range []Request{{Method: "CASH", Result: "SUCCEEDED"}, {Method: "QRIS", Result: "PENDING"}} {
		if _, err := Validate(request); err != ErrInvalidRequest {
			t.Fatalf("Validate(%+v) error = %v, want ErrInvalidRequest", request, err)
		}
	}
}

func TestMidtransEnvironmentEndpointsAndRedirects(t *testing.T) {
	for _, tc := range []struct {
		environment, snap, api, host string
	}{
		{"sandbox", "https://app.sandbox.midtrans.com/snap/v1/transactions", "https://api.sandbox.midtrans.com", "app.sandbox.midtrans.com"},
		{"production", "https://app.midtrans.com/snap/v1/transactions", "https://api.midtrans.com", "app.midtrans.com"},
	} {
		repository := NewRepositoryWithMidtransEnvironment(nil, tc.environment)
		if got := midtransSnapURL(tc.environment); got != tc.snap {
			t.Errorf("%s Snap endpoint = %s; want %s", tc.environment, got, tc.snap)
		}
		if repository.midtransBaseURL != tc.api {
			t.Errorf("%s API endpoint = %s; want %s", tc.environment, repository.midtransBaseURL, tc.api)
		}
		handler := NewHandlerWithMidtrans(repository, nil, nil, "key", "https://tickets.example.com", tc.environment)
		if !handler.validProviderRedirect("https://" + tc.host + "/snap/v1/abc") {
			t.Errorf("%s redirect rejected", tc.environment)
		}
		otherHost := "app.midtrans.com"
		if tc.environment == "production" {
			otherHost = "app.sandbox.midtrans.com"
		}
		if handler.validProviderRedirect("https://"+otherHost+"/snap/v1/abc") || handler.validProviderRedirect("https://"+tc.host+".evil.example/snap/v1/abc") {
			t.Errorf("%s accepted a redirect outside its environment", tc.environment)
		}
	}
}

func TestMidtransStatusUsesConfiguredEndpointAndHonorsTimeout(t *testing.T) {
	t.Run("status and cancel", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if user, _, ok := r.BasicAuth(); !ok || user != "server-key" {
				t.Errorf("unexpected provider authorization")
			}
			switch r.URL.Path {
			case "/v2/order-1/status":
				_, _ = fmt.Fprint(w, `{"order_id":"order-1","gross_amount":"100.00","transaction_status":"pending"}`)
			case "/v2/order-1/cancel":
				w.WriteHeader(http.StatusNoContent)
			default:
				http.NotFound(w, r)
			}
		}))
		defer server.Close()
		repository := NewRepositoryWithMidtransEnvironment(nil, "production")
		repository.midtransBaseURL = server.URL
		status, err := repository.ReadGatewayStatus(context.Background(), "server-key", "order-1")
		if err != nil || status.TransactionStatus != "pending" || status.GrossAmount != "100.00" {
			t.Fatalf("ReadGatewayStatus() = %+v, %v", status, err)
		}
		if err := repository.cancelMidtrans(context.Background(), "server-key", "order-1"); err != nil {
			t.Fatalf("cancelMidtrans() error = %v", err)
		}
	})
	t.Run("context timeout", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
		defer server.Close()
		repository := NewRepositoryWithMidtransEnvironment(nil, "sandbox")
		repository.midtransBaseURL = server.URL
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
		defer cancel()
		if _, err := repository.ReadGatewayStatus(ctx, "server-key", "order-1"); err == nil {
			t.Fatal("ReadGatewayStatus succeeded after its context timed out")
		}
	})
}
