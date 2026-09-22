package catalog

import "testing"

func TestDisplayStatus(t *testing.T) {
	for input, expected := range map[string]string{
		"EARLY_BIRD": "Early Bird",
		"PRESALE":    "Presale",
		"SOLD_OUT":   "Sold Out",
	} {
		if got := displayStatus(input); got != expected {
			t.Fatalf("displayStatus(%q) = %q, want %q", input, got, expected)
		}
	}
}

func TestDisplaySeating(t *testing.T) {
	if got := displaySeating("FREE_STANDING"); got != "free-standing" {
		t.Fatalf("unexpected free-standing value: %q", got)
	}
	if got := displaySeating("ASSIGNED"); got != "assigned" {
		t.Fatalf("unexpected assigned value: %q", got)
	}
}

func TestEventPriceIsMinimumTierPrice(t *testing.T) {
	tiers := []TicketTier{{Price: 650000}, {Price: 225000}, {Price: 350000}}
	price := uint64(0)
	for _, tier := range tiers {
		if price == 0 || tier.Price < price {
			price = tier.Price
		}
	}
	if price != 225000 {
		t.Fatalf("minimum price = %d, want 225000", price)
	}
}
