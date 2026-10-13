import { ApiError, getOrder, listOrderTickets, type ApiTicket, type OrderDetail } from "./api.ts";
import { saveOrderTicketAccess, type OrderAccess } from "./order-access.ts";

export type BuyerOrderTickets = {
  access: OrderAccess;
  detail: OrderDetail | null;
  tickets: ApiTicket[];
  error: Error | null;
  accessSaved?: boolean;
};

export async function loadBuyerOrderTickets(access: OrderAccess, signal?: AbortSignal): Promise<BuyerOrderTickets> {
  let detail: OrderDetail;
  try {
    detail = await getOrder(access.orderId, access.accessToken, signal);
  } catch (error) {
    return { access, detail: null, tickets: [], error: error instanceof Error ? error : new ApiError("Order belum dapat dimuat.", 0) };
  }
  access = { ...access, accessExpiresAt: detail.accessExpiresAt };
  const accessSaved = saveOrderTicketAccess(access, access.accessExpiresAt, []);
  if (!["PAID", "REFUND_PENDING", "REFUNDED"].includes(detail.status)) return { access, detail, tickets: [], error: null, accessSaved };
  try {
    const tickets = await listOrderTickets(access.orderId, access.accessToken, signal);
    const expected = orderedTicketCount(detail);
    const error = tickets.length !== expected ? new Error("Sebagian e-ticket belum dapat dimuat.") : null;
    return { access, detail, tickets, error, accessSaved: saveOrderTicketAccess(access, access.accessExpiresAt, tickets.map((ticket) => ticket.id)) };
  } catch (error) {
    return { access, detail, tickets: [], accessSaved, error: error instanceof Error ? error : new ApiError("Daftar tiket belum dapat dimuat.", 0) };
  }
}

export type BuyerOrderCategory = "active" | "pending" | "history" | "review";

export function orderedTicketCount(detail: OrderDetail) {
  return detail.items.reduce((count, item) => count + item.quantity, 0);
}

export function usableBuyerTickets(order: BuyerOrderTickets) {
  return order.tickets.filter((ticket) => ticket.usable === true);
}

export function buyerOrderCategory(order: BuyerOrderTickets): BuyerOrderCategory {
  const { detail, tickets, error } = order;
  if (!detail) return "review";
  if (detail.status === "PENDING") return "pending";
  if (["EXPIRED", "CANCELLED", "REFUND_PENDING", "REFUNDED"].includes(detail.status)) return "history";
  if (detail.status !== "PAID" || error || !tickets.length || tickets.length !== orderedTicketCount(detail) || new Set(tickets.map((ticket) => ticket.id)).size !== tickets.length || tickets.some((ticket) => typeof ticket.usable !== "boolean")) return "review";
  if (detail.currentEvent?.status === "POSTPONED" || detail.currentEvent?.status === "CANCELLED" || detail.refundRight?.requested || detail.refund) return "history";
  return usableBuyerTickets(order).length ? "active" : "history";
}

export function groupBuyerOrders(orders: BuyerOrderTickets[]) {
  const groups: Record<BuyerOrderCategory, BuyerOrderTickets[]> = { active: [], pending: [], history: [], review: [] };
  for (const order of orders) groups[buyerOrderCategory(order)].push(order);
  const time = (value?: string | null, fallback = Infinity) => Number.isFinite(Date.parse(value ?? "")) ? Date.parse(value!) : fallback;
  groups.active.sort((a, b) => time(a.detail?.currentEvent?.startsAt) - time(b.detail?.currentEvent?.startsAt) || a.access.orderId.localeCompare(b.access.orderId));
  groups.pending.sort((a, b) => time(a.detail?.expiresAt) - time(b.detail?.expiresAt) || a.access.orderId.localeCompare(b.access.orderId));
  groups.history.sort((a, b) => time(b.detail?.createdAt, 0) - time(a.detail?.createdAt, 0) || a.access.orderId.localeCompare(b.access.orderId));
  return groups;
}

export function canResumeOrderPayment(detail: OrderDetail, now = Date.now()) {
  return detail.status === "PENDING" && Date.parse(detail.expiresAt) > now && detail.payment?.status !== "SUCCEEDED" && (!detail.currentEvent || !detail.currentEvent.salesPaused && ["SCHEDULED", "RESCHEDULED"].includes(detail.currentEvent.status));
}

export function buyerOrderNeedsRecovery(order: Pick<BuyerOrderTickets, "error">) {
  return order.error instanceof ApiError && ([401, 403, 404, 410].includes(order.error.status) || order.error.code === "ACCESS_TOKEN_EXPIRED");
}

export function buyerOrderIssue(order: Pick<BuyerOrderTickets, "error" | "detail">) {
  if (buyerOrderNeedsRecovery(order)) return "Akses pesanan berakhir atau tidak dapat digunakan. Pulihkan tiket melalui email pembeli. Ini bukan berarti pesanan kedaluwarsa.";
  return order.detail ? "Sebagian status tiket belum dapat dipastikan. Coba lagi." : "Pesanan belum dapat dimuat. Coba lagi.";
}
