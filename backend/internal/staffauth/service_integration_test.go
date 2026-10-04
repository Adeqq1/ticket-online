package staffauth

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/Adeqq1/ticket-online/backend/migrations"
	_ "github.com/go-sql-driver/mysql"
)

func TestStaffSessionPermissionsAndRevocation(t *testing.T) {
	dsn := os.Getenv("MYSQL_TEST_DSN")
	if dsn == "" {
		t.Skip("set MYSQL_TEST_DSN to a disposable MySQL database")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.PingContext(ctx); err != nil {
		t.Fatal(err)
	}
	if err := migrations.Run(ctx, db); err != nil {
		t.Fatal(err)
	}
	suffix, err := newID()
	if err != nil {
		t.Fatal(err)
	}
	var gate string
	if err := db.QueryRowContext(ctx, "SELECT gate FROM ticket_tiers WHERE event_id = 'nusa-malam' ORDER BY id LIMIT 1").Scan(&gate); err != nil {
		t.Fatal(err)
	}
	service := New(db)
	staff, err := service.CreateStaff(ctx, "Petugas Tes", "staff-"+suffix+"@example.com", "staff password for test", []Assignment{{EventID: "nusa-malam", Gate: gate}})
	if err != nil {
		t.Fatal(err)
	}
	adminID, err := newID()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = db.Exec("DELETE FROM staff_sessions WHERE staff_id IN (?, ?)", staff.ID, adminID)
		_, _ = db.Exec("DELETE FROM staff_assignments WHERE staff_id = ?", staff.ID)
		_, _ = db.Exec("DELETE FROM staff_users WHERE id IN (?, ?)", staff.ID, adminID)
	})
	adminHash, err := hashPassword("admin password for test")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	adminEmail := "admin-" + suffix + "@example.com"
	if _, err := db.ExecContext(ctx, `INSERT INTO staff_users (id, name, email, password_hash, role, active, created_at, updated_at)
		VALUES (?, 'Admin Tes', ?, ?, 'ADMIN', TRUE, ?, ?)`, adminID, adminEmail, adminHash, now, now); err != nil {
		t.Fatal(err)
	}
	staffSession, err := service.Login(ctx, staff.Email, "staff password for test", "192.0.2.10:4000")
	if err != nil {
		t.Fatal(err)
	}
	expiresAt, err := time.Parse(time.RFC3339, staffSession.ExpiresAt)
	if err != nil || time.Until(expiresAt) < sessionDuration-time.Minute || time.Until(expiresAt) > sessionDuration+time.Minute {
		t.Fatalf("session expires at %q; want an eight-hour session (%v)", staffSession.ExpiresAt, err)
	}
	expiredSession, err := service.Login(ctx, staff.Email, "staff password for test", "192.0.2.10:4001")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, "UPDATE staff_sessions SET expires_at = UTC_TIMESTAMP(6) - INTERVAL 1 SECOND WHERE token_hash = ?", tokenHash(expiredSession.AccessToken)); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Authenticate(ctx, expiredSession.AccessToken); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("expired session was accepted: %v", err)
	}
	principal, err := service.Authenticate(ctx, staffSession.AccessToken)
	if err != nil {
		t.Fatal(err)
	}
	if err := service.AuthorizeGate(ctx, principal, "nusa-malam", gate); err != nil {
		t.Fatalf("assigned gate denied: %v", err)
	}
	if err := service.AuthorizeGate(ctx, principal, "nusa-malam", "Unassigned Gate"); !errors.Is(err, ErrForbidden) {
		t.Fatalf("unassigned gate error = %v", err)
	}

	mux := http.NewServeMux()
	NewHandler(service, slog.New(slog.NewTextHandler(io.Discard, nil))).Register(mux)
	adminSession, err := service.Login(ctx, adminEmail, "admin password for test", "192.0.2.11:4000")
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/staff", nil)
	request.Header.Set("Authorization", "Bearer "+staffSession.AccessToken)
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("staff access to admin API = %d, want 403", response.Code)
	}
	request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/staff", nil)
	request.Header.Set("Authorization", "Bearer "+adminSession.AccessToken)
	response = httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("admin access to staff list = %d: %s", response.Code, response.Body.String())
	}

	if err := service.ResetPassword(ctx, staff.ID, "new staff password for test"); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Authenticate(ctx, staffSession.AccessToken); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("reset did not revoke session: %v", err)
	}
	newSession, err := service.Login(ctx, staff.Email, "new staff password for test", "192.0.2.10:4001")
	if err != nil {
		t.Fatal(err)
	}
	if err := service.UpdateStaff(ctx, staff.ID, nil, boolPointer(false)); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Authenticate(ctx, newSession.AccessToken); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("deactivation did not revoke session: %v", err)
	}

	body, _ := json.Marshal(map[string]string{"email": staff.Email, "password": "new staff password for test"})
	request = httptest.NewRequest(http.MethodPost, "/api/v1/staff/login", bytes.NewReader(body))
	response = httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("inactive login = %d, Cache-Control %q", response.Code, response.Header().Get("Cache-Control"))
	}
}

func boolPointer(value bool) *bool { return &value }
