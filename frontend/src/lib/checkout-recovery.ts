import { hasCheckoutMetadata, type OrderAccess } from "./order-access.ts";
import type { Concert } from "./concerts.ts";
import type { Reservation } from "./api.ts";
import type { StoredReservation } from "./reservation.ts";

export function restoredReservationQuantities(concert: Concert, eventId: string, params: URLSearchParams, reservation: Reservation) {
  if (reservation.event.id !== eventId || !/^[0-9a-f]{32}$/.test(reservation.id) || !["ACTIVE", "CONVERTED", "CANCELLED", "EXPIRED"].includes(reservation.status) || !Number.isFinite(Date.parse(reservation.expiresAt)) || !reservation.items.length || reservation.items.length > 10) return null;
  const tiers = new Set(concert.ticketTiers.map((tier) => tier.id));
  const hinted = new Map<string, number>();
  for (const [tierId, raw] of params) {
    if (!tiers.has(tierId)) continue;
    if (hinted.has(tierId) || !/^[1-9]\d*$/.test(raw)) return null;
    const quantity = Number(raw);
    if (!Number.isSafeInteger(quantity)) return null;
    hinted.set(tierId, quantity);
  }
  const confirmed = new Map<string, number>();
  let subtotal = 0;
  for (const item of reservation.items) {
    if (!tiers.has(item.tierId) || confirmed.has(item.tierId) || !Number.isSafeInteger(item.quantity) || item.quantity < 1 || typeof item.name !== "string" || !item.name.trim() || !Number.isSafeInteger(item.unitPrice) || item.unitPrice < 0 || item.lineTotal !== item.quantity * item.unitPrice) return null;
    subtotal += item.lineTotal;
    if (!Number.isSafeInteger(subtotal)) return null;
    confirmed.set(item.tierId, item.quantity);
  }
  if (subtotal !== reservation.subtotal || hinted.size !== confirmed.size || [...hinted].some(([tierId, quantity]) => confirmed.get(tierId) !== quantity)) return null;
  return Object.fromEntries(confirmed);
}

export function orderMatchesReservation(order: { reservationId: string; items: Array<{ tierId: string; quantity: number; unitPrice: number; lineTotal: number }>; attendees: Array<{ tierId: string }> }, reservation: Reservation) {
  if (order.reservationId !== reservation.id || order.items.length !== reservation.items.length) return false;
  if (!reservation.items.every((item) => order.items.some((line) => line.tierId === item.tierId && line.quantity === item.quantity && line.unitPrice === item.unitPrice && line.lineTotal === item.lineTotal))) return false;
  const attendeeCounts = new Map<string, number>();
  for (const attendee of order.attendees) attendeeCounts.set(attendee.tierId, (attendeeCounts.get(attendee.tierId) ?? 0) + 1);
  return order.attendees.length === reservation.items.reduce((sum, item) => sum + item.quantity, 0)
    && reservation.items.every((item) => attendeeCounts.get(item.tierId) === item.quantity);
}

export function findOrderForActiveReservation(
  orders: OrderAccess[],
  reservation: StoredReservation | null,
  basketKey: string,
) {
  if (!reservation?.reservationId) return null;
  return orders.filter(hasCheckoutMetadata).find((order) => order.reservationId === reservation.reservationId && order.basketKey === basketKey) ?? null;
}
