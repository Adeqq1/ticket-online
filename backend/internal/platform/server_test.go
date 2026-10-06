package platform

import (
	"database/sql"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testHandler(db *sql.DB, logger *slog.Logger, staticDir string, development bool) http.Handler {
	return NewHandlerWithOrderAccess(db, logger, 10, staticDir, development, make([]byte, 32))
}

func testDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("mysql", "invalid:invalid@tcp(127.0.0.1:1)/missing")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestHealth(t *testing.T) {
	handler := testHandler(testDB(t), slog.New(slog.NewTextHandler(io.Discard, nil)), "", false)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/health", nil))
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"status":"ok"`) {
		t.Fatalf("unexpected response: %d %s", recorder.Code, recorder.Body.String())
	}
}

func TestReadyWhenDatabaseUnavailable(t *testing.T) {
	handler := testHandler(testDB(t), slog.New(slog.NewTextHandler(io.Discard, nil)), "", false)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/ready", nil))
	if recorder.Code != http.StatusServiceUnavailable || !strings.Contains(recorder.Body.String(), "SERVICE_UNAVAILABLE") {
		t.Fatalf("unexpected response: %d %s", recorder.Code, recorder.Body.String())
	}
}

func TestStaticHandlerKeepsUnknownAPIRoutesJSON(t *testing.T) {
	handler := testHandler(testDB(t), slog.New(slog.NewTextHandler(io.Discard, nil)), t.TempDir(), false)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/unknown", nil))
	if recorder.Code != http.StatusNotFound || !strings.Contains(recorder.Body.String(), "NOT_FOUND") || strings.Contains(recorder.Header().Get("Content-Type"), "text/html") {
		t.Fatalf("unexpected API response: %d %s", recorder.Code, recorder.Body.String())
	}
}

func TestSimulatedPaymentRouteOnlyExistsInDevelopment(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	path := "/api/v1/orders/0123456789abcdef0123456789abcdef/simulate-payment"
	production := testHandler(testDB(t), logger, "", false)
	productionResponse := httptest.NewRecorder()
	production.ServeHTTP(productionResponse, httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"method":"INVALID","result":"FAILED"}`)))
	if productionResponse.Code != http.StatusNotFound {
		t.Fatalf("production route status = %d, want 404", productionResponse.Code)
	}
	development := testHandler(testDB(t), logger, "", true)
	developmentResponse := httptest.NewRecorder()
	development.ServeHTTP(developmentResponse, httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"method":"INVALID","result":"FAILED"}`)))
	if developmentResponse.Code != http.StatusUnauthorized {
		t.Fatalf("development route status = %d, want access token response 401", developmentResponse.Code)
	}
}

func TestTicketReadRoutesAreRegisteredInProduction(t *testing.T) {
	handler := testHandler(testDB(t), slog.New(slog.NewTextHandler(io.Discard, nil)), "", false)
	for _, path := range []string{
		"/api/v1/orders/invalid/tickets",
		"/api/v1/tickets/invalid",
	} {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("%s returned %d, want 400 (registered ticket route)", path, recorder.Code)
		}
	}
}

func TestPrivateOrderRoutesRequireToken(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	id := "0123456789abcdef0123456789abcdef"
	handler := testHandler(testDB(t), logger, "", false)
	for _, path := range []string{
		"/api/v1/orders/" + id,
		"/api/v1/orders/" + id + "/tickets",
		"/api/v1/tickets/" + id,
	} {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
		if recorder.Code != http.StatusUnauthorized || recorder.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("%s returned %d with Cache-Control %q, want 401 and no-store", path, recorder.Code, recorder.Header().Get("Cache-Control"))
		}
	}
}

func TestCheckInRouteIsAvailableInProductionAndRequiresStaffSession(t *testing.T) {
	handler := testHandler(testDB(t), slog.New(slog.NewTextHandler(io.Discard, nil)), "", false)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/v1/staff/check-ins", strings.NewReader(`{}`)))
	if response.Code != http.StatusUnauthorized || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("check-in route returned %d with Cache-Control %q: %s", response.Code, response.Header().Get("Cache-Control"), response.Body.String())
	}
}

func TestAdminEventRoutesRequireAdministratorSession(t *testing.T) {
	handler := testHandler(testDB(t), slog.New(slog.NewTextHandler(io.Discard, nil)), "", false)
	for _, route := range []struct{ method, path string }{
		{http.MethodGet, "/api/v1/admin/events"},
		{http.MethodPost, "/api/v1/admin/events"},
		{http.MethodPut, "/api/v1/admin/events/nusa-malam"},
	} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(route.method, route.path, strings.NewReader(`{}`)))
		if response.Code != http.StatusUnauthorized || response.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("%s %s returned %d with Cache-Control %q, want private 401", route.method, route.path, response.Code, response.Header().Get("Cache-Control"))
		}
	}
}

func TestStaticHandlerFallsBackToIndexForBrowserRoutes(t *testing.T) {
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "index.html"), []byte("spa"), 0o644); err != nil {
		t.Fatal(err)
	}
	handler := testHandler(testDB(t), slog.New(slog.NewTextHandler(io.Discard, nil)), directory, false)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/checkout/nusa-malam", nil))
	if recorder.Code != http.StatusOK || recorder.Body.String() != "spa" {
		t.Fatalf("unexpected SPA response: %d %q", recorder.Code, recorder.Body.String())
	}
}
