package adminreports

import (
	"bytes"
	"database/sql"
	"encoding/csv"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/Adeqq1/ticket-online/backend/internal/orderaccess"
	"github.com/Adeqq1/ticket-online/backend/internal/staffauth"
)

func (h *Handler) salesCSV(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	token, ok := orderaccess.Bearer(r.Header.Get("Authorization"))
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": map[string]string{"code": "UNAUTHORIZED", "message": "Autentikasi administrator diperlukan"}})
		return
	}
	query, err := reportQuery(r, "|eventId|dateFrom|dateTo|", false)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]string{"code": "INVALID_REQUEST", "message": "Filter laporan tidak valid"}})
		return
	}
	report, err := h.service.Sales(r, token, Filter{EventID: query.Get("eventId"), DateFrom: query.Get("dateFrom"), DateTo: query.Get("dateTo")})
	if err != nil {
		h.csvError(w, r, err, "admin sales report CSV failed", "Laporan penjualan belum dapat diekspor")
		return
	}
	body, err := salesCSV(report)
	if err != nil {
		h.logger.ErrorContext(r.Context(), "build admin sales report CSV failed", "request_id", r.Header.Get("X-Request-ID"), "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": map[string]string{"code": "INTERNAL_ERROR", "message": "Laporan penjualan belum dapat diekspor"}})
		return
	}
	filename := fmt.Sprintf("admin-sales-%s-%s.csv", report.Period.DateFrom, report.Period.DateTo)
	writeCSV(w, filename, body)
}

func (h *Handler) attendanceCSV(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	token, ok := orderaccess.Bearer(r.Header.Get("Authorization"))
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": map[string]string{"code": "UNAUTHORIZED", "message": "Autentikasi administrator diperlukan"}})
		return
	}
	query, err := reportQuery(r, "|eventId|gate|", true)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]string{"code": "INVALID_REQUEST", "message": "Filter laporan kehadiran tidak valid"}})
		return
	}
	report, err := h.service.Attendance(r, token, AttendanceFilter{EventID: query.Get("eventId"), Gate: query.Get("gate")})
	if err != nil {
		h.csvError(w, r, err, "admin attendance report CSV failed", "Laporan kehadiran belum dapat diekspor")
		return
	}
	body, err := attendanceCSV(report)
	if err != nil {
		h.logger.ErrorContext(r.Context(), "build admin attendance report CSV failed", "request_id", r.Header.Get("X-Request-ID"), "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": map[string]string{"code": "INTERNAL_ERROR", "message": "Laporan kehadiran belum dapat diekspor"}})
		return
	}
	filename := fmt.Sprintf("admin-attendance-%s.csv", report.DataUpdatedAt.In(wib).Format("2006-01-02"))
	writeCSV(w, filename, body)
}

func (h *Handler) csvError(w http.ResponseWriter, r *http.Request, err error, logMessage, internalMessage string) {
	switch {
	case errors.Is(err, staffauth.ErrUnauthorized):
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": map[string]string{"code": "UNAUTHORIZED", "message": "Sesi administrator tidak valid atau sudah berakhir"}})
	case errors.Is(err, staffauth.ErrForbidden):
		writeJSON(w, http.StatusForbidden, map[string]any{"error": map[string]string{"code": "FORBIDDEN", "message": "Laporan hanya tersedia untuk admin"}})
	case errors.Is(err, ErrInvalidRequest):
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]string{"code": "INVALID_REQUEST", "message": "Filter laporan tidak valid"}})
	case errors.Is(err, sql.ErrNoRows):
		writeJSON(w, http.StatusNotFound, map[string]any{"error": map[string]string{"code": "EVENT_NOT_FOUND", "message": "Event tidak ditemukan"}})
	default:
		h.logger.ErrorContext(r.Context(), logMessage, "request_id", r.Header.Get("X-Request-ID"), "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": map[string]string{"code": "INTERNAL_ERROR", "message": internalMessage}})
	}
}

