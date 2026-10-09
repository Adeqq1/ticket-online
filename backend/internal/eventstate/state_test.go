package eventstate

import (
	"testing"
	"time"
)

func TestAdmissionAndSales(t *testing.T) {
	start := "2026-10-10T17:00:00Z"
	s := State{Status: "RESCHEDULED", Version: 1, StartsAt: &start}
	if !s.CanSell() || s.CanCheckIn(time.Date(2026, 10, 10, 16, 59, 0, 0, time.UTC)) || !s.CanCheckIn(time.Date(2026, 10, 10, 17, 0, 0, 0, time.UTC)) {
		t.Fatal("replacement day must use WIB")
	}
	s.SalesPaused = true
	if s.CanSell() {
		t.Fatal("cleanup must hold sales")
	}
	for _, status := range []string{"POSTPONED", "CANCELLED"} {
		s.Status = status
		if s.CanSell() || s.CanCheckIn(time.Date(2026, 10, 10, 17, 0, 0, 0, time.UTC)) {
			t.Fatal(status)
		}
	}
	if TimeJSON(time.Time{}) != nil {
		t.Fatal("unsettled access has no final expiry")
	}
}
