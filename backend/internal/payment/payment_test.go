package payment

import "testing"

func TestValidateNormalizesPaymentRequest(t *testing.T) {
	got, err := Validate(Request{Method: "  virtual_account ", Result: " succeeded "})
	if err != nil || got != (Request{Method: "VIRTUAL_ACCOUNT", Result: "SUCCEEDED"}) {
		t.Fatalf("Validate() = %+v, %v", got, err)
	}
}

func TestValidateRejectsUnsupportedPaymentValues(t *testing.T) {
	for _, request := range []Request{{Method: "CASH", Result: "SUCCEEDED"}, {Method: "QRIS", Result: "PENDING"}} {
		if _, err := Validate(request); err != ErrInvalidRequest {
			t.Fatalf("Validate(%+v) error = %v, want ErrInvalidRequest", request, err)
		}
	}
}
