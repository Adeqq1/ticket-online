package checkin

import (
	"errors"
	"testing"
)

func TestNormalizeCheckInRequest(t *testing.T) {
	request, err := Normalize(Request{EventID: " nusa-malam ", Gate: " Gate B ", Code: " et-0123456789abcdef0123456789abcdef "})
	if err != nil {
		t.Fatal(err)
	}
	if request.EventID != "nusa-malam" || request.Gate != "Gate B" || request.Code != "ET-0123456789ABCDEF0123456789ABCDEF" {
		t.Fatalf("normalized request = %+v", request)
	}
}

func TestNormalizeRejectsInvalidCheckInRequests(t *testing.T) {
	for _, request := range []Request{
		{},
		{EventID: "event", Gate: "Gate A", Code: "TO-0123456789"},
		{EventID: "event", Gate: "Gate A", Code: "ET-0123456789abcdef0123456789abcdeg"},
		{EventID: "event", Gate: "Gate A", Code: "ET-0123456789abcdef0123456789abcdefx"},
	} {
		if _, err := Normalize(request); !errors.Is(err, ErrInvalidRequest) {
			t.Errorf("Normalize(%+v) error = %v, want ErrInvalidRequest", request, err)
		}
	}
}
