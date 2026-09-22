package reservation

import "testing"

func TestReservationStateErrorsAreDistinct(t *testing.T) {
	if ErrReservationExpired == ErrReservationConverted || ErrReservationExpired == ErrReservationNotExpired {
		t.Fatal("reservation state errors must remain distinguishable")
	}
}
