package catalog

import (
	"testing"
	"time"
)

func TestAdminEventValidationDefaultsToDraftAndSeparatesSaleStatus(t *testing.T) {
	input := adminEventInput{ID: "ruang-senja", Artist: "Ruang Senja", City: "Bandung", Venue: "Gudang Bunyi", Address: "Jalan Musik", StartsAt: time.Date(2027, 8, 30, 12, 30, 0, 0, time.UTC), Genre: "Rock", Status: "Early Bird", Image: "https://example.test/poster.jpg", Lineup: []string{"Ruang Senja"}}
	if !input.valid(true) {
		t.Fatal("valid admin event rejected")
	}
	if input.PublicationStatus != "DRAFT" {
		t.Fatalf("publication status = %q, want DRAFT", input.PublicationStatus)
	}
	if input.repositoryInput().Status != "EARLY_BIRD" {
		t.Fatal("sale label was not mapped separately")
	}
}

func TestAdminEventValidationRejectsUnsafePosterAndUnknownPublicationStatus(t *testing.T) {
	base := adminEventInput{ID: "nusa-malam", Artist: "Nusa Malam", City: "Jakarta", Venue: "Ruang Selatan", Address: "Jalan Musik", StartsAt: time.Now(), Genre: "Indie", Status: "Presale", PublicationStatus: "PUBLISHED", Image: "https://example.test/poster.jpg"}
	unsafe := base
	unsafe.Image = "javascript:alert(1)"
	if unsafe.valid(true) {
		t.Fatal("unsafe poster URL accepted")
	}
	invalid := base
	invalid.PublicationStatus = "EARLY_BIRD"
	if invalid.valid(true) {
		t.Fatal("sale label accepted as publication status")
	}
}
