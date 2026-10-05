import type { OrderAccess } from "./order-access.ts";
import type { StoredReservation } from "./reservation.ts";

export function findOrderForActiveReservation(
  orders: OrderAccess[],
  reservation: StoredReservation | null,
  basketKey: string,
) {
  if (!reservation?.reservationId) return null;
  return orders.find((order) => order.reservationId === reservation.reservationId && order.basketKey === basketKey) ?? null;
}
