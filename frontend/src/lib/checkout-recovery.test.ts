import { describe, expect, test } from "bun:test";
import { findOrderForActiveReservation, orderMatchesReservation, restoredReservationQuantities } from "./checkout-recovery.ts";
import { getConcertById, parseCheckoutQuantities } from "./concerts.ts";
import type { CheckoutOrderAccess, OrderAccess } from "./order-access.ts";
import type { StoredReservation } from "./reservation.ts";

const reservation: StoredReservation = {
  reservationId: "reservation-active",
  idempotencyKey: "idempotency-key",
  eventId: "event-1",
  basketKey: "tier-1=2",
};
const activeReservationId = "reservation-active";

function order(orderId: string, reservationId: string, basketKey = "basket-key"): CheckoutOrderAccess {
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

describe("restoredReservationQuantities", () => {
  const concert = getConcertById("nusa-malam")!;
  const loweredLimits = { ...concert, ticketTiers: concert.ticketTiers.map((tier) => tier.id === "festival" ? { ...tier, maxPerOrder: 2, stock: 0 } : tier) };
  const held = {
    id: "0123456789abcdef0123456789abcdef",
    status: "ACTIVE",
    expiresAt: "2026-10-06T09:00:00Z",
    event: { id: "nusa-malam", artist: "Nusa Malam" },
    items: [
      { tierId: "festival", name: "Festival", quantity: 4, unitPrice: 225000, lineTotal: 900000 },
      { tierId: "vip-a", name: "VIP A", quantity: 1, unitPrice: 650000, lineTotal: 650000 },
    ],
    subtotal: 1550000,
  };

  test("restores the server-confirmed quantities for tiers whose current limits and stock are lower", () => {
    expect(restoredReservationQuantities(loweredLimits, "nusa-malam", new URLSearchParams("festival=4&vip-a=1"), held)).toEqual({ festival: 4, "vip-a": 1 });
  });

  test("still rejects that quantity for a new checkout under the lowered purchase limit", () => {
    expect(parseCheckoutQuantities(loweredLimits, new URLSearchParams("festival=4"))).toEqual({});
  });

  test("rejects a different event, category, or quantity hint", () => {
    expect(restoredReservationQuantities(loweredLimits, "other-event", new URLSearchParams("festival=4&vip-a=1"), held)).toBeNull();
    expect(restoredReservationQuantities(loweredLimits, "nusa-malam", new URLSearchParams("festival=4&tribune=1"), held)).toBeNull();
    expect(restoredReservationQuantities(loweredLimits, "nusa-malam", new URLSearchParams("festival=3&vip-a=1"), held)).toBeNull();
  });

  test("requires the resumed order to retain the confirmed reservation snapshot", () => {
    const order = { reservationId: held.id, items: held.items, attendees: [{ tierId: "festival" }, { tierId: "festival" }, { tierId: "festival" }, { tierId: "festival" }, { tierId: "vip-a" }] };
    expect(orderMatchesReservation(order, held)).toBe(true);
    expect(orderMatchesReservation({ ...order, items: [{ ...held.items[0]!, quantity: 3 }, held.items[1]!] }, held)).toBe(false);
    expect(orderMatchesReservation({ ...order, attendees: order.attendees.slice(1) }, held)).toBe(false);
  });
});
