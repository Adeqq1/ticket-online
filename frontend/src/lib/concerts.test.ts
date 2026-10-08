import { expect, test } from "bun:test";
import { concerts, getConcertById, parseCheckoutQuantities } from "./concerts.ts";

test("resolves unique URL-safe concert IDs", () => {
  expect(new Set(concerts.map((concert) => concert.id)).size).toBe(concerts.length);
  expect(concerts.every((concert) => /^[a-z0-9-]+$/.test(concert.id))).toBe(true);
  expect(getConcertById("nusa-malam")?.artist).toBe("Nusa Malam");
  expect(getConcertById("missing")).toBeUndefined();
});


test("checkout keeps a held basket valid when catalog stock is lower", () => {
  const concert = { ...concerts[0]!, ticketTiers: concerts[0]!.ticketTiers.map((tier) => tier.id === "festival" ? { ...tier, stock: 1 } : tier) };
  expect(parseCheckoutQuantities(concert, new URLSearchParams("festival=4"))).toEqual({ festival: 4 });
});

