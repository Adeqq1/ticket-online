package conversion

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestReportFilterUsesJakartaCalendarDaysAndLimitsRange(t *testing.T) {
	now := time.Date(2026, 10, 8, 18, 0, 0, 0, time.UTC)
	from, to, first, last, err := reportFilter(Filter{}, now)
	if err != nil || first != "2026-09-10" || last != "2026-10-09" || from.In(jakarta).Hour() != 0 || to.In(jakarta).Hour() != 0 {
		t.Fatalf("default period = %s..%s (%s..%s), err %v", first, last, from, to, err)
	}
	if _, _, _, _, err := reportFilter(Filter{DateFrom: "2025-09-30", DateTo: "2026-10-01"}, now); err == nil {
		t.Fatal("accepted a range longer than 366 days")
	}
	if _, _, _, _, err := reportFilter(Filter{Device: "phone"}, now); err == nil {
		t.Fatal("accepted an unknown device class")
	}
}

func TestStageLossNeverGoesNegative(t *testing.T) {
	if got := maxZero(7 - 9); got != 0 {
		t.Fatalf("maxZero = %d; want 0", got)
	}
	rows := []Breakdown{{Total: Stage{Detail: 3}}, {Total: Stage{Detail: 2}}}
	if got := sum(rows).Total.Detail; got != 5 {
		t.Fatalf("summed detail = %d; want 5", got)
	}
}

func TestClientCannotSubmitBackendPaymentOutcomesOrPersonalData(t *testing.T) {
	service := &Service{}
	for _, body := range []string{
		`{"journeyId":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","eventId":"event","device":"mobile","kind":"DETAIL_VIEWED","email":"buyer@example.test"}`,
		`{"journeyId":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","eventId":"event","device":"mobile","kind":"PAYMENT_FAILURE","reason":"PAYMENT_PROVIDER"}`,
	} {
		request := httptest.NewRequest(http.MethodPost, "/api/v1/conversion/events", strings.NewReader(body))
		response := httptest.NewRecorder()
		service.record(response, request)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("record status = %d; want 400", response.Code)
		}
	}
}
