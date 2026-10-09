package email

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"time"

	"github.com/Adeqq1/ticket-online/backend/internal/eventstate"
	"github.com/Adeqq1/ticket-online/backend/internal/orderaccess"
)

func (s *Service) sendEventChange(ctx context.Context, current job) error {
	if !mailbox(current.recipient) {
		return s.finishFailure(ctx, current, "alamat penerima tidak valid")
	}
	var eventID, reference, recipient string
	err := s.db.QueryRowContext(ctx, `SELECT c.event_id,o.reference,b.email FROM email_queue q
 JOIN event_changes c ON c.id=q.event_change_id JOIN orders o ON o.id=q.order_id
 JOIN reservations r ON r.id=o.reservation_id AND r.event_id=c.event_id JOIN order_buyers b ON b.order_id=o.id
 WHERE q.id=? AND q.kind='EVENT_CHANGE'`, current.id).Scan(&eventID, &reference, &recipient)
	if err == sql.ErrNoRows {
		return s.finishFailure(ctx, current, "keputusan acara tidak tersedia")
	}
	if err != nil {
		return s.retry(ctx, current, "keputusan acara belum dapat dibaca")
	}
	if recipient != current.recipient {
		return s.finishFailure(ctx, current, "penerima tidak cocok dengan pesanan")
	}
	state, err := eventstate.Read(ctx, s.db, eventID, false)
	if err != nil {
		return s.retry(ctx, current, "jadwal terbaru belum dapat dibaca")
	}
	labels := map[string]string{"CANCELLED": "Acara dibatalkan", "POSTPONED": "Acara ditunda", "RESCHEDULED": "Jadwal acara berubah"}
	schedule := "Jadwal belum diumumkan."
	if state.StartsAt != nil {
		start, err := time.Parse(time.RFC3339Nano, *state.StartsAt)
		if err != nil {
			return s.finishFailure(ctx, current, "jadwal tidak valid")
		}
		schedule = "Jadwal terbaru: " + start.In(time.FixedZone("WIB", 7*3600)).Format("02 January 2006 15:04") + " WIB."
	}
	if state.Status == "CANCELLED" {
		schedule = "Acara tidak akan berlangsung. Semua order lunas berhak refund penuh."
	}
	rights := "Periksa hak dan status refund pada halaman pesanan."
	if state.RefundDeadline != nil {
		d, _ := time.Parse(time.RFC3339Nano, *state.RefundDeadline)
		rights += " Tenggat permintaan refund: " + d.In(time.FixedZone("WIB", 7*3600)).Format("02 January 2006 15:04") + " WIB."
	}
	token := orderaccess.New(s.db, s.config.AccessSecret).Token(current.orderID)
	link := s.config.FrontendURL + "/pesanan/" + url.PathEscape(current.orderID) + "#access_token=" + url.QueryEscape(token)
	plain := fmt.Sprintf("%s\n\nPesanan %s\n%s\n%s\n%s\n\nInformasi terbaru dan status refund:\n%s", labels[state.Status], reference, state.Announcement, schedule, rights, link)
	html := fmt.Sprintf("<!doctype html><html lang=\"id\"><meta charset=\"utf-8\"><body><h1>%s</h1><p>Pesanan %s</p><p>%s</p><p>%s</p><p>%s</p><p><a href=\"%s\">Buka informasi terbaru dan status refund</a></p></body></html>", escaped(labels[state.Status]), escaped(reference), escaped(state.Announcement), escaped(schedule), escaped(rights), escaped(link))
	body, from, to, err := compose(s.config, current, labels[state.Status]+" · "+reference, plain, html)
	if err != nil {
		return s.retry(ctx, current, err.Error())
	}
	if err := s.deliver(ctx, body, from, to); err != nil {
		return s.retry(ctx, current, err.Error())
	}
	return s.markSent(ctx, current)
}
