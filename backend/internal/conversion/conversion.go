package conversion

import (
	"context"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/Adeqq1/ticket-online/backend/internal/staffauth"
)

var jakarta = time.FixedZone("Asia/Jakarta", 7*60*60)

type Service struct {
	db    *sql.DB
	staff *staffauth.Service
}

func New(db *sql.DB, staff *staffauth.Service) *Service { return &Service{db: db, staff: staff} }

func validID(value string) bool {
	if len(value) != 32 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

type eventRequest struct {
	JourneyID string `json:"journeyId"`
	EventID   string `json:"eventId"`
	Device    string `json:"device"`
	Kind      string `json:"kind"`
	Reason    string `json:"reason"`
}

func (s *Service) Register(mux *http.ServeMux, logger *slog.Logger) {
	mux.HandleFunc("POST /api/v1/conversion/events", s.record)
	mux.HandleFunc("GET /api/v1/admin/reports/conversion", s.report(logger))
}

func (s *Service) record(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2048))
	decoder.DisallowUnknownFields()
	var input eventRequest
	if decoder.Decode(&input) != nil || !validID(input.JourneyID) || len(input.EventID) == 0 || len(input.EventID) > 64 {
		writeJSON(w, http.StatusBadRequest, `{"error":{"code":"INVALID_REQUEST","message":"Data aktivitas tidak valid"}}`)
		return
	}
	var extra any
	if !errors.Is(decoder.Decode(&extra), io.EOF) {
		writeJSON(w, http.StatusBadRequest, `{"error":{"code":"INVALID_REQUEST","message":"Data aktivitas tidak valid"}}`)
		return
	}
	if input.Device != "mobile" && input.Device != "desktop" && input.Device != "unknown" {
		writeJSON(w, http.StatusBadRequest, `{"error":{"code":"INVALID_REQUEST","message":"Perangkat tidak valid"}}`)
		return
	}
	if input.Kind != "DETAIL_VIEWED" && input.Kind != "CHECKOUT_STARTED" && input.Kind != "VALIDATION_FAILED" && input.Kind != "SERVICE_FAILURE" {
		writeJSON(w, http.StatusBadRequest, `{"error":{"code":"INVALID_REQUEST","message":"Aktivitas tidak valid"}}`)
		return
	}
	if input.Reason != "" && input.Reason != "BUYER_DATA" && input.Reason != "ATTENDEE_DATA" && input.Reason != "RESERVATION" && input.Reason != "PAYMENT_PROVIDER" && input.Reason != "SERVICE" {
		writeJSON(w, http.StatusBadRequest, `{"error":{"code":"INVALID_REQUEST","message":"Kategori tidak valid"}}`)
		return
	}
	validReason := input.Kind == "DETAIL_VIEWED" || input.Kind == "CHECKOUT_STARTED"
	validReason = validReason && input.Reason == "" || input.Kind == "VALIDATION_FAILED" && (input.Reason == "BUYER_DATA" || input.Reason == "ATTENDEE_DATA") || input.Kind == "SERVICE_FAILURE" && input.Reason == "SERVICE"
	if !validReason {
		writeJSON(w, http.StatusBadRequest, `{"error":{"code":"INVALID_REQUEST","message":"Kategori tidak valid"}}`)
		return
	}
	if _, err := s.db.ExecContext(r.Context(), `INSERT INTO conversion_journeys (id,event_id,device,started_at,detail_viewed_at)
		SELECT ?,id,?,UTC_TIMESTAMP(6),IF(?,UTC_TIMESTAMP(6),NULL) FROM events WHERE id=? AND publication_status='PUBLISHED'
		ON DUPLICATE KEY UPDATE id=VALUES(id)`, input.JourneyID, input.Device, input.Kind == "DETAIL_VIEWED", input.EventID); err != nil {
		writeJSON(w, 503, `{"error":{"code":"SERVICE_UNAVAILABLE","message":"Pencatatan belum tersedia"}}`)
		return
	}
	var eventID string
	if err := s.db.QueryRowContext(r.Context(), "SELECT event_id FROM conversion_journeys WHERE id=?", input.JourneyID).Scan(&eventID); err != nil || eventID != input.EventID {
		writeJSON(w, 400, `{"error":{"code":"INVALID_REQUEST","message":"Perjalanan tidak valid"}}`)
		return
	}
	if input.Kind == "DETAIL_VIEWED" {
		_, _ = s.db.ExecContext(r.Context(), "UPDATE conversion_journeys SET detail_viewed_at=COALESCE(detail_viewed_at,UTC_TIMESTAMP(6)) WHERE id=?", input.JourneyID)
	} else {
		_, _ = s.db.ExecContext(r.Context(), "INSERT IGNORE INTO conversion_events (journey_id,kind,reason,created_at) VALUES (?,?,?,UTC_TIMESTAMP(6))", input.JourneyID, input.Kind, input.Reason)
	}
	w.WriteHeader(http.StatusNoContent)
}

