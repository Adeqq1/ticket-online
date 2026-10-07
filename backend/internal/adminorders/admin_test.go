package adminorders

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestParseFilterUsesJakartaInclusiveDateRangeAndValidatesInputs(t *testing.T) {
	parsed, err := parseFilter(Filter{DateFrom: "2026-10-06", DateTo: "2026-10-07"})
	if err != nil {
		t.Fatal(err)
	}
	from := parsed.From.(time.Time)
	to := parsed.To.(time.Time)
	if from.Format(time.RFC3339) != "2026-10-05T17:00:00Z" || to.Format(time.RFC3339) != "2026-10-07T17:00:00Z" {
		t.Fatalf("UTC range = %s through %s", from.Format(time.RFC3339), to.Format(time.RFC3339))
	}
	for _, filter := range []Filter{
		{DateFrom: "2026-02-30"}, {DateFrom: "2026-10-07", DateTo: "2026-10-06"},
		{Status: "NOPE"}, {Cursor: "bad"}, {Query: string(make([]byte, 33))},
	} {
		if _, err := parseFilter(filter); err == nil {
			t.Errorf("parseFilter(%+v) unexpectedly succeeded", filter)
		}
	}
}

func TestParseFilterAcceptsRefundStatuses(t *testing.T) {
	for _, status := range []string{"REFUND_PENDING", "REFUNDED"} {
		if _, err := parseFilter(Filter{Status: status}); err != nil {
			t.Errorf("status %s rejected: %v", status, err)
		}
	}
}

func TestParseFilterClassifiesInvalidRangesAndCursorsAsClientErrors(t *testing.T) {
	validID := "0123456789abcdef0123456789abcdef"
	encode := func(value string) string { return base64.RawURLEncoding.EncodeToString([]byte(value)) }
	cases := []struct {
		name   string
		filter Filter
		query  string
	}{
		{name: "reversed date range", filter: Filter{DateFrom: "2026-10-07", DateTo: "2026-10-06"}, query: "dateFrom=2026-10-07&dateTo=2026-10-06"},
		{name: "invalid cursor JSON", filter: Filter{Cursor: encode("{")}, query: "cursor=" + encode("{")},
		{name: "invalid cursor ID", filter: Filter{Cursor: encode(fmt.Sprintf(`{"id":"wrong","createdAt":"2026-10-06T00:00:00Z"}`))}, query: "cursor=" + encode(fmt.Sprintf(`{"id":"wrong","createdAt":"2026-10-06T00:00:00Z"}`))},
		{name: "invalid cursor timestamp", filter: Filter{Cursor: encode(fmt.Sprintf(`{"id":%q,"createdAt":"invalid"}`, validID))}, query: "cursor=" + encode(fmt.Sprintf(`{"id":%q,"createdAt":"invalid"}`, validID))},
		{name: "noncanonical cursor timestamp", filter: Filter{Cursor: encode(fmt.Sprintf(`{"id":%q,"createdAt":"2026-10-06T00:00:00+00:00"}`, validID))}, query: "cursor=" + encode(fmt.Sprintf(`{"id":%q,"createdAt":"2026-10-06T00:00:00+00:00"}`, validID))},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if _, err := parseFilter(test.filter); !errors.Is(err, ErrInvalidRequest) {
				t.Fatalf("parseFilter error = %v, want ErrInvalidRequest", err)
			}
			request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/orders?"+test.query, nil)
			request.Header.Set("Authorization", "Bearer valid-shaped-staff-token")
			response := httptest.NewRecorder()
			NewHandler(NewService(nil, nil), nil).list(response, request)
			if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), `"code":"INVALID_REQUEST"`) {
				t.Fatalf("response = %d %s, want 400 INVALID_REQUEST", response.Code, response.Body.String())
			}
		})
	}
}

func TestAdminOrderCursorRoundTrip(t *testing.T) {
	createdAt, _ := time.Parse(time.RFC3339Nano, "2026-10-06T10:11:12.123456Z")
	want := Order{ID: "0123456789abcdef0123456789abcdef", CreatedAt: createdAt}
	encoded, err := encodeCursor(want)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := parseFilter(Filter{Cursor: encoded})
	if err != nil || parsed.CursorID != want.ID || !parsed.CursorAt.(time.Time).Equal(createdAt) {
		t.Fatalf("cursor round trip = %#v, error %v", parsed, err)
	}
}

func TestMaskIdentityOnlyKeepsLastFourDigits(t *testing.T) {
	if got := maskIdentity("123456789012"); got != "••••••••9012" {
		t.Fatalf("masked identity = %q", got)
	}
}

func TestAdminOrderRoutesRequireStaffBearerToken(t *testing.T) {
	handler := NewHandler(NewService(nil, nil), nil)
	mux := http.NewServeMux()
	handler.Register(mux)
	for _, path := range []string{"/api/v1/admin/orders", "/api/v1/admin/orders/0123456789abcdef0123456789abcdef"} {
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, httptest.NewRequest("GET", path, nil))
		if response.Code != 401 {
			t.Errorf("GET %s status = %d, want 401", path, response.Code)
		}
	}
}
