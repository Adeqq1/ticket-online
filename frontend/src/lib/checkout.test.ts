import { expect, test } from "bun:test";
import { getConcertById, parseQuantities } from "./concerts.ts";
import { checkoutLines, isValidEmail, isValidIdentity, isValidPhone, orderSubtotal } from "./checkout.ts";

test("accepts only valid ticket quantities from checkout URL", () => {
  const concert = getConcertById("nusa-malam")!;
  const lines = checkoutLines(concert, new URLSearchParams("vip-a=4&festival=3&tribune=-1&unknown=8"));
  expect(lines.map(({ tier, quantity }) => [tier.id, quantity])).toEqual([["vip-a", 4], ["festival", 3]]);
  expect(checkoutLines(concert, new URLSearchParams("vip-a=5"))).toEqual([]);
});

test("restores only bounded ticket quantities", () => {
  const concert = getConcertById("nusa-malam")!;
  expect(parseQuantities(concert, new URLSearchParams("festival=2&vip-a=99&unknown=4"))).toEqual({ festival: 2 });
});

test("calculates ticket subtotal and validates buyer input", () => {
  const concert = getConcertById("nusa-malam")!;
  const subtotal = orderSubtotal(checkoutLines(concert, new URLSearchParams("festival=2")));
  expect(subtotal).toBe(450_000);
  expect(isValidEmail("halo@example.com")).toBe(true);
  expect(isValidEmail("halo.example.com")).toBe(false);
  expect(isValidPhone("0812 3456 7890")).toBe(true);
  expect(isValidPhone("+62-812-3456-7890")).toBe(true);
  expect(isValidPhone("+        1")).toBe(false);
  expect(isValidPhone("0812abc3456")).toBe(false);
  expect(isValidPhone("12345678")).toBe(false);
  expect(isValidIdentity("1234567890123456")).toBe(true);
});
