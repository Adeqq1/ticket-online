import { expect, test } from "bun:test";
import { ApiError, type Voucher } from "./api.ts";
import { pricingHistoryFields, sameVoucher, uncertainPricingSave } from "./pricing-editor.ts";

test("pricing history keeps readable values, WIB dates, zero amounts, and explicit status without guessing malformed data", () => {
  expect(pricingHistoryFields({ adminFee: 7500 })).toEqual([["Biaya admin", "Rp\u00a07.500"]]);
  const fields = Object.fromEntries(pricingHistoryFields({ code: "HEMAT10", kind: "PERCENT", value: 10, active: false, quota: 1000000000, minimumSubtotal: 0, eventId: "", startsAt: "2026-10-11T17:00:00Z", endsAt: "invalid" }));
  expect(fields).toEqual({ "Kode voucher": "HEMAT10", Diskon: "10%", Status: "Nonaktif", "Kuota order": "1.000.000.000", "Minimum subtotal": "Rp\u00a00", Konser: "Semua konser", "Mulai berlaku": "12 Okt 2026, 00.00 WIB", Berakhir: "Tidak tersedia" });
  for (const value of [null, [], "bad", {}, { unrelated: true }]) expect(pricingHistoryFields(value)).toEqual([]);
  expect(pricingHistoryFields({ adminFee: -1 })).toEqual([["Biaya admin", "Tidak tersedia"]]);
  const malformed = Object.fromEntries(pricingHistoryFields({ code: "TEST", active: "false", kind: "FUTURE", quota: Infinity }));
  for (const field of ["Status", "Diskon", "Kuota order", "Mulai berlaku"]) expect(malformed[field]).toBe("Tidak tersedia");
  expect(Object.fromEntries(pricingHistoryFields({ code: "TEST", kind: "FIXED", value: 100000 })).Diskon).toBe("Rp\u00a0100.000");
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
