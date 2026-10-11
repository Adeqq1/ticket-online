import { ApiError, type Voucher } from "./api.ts";

export function parseRupiahInput(value: string): number | undefined {
  const text = value.trim();
  if (!/^(?:\d+|\d{1,3}(?:\.\d{3})+)$/.test(text)) return;
  const amount = Number(text.replaceAll(".", ""));
  return Number.isSafeInteger(amount) && amount <= 1_000_000_000_000 ? amount : undefined;
}

export function uncertainPricingSave(cause: unknown): boolean {
  return cause instanceof ApiError && (cause.code === "UNKNOWN_OUTCOME" || cause.code === "NETWORK_ERROR" || cause.status >= 500);
}

export function sameVoucher(a: Voucher, b: Voucher): boolean {
  return (["code", "kind", "value", "active", "eventId", "minimumSubtotal", "quota"] as const).every(key => a[key] === b[key]) &&
    Date.parse(a.startsAt) === Date.parse(b.startsAt) && Date.parse(a.endsAt) === Date.parse(b.endsAt);
}
