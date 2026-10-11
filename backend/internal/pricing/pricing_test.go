package pricing

import (
	"math"
	"testing"
)

func TestVoucherValidationAndIntegerDiscount(t *testing.T) {
	v := Voucher{Code: "DUMMY_25", Kind: "PERCENT", Value: 25, StartsAt: "2026-01-01T00:00:00Z", EndsAt: "2027-01-01T00:00:00Z", Quota: 1}
	if err := v.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, change := range []func(*Voucher){func(v *Voucher) { v.Code = "BAD CODE" }, func(v *Voucher) { v.Value = 101 }, func(v *Voucher) { v.Quota = 0 }, func(v *Voucher) { v.EndsAt = v.StartsAt }, func(v *Voucher) { v.Kind = "UNKNOWN" }, func(v *Voucher) { v.MinimumSubtotal = 1000000000001 }} {
		bad := v
		change(&bad)
		if bad.Validate() == nil {
			t.Fatalf("accepted invalid voucher: %+v", bad)
		}
	}
	for _, c := range []struct {
		s       uint64
		k       string
		v, want uint64
	}{{10001, "PERCENT", 25, 2500}, {99, "PERCENT", 10, 9}, {math.MaxUint64, "PERCENT", 100, math.MaxUint64}, {10000, "FIXED", 15000, 10000}, {10000, "FIXED", 3000, 3000}} {
		if got := Discount(c.s, c.k, c.v); got != c.want {
			t.Fatalf("Discount(%d,%s,%d)=%d want %d", c.s, c.k, c.v, got, c.want)
		}
	}
}
