package checkout

import "testing"

func TestValidateNormalizesBuyerAndVoucher(t *testing.T) {
	got, err := Validate(Request{Buyer: Buyer{
		Name: "  Nusa Malam  ", Email: "  buyer@example.com ",
		Phone: "+62 812-3456-7890", Identity: "123456789012",
	}, VoucherCode: " hemat10 "})
	if err != nil {
		t.Fatal(err)
	}
	if got.Buyer.Name != "Nusa Malam" || got.Buyer.Email != "buyer@example.com" || got.Buyer.Phone != "+6281234567890" || got.VoucherCode != "HEMAT10" {
		t.Fatalf("request was not normalized: %+v", got)
	}
}

func TestValidateRejectsInvalidBuyerAndVoucher(t *testing.T) {
	valid := Request{Buyer: Buyer{Name: "Buyer", Email: "buyer@example.com", Phone: "081234567890", Identity: "123456789012"}}
	cases := []struct {
		name    string
		request Request
		want    error
	}{
		{"email", Request{Buyer: Buyer{Name: "Buyer", Email: "invalid", Phone: "081234567890", Identity: "123456789012"}}, ErrInvalidRequest},
		{"phone", Request{Buyer: Buyer{Name: "Buyer", Email: "buyer@example.com", Phone: "abc", Identity: "123456789012"}}, ErrInvalidRequest},
		{"identity", Request{Buyer: Buyer{Name: "Buyer", Email: "buyer@example.com", Phone: "081234567890", Identity: "12345"}}, ErrInvalidRequest},
		{"voucher", Request{Buyer: valid.Buyer, VoucherCode: "UNKNOWN"}, ErrInvalidVoucher},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if _, err := Validate(test.request); err != test.want {
				t.Fatalf("Validate() error = %v, want %v", err, test.want)
			}
		})
	}
}

func TestRequestHashBindsBuyerAndVoucherToReservation(t *testing.T) {
	request := Request{Buyer: Buyer{Name: "Buyer", Email: "buyer@example.com", Phone: "081234567890", Identity: "123456789012"}, VoucherCode: "HEMAT10"}
	first, err := requestHash("reservation-a", request)
	if err != nil {
		t.Fatal(err)
	}
	second, err := requestHash("reservation-a", request)
	if err != nil {
		t.Fatal(err)
	}
	changed, err := requestHash("reservation-a", Request{Buyer: Buyer{Name: "Other", Email: request.Buyer.Email, Phone: request.Buyer.Phone, Identity: request.Buyer.Identity}, VoucherCode: request.VoucherCode})
	if err != nil {
		t.Fatal(err)
	}
	if first != second || first == changed {
		t.Fatalf("unexpected checkout hashes: %q %q %q", first, second, changed)
	}
}
