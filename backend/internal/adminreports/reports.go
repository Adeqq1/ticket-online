package adminreports

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Adeqq1/ticket-online/backend/internal/orderaccess"
	"github.com/Adeqq1/ticket-online/backend/internal/staffauth"
)

var ErrInvalidRequest = errors.New("invalid admin sales report request")
var wib = time.FixedZone("Asia/Jakarta", 7*60*60)

type Service struct {
	db    *sql.DB
	staff *staffauth.Service
}

func NewService(db *sql.DB, staff *staffauth.Service) *Service { return &Service{db: db, staff: staff} }

type Handler struct {
	service *Service
	logger  *slog.Logger
}

func NewHandler(service *Service, logger *slog.Logger) *Handler {
	if logger == nil {
		logger = slog.Default()
	}
	return &Handler{service: service, logger: logger}
}

type Filter struct{ EventID, DateFrom, DateTo string }

type Event struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Amounts struct {
	SuccessfulTransactions  int64 `json:"successfulTransactions"`
	PaymentAmount           int64 `json:"paymentAmount"`
	RefundAmount            int64 `json:"refundAmount"`
	NetAmount               int64 `json:"netAmount"`
	UnfinishedRefunds       int64 `json:"unfinishedRefunds"`
	OpenReconciliationCases int64 `json:"openReconciliationCases"`
}

type Day struct {
	Date string `json:"date"`
	Amounts
}

type EventRow struct {
	Event
	Amounts
}

type Report struct {
	Period struct {
		EventID  *string `json:"eventId"`
		DateFrom string  `json:"dateFrom"`
		DateTo   string  `json:"dateTo"`
		TimeZone string  `json:"timeZone"`
	} `json:"period"`
	Summary       Amounts    `json:"summary"`
	Daily         []Day      `json:"daily"`
	ByEvent       []EventRow `json:"byEvent"`
	FilterOptions struct {
		Events []Event `json:"events"`
	} `json:"filterOptions"`
	DataUpdatedAt time.Time `json:"dataUpdatedAt"`
}

type parsedFilter struct {
	EventID          string
	From, To         time.Time
	DateFrom, DateTo string
}

func parseFilter(filter Filter, now time.Time) (parsedFilter, error) {
	if strings.TrimSpace(filter.EventID) != filter.EventID || len(filter.EventID) > 64 {
		return parsedFilter{}, ErrInvalidRequest
	}
	today := now.In(wib)
	fromText, toText := filter.DateFrom, filter.DateTo
	if fromText == "" && toText == "" {
		toText = today.Format("2006-01-02")
		fromText = today.AddDate(0, 0, -29).Format("2006-01-02")
	} else if fromText == "" {
		to, err := parseDate(toText)
		if err != nil {
			return parsedFilter{}, err
		}
		fromText = to.AddDate(0, 0, -29).Format("2006-01-02")
	} else if toText == "" {
		toText = today.Format("2006-01-02")
	}
	from, err := parseDate(fromText)
	if err != nil {
		return parsedFilter{}, err
	}
	to, err := parseDate(toText)
	if err != nil || to.Before(from) || to.Sub(from) >= 366*24*time.Hour {
		return parsedFilter{}, ErrInvalidRequest
	}
	return parsedFilter{EventID: filter.EventID, From: from, To: to.AddDate(0, 0, 1), DateFrom: fromText, DateTo: toText}, nil
}

func parseDate(value string) (time.Time, error) {
	date, err := time.ParseInLocation("2006-01-02", value, wib)
	if err != nil || date.Format("2006-01-02") != value {
		return time.Time{}, ErrInvalidRequest
	}
	return date, nil
}

