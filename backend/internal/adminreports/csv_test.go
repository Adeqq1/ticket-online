package adminreports

import (
	"encoding/csv"
	"strings"
	"testing"
	"time"
)

func TestTextCSVProtectsSpreadsheetFormulasAfterLeadingWhitespace(t *testing.T) {
	for _, value := range []string{"=1+1", "  +SUM(A1)", "\t-1+1", "\ufeff@cmd"} {
		if got := textCSV(value); !strings.HasPrefix(got, "'") {
			t.Errorf("textCSV(%q) = %q, want apostrophe prefix", value, got)
		}
	}
	if got := textCSV("normal, text"); got != "normal, text" {
		t.Fatalf("textCSV(normal text) = %q", got)
	}
}

func TestSalesCSVKeepsNegativeAmountsNumericAndIncludesMetadata(t *testing.T) {
	var report Report
	report.Period.DateFrom, report.Period.DateTo, report.Period.TimeZone = "2026-09-01", "2026-09-30", "Asia/Jakarta"
	report.DataUpdatedAt = time.Date(2026, 10, 8, 1, 2, 3, 0, time.UTC)
	report.Summary.NetAmount = -500
	data, err := salesCSV(report)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(data), "\xef\xbb\xbf") {
		t.Fatal("CSV is missing its UTF-8 BOM")
	}
	reader := csv.NewReader(strings.NewReader(strings.TrimPrefix(string(data), "\xef\xbb\xbf")))
	header, err := reader.Read()
	if err != nil {
		t.Fatal(err)
	}
	row, err := reader.Read()
	if err != nil {
		t.Fatal(err)
	}
	if len(row) != len(header) || row[0] != "penjualan" || row[1] != "2026-09-01" || row[2] != "2026-09-30" || row[3] != "Asia/Jakarta" || row[7] != "2026-10-08T08:02:03+07:00" || row[8] != "ringkasan" || row[15] != "-500" {
		t.Fatalf("sales CSV row = %#v", row)
	}
}

func TestAttendanceCSVIncludesSnapshotDateAndEventMetadata(t *testing.T) {
	var report AttendanceReport
	report.Event = Event{ID: "evt-1", Name: "Konser"}
	report.TimeZone = "Asia/Jakarta"
	report.DataUpdatedAt = time.Date(2026, 10, 8, 1, 2, 3, 0, time.UTC)
	data, err := attendanceCSV(report)
	if err != nil {
		t.Fatal(err)
	}
	reader := csv.NewReader(strings.NewReader(strings.TrimPrefix(string(data), "\xef\xbb\xbf")))
	if _, err := reader.Read(); err != nil {
		t.Fatal(err)
	}
	row, err := reader.Read()
	if err != nil {
		t.Fatal(err)
	}
	if row[0] != "kehadiran_terkini" || row[1] != "2026-10-08" || row[2] != "2026-10-08" || row[3] != "Asia/Jakarta" || row[4] != "evt-1" || row[5] != "Konser" || row[7] != "2026-10-08T08:02:03+07:00" {
		t.Fatalf("attendance CSV row = %#v", row)
	}
}
