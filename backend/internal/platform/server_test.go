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
	handler := NewHandlerWithConfig(testDB(t), slog.New(slog.NewTextHandler(io.Discard, nil)), 10, t.TempDir())
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/unknown", nil))
	if recorder.Code != http.StatusNotFound || !strings.Contains(recorder.Body.String(), "NOT_FOUND") || strings.Contains(recorder.Header().Get("Content-Type"), "text/html") {
		t.Fatalf("unexpected API response: %d %s", recorder.Code, recorder.Body.String())
	}
}

func TestStaticHandlerFallsBackToIndexForBrowserRoutes(t *testing.T) {
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "index.html"), []byte("spa"), 0o644); err != nil {
		t.Fatal(err)
	}
	handler := NewHandlerWithConfig(testDB(t), slog.New(slog.NewTextHandler(io.Discard, nil)), 10, directory)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/checkout/nusa-malam", nil))
	if recorder.Code != http.StatusOK || recorder.Body.String() != "spa" {
		t.Fatalf("unexpected SPA response: %d %q", recorder.Code, recorder.Body.String())
	}
}
