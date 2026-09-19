import { expect, test } from "bun:test";
import { getConcertById } from "./concerts.ts";
import { ADMIN_FEE, checkoutLines, isValidEmail, isValidIdentity, isValidPhone, orderSubtotal, voucherDiscount } from "./checkout.ts";

test("accepts only valid ticket quantities from checkout URL", () => {
  const concert = getConcertById("nusa-malam")!;
  const lines = checkoutLines(concert, new URLSearchParams("vip-a=4&festival=3&tribune=-1&unknown=8"));
  expect(lines.map(({ tier, quantity }) => [tier.id, quantity])).toEqual([["vip-a", 4], ["festival", 3]]);
  expect(checkoutLines(concert, new URLSearchParams("vip-a=5"))).toEqual([]);
});

test("calculates promo totals and validates buyer input", () => {
  const concert = getConcertById("nusa-malam")!;
  const subtotal = orderSubtotal(checkoutLines(concert, new URLSearchParams("festival=2")));
  expect(subtotal).toBe(450_000);
  expect(voucherDiscount(subtotal, "hemat10")).toBe(45_000);
  expect(subtotal + ADMIN_FEE - voucherDiscount(subtotal, "HEMAT10")).toBe(412_500);
  expect(isValidEmail("halo@example.com")).toBe(true);
  expect(isValidEmail("halo.example.com")).toBe(false);
  expect(isValidPhone("0812 3456 7890")).toBe(true);
  expect(isValidIdentity("1234567890123456")).toBe(true);
});