func salesCSV(report Report) ([]byte, error) {
	var output bytes.Buffer
	output.Write([]byte{0xef, 0xbb, 0xbf})
	w := csv.NewWriter(&output)
	header := []string{"jenis_laporan", "periode_dari", "periode_sampai", "zona_waktu", "filter_event_id", "filter_event", "filter_gate", "waktu_pembuatan", "jenis_baris", "tanggal", "event_id", "event", "transaksi_berhasil", "pembayaran_rp", "refund_rp", "penerimaan_setelah_refund_rp", "refund_belum_selesai", "rekonsiliasi_terbuka"}
	if err := w.Write(header); err != nil {
		return nil, err
	}
	var filterEventID, filterEventName string
	if report.Period.EventID != nil {
		filterEventID = *report.Period.EventID
		for _, event := range report.FilterOptions.Events {
			if event.ID == filterEventID {
				filterEventName = event.Name
				break
			}
		}
	}
	metadata := func(kind string) []string {
		return []string{textCSV(kind), textCSV(report.Period.DateFrom), textCSV(report.Period.DateTo), textCSV(report.Period.TimeZone), textCSV(filterEventID), textCSV(filterEventName), "", textCSV(report.DataUpdatedAt.In(wib).Format(time.RFC3339Nano))}
	}
	write := func(kind, date, eventID, eventName string, amounts Amounts, current bool) error {
		row := append(metadata("penjualan"), textCSV(kind), textCSV(date), textCSV(eventID), textCSV(eventName), strconv.FormatInt(amounts.SuccessfulTransactions, 10), strconv.FormatInt(amounts.PaymentAmount, 10), strconv.FormatInt(amounts.RefundAmount, 10), strconv.FormatInt(amounts.NetAmount, 10))
		if current {
			row = append(row, strconv.FormatInt(amounts.UnfinishedRefunds, 10), strconv.FormatInt(amounts.OpenReconciliationCases, 10))
		} else {
			row = append(row, "", "")
		}
		return w.Write(row)
	}
	if err := write("ringkasan", "", "", "", report.Summary, true); err != nil {
		return nil, err
	}
	for _, day := range report.Daily {
		if err := write("harian", day.Date, "", "", day.Amounts, false); err != nil {
			return nil, err
		}
	}
	for _, event := range report.ByEvent {
		if err := write("event", "", event.ID, event.Name, event.Amounts, true); err != nil {
			return nil, err
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

func attendanceCSV(report AttendanceReport) ([]byte, error) {
	var output bytes.Buffer
	output.Write([]byte{0xef, 0xbb, 0xbf})
	w := csv.NewWriter(&output)
	header := []string{"jenis_laporan", "periode_dari", "periode_sampai", "zona_waktu", "filter_event_id", "filter_event", "filter_gate", "waktu_pembuatan", "jenis_baris", "jam", "kategori_id", "kategori", "gate", "kapasitas", "stok_tersedia", "tiket_diterbitkan", "tiket_berhak_masuk", "tiket_tertahan_refund", "tiket_checkin", "tingkat_kehadiran_persen"}
	if err := w.Write(header); err != nil {
		return nil, err
	}
	filterGate := ""
	if report.Gate != nil {
		filterGate = *report.Gate
	}
	metadata := func(kind string) []string {
		snapshotDate := report.DataUpdatedAt.In(wib).Format("2006-01-02")
		return []string{textCSV(kind), textCSV(snapshotDate), textCSV(snapshotDate), textCSV(report.TimeZone), textCSV(report.Event.ID), textCSV(report.Event.Name), textCSV(filterGate), textCSV(report.DataUpdatedAt.In(wib).Format(time.RFC3339Nano))}
	}
	write := func(kind, hour string, category AttendanceCategory, gate *string, amounts AttendanceAmounts) error {
		categoryID, categoryName, gateName := "", "", ""
		if category.ID != 0 {
			categoryID, categoryName = strconv.FormatUint(category.ID, 10), textCSV(category.Name)
		}
		if gate != nil {
			gateName = textCSV(*gate)
		}
		rate := ""
		if amounts.AttendanceRate != nil {
			rate = strconv.FormatFloat(*amounts.AttendanceRate, 'f', -1, 64)
		}
		return w.Write(append(metadata("kehadiran_terkini"), textCSV(kind), textCSV(hour), categoryID, categoryName, gateName,
			strconv.FormatInt(amounts.Capacity, 10), strconv.FormatInt(amounts.Available, 10), strconv.FormatInt(amounts.Issued, 10), strconv.FormatInt(amounts.Eligible, 10), strconv.FormatInt(amounts.HeldForRefund, 10), strconv.FormatInt(amounts.CheckedIn, 10), rate))
	}
	if err := write("ringkasan", "", AttendanceCategory{}, nil, report.Summary); err != nil {
		return nil, err
	}
	for _, category := range report.ByCategory {
		if err := write("kategori", "", category, nil, category.AttendanceAmounts); err != nil {
			return nil, err
		}
	}
	for _, gate := range report.ByGate {
		if err := write("gate", "", AttendanceCategory{}, gate.Gate, gate.AttendanceAmounts); err != nil {
			return nil, err
		}
	}
	for _, hour := range report.Hourly {
		if err := write("per_jam", hour.Hour, AttendanceCategory{}, nil, AttendanceAmounts{CheckedIn: hour.CheckedIn}); err != nil {
			return nil, err
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

func textCSV(value string) string {
	for _, r := range value {
		if unicode.IsSpace(r) || r < 0x20 || r == 0x7f || r == '\ufeff' {
			continue
		}
		if strings.ContainsRune("=+-@", r) {
			return "'" + value
		}
		return value
	}
	return value
}

func writeCSV(w http.ResponseWriter, filename string, body []byte) {
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}
