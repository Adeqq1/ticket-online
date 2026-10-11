import { expect, test } from "bun:test";
import { ApiError, type Voucher } from "./api.ts";
import { parseRupiahInput, sameVoucher, uncertainPricingSave } from "./pricing-editor.ts";

test("rupiah preview accepts Indonesian integer grouping and rejects ambiguous or invalid amounts", () => {
  for (const text of ["100000", "100.000", " 100.000 "]) expect(parseRupiahInput(text)).toBe(100000);
  expect(parseRupiahInput("0")).toBe(0);
  expect(parseRupiahInput("1.000.000.000.000")).toBe(1e12);
  for (const text of ["", "-1", "1,5", "1.50", "1e5", "Rp 100", "1 000", "1000000000001", "Infinity"]) expect(parseRupiahInput(text)).toBeUndefined();
});

test("only unconfirmed mutation outcomes require verification before repeating", () => {
  for (const code of ["UNKNOWN_OUTCOME", "NETWORK_ERROR"]) expect(uncertainPricingSave(new ApiError("lost", 0, code))).toBe(true);
  expect(uncertainPricingSave(new ApiError("error", 503, "INTERNAL"))).toBe(true);
  for (const status of [401, 403, 409, 422]) expect(uncertainPricingSave(new ApiError("rejected", status, "REJECTED"))).toBe(false);
});

test("voucher reconciliation compares all submitted rules and equivalent timestamps, not revision", () => {
  const voucher: Voucher = { code: "TEST", kind: "PERCENT", value: 10, active: true, eventId: "event", minimumSubtotal: 0, quota: 10, revision: 1, startsAt: "2026-10-01T00:00:00Z", endsAt: "2026-11-01T00:00:00Z" };
  expect(sameVoucher(voucher, { ...voucher, revision: 2, startsAt: "2026-10-01T07:00:00+07:00" })).toBe(true);
  for (const change of [{ value: 11 }, { quota: 11 }, { active: false }, { eventId: "other" }, { kind: "FIXED" as const }, { minimumSubtotal: 1 }, { code: "OTHER" }, { endsAt: "2026-12-01T00:00:00Z" }]) expect(sameVoucher(voucher, { ...voucher, ...change })).toBe(false);
});