func (s *Service) Sales(r *http.Request, token string, filter Filter) (Report, error) {
	var result Report
	result.ByEvent = make([]EventRow, 0)
	query, err := parseFilter(filter, time.Now())
	if err != nil {
		return result, err
	}
	tx, err := s.db.BeginTx(r.Context(), &sql.TxOptions{Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return result, fmt.Errorf("begin admin sales report: %w", err)
	}
	defer tx.Rollback()
	principal, err := s.staff.AuthenticateTx(r.Context(), tx, token)
	if err != nil {
		return result, err
	}
	if principal.Role != "ADMIN" {
		return result, staffauth.ErrForbidden
	}
	if err := tx.QueryRowContext(r.Context(), "SELECT UTC_TIMESTAMP(6), COUNT(*) FROM events").Scan(&result.DataUpdatedAt, new(int64)); err != nil {
		return result, fmt.Errorf("read admin sales report snapshot: %w", err)
	}
	result.DataUpdatedAt = result.DataUpdatedAt.UTC()
	result.Period.EventID = nil
	if query.EventID != "" {
		result.Period.EventID = &query.EventID
	}
	result.Period.DateFrom, result.Period.DateTo, result.Period.TimeZone = query.DateFrom, query.DateTo, "Asia/Jakarta"
	result.Daily = make([]Day, 0, int(query.To.Sub(query.From)/(24*time.Hour)))
	days := make(map[string]*Day, cap(result.Daily))
	for date := query.From; date.Before(query.To); date = date.AddDate(0, 0, 1) {
		key := date.Format("2006-01-02")
		result.Daily = append(result.Daily, Day{Date: key})
		days[key] = &result.Daily[len(result.Daily)-1]
	}
	// Aggregate each one-to-one financial source independently before combining them.
	rows, err := tx.QueryContext(r.Context(), `SELECT DATE(DATE_ADD(source.at_time, INTERVAL 7 HOUR)) AS report_date, source.event_id, source.kind,
		COUNT(*) AS transactions, SUM(source.amount) AS amount
		FROM (
			SELECT p.paid_at AS at_time, r.event_id, 'PAYMENT' AS kind, p.amount
			FROM payments p JOIN orders o ON o.id = p.order_id JOIN reservations r ON r.id = o.reservation_id
			WHERE p.status = 'SUCCEEDED' AND p.paid_at >= ? AND p.paid_at < ? AND (? = '' OR r.event_id = ?)
			UNION ALL
			SELECT f.completed_at AS at_time, r.event_id, 'REFUND' AS kind, f.amount
			FROM order_refunds f JOIN orders o ON o.id = f.order_id JOIN reservations r ON r.id = o.reservation_id
			WHERE f.status = 'SUCCEEDED' AND f.completed_at >= ? AND f.completed_at < ? AND (? = '' OR r.event_id = ?)
		) source GROUP BY report_date, source.event_id, source.kind ORDER BY report_date, source.event_id, source.kind`,
		query.From.UTC(), query.To.UTC(), query.EventID, query.EventID,
		query.From.UTC(), query.To.UTC(), query.EventID, query.EventID)
	if err != nil {
		return result, fmt.Errorf("query admin sales report aggregates: %w", err)
	}
	eventAmounts := make(map[string]Amounts)
	for rows.Next() {
		var date time.Time
		var eventID, kind string
		var count, amount int64
		if err := rows.Scan(&date, &eventID, &kind, &count, &amount); err != nil {
			rows.Close()
			return result, fmt.Errorf("scan admin sales report aggregate: %w", err)
		}
		day := days[date.Format("2006-01-02")]
		if day == nil {
			rows.Close()
			return result, fmt.Errorf("admin sales report returned date outside requested range")
		}
		if kind == "PAYMENT" {
			day.SuccessfulTransactions += count
			day.PaymentAmount += amount
			result.Summary.SuccessfulTransactions += count
			result.Summary.PaymentAmount += amount
			current := eventAmounts[eventID]
			current.SuccessfulTransactions += count
			current.PaymentAmount += amount
			eventAmounts[eventID] = current
		} else {
			day.RefundAmount += amount
			result.Summary.RefundAmount += amount
			current := eventAmounts[eventID]
			current.RefundAmount += amount
			eventAmounts[eventID] = current
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return result, fmt.Errorf("iterate admin sales report aggregates: %w", err)
	}
	if err := rows.Close(); err != nil {
		return result, fmt.Errorf("close admin sales report aggregates: %w", err)
	}
	for index := range result.Daily {
		result.Daily[index].NetAmount = result.Daily[index].PaymentAmount - result.Daily[index].RefundAmount
	}
	result.Summary.NetAmount = result.Summary.PaymentAmount - result.Summary.RefundAmount
	result.FilterOptions.Events = make([]Event, 0)
	rows, err = tx.QueryContext(r.Context(), "SELECT id, artist FROM events WHERE (? = '' OR id = ?) ORDER BY artist, id", query.EventID, query.EventID)
	if err != nil {
		return result, fmt.Errorf("query admin sales report events: %w", err)
	}
	for rows.Next() {
		var event Event
		if err := rows.Scan(&event.ID, &event.Name); err != nil {
			rows.Close()
			return result, fmt.Errorf("scan admin sales report event: %w", err)
		}
		result.FilterOptions.Events = append(result.FilterOptions.Events, event)
		amounts := eventAmounts[event.ID]
		amounts.NetAmount = amounts.PaymentAmount - amounts.RefundAmount
		result.ByEvent = append(result.ByEvent, EventRow{Event: event, Amounts: amounts})
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return result, fmt.Errorf("iterate admin sales report events: %w", err)
	}
	if err := rows.Close(); err != nil {
		return result, fmt.Errorf("close admin sales report events: %w", err)
	}
	if query.EventID != "" && len(result.ByEvent) == 0 {
		return result, sql.ErrNoRows
	}
	rows, err = tx.QueryContext(r.Context(), `SELECT event_id, SUM(unfinished_refunds), SUM(open_cases) FROM (
		SELECT r.event_id, COUNT(*) AS unfinished_refunds, 0 AS open_cases
		FROM order_refunds f JOIN orders o ON o.id=f.order_id JOIN reservations r ON r.id=o.reservation_id
		WHERE f.status IN ('REQUESTED','PROCESSING','UNKNOWN') AND (? = '' OR r.event_id = ?) GROUP BY r.event_id
		UNION ALL
		SELECT r.event_id, 0 AS unfinished_refunds, COUNT(*) AS open_cases
		FROM payment_reconciliation_cases c JOIN orders o ON o.id=c.order_id JOIN reservations r ON r.id=o.reservation_id
		WHERE c.status='OPEN' AND (? = '' OR r.event_id = ?) GROUP BY r.event_id
	) current_cases GROUP BY event_id`, query.EventID, query.EventID, query.EventID, query.EventID)
	if err != nil {
		return result, fmt.Errorf("query admin sales report current cases: %w", err)
	}
	currentByEvent := make(map[string][2]int64)
	for rows.Next() {
		var eventID string
		var counts [2]int64
		if err := rows.Scan(&eventID, &counts[0], &counts[1]); err != nil {
			rows.Close()
			return result, fmt.Errorf("scan admin sales report current cases: %w", err)
		}
		currentByEvent[eventID] = counts
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return result, fmt.Errorf("iterate admin sales report current cases: %w", err)
	}
	if err := rows.Close(); err != nil {
		return result, fmt.Errorf("close admin sales report current cases: %w", err)
	}
	for index := range result.ByEvent {
		counts := currentByEvent[result.ByEvent[index].ID]
		result.ByEvent[index].UnfinishedRefunds = counts[0]
		result.ByEvent[index].OpenReconciliationCases = counts[1]
		result.Summary.UnfinishedRefunds += counts[0]
		result.Summary.OpenReconciliationCases += counts[1]
	}
	if err := tx.Commit(); err != nil {
		return result, fmt.Errorf("commit admin sales report: %w", err)
	}
	return result, nil
}

func reportQuery(r *http.Request, allowed string, requireEvent bool) (url.Values, error) {
	query := r.URL.Query()
	for name, values := range query {
		if !strings.Contains(allowed, "|"+name+"|") || len(values) != 1 {
			return nil, ErrInvalidRequest
		}
	}
	if requireEvent {
		if _, ok := query["eventId"]; !ok || query.Get("eventId") == "" {
			return nil, ErrInvalidRequest
		}
	}
	return query, nil
}

func (h *Handler) sales(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	token, ok := orderaccess.Bearer(r.Header.Get("Authorization"))
	if !ok {
		writeJSON(w, 401, map[string]any{"error": map[string]string{"code": "UNAUTHORIZED", "message": "Autentikasi administrator diperlukan"}})
		return
	}
	query, err := reportQuery(r, "|eventId|dateFrom|dateTo|", false)
	if err != nil {
		writeJSON(w, 400, map[string]any{"error": map[string]string{"code": "INVALID_REQUEST", "message": "Filter laporan tidak valid"}})
		return
	}
	result, err := h.service.Sales(r, token, Filter{EventID: query.Get("eventId"), DateFrom: query.Get("dateFrom"), DateTo: query.Get("dateTo")})
	if err != nil {
		switch {
		case errors.Is(err, staffauth.ErrUnauthorized):
			writeJSON(w, 401, map[string]any{"error": map[string]string{"code": "UNAUTHORIZED", "message": "Sesi administrator tidak valid atau sudah berakhir"}})
		case errors.Is(err, staffauth.ErrForbidden):
			writeJSON(w, 403, map[string]any{"error": map[string]string{"code": "FORBIDDEN", "message": "Laporan hanya tersedia untuk admin"}})
		case errors.Is(err, ErrInvalidRequest):
			writeJSON(w, 400, map[string]any{"error": map[string]string{"code": "INVALID_REQUEST", "message": "Filter laporan tidak valid"}})
		case errors.Is(err, sql.ErrNoRows):
			writeJSON(w, 404, map[string]any{"error": map[string]string{"code": "EVENT_NOT_FOUND", "message": "Event tidak ditemukan"}})
		default:
			h.logger.ErrorContext(r.Context(), "admin sales report failed", "request_id", r.Header.Get("X-Request-ID"), "error", err)
			writeJSON(w, 500, map[string]any{"error": map[string]string{"code": "INTERNAL_ERROR", "message": "Laporan belum dapat dimuat"}})
		}
		return
	}
	writeJSON(w, 200, result)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
