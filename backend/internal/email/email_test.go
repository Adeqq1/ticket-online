package email

import (
	"strings"
	"testing"
	"time"
)

func TestRenderMessageIncludesEscapedOrderAndTicketSummary(t *testing.T) {
	config := Config{Host: "smtp.example.com", From: "tickets@example.com", FrontendURL: "https://tickets.example.com"}
	current := job{orderID: "order-1", recipient: "buyer@example.com"}
	summary := orderSummary{buyer: "<Buyer>", reference: "TO-123", artist: "<script>Event</script>", city: "Jakarta", venue: "Arena", address: "Jalan Utama", startsAt: time.Date(2027, time.April, 3, 9, 0, 0, 0, time.UTC), subtotal: 1000000, fee: 7500, discount: 0, total: 1007500}
	items := []line{{name: "<VIP>", quantity: 1, unit: 1000000, amount: 1000000}}
	tickets := []ticket{{ID: "ticket-1", Code: "ET-1", AttendeeName: "<Buyer>", TierName: "VIP", Gate: "A", link: "https://tickets.example.com/tiket/ticket-1#access_token=abc"}}
	message, from, to, err := renderMessage(config, current, summary, items, tickets)
	if err != nil {
		t.Fatal(err)
	}
	if from != "tickets@example.com" || to != "buyer@example.com" {
		t.Fatalf("envelope addresses = %q, %q", from, to)
	}
	value := string(message)
	for _, expected := range []string{"multipart/alternative", "Content-Type: text/plain", "Content-Type: text/html", "&lt;script&gt;Event&lt;/script&gt;", "&lt;VIP&gt;", "Rp1.007.500", "access_token=3Dabc", "Jalan Utama", "03 April 2027"} {
		if !strings.Contains(value, expected) {
			t.Errorf("email message does not include %q", expected)
		}
	}
	if strings.Contains(value, "href=\"javascript:") {
		t.Fatal("email message contains an unsafe link")
	}
}

func TestRenderMessageRejectsUnsafeAddressAndURL(t *testing.T) {
	config := Config{Host: "smtp.example.com", From: "tickets@example.com", FrontendURL: "https://tickets.example.com"}
	current := job{orderID: "order-1", recipient: "buyer@example.com"}
	if _, _, _, err := renderMessage(config, job{orderID: "order-1", recipient: "bad\r\nBcc: attacker@example.com"}, orderSummary{}, nil, nil); err == nil {
		t.Fatal("unsafe recipient was accepted")
	}
	config.FrontendURL = "https://tickets.example.com/?next=javascript:alert(1)"
	if _, _, _, err := renderMessage(config, current, orderSummary{}, nil, nil); err == nil {
		t.Fatal("frontend URL with a query was accepted")
	}
}

func TestMoney(t *testing.T) {
	if got := money(123456789); got != "Rp123.456.789" {
		t.Fatalf("money = %q", got)
	}
}