func writeJSON(w http.ResponseWriter, status int, body string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(body))
}

// LinkReservation is best effort so analytics never changes a successful reservation.
func LinkReservation(ctx context.Context, db *sql.DB, journeyID, eventID, reservationID string) {
	if !validID(journeyID) || !validID(reservationID) {
		return
	}
	_, _ = db.ExecContext(ctx, `INSERT IGNORE INTO conversion_reservations (reservation_id,journey_id,created_at)
		SELECT ?,j.id,UTC_TIMESTAMP(6) FROM conversion_journeys j JOIN reservations r ON r.id=? AND r.event_id=j.event_id WHERE j.id=? AND j.event_id=?`, reservationID, reservationID, journeyID, eventID)
}

func Record(ctx context.Context, db *sql.DB, journeyID, kind, reason string) {
	if !validID(journeyID) {
		return
	}
	_, _ = db.ExecContext(ctx, `INSERT IGNORE INTO conversion_events (journey_id,kind,reason,created_at)
		SELECT id,?,?,UTC_TIMESTAMP(6) FROM conversion_journeys WHERE id=?`, kind, reason, journeyID)
}

func RecordOrder(ctx context.Context, db *sql.DB, orderID, kind, reason string) {
	if !validID(orderID) {
		return
	}
	_, _ = db.ExecContext(ctx, `INSERT IGNORE INTO conversion_events (journey_id,kind,reason,created_at)
		SELECT cr.journey_id,?,?,UTC_TIMESTAMP(6) FROM orders o JOIN conversion_reservations cr ON cr.reservation_id=o.reservation_id WHERE o.id=?`, kind, reason, orderID)
}

type Filter struct{ EventID, Device, DateFrom, DateTo string }
type Stage struct {
	Detail           int64 `json:"detail"`
	Reservation      int64 `json:"reservation"`
	Order            int64 `json:"order"`
	PaymentStarted   int64 `json:"paymentStarted"`
	PaymentSucceeded int64 `json:"paymentSucceeded"`
}
type Breakdown struct {
	EventID            string           `json:"eventId,omitempty"`
	EventName          string           `json:"eventName,omitempty"`
	Device             string           `json:"device,omitempty"`
	Total              Stage            `json:"total"`
	Matured            Stage            `json:"matured"`
	PendingObservation int64            `json:"pendingObservation"`
	Lost               map[string]int64 `json:"lost"`
}
type Blocker struct {
	Kind   string `json:"kind"`
	Reason string `json:"reason"`
	Count  int64  `json:"count"`
}
type Report struct {
	Period struct {
		EventID          string `json:"eventId"`
		Device           string `json:"device"`
		DateFrom         string `json:"dateFrom"`
		DateTo           string `json:"dateTo"`
		TimeZone         string `json:"timeZone"`
		ObservationHours int    `json:"observationHours"`
	} `json:"period"`
	Summary                  Breakdown   `json:"summary"`
	ByEvent                  []Breakdown `json:"byEvent"`
	ByDevice                 []Breakdown `json:"byDevice"`
	Blockers                 []Blocker   `json:"blockers"`
	UnattributedReservations *int64      `json:"unattributedReservations"`
	UnattributedPayments     *int64      `json:"unattributedPayments"`
	DataUpdatedAt            time.Time   `json:"dataUpdatedAt"`
}

