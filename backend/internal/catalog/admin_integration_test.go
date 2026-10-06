package catalog

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/Adeqq1/ticket-online/backend/internal/checkout"
	"github.com/Adeqq1/ticket-online/backend/internal/reservation"
	"github.com/Adeqq1/ticket-online/backend/internal/staffauth"
	"github.com/Adeqq1/ticket-online/backend/migrations"
	_ "github.com/go-sql-driver/mysql"
)

func TestAdminLifecycleLocksAuditsArchivesAndPreservesReservationSnapshot(t *testing.T) {
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
	defer db.Close()
	if err := db.PingContext(ctx); err != nil {
		t.Fatal(err)
	}
	if err := migrations.Run(ctx, db); err != nil {
		t.Fatal(err)
	}
	suffix, err := reservationID()
	if err != nil {
		t.Fatal(err)
	}
	staffService := staffauth.New(db)
	staff, err := staffService.CreateStaff(ctx, "Catalog Staff", "catalog-"+suffix+"@example.test", "catalog test password", nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), "DELETE FROM staff_sessions WHERE staff_id = ?", staff.ID)
		_, _ = db.ExecContext(context.Background(), "DELETE FROM staff_assignments WHERE staff_id = ?", staff.ID)
		_, _ = db.ExecContext(context.Background(), "DELETE FROM staff_users WHERE id = ?", staff.ID)
	})
	staffSession, err := staffService.Login(ctx, staff.Email, "catalog test password", "192.0.2.50:4000")
	if err != nil {
		t.Fatal(err)
	}
	adminMux := http.NewServeMux()
	NewAdminHandler(NewRepository(db), staffService, slog.Default()).Register(adminMux)
	adminRequest := httptest.NewRequest(http.MethodGet, "/api/v1/admin/events", nil)
	adminRequest.Header.Set("Authorization", "Bearer "+staffSession.AccessToken)
	adminResponse := httptest.NewRecorder()
	adminMux.ServeHTTP(adminResponse, adminRequest)
	if adminResponse.Code != http.StatusForbidden {
		t.Fatalf("staff access status = %d, want %d", adminResponse.Code, http.StatusForbidden)
	}

	eventID := "admin-test-" + suffix[:12]
	actor := AdminActor{ID: "00000000000000000000000000000001", Name: "Admin Integration"}
	now := time.Now().UTC().Add(48 * time.Hour).Truncate(time.Second)
	input := EventInput{ID: eventID, Artist: "Admin Test", City: "Jakarta", Venue: "Venue A", Address: "Address A", StartsAt: &now, Genre: "Pop", Status: "PRESALE", PublicationStatus: "DRAFT", Image: "https://example.test/poster.jpg", Description: "Test", Lineup: []string{"Admin Test"}}
	repo := NewRepository(db)
	if err := repo.CreateEvent(ctx, input, actor); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), `DELETE et FROM etickets et JOIN orders o ON o.id = et.order_id JOIN reservations r ON r.id = o.reservation_id WHERE r.event_id = ?`, eventID)
		_, _ = db.ExecContext(context.Background(), `DELETE oa FROM order_attendees oa JOIN orders o ON o.id = oa.order_id JOIN reservations r ON r.id = o.reservation_id WHERE r.event_id = ?`, eventID)
		_, _ = db.ExecContext(context.Background(), `DELETE ob FROM order_buyers ob JOIN orders o ON o.id = ob.order_id JOIN reservations r ON r.id = o.reservation_id WHERE r.event_id = ?`, eventID)
		_, _ = db.ExecContext(context.Background(), `DELETE oi FROM order_items oi JOIN orders o ON o.id = oi.order_id JOIN reservations r ON r.id = o.reservation_id WHERE r.event_id = ?`, eventID)
		_, _ = db.ExecContext(context.Background(), `DELETE o FROM orders o JOIN reservations r ON r.id = o.reservation_id WHERE r.event_id = ?`, eventID)
		_, _ = db.ExecContext(context.Background(), `DELETE ri FROM reservation_items ri JOIN reservations r ON r.id = ri.reservation_id WHERE r.event_id = ?`, eventID)
		_, _ = db.ExecContext(context.Background(), "DELETE FROM reservations WHERE event_id = ?", eventID)
		_, _ = db.ExecContext(context.Background(), "DELETE FROM admin_audit_log WHERE object_id = ? OR object_id LIKE ?", eventID, eventID+"/%")
		_, _ = db.ExecContext(context.Background(), "DELETE FROM event_lineups WHERE event_id = ?", eventID)
		_, _ = db.ExecContext(context.Background(), "DELETE FROM ticket_tiers WHERE event_id = ?", eventID)
		_, _ = db.ExecContext(context.Background(), "DELETE FROM event_zones WHERE event_id = ?", eventID)
		_, _ = db.ExecContext(context.Background(), "DELETE FROM events WHERE id = ?", eventID)
	})
	if err := repo.SaveZone(ctx, eventID, ZoneInput{ID: "main", Name: "Main", Description: "Main zone"}, true, actor); err != nil {
		t.Fatal(err)
	}
	tier := TierInput{ID: "general", Name: "General", ZoneID: "main", Price: 10000, Capacity: 10, MaxPerOrder: 4, Benefit: "Entry", Gate: "Gate A", Seating: "FREE_STANDING"}
	if err := repo.SaveTier(ctx, eventID, tier, true, actor); err != nil {
		t.Fatal(err)
	}
	input.PublicationStatus = "PUBLISHED"
	if err := repo.UpdateEvent(ctx, eventID, input, actor); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.GetEvent(ctx, eventID); err != nil {
		t.Fatalf("published event missing from catalog: %v", err)
	}

	reservations := reservation.NewRepository(db, 15*time.Minute)
	held, err := reservations.Create(ctx, reservation.Request{EventID: eventID, Items: []reservation.ItemRequest{{TierID: "general", Quantity: 1}}}, "admin-lifecycle-"+suffix)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.SaveTier(ctx, eventID, TierInput{ID: tier.ID, Name: tier.Name, ZoneID: tier.ZoneID, Price: 20000, Capacity: tier.Capacity, MaxPerOrder: tier.MaxPerOrder, Benefit: tier.Benefit, Gate: tier.Gate, Seating: tier.Seating}, false, actor); err != nil {
		t.Fatal(err)
	}
	if held.Items[0].UnitPrice != 10000 {
		t.Fatalf("held unit price = %d, want original price", held.Items[0].UnitPrice)
	}
	start := make(chan struct{})
	results := make(chan error, 2)
	var concurrent sync.WaitGroup
	var concurrentReservationID string
	concurrent.Add(2)
	go func() {
		defer concurrent.Done()
		<-start
		results <- repo.SaveTier(ctx, eventID, TierInput{ID: tier.ID, Name: tier.Name, ZoneID: tier.ZoneID, Price: 20000, Capacity: 2, MaxPerOrder: tier.MaxPerOrder, Benefit: tier.Benefit, Gate: tier.Gate, Seating: tier.Seating}, false, actor)
	}()
	go func() {
		defer concurrent.Done()
		<-start
		created, err := reservations.Create(ctx, reservation.Request{EventID: eventID, Items: []reservation.ItemRequest{{TierID: "general", Quantity: 1}}}, "admin-concurrent-"+suffix)
		if err == nil {
			concurrentReservationID = created.ID
		}
		results <- err
	}()
	close(start)
	concurrent.Wait()
	close(results)
	for err := range results {
		if err != nil {
			t.Fatalf("concurrent capacity/reservation operation: %v", err)
		}
	}
	detail, err := repo.adminDetail(ctx, eventID)
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.TicketTiers) != 1 || detail.TicketTiers[0].Capacity != 2 || detail.TicketTiers[0].BoundQuantity != 2 || detail.TicketTiers[0].AvailableQuantity != 0 {
		t.Fatalf("concurrent capacity result = %+v", detail.TicketTiers)
	}
	tooSmall := tier
	tooSmall.Capacity = 0
	if err := repo.SaveTier(ctx, eventID, tooSmall, false, actor); !errors.Is(err, ErrCapacityBelowBound) {
		t.Fatalf("capacity error = %v, want ErrCapacityBelowBound", err)
	}
	changedGate := tier
	changedGate.Gate = "Gate B"
	if err := repo.SaveTier(ctx, eventID, changedGate, false, actor); !errors.Is(err, ErrGateLocked) {
		t.Fatalf("gate error = %v, want ErrGateLocked", err)
	}

	changedLocation := input
	changedLocation.Venue = "Venue B"
	if err := repo.UpdateEvent(ctx, eventID, changedLocation, actor); !errors.Is(err, ErrLocationLocked) {
		t.Fatalf("location error = %v, want ErrLocationLocked", err)
	}
	changedSchedule := input
	later := now.Add(time.Hour)
	changedSchedule.StartsAt = &later
	if err := repo.UpdateEvent(ctx, eventID, changedSchedule, actor); !errors.Is(err, ErrScheduleLocked) {
		t.Fatalf("schedule error = %v, want ErrScheduleLocked", err)
	}
	orderRequest := checkout.Request{Buyer: checkout.Buyer{Name: "Buyer Test", Email: "buyer@example.test", Phone: "081234567890", Identity: "123456789012"}, Attendees: []checkout.Attendees{{TierID: "general", Names: []string{"Attendee One"}}}}
	order, _, err := checkout.NewRepository(db).Create(ctx, held.ID, "admin-lifecycle-"+suffix, orderRequest)
	if err != nil {
		t.Fatalf("create pending order before archive: %v", err)
	}

	input.PublicationStatus = "ARCHIVED"
	if err := repo.UpdateEvent(ctx, eventID, input, actor); err != nil {
		t.Fatalf("archive event: %v", err)
	}
	if _, err := repo.GetEvent(ctx, eventID); !errors.Is(err, ErrEventNotFound) {
		t.Fatalf("archived catalog lookup = %v, want ErrEventNotFound", err)
	}
	if _, err := reservations.Create(ctx, reservation.Request{EventID: eventID, Items: []reservation.ItemRequest{{TierID: "general", Quantity: 1}}}, "admin-archive-"+suffix); !errors.Is(err, reservation.ErrEventNotFound) {
		t.Fatalf("new reservation on archived event = %v", err)
	}
	stillHeld, err := reservations.Get(ctx, concurrentReservationID)
	if err != nil || stillHeld.ID != concurrentReservationID || stillHeld.Items[0].UnitPrice != 20000 {
		t.Fatalf("existing reservation after archive = %+v, %v", stillHeld, err)
	}
	var orderStatus string
	var snapshotPrice uint64
	if err := db.QueryRowContext(ctx, "SELECT o.status, oi.unit_price FROM orders o JOIN order_items oi ON oi.order_id = o.id WHERE o.id = ?", order.ID).Scan(&orderStatus, &snapshotPrice); err != nil {
		t.Fatal(err)
	}
	if orderStatus != "PENDING" || snapshotPrice != 10000 {
		t.Fatalf("archived order state/price = %s/%d, want PENDING/10000", orderStatus, snapshotPrice)
	}
	var auditCount int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM admin_audit_log WHERE actor_id = ? AND (object_id = ? OR object_id LIKE ?)`, actor.ID, eventID, eventID+"/%").Scan(&auditCount); err != nil {
		t.Fatal(err)
	}
	if auditCount < 5 {
		t.Fatalf("audit entries = %d, want at least 5", auditCount)
	}
}

func reservationID() (string, error) {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return hex.EncodeToString(value), nil
}
