package email

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/textproto"
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

func TestRenderRefundMessageUsesSnapshotAndEscapesReason(t *testing.T) {
	cfg := Config{Host: "smtp.example.com", From: "tickets@example.com", FrontendURL: "https://tickets.example.com"}
	current := job{id: "job-1", claimToken: "claim-1", recipient: "buyer@example.com", refundSnapshot: []byte(`{"status":"SUCCEEDED","amount":125000,"reference":"TO-123","reason":"<event canceled>"}`)}
	message, from, to, err := renderRefundMessage(cfg, current)
	if err != nil || from != "tickets@example.com" || to != "buyer@example.com" {
		t.Fatalf("render refund: %q %q %v", from, to, err)
	}
	for _, want := range []string{"TO-123", "Rp125.000", "&lt;event canceled&gt;"} {
		if !strings.Contains(string(message), want) {
			t.Errorf("refund email missing %q", want)
		}
	}
	current.refundSnapshot = []byte(`{"status":"UNKNOWN","amount":125000,"reference":"TO-123"}`)
	if _, _, _, err := renderRefundMessage(cfg, current); err == nil {
		t.Fatal("unknown refund result could be emailed as final")
	}
}

func TestEmailOrigins(t *testing.T) {
	for _, value := range []string{"https://tickets.example.com", "http://localhost:5173"} {
		if !safeURL(value) {
			t.Fatalf("valid origin %q rejected", value)
		}
	}
	for _, value := range []string{"https://tickets.example.com/app", "ftp://tickets.example.com", "https://user@tickets.example.com", "https://tickets.example.com?x=1", "https://tickets.example.com#fragment", "https:///"} {
		if safeURL(value) {
			t.Fatalf("invalid origin %q accepted", value)
		}
	}
}

func TestDeliverRecipientResponses(t *testing.T) {
	for _, tc := range []struct {
		rcpt, confirmation int
		success            bool
	}{
		{250, 250, true}, {251, 250, true}, {252, 250, true}, {450, 250, false}, {550, 250, false}, {251, 451, false},
	} {
		t.Run(fmt.Sprintf("rcpt_%d_confirmation_%d", tc.rcpt, tc.confirmation), func(t *testing.T) {
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			defer listener.Close()
			result := make(chan error, 1)
			go func() {
				result <- func() error {
					conn, err := listener.Accept()
					if err != nil {
						return err
					}
					defer conn.Close()
					if err := conn.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
						return err
					}
					server := textproto.NewConn(conn)
					if err := server.PrintfLine("220 test SMTP"); err != nil {
						return err
					}
					for _, command := range []struct {
						want string
						code int
					}{
						{"EHLO ticket-online", 250}, {"MAIL FROM:<tickets@example.com>", 250}, {"RCPT TO:<buyer@example.com>", tc.rcpt},
					} {
						line, err := server.ReadLine()
						if err != nil {
							return err
						}
						if line != command.want {
							return fmt.Errorf("command = %q, want %q", line, command.want)
						}
						if err := server.PrintfLine("%d test response", command.code); err != nil {
							return err
						}
					}
					line, err := server.ReadLine()
					if tc.rcpt >= 400 {
						if err != io.EOF {
							return fmt.Errorf("rejected recipient continued: %q, %v", line, err)
						}
						return nil
					}
					if err != nil {
						return err
					}
					if line != "DATA" {
						return fmt.Errorf("command = %q, want DATA", line)
					}
					if err := server.PrintfLine("354 send message"); err != nil {
						return err
					}
					body, err := io.ReadAll(server.DotReader())
					if err != nil {
						return err
					}
					if string(body) != "Subject: test\n\nTicket message\n" {
						return fmt.Errorf("unexpected message %q", body)
					}
					return server.PrintfLine("%d message response", tc.confirmation)
				}()
			}()
			s := &Service{config: Config{Host: "127.0.0.1", Port: listener.Addr().(*net.TCPAddr).Port, TLSMode: "none"}}
			err = s.deliver(context.Background(), []byte("Subject: test\r\n\r\nTicket message\r\n"), "tickets@example.com", "buyer@example.com")
			if (err == nil) != tc.success {
				t.Errorf("deliver error = %v, want success %v", err, tc.success)
			}
			if err := <-result; err != nil {
				t.Fatal(err)
			}
		})
	}
}
