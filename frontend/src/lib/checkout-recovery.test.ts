import { describe, expect, test } from "bun:test";
import { findOrderForActiveReservation } from "./checkout-recovery.ts";
import type { OrderAccess } from "./order-access.ts";
import type { StoredReservation } from "./reservation.ts";

const reservation: StoredReservation = {
  reservationId: "reservation-active",
  idempotencyKey: "idempotency-key",
  eventId: "event-1",
  basketKey: "tier-1=2",
};
const activeReservationId = "reservation-active";

function order(orderId: string, reservationId: string, basketKey = "basket-key"): OrderAccess {
  return {
    orderId,
    accessToken: `token-${orderId}`,
    expiresAt: "2026-10-05T12:00:00Z",
    accessExpiresAt: "2026-10-06T12:00:00Z",
    reservationId,
    idempotencyKey: `key-${orderId}`,
    basketKey,
    reference: `ref-${orderId}`,
    ticketIds: [],
  };
}

describe("findOrderForActiveReservation", () => {
  test("resumes an order tied to the active reservation", () => {
    const matching = order("matching", activeReservationId);
    expect(findOrderForActiveReservation([order("other", "reservation-other"), matching], reservation, "basket-key")).toBe(matching);
  });

  test("does not resume an order from the same basket with another reservation", () => {
    expect(findOrderForActiveReservation([order("old", "reservation-old")], reservation, "basket-key")).toBeNull();
  });

  test("does not select an order when the active reservation ID is absent", () => {
    expect(findOrderForActiveReservation([order("saved", activeReservationId)], { ...reservation, reservationId: undefined }, "basket-key")).toBeNull();
  });

  test("does not depend on saved-order enumeration when multiple orders share a basket", () => {
    const expected = order("expected", activeReservationId);
    const unrelated = order("unrelated", "reservation-unrelated");
    expect(findOrderForActiveReservation([unrelated, expected], reservation, "basket-key")).toBe(expected);
    expect(findOrderForActiveReservation([expected, unrelated], reservation, "basket-key")).toBe(expected);
  });
});
