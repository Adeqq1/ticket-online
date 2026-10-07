package adminreports

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/Adeqq1/ticket-online/backend/internal/orderaccess"
	"github.com/Adeqq1/ticket-online/backend/internal/staffauth"
)

type AttendanceFilter struct{ EventID, Gate string }

type AttendanceAmounts struct {
	Capacity       int64    `json:"capacity"`
	Available      int64    `json:"available"`
	Issued         int64    `json:"issued"`
	Eligible       int64    `json:"eligible"`
	HeldForRefund  int64    `json:"heldForRefund"`
	CheckedIn      int64    `json:"checkedIn"`
	AttendanceRate *float64 `json:"attendanceRate"`
}

type AttendanceCategory struct {
	ID   uint64 `json:"id"`
	Name string `json:"name"`
	AttendanceAmounts
}

type AttendanceGate struct {
	Gate *string `json:"gate"`
	AttendanceAmounts
}

type HourlyCheckIns struct {
	Hour      string `json:"hour"`
	CheckedIn int64  `json:"checkedIn"`
}

type AttendanceReport struct {
	Event         Event                `json:"event"`
	Gate          *string              `json:"gate"`
	TimeZone      string               `json:"timeZone"`
	Summary       AttendanceAmounts    `json:"summary"`
	ByCategory    []AttendanceCategory `json:"byCategory"`
	ByGate        []AttendanceGate     `json:"byGate"`
	Hourly        []HourlyCheckIns     `json:"hourly"`
	FilterOptions struct {
		Gates []string `json:"gates"`
	} `json:"filterOptions"`
	DataUpdatedAt time.Time `json:"dataUpdatedAt"`
}

type attendanceCategoryState struct {
	row  AttendanceCategory
	gate string
}

type attendanceGateState struct {
	row AttendanceGate
}

func attendanceRate(checkedIn, eligible int64) *float64 {
	if eligible == 0 {
		return nil
	}
	rate := float64(checkedIn) * 100 / float64(eligible)
	return &rate
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/admin/reports/sales", h.sales)
	mux.HandleFunc("GET /api/v1/admin/reports/attendance", h.attendance)
}

