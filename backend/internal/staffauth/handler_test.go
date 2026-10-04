package staffauth

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestProtectedStaffRoutesRequireNoStoreAuthentication(t *testing.T) {
	mux := http.NewServeMux()
	NewHandler(New(nil), slog.New(slog.NewTextHandler(io.Discard, nil))).Register(mux)
	for _, path := range []string{"/api/v1/staff/me", "/api/v1/admin/staff", "/api/v1/staff/logout"} {
		method := http.MethodGet
		if path == "/api/v1/staff/logout" {
			method = http.MethodPost
		}
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, httptest.NewRequest(method, path, nil))
		if response.Code != http.StatusUnauthorized || response.Header().Get("Cache-Control") != "no-store" {
			t.Errorf("%s returned %d and cache %q; want 401 and no-store", path, response.Code, response.Header().Get("Cache-Control"))
		}
	}
}
