import { expect, test } from "bun:test";
import { getConcertById } from "./concerts.ts";
import { cartTotal, changeQuantity, initialState, ticketCount } from "./cart.ts";

test("caps ticket quantities at stock and order limit", () => {
  const concert = getConcertById("nusa-malam")!;
  let state = initialState;
  for (let index = 0; index < 8; index++) state = changeQuantity(concert, state, "vip-a", 1);
  expect(state.quantities["vip-a"]).toBe(4);
  for (let index = 0; index < 8; index++) state = changeQuantity(concert, state, "vip-a", -1);
  expect(state.quantities["vip-a"]).toBe(0);
  expect(changeQuantity(concert, state, "unknown", 1)).toBe(state);
});


test("keeps sold-out tiers at zero and calculates cart totals", () => {
  const soldOut = getConcertById("laut-kaca")!;
  expect(changeQuantity(soldOut, initialState, "festival", 1).quantities.festival).toBe(0);
  const concert = getConcertById("nusa-malam")!;
  const quantities = { "vip-a": 2, festival: 3 };
  expect(ticketCount(quantities)).toBe(5);
  expect(cartTotal(concert, quantities)).toBe(1_975_000);
});
