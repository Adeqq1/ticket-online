package catalog

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAdminEventValidationDefaultsToDraftAndSeparatesSaleStatus(t *testing.T) {
	input := adminEventInput{ID: "ruang-senja", Artist: "Ruang Senja", City: "Bandung", Venue: "Gudang Bunyi", Address: "Jalan Musik", StartsAt: "2027-08-30T12:30:00Z", Genre: "Rock", Status: "Early Bird", Image: "https://example.test/poster.jpg", Lineup: []string{"Ruang Senja"}}
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

func TestAdminCatalogRejectsMissingSession(t *testing.T) {
	mux := http.NewServeMux()
	NewAdminHandler(nil, nil, slog.Default()).Register(mux)
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/admin/events", nil))
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
}

func TestAdminEventValidationRejectsUnsafePosterAndUnknownPublicationStatus(t *testing.T) {
	base := adminEventInput{ID: "nusa-malam", Artist: "Nusa Malam", City: "Jakarta", Venue: "Ruang Selatan", Address: "Jalan Musik", StartsAt: "2027-08-30T12:30:00Z", Genre: "Indie", Status: "Presale", PublicationStatus: "PUBLISHED", Image: "https://example.test/poster.jpg"}
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
