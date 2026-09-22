import { expect, test } from "bun:test";
import { parseCheckoutQuantities } from "./src/lib/concerts.ts";
import { concerts } from "./src/lib/concerts.ts";
import { parseStoredReservation, serializeStoredReservation } from "./src/lib/reservation.ts";

test("checkout keeps a held basket valid when catalog stock is lower", () => {
  const concert = { ...concerts[0]!, ticketTiers: concerts[0]!.ticketTiers.map((tier) => tier.id === "festival" ? { ...tier, stock: 1 } : tier) };
  expect(parseCheckoutQuantities(concert, new URLSearchParams("festival=4"))).toEqual({ festival: 4 });
});

test("pending reservation metadata can be retried without a reservation id", () => {
  const attempt = { idempotencyKey: "key-12345678901234", eventId: "nusa-malam", basketKey: "festival=2" };
  expect(parseStoredReservation(serializeStoredReservation(attempt))).toEqual(attempt);
});