func reportFilter(q Filter, now time.Time) (time.Time, time.Time, string, string, error) {
	to := q.DateTo
	from := q.DateFrom
	if to == "" {
		to = now.In(jakarta).Format("2006-01-02")
	}
	end, err := time.ParseInLocation("2006-01-02", to, jakarta)
	if err != nil {
		return time.Time{}, time.Time{}, "", "", err
	}
	end = end.AddDate(0, 0, 1)
	if from == "" {
		from = end.AddDate(0, 0, -30).Format("2006-01-02")
	}
	start, err := time.ParseInLocation("2006-01-02", from, jakarta)
	if err != nil || !start.Before(end) || end.Sub(start) > 366*24*time.Hour {
		return time.Time{}, time.Time{}, "", "", errors.New("invalid period")
	}
	if strings.TrimSpace(q.EventID) != q.EventID || len(q.EventID) > 64 || q.Device != "" && q.Device != "mobile" && q.Device != "desktop" && q.Device != "unknown" {
		return time.Time{}, time.Time{}, "", "", errors.New("invalid filter")
	}
	return start.UTC(), end.UTC(), from, to, nil
}

func stageRows(ctx context.Context, tx *sql.Tx, start, end time.Time, eventID, device string, dimension string) ([]Breakdown, error) {
	selectDim := "'' AS dim_id,'' AS dim_name,'all' AS dim_device"
	group := ""
	if dimension == "event" {
		selectDim = "e.id AS dim_id,e.artist AS dim_name,'all' AS dim_device"
		group = ",e.id,e.artist"
	}
	if dimension == "device" {
		selectDim = "'' AS dim_id,'' AS dim_name,j.device AS dim_device"
		group = ",j.device"
	}
	query := `SELECT ` + selectDim + `,COUNT(DISTINCT j.id),
	COUNT(DISTINCT IF(j.detail_viewed_at <= j.started_at+INTERVAL 24 HOUR,j.id,NULL)),
	COUNT(DISTINCT IF(rs.created_at <= j.started_at+INTERVAL 24 HOUR,j.id,NULL)),
	COUNT(DISTINCT IF(o.created_at <= j.started_at+INTERVAL 24 HOUR,j.id,NULL)),
	COUNT(DISTINCT IF(p.started_at <= j.started_at+INTERVAL 24 HOUR,j.id,NULL)),
	COUNT(DISTINCT IF(p.status='SUCCEEDED' AND p.paid_at <= j.started_at+INTERVAL 24 HOUR,j.id,NULL)),
	COUNT(DISTINCT IF(j.started_at <= UTC_TIMESTAMP(6)-INTERVAL 24 HOUR,j.id,NULL)),
	COUNT(DISTINCT IF(j.started_at <= UTC_TIMESTAMP(6)-INTERVAL 24 HOUR AND j.detail_viewed_at <= j.started_at+INTERVAL 24 HOUR,j.id,NULL)),
	COUNT(DISTINCT IF(j.started_at <= UTC_TIMESTAMP(6)-INTERVAL 24 HOUR AND rs.created_at <= j.started_at+INTERVAL 24 HOUR,j.id,NULL)),
	COUNT(DISTINCT IF(j.started_at <= UTC_TIMESTAMP(6)-INTERVAL 24 HOUR AND o.created_at <= j.started_at+INTERVAL 24 HOUR,j.id,NULL)),
	COUNT(DISTINCT IF(j.started_at <= UTC_TIMESTAMP(6)-INTERVAL 24 HOUR AND p.started_at <= j.started_at+INTERVAL 24 HOUR,j.id,NULL)),
	COUNT(DISTINCT IF(j.started_at <= UTC_TIMESTAMP(6)-INTERVAL 24 HOUR AND p.status='SUCCEEDED' AND p.paid_at <= j.started_at+INTERVAL 24 HOUR,j.id,NULL))
	FROM conversion_journeys j JOIN events e ON e.id=j.event_id
	LEFT JOIN conversion_reservations cr ON cr.journey_id=j.id
	LEFT JOIN reservations rs ON rs.id=cr.reservation_id
	LEFT JOIN orders o ON o.reservation_id=cr.reservation_id
	LEFT JOIN payments p ON p.order_id=o.id
	WHERE j.started_at>=? AND j.started_at<? AND j.detail_viewed_at IS NOT NULL AND (?='' OR j.event_id=?) AND (?='' OR j.device=?)
	GROUP BY 1,2,3` + group
	rows, err := tx.QueryContext(ctx, query, start, end, eventID, eventID, device, device)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Breakdown, 0)
	for rows.Next() {
		var b Breakdown
		var n, detail, res, orders, pay, payed, matured int64
		if err := rows.Scan(&b.EventID, &b.EventName, &b.Device, &n, &detail, &res, &orders, &pay, &payed, &matured, &b.Matured.Detail, &b.Matured.Reservation, &b.Matured.Order, &b.Matured.PaymentStarted, &b.Matured.PaymentSucceeded); err != nil {
			return nil, err
		}
		b.Total = Stage{detail, res, orders, pay, payed}
		b.PendingObservation = n - matured
		b.Lost = map[string]int64{"detailToReservation": maxZero(b.Matured.Detail - b.Matured.Reservation), "reservationToOrder": maxZero(b.Matured.Reservation - b.Matured.Order), "orderToPayment": maxZero(b.Matured.Order - b.Matured.PaymentStarted), "paymentToSuccess": maxZero(b.Matured.PaymentStarted - b.Matured.PaymentSucceeded)}
		out = append(out, b)
	}
	return out, rows.Err()
}
func maxZero(v int64) int64 {
	if v < 0 {
		return 0
	}
	return v
}
func sum(rows []Breakdown) Breakdown {
	var b Breakdown
	b.Lost = map[string]int64{}
	for _, r := range rows {
		b.Total.Detail += r.Total.Detail
		b.Total.Reservation += r.Total.Reservation
		b.Total.Order += r.Total.Order
		b.Total.PaymentStarted += r.Total.PaymentStarted
		b.Total.PaymentSucceeded += r.Total.PaymentSucceeded
		b.Matured.Detail += r.Matured.Detail
		b.Matured.Reservation += r.Matured.Reservation
		b.Matured.Order += r.Matured.Order
		b.Matured.PaymentStarted += r.Matured.PaymentStarted
		b.Matured.PaymentSucceeded += r.Matured.PaymentSucceeded
		b.PendingObservation += r.PendingObservation
		for k, v := range r.Lost {
			b.Lost[k] += v
		}
	}
	return b
}

