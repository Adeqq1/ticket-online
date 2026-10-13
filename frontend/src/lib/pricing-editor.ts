import { ApiError, type Voucher } from "./api.ts";
import { adminDate, adminTime, adminNumber } from "./admin-operations.ts";
import { formatRupiah } from "./concerts.ts";

export function pricingHistoryFields(value: unknown): Array<[string, string]> {
  if (!value || typeof value !== "object" || Array.isArray(value)) return [];
  const v = value as Record<string, unknown>;
  const integer = (n: unknown): n is number => typeof n === "number" && Number.isSafeInteger(n) && n >= 0;
  const amount = (n: unknown, format: Intl.NumberFormat) => integer(n) ? format.format(n) : "Tidak tersedia";
  const time = (date: unknown) => typeof date === "string" && Number.isFinite(Date.parse(date))
    ? `${adminDate.format(new Date(date))}, ${adminTime.format(new Date(date))} WIB` : "Tidak tersedia";
  if ("adminFee" in v) return [["Biaya admin", amount(v.adminFee, formatRupiah)]];
  if (typeof v.code !== "string" || !v.code) return [];
  return [
    ["Kode voucher", v.code],
    ["Diskon", v.kind === "PERCENT" && integer(v.value) ? `${adminNumber.format(v.value)}%` : v.kind === "FIXED" ? amount(v.value, formatRupiah) : "Tidak tersedia"],
    ["Status", v.active === true ? "Aktif" : v.active === false ? "Nonaktif" : "Tidak tersedia"],
    ["Kuota order", amount(v.quota, adminNumber)],
    ["Minimum subtotal", amount(v.minimumSubtotal, formatRupiah)],
    ["Konser", typeof v.eventId === "string" ? v.eventId || "Semua konser" : "Tidak tersedia"],
    ["Mulai berlaku", time(v.startsAt)],
    ["Berakhir", time(v.endsAt)],
  ];
}

export function uncertainPricingSave(cause: unknown): boolean {
  return cause instanceof ApiError && (cause.code === "UNKNOWN_OUTCOME" || cause.code === "NETWORK_ERROR" || cause.status >= 500);
}

export function sameVoucher(a: Voucher, b: Voucher): boolean {
  return (["code", "kind", "value", "active", "eventId", "minimumSubtotal", "quota"] as const).every(key => a[key] === b[key]) &&
    Date.parse(a.startsAt) === Date.parse(b.startsAt) && Date.parse(a.endsAt) === Date.parse(b.endsAt);
}
