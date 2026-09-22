package reservation

import "testing"

func TestNormalizeAndValidateRequest(t *testing.T) {
	request := NormalizeRequest(Request{EventID: " nusa-malam ", Items: []ItemRequest{{TierID: " vip-a ", Quantity: 2}}})
	if err := ValidateRequest(request); err != nil {
		t.Fatal(err)
	}
	if request.EventID != "nusa-malam" || request.Items[0].TierID != "vip-a" {
		t.Fatalf("request was not normalized: %+v", request)
	}
}

func TestRequestHashIsCanonical(t *testing.T) {
	first := Request{EventID: "nusa-malam", Items: []ItemRequest{{TierID: "vip-a", Quantity: 2}, {TierID: "festival", Quantity: 1}}}
	second := Request{EventID: " nusa-malam ", Items: []ItemRequest{{TierID: " festival ", Quantity: 1}, {TierID: " vip-a ", Quantity: 2}}}
	left, err := RequestHash(first)
	if err != nil {
		t.Fatal(err)
	}
	right, err := RequestHash(second)
	if err != nil {
		t.Fatal(err)
	}
	if left != right {
		t.Fatalf("canonical hashes differ: %s != %s", left, right)
	}
}

func TestValidateRequestRejectsDuplicateAndInvalidItems(t *testing.T) {
	cases := []Request{
		{EventID: "event", Items: []ItemRequest{{TierID: "vip-a", Quantity: 1}, {TierID: "vip-a", Quantity: 1}}},
		{EventID: "event", Items: []ItemRequest{{TierID: "vip-a"}}},
		{EventID: "event", Items: nil},
	}
	for _, request := range cases {
		if err := ValidateRequest(request); err != ErrInvalidRequest {
			t.Fatalf("ValidateRequest(%+v) = %v, want ErrInvalidRequest", request, err)
		}
	}
}