func (s *Service) report(logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		q := Filter{EventID: r.URL.Query().Get("eventId"), Device: r.URL.Query().Get("device"), DateFrom: r.URL.Query().Get("dateFrom"), DateTo: r.URL.Query().Get("dateTo")}
		start, end, from, to, err := reportFilter(q, time.Now())
		if err != nil {
			writeJSON(w, 400, `{"error":{"code":"INVALID_REQUEST","message":"Filter laporan tidak valid"}}`)
			return
		}
		tx, err := s.db.BeginTx(r.Context(), &sql.TxOptions{Isolation: sql.LevelRepeatableRead})
		if err != nil {
			logger.Error("conversion report transaction", "error", err)
			writeJSON(w, 500, `{"error":{"code":"INTERNAL_ERROR","message":"Laporan tidak tersedia"}}`)
			return
		}
		defer tx.Rollback()
		token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		principal, err := s.staff.AuthenticateTx(r.Context(), tx, token)
		if err != nil {
			writeJSON(w, 401, `{"error":{"code":"UNAUTHORIZED","message":"Sesi tidak valid"}}`)
			return
		}
		if principal.Staff.Role != "ADMIN" {
			writeJSON(w, 403, `{"error":{"code":"FORBIDDEN","message":"Akses admin diperlukan"}}`)
			return
		}
		var out Report
		out.Period.EventID = q.EventID
		out.Period.Device = q.Device
		out.Period.DateFrom = from
		out.Period.DateTo = to
		out.Period.TimeZone = "Asia/Jakarta"
		out.Period.ObservationHours = 24
		all, err := stageRows(r.Context(), tx, start, end, q.EventID, q.Device, "")
		if err != nil {
			logger.Error("conversion report", "error", err)
			writeJSON(w, 500, `{"error":{"code":"INTERNAL_ERROR","message":"Laporan tidak tersedia"}}`)
			return
		}
		out.Summary = sum(all)
		out.ByEvent, err = stageRows(r.Context(), tx, start, end, q.EventID, q.Device, "event")
		if err != nil {
			writeJSON(w, 500, `{"error":{"code":"INTERNAL_ERROR","message":"Laporan tidak tersedia"}}`)
			return
		}
		out.ByDevice, err = stageRows(r.Context(), tx, start, end, q.EventID, q.Device, "device")
		if err != nil {
			writeJSON(w, 500, `{"error":{"code":"INTERNAL_ERROR","message":"Laporan tidak tersedia"}}`)
			return
		}
		blockRows, err := tx.QueryContext(r.Context(), `SELECT ce.kind,ce.reason,COUNT(*) FROM conversion_events ce JOIN conversion_journeys j ON j.id=ce.journey_id WHERE j.started_at>=? AND j.started_at<? AND (?='' OR j.event_id=?) AND (?='' OR j.device=?) GROUP BY ce.kind,ce.reason ORDER BY COUNT(*) DESC`, start, end, q.EventID, q.EventID, q.Device, q.Device)
		if err != nil {
			writeJSON(w, 500, `{"error":{"code":"INTERNAL_ERROR","message":"Laporan tidak tersedia"}}`)
			return
		}
		out.Blockers = make([]Blocker, 0)
		for blockRows.Next() {
			var b Blocker
			if blockRows.Scan(&b.Kind, &b.Reason, &b.Count) != nil {
				blockRows.Close()
				writeJSON(w, 500, `{"error":{"code":"INTERNAL_ERROR","message":"Laporan tidak tersedia"}}`)
				return
			}
			out.Blockers = append(out.Blockers, b)
		}
		blockRows.Close()
		var expired int64
		if err := tx.QueryRowContext(r.Context(), `SELECT COUNT(DISTINCT j.id) FROM conversion_journeys j JOIN conversion_reservations cr ON cr.journey_id=j.id JOIN reservations rs ON rs.id=cr.reservation_id WHERE j.started_at>=? AND j.started_at<? AND j.detail_viewed_at IS NOT NULL AND rs.status='EXPIRED' AND (?='' OR j.event_id=?) AND (?='' OR j.device=?)`, start, end, q.EventID, q.EventID, q.Device, q.Device).Scan(&expired); err != nil {
			writeJSON(w, 500, `{"error":{"code":"INTERNAL_ERROR","message":"Laporan tidak tersedia"}}`)
			return
		}
		if expired > 0 {
			found := false
			for i := range out.Blockers {
				if out.Blockers[i].Kind == "RESERVATION_EXPIRED" && out.Blockers[i].Reason == "RESERVATION" {
					if expired > out.Blockers[i].Count {
						out.Blockers[i].Count = expired
					}
					found = true
					break
				}
			}
			if !found {
				out.Blockers = append(out.Blockers, Blocker{Kind: "RESERVATION_EXPIRED", Reason: "RESERVATION", Count: expired})
			}
		}
		if err := tx.QueryRowContext(r.Context(), "SELECT UTC_TIMESTAMP(6)").Scan(&out.DataUpdatedAt); err != nil {
			writeJSON(w, 500, `{"error":{"code":"INTERNAL_ERROR","message":"Laporan tidak tersedia"}}`)
			return
		}
		out.DataUpdatedAt = out.DataUpdatedAt.UTC()
		if q.Device == "" {
			var reservations, payments int64
			if err := tx.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM reservations r LEFT JOIN conversion_reservations cr ON cr.reservation_id=r.id WHERE r.created_at>=? AND r.created_at<? AND cr.reservation_id IS NULL AND (?='' OR r.event_id=?)`, start, end, q.EventID, q.EventID).Scan(&reservations); err != nil {
				writeJSON(w, 500, `{"error":{"code":"INTERNAL_ERROR","message":"Laporan tidak tersedia"}}`)
				return
			}
			if err := tx.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM payments p JOIN orders o ON o.id=p.order_id JOIN reservations r ON r.id=o.reservation_id LEFT JOIN conversion_reservations cr ON cr.reservation_id=r.id WHERE p.created_at>=? AND p.created_at<? AND cr.reservation_id IS NULL AND (?='' OR r.event_id=?)`, start, end, q.EventID, q.EventID).Scan(&payments); err != nil {
				writeJSON(w, 500, `{"error":{"code":"INTERNAL_ERROR","message":"Laporan tidak tersedia"}}`)
				return
			}
			out.UnattributedReservations, out.UnattributedPayments = &reservations, &payments
		}
		if err := json.NewEncoder(w).Encode(out); err != nil {
			logger.Error("encode conversion report", "error", err)
		}
	}
}

func (s *Service) Record(ctx context.Context, id, kind, reason string) {
	Record(ctx, s.db, id, kind, reason)
}
func (s *Service) Link(ctx context.Context, id, eventID, reservationID string) {
	LinkReservation(ctx, s.db, id, eventID, reservationID)
}
