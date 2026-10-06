package adminorders

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/Adeqq1/ticket-online/backend/internal/staffauth"
	"github.com/Adeqq1/ticket-online/backend/migrations"
	_ "github.com/go-sql-driver/mysql"
)

func TestAdminOrderRoutesEnforceRoleAndSession(t *testing.T) {
	dsn := os.Getenv("MYSQL_TEST_DSN")
	if dsn == "" {
		t.Skip("set MYSQL_TEST_DSN to a disposable MySQL database")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := migrations.Run(ctx, db); err != nil {
		t.Fatal(err)
	}
	newID := func() string {
		t.Helper()
		var value [16]byte
		if _, err := rand.Read(value[:]); err != nil {
			t.Fatal(err)
		}
		return hex.EncodeToString(value[:])
	}
	now := time.Now().UTC()
	createSession := func(role string) (string, string) {
		t.Helper()
		id, token := newID(), newID()+newID()+newID()[0:11]
		// A 43-character token matches the opaque staff session format.
		token = token[:43]
		hash := sha256.Sum256([]byte(token))
		if _, err := db.ExecContext(ctx, `INSERT INTO staff_users (id, name, email, password_hash, role, active, created_at, updated_at)
			VALUES (?, 'Order Integration', ?, 'unused', ?, TRUE, ?, ?)`, id, "admin-order-"+id+"@example.test", role, now, now); err != nil {
			t.Fatal(err)
		}
		if _, err := db.ExecContext(ctx, `INSERT INTO staff_sessions (token_hash, staff_id, expires_at, created_at) VALUES (?, ?, ?, ?)`, hash[:], id, now.Add(time.Hour), now); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			_, _ = db.Exec("DELETE FROM staff_sessions WHERE staff_id = ?", id)
			_, _ = db.Exec("DELETE FROM staff_users WHERE id = ?", id)
		})
		return id, token
	}
	_, adminToken := createSession("ADMIN")
	_, staffToken := createSession("STAFF")
	handler := NewHandler(NewService(db, staffauth.New(db)), slog.Default())
	mux := http.NewServeMux()
	handler.Register(mux)
	call := func(path, token string) *httptest.ResponseRecorder {
		t.Helper()
		request := httptest.NewRequest(http.MethodGet, path, nil)
		if token != "" {
			request.Header.Set("Authorization", "Bearer "+token)
		}
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, request)
		return response
	}
	for _, path := range []string{"/api/v1/admin/orders", "/api/v1/admin/orders/0123456789abcdef0123456789abcdef"} {
		if response := call(path, ""); response.Code != http.StatusUnauthorized {
			t.Errorf("unauthenticated %s status = %d", path, response.Code)
		}
		if response := call(path, staffToken); response.Code != http.StatusForbidden {
			t.Errorf("STAFF %s status = %d: %s", path, response.Code, response.Body.String())
		}
	}
	listed := call("/api/v1/admin/orders?dateFrom=2026-10-06&dateTo=2026-10-06", adminToken)
	if listed.Code != http.StatusOK || listed.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("admin list status=%d headers=%v body=%s", listed.Code, listed.Header(), listed.Body.String())
	}
	var got page
	if err := json.Unmarshal(listed.Body.Bytes(), &got); err != nil || got.Items == nil || got.FilterOptions.Events == nil {
		t.Fatalf("admin list response = %s, decode error = %v", listed.Body.String(), err)
	}
	if response := call("/api/v1/admin/orders/0123456789abcdef0123456789abcdef", adminToken); response.Code != http.StatusNotFound {
		t.Fatalf("missing order status=%d body=%s", response.Code, response.Body.String())
	}
	if response := call("/api/v1/admin/orders?unknown=x", adminToken); response.Code != http.StatusBadRequest {
		t.Fatalf("unknown filter status=%d body=%s", response.Code, response.Body.String())
	}
}
