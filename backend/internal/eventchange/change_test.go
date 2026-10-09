package eventchange

import (
	"testing"
	"time"
)

func TestDecisionValidation(t *testing.T) {
	now := time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)
	start := now.Add(48 * time.Hour).Format(time.RFC3339)
	deadline := now.Add(24 * time.Hour).Format(time.RFC3339)
	valid := Input{Action: "RESCHEDULED", Reason: "Venue belum siap", Announcement: "Konser dipindah jadwal.", StartsAt: &start, RefundDeadline: &deadline}
	if err := validate(valid, now); err != nil {
		t.Fatal(err)
	}
	for _, change := range []func(*Input){
		func(v *Input) { v.Action = "UNKNOWN" }, func(v *Input) { v.Reason = "" }, func(v *Input) { v.Announcement = "" },
		func(v *Input) { v.StartsAt = nil }, func(v *Input) { v.RefundDeadline = nil }, func(v *Input) { v.RefundDeadline = &start },
		func(v *Input) { v.Action = "CANCELLED" },
	} {
		v := valid
		change(&v)
		if validate(v, now) == nil {
			t.Fatalf("invalid decision accepted: %+v", v)
		}
	}
	for _, action := range []string{"POSTPONED", "CANCELLED"} {
		v := valid
		v.Action = action
		v.StartsAt = nil
		v.RefundDeadline = nil
		if err := validate(v, now); err != nil {
			t.Fatal(err)
		}
	}
}
