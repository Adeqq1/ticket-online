package adminissues

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/Adeqq1/ticket-online/backend/internal/orderaccess"
	"github.com/Adeqq1/ticket-online/backend/internal/payment"
	"github.com/Adeqq1/ticket-online/backend/internal/staffauth"
	"github.com/Adeqq1/ticket-online/backend/migrations"
	_ "github.com/go-sql-driver/mysql"
)

func TestAdminIssueRoutesRequireAdminAndValidateFilters(t *testing.T) {
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
	makeSession := func(role string) string {
		t.Helper()
		var raw [32]byte
		if _, err := rand.Read(raw[:]); err != nil {
			t.Fatal(err)
		}
		id, token := hex.EncodeToString(raw[:16]), hex.EncodeToString(raw[:])[:43]
		hash := sha256.Sum256([]byte(token))
		now := time.Now().UTC()
		if _, err := db.ExecContext(ctx, `INSERT INTO staff_users (id, name, email, password_hash, role, active, created_at, updated_at)
			VALUES (?, 'Issue Integration', ?, 'unused', ?, TRUE, ?, ?)`, id, "admin-issue-"+id+"@example.test", role, now, now); err != nil {
			t.Fatal(err)
		}
		if _, err := db.ExecContext(ctx, `INSERT INTO staff_sessions (token_hash, staff_id, expires_at, created_at) VALUES (?, ?, ?, ?)`, hash[:], id, now.Add(time.Hour), now); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			_, _ = db.Exec("DELETE FROM staff_sessions WHERE staff_id = ?", id)
			_, _ = db.Exec("DELETE FROM staff_users WHERE id = ?", id)
		})
		return token
	}
	admin, staff := makeSession("ADMIN"), makeSession("STAFF")
	service := NewService(db, staffauth.New(db), orderaccess.New(db, []byte("test-secret-for-admin-issues")), payment.NewRepository(db), "")
	mux := http.NewServeMux()
	NewHandler(service, slog.Default()).Register(mux)
	call := func(path, token string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, path, nil)
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		res := httptest.NewRecorder()
		mux.ServeHTTP(res, req)
		return res
	}
	for _, path := range []string{"/api/v1/admin/payment-cases", "/api/v1/admin/email-jobs"} {
		if res := call(path, ""); res.Code != http.StatusUnauthorized {
			t.Errorf("unauthenticated %s status=%d", path, res.Code)
		}
		if res := call(path, staff); res.Code != http.StatusForbidden {
			t.Errorf("STAFF %s status=%d body=%s", path, res.Code, res.Body.String())
		}
		if res := call(path, admin); res.Code != http.StatusOK || res.Header().Get("Cache-Control") != "no-store" {
			t.Errorf("ADMIN %s status=%d body=%s", path, res.Code, res.Body.String())
		}
	}
	for _, path := range []string{"/api/v1/admin/payment-cases?unknown=x", "/api/v1/admin/email-jobs?kind=OTHER"} {
		if res := call(path, admin); res.Code != http.StatusBadRequest {
			t.Errorf("invalid filter %s status=%d", path, res.Code)
		}
	}
	if res := call("/api/v1/admin/email-jobs?kind=REFUND", admin); res.Code != http.StatusOK {
		t.Errorf("REFUND filter status=%d body=%s", res.Code, res.Body.String())
	}
}
