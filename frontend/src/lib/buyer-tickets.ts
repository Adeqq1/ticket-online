import { ApiError, getOrder, listOrderTickets, type ApiTicket, type OrderDetail } from "./api.ts";
import type { OrderAccess } from "./order-access.ts";

export type BuyerOrderTickets = {
  access: OrderAccess;
  detail: OrderDetail | null;
  tickets: ApiTicket[];
  error: Error | null;
};

export async function loadBuyerOrderTickets(access: OrderAccess, signal?: AbortSignal): Promise<BuyerOrderTickets> {
  let detail: OrderDetail;
  try {
    detail = await getOrder(access.orderId, access.accessToken, signal);
  } catch (error) {
    return { access, detail: null, tickets: [], error: error instanceof Error ? error : new ApiError("Order belum dapat dimuat.", 0) };
  }
  if (detail.status !== "PAID") return { access, detail, tickets: [], error: null };
  try {
    const tickets = await listOrderTickets(access.orderId, access.accessToken, signal);
    const expected = detail.items.reduce((count, item) => count + item.quantity, 0);
    const error = tickets.length !== expected ? new Error("Sebagian e-ticket belum dapat dimuat.") : null;
    return { access, detail, tickets, error };
  } catch (error) {
    return { access, detail, tickets: [], error: error instanceof Error ? error : new ApiError("Daftar tiket belum dapat dimuat.", 0) };
  }
}

export function uniqueBuyerTickets(orders: BuyerOrderTickets[]) {
  const tickets = new Map<string, ApiTicket>();
  for (const { tickets: orderTickets } of orders) for (const ticket of orderTickets) tickets.set(ticket.id, ticket);
  return [...tickets.values()].sort((a, b) => Date.parse(a.eventStartsAt) - Date.parse(b.eventStartsAt) || a.id.localeCompare(b.id));
}
