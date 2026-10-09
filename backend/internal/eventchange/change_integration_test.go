package eventchange

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/Adeqq1/ticket-online/backend/internal/checkin"
	"github.com/Adeqq1/ticket-online/backend/internal/checkout"
	"github.com/Adeqq1/ticket-online/backend/internal/eventstate"
	"github.com/Adeqq1/ticket-online/backend/internal/orderaccess"
	"github.com/Adeqq1/ticket-online/backend/internal/payment"
	"github.com/Adeqq1/ticket-online/backend/internal/recovery"
	"github.com/Adeqq1/ticket-online/backend/internal/refund"
	"github.com/Adeqq1/ticket-online/backend/internal/reservation"
	"github.com/Adeqq1/ticket-online/backend/internal/staffauth"
	"github.com/Adeqq1/ticket-online/backend/migrations"
	"github.com/go-sql-driver/mysql"
)

type fixture struct {
	s                     *Service
	db                    *sql.DB
	admin, staff, staffID string
	mux                   *http.ServeMux
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	dsn := os.Getenv("MYSQL_TEST_DSN")
	if dsn == "" {
		t.Skip("set MYSQL_TEST_DSN to a disposable MySQL account with CREATE DATABASE")
	}
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		t.Fatal(err)
	}
	cfg.DBName = ""
	admin, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	id, _ := recovery.NewID()
	name := "ticket_change_" + id
	if _, err := admin.Exec("CREATE DATABASE " + name); err != nil {
		admin.Close()
		t.Fatal(err)
	}
	cfg.DBName = name
	cfg.ParseTime = true
	cfg.Loc = time.UTC
	db, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		db.Close()
		if _, err := admin.Exec("DROP DATABASE " + name); err != nil {
			t.Error(err)
		}
		admin.Close()
	})
	if err := migrations.Run(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	f := fixture{db: db}
	for _, role := range []string{"ADMIN", "STAFF"} {
		id, _ := recovery.NewID()
		token, _, _ := recovery.NewToken()
		hash := sha256.Sum256([]byte(token))
		if _, err := db.Exec(`INSERT INTO staff_users(id,name,email,password_hash,role,active,created_at,updated_at) VALUES (?,'Test Staff',?,'unused',?,TRUE,UTC_TIMESTAMP(6),UTC_TIMESTAMP(6))`, id, id+"@example.test", role); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`INSERT INTO staff_sessions(token_hash,staff_id,expires_at,created_at) VALUES (?,?,UTC_TIMESTAMP(6)+INTERVAL 1 DAY,UTC_TIMESTAMP(6))`, hash[:], id); err != nil {
			t.Fatal(err)
		}
		if role == "ADMIN" {
			f.admin = token
		} else {
			f.staff = token
			f.staffID = id
		}
	}
	if _, err := db.Exec("INSERT INTO staff_assignments(staff_id,event_id,gate,created_at) VALUES (?,'nusa-malam','Gate B',UTC_TIMESTAMP(6))", f.staffID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("UPDATE events SET starts_at=UTC_TIMESTAMP(6)+INTERVAL 2 DAY WHERE id='nusa-malam'"); err != nil {
		t.Fatal(err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	staff := staffauth.New(db)
	access := orderaccess.New(db, make([]byte, 32))
	payments := payment.NewRepository(db)
	refunds := refund.New(db, staff, "", "sandbox", nil, logger)
	f.s = New(db, staff, access, refunds, payments, "", logger)
	f.mux = http.NewServeMux()
	f.s.Register(f.mux)
	refund.NewHandler(refunds, logger).Register(f.mux)
	return f
}
func (f fixture) call(t *testing.T, method, path, token, key string, body any) *httptest.ResponseRecorder {
	t.Helper()
	raw, _ := json.Marshal(body)
	r := httptest.NewRequest(method, path, bytes.NewReader(raw))
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	r.Header.Set("Idempotency-Key", key)
	w := httptest.NewRecorder()
	f.mux.ServeHTTP(w, r)
	return w
}
func (f fixture) order(t *testing.T, paid bool) checkout.Order {
	t.Helper()
	ctx := context.Background()
	key, _ := recovery.NewID()
	r, err := f.s.reservations.Create(ctx, reservation.Request{EventID: "nusa-malam", Items: []reservation.ItemRequest{{TierID: "festival", Quantity: 1}}}, key)
	if err != nil {
		t.Fatal(err)
	}
	o, _, err := checkout.NewRepository(f.db).Create(ctx, r.ID, key, checkout.Request{Buyer: checkout.Buyer{Name: "Test Buyer", Email: "buyer@example.test", Phone: "081234567890", Identity: "123456789012"}, Attendees: []checkout.Attendees{{TierID: "festival", Names: []string{"Test Buyer"}}}})
	if err != nil {
		t.Fatal(err)
	}
	if paid {
		if _, _, err := f.s.payments.Simulate(ctx, o.ID, payment.Request{Method: "VIRTUAL_ACCOUNT", Result: "SUCCEEDED"}); err != nil {
			t.Fatal(err)
		}
	}
	return o
}
func (f fixture) prepare(t *testing.T, action string, version uint64) (Input, *httptest.ResponseRecorder) {
	t.Helper()
	in := Input{Action: action, Reason: "Kondisi acara berubah", Announcement: "Periksa informasi jadwal terbaru.", ExpectedVersion: version}
	if action == "RESCHEDULED" {
		start := time.Now().Add(72 * time.Hour).UTC().Truncate(time.Microsecond).Format(time.RFC3339Nano)
		deadline := time.Now().Add(48 * time.Hour).UTC().Truncate(time.Microsecond).Format(time.RFC3339Nano)
		in.StartsAt = &start
		in.RefundDeadline = &deadline
	}
	w := f.call(t, "POST", "/api/v1/admin/events/nusa-malam/changes/preview", f.admin, "", in)
	if w.Code == 200 {
		var p Preview
		if err := json.Unmarshal(w.Body.Bytes(), &p); err != nil {
			t.Fatal(err)
		}
		in.Snapshot = p.Snapshot
	}
	return in, w
}
func (f fixture) decide(t *testing.T, action string, version uint64) Input {
	t.Helper()
	in, w := f.prepare(t, action, version)
	if w.Code != 200 {
		t.Fatalf("preview %d: %s", w.Code, w.Body.String())
	}
	key, _ := recovery.NewID()
	w = f.call(t, "POST", "/api/v1/admin/events/nusa-malam/changes", f.admin, key, in)
	if w.Code != 200 {
		t.Fatalf("commit %d: %s", w.Code, w.Body.String())
	}
	repeat := f.call(t, "POST", "/api/v1/admin/events/nusa-malam/changes", f.admin, key, in)
	if repeat.Code != 200 || repeat.Body.String() != w.Body.String() {
		t.Fatalf("replay differs: %d %s", repeat.Code, repeat.Body.String())
	}
	return in
}
func count(t *testing.T, db *sql.DB, query string, args ...any) int {
	t.Helper()
	var n int
	if err := db.QueryRow(query, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestCancellationPreservesCheckInAndRefundsFullOrderOnce(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	o := f.order(t, true)
	tickets, err := f.s.payments.TicketsForOrder(ctx, o.ID)
	if err != nil || len(tickets) != 1 {
		t.Fatalf("tickets: %v", err)
	}
	if _, err := checkin.NewService(f.db, f.s.staff).CheckIn(ctx, f.staff, checkin.Request{EventID: "nusa-malam", Gate: "Gate B", Code: tickets[0].Code}); err != nil {
		t.Fatal(err)
	}
	f.decide(t, "CANCELLED", 0)
	if _, err := f.db.Exec("UPDATE payments SET status='FAILED' WHERE order_id=?", o.ID); err != nil {
		t.Fatal(err)
	}
	if err := f.s.Process(ctx); err == nil {
		t.Fatal("unverified payment silently skipped refund")
	}
	if count(t, f.db, "SELECT COUNT(*) FROM event_change_orders WHERE order_id=? AND last_error<>''", o.ID) != 1 {
		t.Fatal("refund submission error is missing from decision progress")
	}
	if _, err := f.db.Exec("UPDATE payments SET status='SUCCEEDED' WHERE order_id=?", o.ID); err != nil {
		t.Fatal(err)
	}
	if err := f.s.Process(ctx); err != nil {
		t.Fatal(err)
	}
	if count(t, f.db, "SELECT COUNT(*) FROM event_change_orders WHERE order_id=? AND last_error<>''", o.ID) != 0 {
		t.Fatal("successful refund submission did not clear its error")
	}
	if err := f.s.Process(ctx); err != nil {
		t.Fatal(err)
	}
	if n := count(t, f.db, "SELECT COUNT(*) FROM order_refunds WHERE order_id=? AND status='MANUAL_REQUIRED' AND amount=?", o.ID, o.Total); n != 1 {
		t.Fatal("full manual refund not queued")
	}
	view, err := f.s.payments.Ticket(ctx, tickets[0].ID)
	if err != nil || view.Usable == nil || *view.Usable || view.CurrentEvent.StartsAt != nil {
		t.Fatalf("cancelled ticket remained active: %+v %v", view, err)
	}
	if deadline, err := f.s.access.AuthorizeOrder(ctx, o.ID, f.s.access.Token(o.ID)); err != nil || !deadline.IsZero() {
		t.Fatalf("unsettled access expired: %v %v", deadline, err)
	}
	in := refund.ManualInput{Reference: "bank-confirmed-123", PaidAt: time.Now().UTC().Format(time.RFC3339Nano), Note: "Transfer penuh terverifikasi"}
	if _, err := f.s.refunds.CompleteManual(ctx, f.staff, o.ID, in); !errors.Is(err, staffauth.ErrForbidden) {
		t.Fatalf("staff completed refund: %v", err)
	}
	for i := 0; i < 2; i++ {
		if _, err := f.s.refunds.CompleteManual(ctx, f.admin, o.ID, in); err != nil {
			t.Fatal(err)
		}
	}
	if count(t, f.db, "SELECT COUNT(*) FROM ticket_checkins WHERE ticket_id=?", tickets[0].ID) != 1 {
		t.Fatal("check-in history erased")
	}
	if count(t, f.db, "SELECT available_quantity FROM ticket_tiers WHERE event_id='nusa-malam' AND slug='festival'") != 42 {
		t.Fatal("stock restored more than once")
	}
	if count(t, f.db, "SELECT COUNT(*) FROM email_queue WHERE order_id=? AND kind='EVENT_CHANGE'", o.ID) != 1 {
		t.Fatal("duplicate decision email")
	}
	if deadline, err := f.s.access.AuthorizeOrder(ctx, o.ID, f.s.access.Token(o.ID)); err != nil || deadline.Before(time.Now().Add(89*24*time.Hour)) {
		t.Fatalf("final access period: %v %v", deadline, err)
	}
	restored, found, err := recovery.FindOrderByID(ctx, f.db, o.ID)
	if err != nil || !found || !restored.Eligible(time.Now()) || restored.TicketActive {
		t.Fatalf("refunded access not recoverable: %+v %v", restored, err)
	}
}

func TestPostponementAndRescheduleKeepQRAndRejectOldTransactions(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	paid := f.order(t, true)
	pending := f.order(t, false)
	before, _ := f.s.payments.TicketsForOrder(ctx, paid.ID)
	f.decide(t, "POSTPONED", 0)
	if _, _, err := f.s.payments.Simulate(ctx, pending.ID, payment.Request{Method: "QRIS", Result: "SUCCEEDED"}); !errors.Is(err, payment.ErrOrderNotPayable) {
		t.Fatalf("old payment accepted: %v", err)
	}
	if err := f.s.Process(ctx); err != nil {
		t.Fatal(err)
	}
	view, err := f.s.payments.Ticket(ctx, before[0].ID)
	if err != nil || *view.Usable || view.CurrentEvent.StartsAt != nil {
		t.Fatalf("postponed ticket: %+v %v", view, err)
	}
	if _, err := checkin.NewService(f.db, f.s.staff).CheckIn(ctx, f.staff, checkin.Request{EventID: "nusa-malam", Gate: "Gate B", Code: before[0].Code}); !errors.Is(err, eventstate.ErrClosed) {
		t.Fatalf("postponed admission: %v", err)
	}
	deadline, err := f.s.access.AuthorizeOrder(ctx, paid.ID, f.s.access.Token(paid.ID))
	if err != nil || !deadline.IsZero() {
		t.Fatalf("postponed access: %v %v", deadline, err)
	}
	next := f.decide(t, "RESCHEDULED", 1)
	if err := f.s.Process(ctx); err != nil {
		t.Fatal(err)
	}
	view, err = f.s.payments.Ticket(ctx, before[0].ID)
	if err != nil || view.Code != before[0].Code || !*view.Usable || *view.CurrentEvent.StartsAt != *next.StartsAt {
		t.Fatalf("replacement view: %+v %v", view, err)
	}
	if _, _, err := f.s.payments.Simulate(ctx, pending.ID, payment.Request{Method: "QRIS", Result: "SUCCEEDED"}); !errors.Is(err, payment.ErrOrderNotPayable) {
		t.Fatalf("old payment revived: %v", err)
	}
	state, err := eventstate.Read(ctx, f.db, "nusa-malam", false)
	if err != nil || !state.CanSell() {
		t.Fatalf("sales not reopened: %+v %v", state, err)
	}
	if _, err := checkin.NewService(f.db, f.s.staff).CheckIn(ctx, f.staff, checkin.Request{EventID: "nusa-malam", Gate: "Gate B", Code: before[0].Code}); !errors.Is(err, eventstate.ErrClosed) {
		t.Fatalf("wrong day admission: %v", err)
	}
	for i := 0; i < 2; i++ {
		w := f.call(t, "POST", "/api/v1/orders/"+paid.ID+"/refund-request", f.s.access.Token(paid.ID), "", nil)
		if w.Code != 200 {
			t.Fatalf("buyer refund %d: %s", w.Code, w.Body.String())
		}
	}
	view, err = f.s.payments.Ticket(ctx, before[0].ID)
	if err != nil || *view.Usable {
		t.Fatalf("refund request kept QR active before worker processing: %+v %v", view, err)
	}
	if err := f.s.Process(ctx); err != nil {
		t.Fatal(err)
	}
	if count(t, f.db, "SELECT COUNT(*) FROM order_refunds WHERE order_id=?", paid.ID) != 1 {
		t.Fatal("buyer request duplicated refund")
	}
	view, _ = f.s.payments.Ticket(ctx, before[0].ID)
	if *view.Usable {
		t.Fatal("requested refund still has active QR")
	}
}

func TestChangedPreviewAndStaffCannotCommitDecision(t *testing.T) {
	f := newFixture(t)
	in, w := f.prepare(t, "CANCELLED", 0)
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	f.order(t, false)
	key, _ := recovery.NewID()
	w = f.call(t, "POST", "/api/v1/admin/events/nusa-malam/changes", f.admin, key, in)
	if w.Code != 409 {
		t.Fatalf("stale preview accepted: %d %s", w.Code, w.Body.String())
	}
	w = f.call(t, "POST", "/api/v1/admin/events/nusa-malam/changes", f.staff, key, in)
	if w.Code != 403 {
		t.Fatalf("staff decision: %d", w.Code)
	}
	if count(t, f.db, "SELECT change_version FROM events WHERE id='nusa-malam'") != 0 {
		t.Fatal("failed decision mutated event")
	}
}

func TestCancellationRacesSettlementAndCheckIn(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	pending := f.order(t, false)
	paid := f.order(t, true)
	tickets, _ := f.s.payments.TicketsForOrder(ctx, paid.ID)
	gateway := "gw-" + pending.ID
	if _, err := f.db.Exec(`INSERT INTO payments(id,order_id,method,amount,status,gateway_order_id,created_at,updated_at) VALUES (?,?,'VIRTUAL_ACCOUNT',?,'PENDING',?,UTC_TIMESTAMP(6),UTC_TIMESTAMP(6))`, pending.ID, pending.ID, pending.Total, gateway); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	errs := make(chan error, 3)
	wg.Add(3)
	go func() {
		defer wg.Done()
		tx, err := f.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
		if err == nil {
			defer tx.Rollback()
			err = f.s.payments.ApplyGatewayStatusTx(ctx, tx, gateway, fmt.Sprintf("%d.00", pending.Total), "settlement", "")
			if err == nil {
				err = tx.Commit()
			}
		}
		errs <- err
	}()
	go func() {
		defer wg.Done()
		_, err := checkin.NewService(f.db, f.s.staff).CheckIn(ctx, f.staff, checkin.Request{EventID: "nusa-malam", Gate: "Gate B", Code: tickets[0].Code})
		if errors.Is(err, eventstate.ErrClosed) {
			err = nil
		}
		errs <- err
	}()
	go func() {
		defer wg.Done()
		in, w := f.prepare(t, "CANCELLED", 0)
		if w.Code != 200 {
			errs <- fmt.Errorf("preview %d %s", w.Code, w.Body.String())
			return
		}
		key, _ := recovery.NewID()
		w = f.call(t, "POST", "/api/v1/admin/events/nusa-malam/changes", f.admin, key, in)
		if w.Code == 409 {
			in, w = f.prepare(t, "CANCELLED", 0)
			if w.Code == 200 {
				w = f.call(t, "POST", "/api/v1/admin/events/nusa-malam/changes", f.admin, key, in)
			}
		}
		if w.Code != 200 {
			errs <- fmt.Errorf("decision %d %s", w.Code, w.Body.String())
		} else {
			errs <- nil
		}
	}()
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if err := f.s.Process(ctx); err != nil {
		t.Fatal(err)
	}
	if count(t, f.db, "SELECT COUNT(*) FROM event_refund_rights WHERE requested=TRUE") != 2 {
		t.Fatal("race lost a refund right")
	}
	if count(t, f.db, "SELECT COUNT(*) FROM orders WHERE status='PAID'") != 0 {
		t.Fatal("cancelled paid order not held for refund")
	}
	for _, id := range []string{pending.ID, paid.ID} {
		in := refund.ManualInput{Reference: "transfer-" + id, PaidAt: time.Now().UTC().Format(time.RFC3339Nano), Note: "Full refund verified"}
		if _, err := f.s.refunds.CompleteManual(ctx, f.admin, id, in); err != nil {
			t.Fatal(err)
		}
	}
	if count(t, f.db, "SELECT available_quantity FROM ticket_tiers WHERE event_id='nusa-malam' AND slug='festival'") != 42 {
		t.Fatal("race duplicated stock release")
	}
}
