import type { CreateOrderRequest } from "./api.ts";

export const RESERVATION_DURATION_MS = 10 * 60 * 1_000;
export type StoredReservation = { reservationId?: string; idempotencyKey: string; eventId: string; basketKey: string };

export function reservationBasketKey(concertId: string, quantities: Record<string, number>) {
  const basket = Object.entries(quantities)
    .filter(([, quantity]) => Number.isInteger(quantity) && quantity > 0)
    .sort(([a], [b]) => a.localeCompare(b))
    .map(([tierId, quantity]) => `${encodeURIComponent(tierId)}=${quantity}`)
    .join("&");
  return `ticket-online:reservation:${encodeURIComponent(concertId)}:${basket}`;
}

export function parseStoredReservation(value: string | null): StoredReservation | null {
  if (!value) return null;
  try {
    const parsed = JSON.parse(value) as Partial<StoredReservation>;
    if ([parsed.idempotencyKey, parsed.eventId, parsed.basketKey].every((item) => typeof item === "string" && item.length > 0) && (parsed.reservationId === undefined || (typeof parsed.reservationId === "string" && parsed.reservationId.length > 0))) return { reservationId: parsed.reservationId, idempotencyKey: parsed.idempotencyKey!, eventId: parsed.eventId!, basketKey: parsed.basketKey! };
  } catch { /* invalid browser state */ }
  return null;
}

export function serializeStoredReservation(value: StoredReservation) { return JSON.stringify(value); }

export function parseCheckoutAttempt(value: string | null, quantities: Record<string, number>): CreateOrderRequest | null {
  try {
    const payload = JSON.parse(value ?? "") as Partial<CreateOrderRequest>;
    const buyer = payload.buyer;
    const attendees = payload.attendees;
    if (!buyer || ![buyer.name, buyer.email, buyer.phone, buyer.identity].every((part) => typeof part === "string") || !Array.isArray(attendees) || attendees.length !== Object.keys(quantities).length || (payload.voucherCode !== undefined && typeof payload.voucherCode !== "string")) return null;
    const seen = new Set<string>();
    for (const attendee of attendees) {
      const quantity = quantities[attendee.tierId];
      if (typeof attendee.tierId !== "string" || typeof quantity !== "number" || !Number.isInteger(quantity) || quantity < 1 || seen.has(attendee.tierId) || !Array.isArray(attendee.names) || attendee.names.length !== quantity || attendee.names.some((name) => typeof name !== "string")) return null;
      seen.add(attendee.tierId);
    }
    return payload as CreateOrderRequest;
  } catch { return null; }
}

export function createReservationExpiry(now = Date.now()) {
  return now + RESERVATION_DURATION_MS;
}

export function remainingReservationSeconds(expiresAt: number, now = Date.now()) {
  if (!Number.isFinite(expiresAt)) return 0;
  return Math.max(0, Math.ceil((expiresAt - now) / 1_000));
}

export function isReservationExpired(expiresAt: number, now = Date.now()) {
  return !Number.isFinite(expiresAt) || expiresAt <= now;
}

export function validReservationExpiry(value: string | null, now = Date.now()) {
  const expiresAt = Number(value);
  return Number.isFinite(expiresAt) && expiresAt > now && expiresAt <= now + RESERVATION_DURATION_MS;
}

export function reservationExpiryFromStorage(value: string | null, now = Date.now()) {
  if (value === null) return createReservationExpiry(now);
  if (!value.trim()) return now;
  const expiresAt = Number(value);
  return validReservationExpiry(value, now) || (Number.isFinite(expiresAt) && expiresAt <= now) ? expiresAt : now;
}