func (s *Service) Attendance(r *http.Request, token string, filter AttendanceFilter) (AttendanceReport, error) {
	var result AttendanceReport
	filter.EventID = strings.TrimSpace(filter.EventID)
	if filter.EventID == "" || len(filter.EventID) > 64 || strings.TrimSpace(filter.Gate) != filter.Gate || len(filter.Gate) > 100 {
		return result, ErrInvalidRequest
	}
	tx, err := s.db.BeginTx(r.Context(), &sql.TxOptions{Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return result, fmt.Errorf("begin admin attendance report: %w", err)
	}
	defer tx.Rollback()
	principal, err := s.staff.AuthenticateTx(r.Context(), tx, token)
	if err != nil {
		return result, err
	}
	if principal.Role != "ADMIN" {
		return result, staffauth.ErrForbidden
	}
	var eventCount int64
	if err := tx.QueryRowContext(r.Context(), `SELECT UTC_TIMESTAMP(6), COUNT(*) FROM events WHERE id=?`, filter.EventID).Scan(&result.DataUpdatedAt, &eventCount); err != nil {
		return result, fmt.Errorf("read admin attendance report snapshot: %w", err)
	}
	if eventCount == 0 {
		return result, sql.ErrNoRows
	}
	if err := tx.QueryRowContext(r.Context(), "SELECT id, artist FROM events WHERE id=?", filter.EventID).Scan(&result.Event.ID, &result.Event.Name); err != nil {
		return result, fmt.Errorf("read admin attendance report event: %w", err)
	}
	result.DataUpdatedAt = result.DataUpdatedAt.UTC()
	result.TimeZone = "Asia/Jakarta"
	if filter.Gate != "" {
		result.Gate = &filter.Gate
	}
	result.FilterOptions.Gates = make([]string, 0)
	rows, err := tx.QueryContext(r.Context(), `SELECT gate FROM ticket_tiers WHERE event_id=?
		UNION SELECT DISTINCT NULLIF(JSON_UNQUOTE(JSON_EXTRACT(t.snapshot,'$.gate')),'') FROM etickets t
		JOIN orders o ON o.id=t.order_id JOIN reservations r ON r.id=o.reservation_id
		WHERE r.event_id=? AND JSON_TYPE(JSON_EXTRACT(t.snapshot,'$.gate'))='STRING'
		ORDER BY gate`, filter.EventID, filter.EventID)
	if err != nil {
		return result, fmt.Errorf("query admin attendance gates: %w", err)
	}
	gateSet := make(map[string]struct{})
	for rows.Next() {
		var gate sql.NullString
		if err := rows.Scan(&gate); err != nil {
			rows.Close()
			return result, fmt.Errorf("scan admin attendance gate: %w", err)
		}
		if gate.Valid && gate.String != "" {
			gateSet[gate.String] = struct{}{}
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return result, fmt.Errorf("iterate admin attendance gates: %w", err)
	}
	if err := rows.Close(); err != nil {
		return result, fmt.Errorf("close admin attendance gates: %w", err)
	}
	for gate := range gateSet {
		result.FilterOptions.Gates = append(result.FilterOptions.Gates, gate)
	}
	sort.Strings(result.FilterOptions.Gates)
	if filter.Gate != "" {
		if _, ok := gateSet[filter.Gate]; !ok {
			return result, ErrInvalidRequest
		}
	}

	categories := make(map[uint64]*attendanceCategoryState)
	rows, err = tx.QueryContext(r.Context(), `SELECT id,name,gate,capacity,available_quantity FROM ticket_tiers
		WHERE event_id=? AND (?='' OR BINARY gate=BINARY ?) ORDER BY name,id`, filter.EventID, filter.Gate, filter.Gate)
	if err != nil {
		return result, fmt.Errorf("query admin attendance categories: %w", err)
	}
	for rows.Next() {
		state := &attendanceCategoryState{}
		if err := rows.Scan(&state.row.ID, &state.row.Name, &state.gate, &state.row.Capacity, &state.row.Available); err != nil {
			rows.Close()
			return result, fmt.Errorf("scan admin attendance category: %w", err)
		}
		categories[state.row.ID] = state
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return result, fmt.Errorf("iterate admin attendance categories: %w", err)
	}
	if err := rows.Close(); err != nil {
		return result, fmt.Errorf("close admin attendance categories: %w", err)
	}

	const validTicketSnapshot = `
		JSON_TYPE(JSON_EXTRACT(t.snapshot,'$.id'))='STRING'
		AND BINARY JSON_UNQUOTE(JSON_EXTRACT(t.snapshot,'$.id'))=BINARY LOWER(t.id)
		AND JSON_TYPE(JSON_EXTRACT(t.snapshot,'$.eventId'))='STRING'
		AND BINARY JSON_UNQUOTE(JSON_EXTRACT(t.snapshot,'$.eventId'))=BINARY r.event_id
		AND JSON_TYPE(JSON_EXTRACT(t.snapshot,'$.gate'))='STRING'
		AND JSON_UNQUOTE(JSON_EXTRACT(t.snapshot,'$.gate'))=TRIM(JSON_UNQUOTE(JSON_EXTRACT(t.snapshot,'$.gate')))
		AND JSON_UNQUOTE(JSON_EXTRACT(t.snapshot,'$.gate')) NOT REGEXP '^[[:space:]]|[[:space:]]$'
		AND OCTET_LENGTH(JSON_UNQUOTE(JSON_EXTRACT(t.snapshot,'$.gate'))) BETWEEN 1 AND 100
		AND COALESCE(JSON_TYPE(JSON_EXTRACT(t.snapshot,'$.code')),'NULL') IN ('STRING','NULL')
		AND COALESCE(JSON_TYPE(JSON_EXTRACT(t.snapshot,'$.attendeeName')),'NULL') IN ('STRING','NULL')
		AND COALESCE(JSON_TYPE(JSON_EXTRACT(t.snapshot,'$.tierName')),'NULL') IN ('STRING','NULL')`
	rows, err = tx.QueryContext(r.Context(), `SELECT t.ticket_tier_id, COALESCE(JSON_UNQUOTE(JSON_EXTRACT(t.snapshot,'$.gate')),''),
		COALESCE(tt.name,'Kategori tidak diketahui'), COALESCE(tt.gate,''),
		COUNT(*) AS issued, COALESCE(SUM(o.status='PAID' AND (`+validTicketSnapshot+`)),0) AS eligible,
		SUM(o.status='REFUND_PENDING'
			AND (?='' OR BINARY JSON_UNQUOTE(JSON_EXTRACT(t.snapshot,'$.gate'))=BINARY ?)) AS held,
		SUM(c.ticket_id IS NOT NULL) AS checked_in
		FROM etickets t JOIN orders o ON o.id=t.order_id JOIN reservations r ON r.id=o.reservation_id
		LEFT JOIN ticket_tiers tt ON tt.id=t.ticket_tier_id AND tt.event_id=r.event_id
		LEFT JOIN ticket_checkins c ON c.ticket_id=t.id
		WHERE r.event_id=? AND (?='' OR (JSON_TYPE(JSON_EXTRACT(t.snapshot,'$.gate'))='STRING'
			AND BINARY JSON_UNQUOTE(JSON_EXTRACT(t.snapshot,'$.gate'))=BINARY ?))
		GROUP BY t.ticket_tier_id,ticket_gate,tt.name,tt.gate ORDER BY t.ticket_tier_id,ticket_gate`,
		filter.Gate, filter.Gate, filter.EventID, filter.Gate, filter.Gate)
	if err != nil {
		return result, fmt.Errorf("query admin attendance ticket aggregates: %w", err)
	}
	gates := make(map[string]*attendanceGateState)
	for rows.Next() {
		var tierID uint64
		var gate, categoryName, tierGate string
		var issued, eligible, held, checkedIn int64
		if err := rows.Scan(&tierID, &gate, &categoryName, &tierGate, &issued, &eligible, &held, &checkedIn); err != nil {
			rows.Close()
			return result, fmt.Errorf("scan admin attendance ticket aggregate: %w", err)
		}
		category := categories[tierID]
		if category == nil {
			category = &attendanceCategoryState{row: AttendanceCategory{ID: tierID, Name: categoryName}, gate: tierGate}
			categories[tierID] = category
		}
		category.row.Issued += issued
		category.row.Eligible += eligible
		category.row.HeldForRefund += held
		category.row.CheckedIn += checkedIn
		key := gate
		gateState := gates[key]
		if gateState == nil {
			var value *string
			if gate != "" {
				value = &gate
			}
			gateState = &attendanceGateState{row: AttendanceGate{Gate: value}}
			gates[key] = gateState
		}
		gateState.row.Issued += issued
		gateState.row.Eligible += eligible
		gateState.row.HeldForRefund += held
		gateState.row.CheckedIn += checkedIn
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return result, fmt.Errorf("iterate admin attendance ticket aggregates: %w", err)
	}
	if err := rows.Close(); err != nil {
		return result, fmt.Errorf("close admin attendance ticket aggregates: %w", err)
	}
	for _, category := range categories {
		gate := gates[category.gate]
		if gate == nil {
			value := category.gate
			gate = &attendanceGateState{row: AttendanceGate{Gate: &value}}
			gates[category.gate] = gate
		}
		gate.row.Capacity += category.row.Capacity
		gate.row.Available += category.row.Available
		category.row.AttendanceRate = attendanceRate(category.row.CheckedIn, category.row.Eligible)
		result.ByCategory = append(result.ByCategory, category.row)
		result.Summary.Capacity += category.row.Capacity
		result.Summary.Available += category.row.Available
		result.Summary.Issued += category.row.Issued
		result.Summary.Eligible += category.row.Eligible
		result.Summary.HeldForRefund += category.row.HeldForRefund
		result.Summary.CheckedIn += category.row.CheckedIn
	}
	result.Summary.AttendanceRate = attendanceRate(result.Summary.CheckedIn, result.Summary.Eligible)
	for _, gate := range gates {
		gate.row.AttendanceRate = attendanceRate(gate.row.CheckedIn, gate.row.Eligible)
		result.ByGate = append(result.ByGate, gate.row)
	}
	sort.Slice(result.ByCategory, func(i, j int) bool {
		if result.ByCategory[i].Name != result.ByCategory[j].Name {
			return result.ByCategory[i].Name < result.ByCategory[j].Name
		}
		return result.ByCategory[i].ID < result.ByCategory[j].ID
	})
	sort.Slice(result.ByGate, func(i, j int) bool {
		if result.ByGate[i].Gate == nil {
			return result.ByGate[j].Gate != nil
		}
		if result.ByGate[j].Gate == nil {
			return false
		}
		return *result.ByGate[i].Gate < *result.ByGate[j].Gate
	})
	result.Hourly = make([]HourlyCheckIns, 0)
	rows, err = tx.QueryContext(r.Context(), `SELECT CONCAT(DATE_FORMAT(DATE_ADD(checked_in_at,INTERVAL 7 HOUR),'%Y-%m-%dT%H:00:00'),'+07:00'), COUNT(*)
		FROM ticket_checkins WHERE event_id=? AND (?='' OR BINARY gate=BINARY ?)
		GROUP BY DATE( DATE_ADD(checked_in_at,INTERVAL 7 HOUR)), HOUR(DATE_ADD(checked_in_at,INTERVAL 7 HOUR))
		ORDER BY DATE(DATE_ADD(checked_in_at,INTERVAL 7 HOUR)), HOUR(DATE_ADD(checked_in_at,INTERVAL 7 HOUR))`,
		filter.EventID, filter.Gate, filter.Gate)
	if err != nil {
		return result, fmt.Errorf("query admin attendance hourly check-ins: %w", err)
	}
	for rows.Next() {
		var hour string
		var count int64
		if err := rows.Scan(&hour, &count); err != nil {
			rows.Close()
			return result, fmt.Errorf("scan admin attendance hourly check-ins: %w", err)
		}
		result.Hourly = append(result.Hourly, HourlyCheckIns{Hour: hour, CheckedIn: count})
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return result, fmt.Errorf("iterate admin attendance hourly check-ins: %w", err)
	}
	if err := rows.Close(); err != nil {
		return result, fmt.Errorf("close admin attendance hourly check-ins: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return result, fmt.Errorf("commit admin attendance report: %w", err)
	}
	return result, nil
}

func (h *Handler) attendance(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	token, ok := orderaccess.Bearer(r.Header.Get("Authorization"))
	if !ok {
		writeJSON(w, 401, map[string]any{"error": map[string]string{"code": "UNAUTHORIZED", "message": "Autentikasi administrator diperlukan"}})
		return
	}
	query := r.URL.Query()
	for name, values := range query {
		if !strings.Contains("|eventId|gate|", "|"+name+"|") || len(values) != 1 {
			writeJSON(w, 400, map[string]any{"error": map[string]string{"code": "INVALID_REQUEST", "message": "Filter laporan kehadiran tidak valid"}})
			return
		}
	}
	if _, ok := query["eventId"]; !ok || query.Get("eventId") == "" {
		writeJSON(w, 400, map[string]any{"error": map[string]string{"code": "INVALID_REQUEST", "message": "Event wajib dipilih"}})
		return
	}
	result, err := h.service.Attendance(r, token, AttendanceFilter{EventID: query.Get("eventId"), Gate: query.Get("gate")})
	if err != nil {
		switch {
		case errors.Is(err, staffauth.ErrUnauthorized):
			writeJSON(w, 401, map[string]any{"error": map[string]string{"code": "UNAUTHORIZED", "message": "Sesi administrator tidak valid atau sudah berakhir"}})
		case errors.Is(err, staffauth.ErrForbidden):
			writeJSON(w, 403, map[string]any{"error": map[string]string{"code": "FORBIDDEN", "message": "Laporan kehadiran hanya tersedia untuk admin"}})
		case errors.Is(err, ErrInvalidRequest):
			writeJSON(w, 400, map[string]any{"error": map[string]string{"code": "INVALID_REQUEST", "message": "Filter laporan kehadiran tidak valid"}})
		case errors.Is(err, sql.ErrNoRows):
			writeJSON(w, 404, map[string]any{"error": map[string]string{"code": "EVENT_NOT_FOUND", "message": "Event tidak ditemukan"}})
		default:
			h.logger.ErrorContext(r.Context(), "admin attendance report failed", "request_id", r.Header.Get("X-Request-ID"), "error", err)
			writeJSON(w, 500, map[string]any{"error": map[string]string{"code": "INTERNAL_ERROR", "message": "Laporan kehadiran belum dapat dimuat"}})
		}
		return
	}
	writeJSON(w, 200, result)
}
