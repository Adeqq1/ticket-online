package adminreports

import (
	"errors"
	"testing"
	"time"
)

func TestParseFilterUsesJakartaCalendarDaysAnd366DayLimit(t *testing.T) {
	now := time.Date(2026, 10, 8, 3, 0, 0, 0, time.UTC)
	got, err := parseFilter(Filter{}, now)
	if err != nil || got.DateFrom != "2026-09-09" || got.DateTo != "2026-10-08" {
		t.Fatalf("default range = %+v, %v", got, err)
	}
	oneDate, err := parseFilter(Filter{DateFrom: "2024-02-29", DateTo: "2024-02-29"}, now)
	if err != nil || oneDate.From.UTC().Format(time.RFC3339) != "2024-02-28T17:00:00Z" || oneDate.To.UTC().Format(time.RFC3339) != "2024-02-29T17:00:00Z" {
		t.Fatalf("Jakarta date bounds = %+v, %v", oneDate, err)
	}
	if _, err := parseFilter(Filter{DateFrom: "2025-01-01", DateTo: "2026-01-01"}, now); err != nil {
		t.Fatalf("366-day period rejected: %v", err)
	}
	for _, filter := range []Filter{
		{DateFrom: "2025-01-01", DateTo: "2026-01-02"},
		{DateFrom: "2026-10-09", DateTo: "2026-10-08"},
		{DateFrom: "2026-02-30"},
		{EventID: " event"},
	} {
		if _, err := parseFilter(filter, now); !errors.Is(err, ErrInvalidRequest) {
			t.Errorf("parseFilter(%+v) error = %v, want ErrInvalidRequest", filter, err)
		}
	}
}

func TestAttendanceRateUsesEligibleTicketsAndLeavesEmptyDenominatorUnset(t *testing.T) {
	if got := attendanceRate(3, 4); got == nil || *got != 75 {
		t.Fatalf("attendanceRate(3, 4) = %v, want 75%%", got)
	}
	if got := attendanceRate(0, 0); got != nil {
		t.Fatalf("attendanceRate(0, 0) = %v, want nil", got)
	}
}
