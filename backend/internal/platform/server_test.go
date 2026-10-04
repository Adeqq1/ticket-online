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
	handler := NewHandler(testDB(t), slog.New(slog.NewTextHandler(io.Discard, nil)))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/health", nil))
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"status":"ok"`) {
		t.Fatalf("unexpected response: %d %s", recorder.Code, recorder.Body.String())
	}
}

func TestReadyWhenDatabaseUnavailable(t *testing.T) {
	handler := NewHandler(testDB(t), slog.New(slog.NewTextHandler(io.Discard, nil)))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/ready", nil))
	if recorder.Code != http.StatusServiceUnavailable || !strings.Contains(recorder.Body.String(), "SERVICE_UNAVAILABLE") {
		t.Fatalf("unexpected response: %d %s", recorder.Code, recorder.Body.String())
	}
}

func TestStaticHandlerKeepsUnknownAPIRoutesJSON(t *testing.T) {
	handler := NewHandlerWithConfig(testDB(t), slog.New(slog.NewTextHandler(io.Discard, nil)), 10, t.TempDir(), false)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/unknown", nil))
	if recorder.Code != http.StatusNotFound || !strings.Contains(recorder.Body.String(), "NOT_FOUND") || strings.Contains(recorder.Header().Get("Content-Type"), "text/html") {
		t.Fatalf("unexpected API response: %d %s", recorder.Code, recorder.Body.String())
	}
}

func TestSimulatedPaymentRouteOnlyExistsInDevelopment(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	path := "/api/v1/orders/0123456789abcdef0123456789abcdef/simulate-payment"
	production := NewHandlerWithConfig(testDB(t), logger, 10, "", false)
	productionResponse := httptest.NewRecorder()
	production.ServeHTTP(productionResponse, httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"method":"INVALID","result":"FAILED"}`)))
	if productionResponse.Code != http.StatusNotFound {
		t.Fatalf("production route status = %d, want 404", productionResponse.Code)
	}
	development := NewHandlerWithConfig(testDB(t), logger, 10, "", true)
	developmentResponse := httptest.NewRecorder()
	development.ServeHTTP(developmentResponse, httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"method":"INVALID","result":"FAILED"}`)))
	if developmentResponse.Code != http.StatusUnprocessableEntity {
		t.Fatalf("development route status = %d, want validation response 422", developmentResponse.Code)
	}
}

func TestStaticHandlerFallsBackToIndexForBrowserRoutes(t *testing.T) {
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "index.html"), []byte("spa"), 0o644); err != nil {
		t.Fatal(err)
	}
	handler := NewHandlerWithConfig(testDB(t), slog.New(slog.NewTextHandler(io.Discard, nil)), 10, directory, false)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/checkout/nusa-malam", nil))
	if recorder.Code != http.StatusOK || recorder.Body.String() != "spa" {
		t.Fatalf("unexpected SPA response: %d %q", recorder.Code, recorder.Body.String())
	}
}
