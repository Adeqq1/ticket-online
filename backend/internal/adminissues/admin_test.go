package adminissues

import (
	"testing"
	"time"
)

func TestIssueInputHelpersRejectMalformedValues(t *testing.T) {
	if _, _, err := decodeCursor("not-a-cursor", true); err == nil {
		t.Fatal("malformed cursor accepted")
	}
	if _, err := normalizeNote(" \n "); err == nil {
		t.Fatal("empty admin note accepted")
	}
	if _, err := normalizeNote(string(make([]rune, 2001))); err == nil {
		t.Fatal("oversized admin note accepted")
	}
	if got := truncate("aébc", 3); got != "aé" {
		t.Fatalf("truncate split UTF-8: %q", got)
	}
	encoded, err := encodeCursor(time.Date(2026, 10, 6, 1, 2, 3, 4, time.FixedZone("x", 3600)), "12")
	if err != nil {
		t.Fatal(err)
	}
	_, at, err := decodeCursor(encoded, true)
	if err != nil || at.Location() != time.UTC {
		t.Fatalf("cursor timestamp = %v, err = %v", at, err)
	}
}
