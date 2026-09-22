import { expect, test } from "bun:test";
import { RESERVATION_DURATION_MS, createReservationExpiry, isReservationExpired, parseStoredReservation, remainingReservationSeconds, reservationBasketKey, reservationExpiryFromStorage, serializeStoredReservation, validReservationExpiry } from "./src/lib/reservation.ts";

test("creates and evaluates an absolute reservation deadline", () => {
  const now = Date.parse("2027-01-01T00:00:00Z");
  const expiresAt = createReservationExpiry(now);
  expect(expiresAt).toBe(now + RESERVATION_DURATION_MS);
  expect(remainingReservationSeconds(expiresAt, now + 1_001)).toBe(599);
  expect(remainingReservationSeconds(expiresAt, expiresAt + 1)).toBe(0);
  expect(isReservationExpired(expiresAt, expiresAt)).toBe(true);
});

test("uses a stable basket key and rejects invalid persisted deadlines", () => {
  expect(reservationBasketKey("nusa-malam", { festival: 2, "vip-a": 1 })).toBe(reservationBasketKey("nusa-malam", { "vip-a": 1, festival: 2 }));
  const now = 1_000_000;
  expect(validReservationExpiry(String(now + 1), now)).toBe(true);
  expect(validReservationExpiry(String(now - 1), now)).toBe(false);
  expect(validReservationExpiry("not-a-time", now)).toBe(false);
  expect(validReservationExpiry(String(now + RESERVATION_DURATION_MS + 1), now)).toBe(false);
  expect(reservationExpiryFromStorage(null, now)).toBe(now + RESERVATION_DURATION_MS);
  expect(reservationExpiryFromStorage("0", now)).toBe(0);
  expect(reservationExpiryFromStorage("", now)).toBe(now);
  expect(reservationExpiryFromStorage("-1", now)).toBe(-1);
  expect(reservationExpiryFromStorage("broken", now)).toBe(now);
  expect(reservationExpiryFromStorage(String(now + RESERVATION_DURATION_MS + 1), now)).toBe(now);
});

test("accepts only opaque persisted reservation metadata", () => {
  const value = { reservationId: "reservation-1", idempotencyKey: "key-12345678901234", eventId: "nusa-malam", basketKey: "festival=2" };
  expect(parseStoredReservation(serializeStoredReservation(value))).toEqual(value);
  expect(parseStoredReservation(JSON.stringify({ ...value, expiresAt: Date.now() + 1_000 }))).toEqual(value);
  expect(parseStoredReservation(JSON.stringify({ ...value, reservationId: "" }))).toBeNull();
  expect(parseStoredReservation("not-json")).toBeNull();
});
