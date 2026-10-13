import { expect, test } from "bun:test";
import { loadBuyerOrderTickets } from "./buyer-tickets.ts";
import { removeOrderAccess, type OrderAccess } from "./order-access.ts";

const accessForOrder = (orderId: string): OrderAccess => ({ orderId, accessToken: `${orderId}-token`, expiresAt: "2027-08-24T12:30:00Z", accessExpiresAt: "2027-08-25T00:00:00Z", reservationId: "reservation-id", idempotencyKey: "stable-key", basketKey: "basket", reference: orderId, ticketIds: [] });
const detail = (orderId: string, status: "PAID" | "PENDING") => ({ id: orderId, reference: orderId, reservationId: "reservation-id", status, expiresAt: "2027-08-24T12:30:00Z", subtotal: 100, adminFee: 10, discount: 0, total: 110, items: [{ quantity: 1 }], buyer: { name: "Pembeli", email: "buyer@example.com", phone: "08123456789", identity: "123456789012" }, attendees: [], payment: null, createdAt: "2027-08-24T10:00:00Z", updatedAt: "2027-08-24T10:00:00Z", eventStartsAt: "2027-08-24T12:30:00Z", accessExpiresAt: "2027-08-25T00:00:00Z" });
const ticket = { id: "ticket-id", code: "ET-0123456789ABCDEF0123456789ABCDEF", attendeeName: "Peserta", orderReference: "order-paid", eventId: "event-id", eventArtist: "Konser", eventCity: "Jakarta", eventVenue: "Venue", eventAddress: "Alamat", eventStartsAt: "2027-08-24T12:30:00Z", tierName: "Festival", gate: "Gate A", issuedAt: "2027-08-24T10:00:00Z" };

test("loads paid order tickets independently, skips pending tickets, and keeps successful orders", async () => {
  const originalFetch = globalThis.fetch;
  const calls: string[] = [];
  globalThis.fetch = (async (input: RequestInfo | URL) => {
    const url = String(input);
    calls.push(url);
    if (url.endsWith("/failed-order")) return new Response(JSON.stringify({ error: { code: "ACCESS_TOKEN_EXPIRED", message: "Token akses sudah kedaluwarsa" } }), { status: 410 });
    if (url.endsWith("/partial-order/tickets")) return new Response(JSON.stringify({ tickets: [] }), { status: 200 });
    if (url.endsWith("/paid-order/tickets")) return new Response(JSON.stringify({ tickets: [ticket] }), { status: 200 });
    const orderId = ["pending-order", "partial-order"].find((id) => url.endsWith(`/${id}`)) ?? "paid-order";
    return new Response(JSON.stringify(detail(orderId, orderId === "pending-order" ? "PENDING" : "PAID")), { status: 200 });
  }) as typeof fetch;
  try {
    const results = await Promise.all(["paid-order", "pending-order", "failed-order", "partial-order"].map((id) => loadBuyerOrderTickets(accessForOrder(id))));
    expect(results.map(({ error, tickets }) => [Boolean(error), tickets.length])).toEqual([[false, 1], [false, 0], [true, 0], [true, 0]]);
    expect(results[0]!.tickets).toEqual([ticket]);
    expect(calls).not.toContain("/api/v1/orders/pending-order/tickets");
    expect(calls).toContain("/api/v1/orders/failed-order");
  } finally {
    for (const id of ["paid-order", "pending-order", "failed-order", "partial-order"]) removeOrderAccess(id);
    globalThis.fetch = originalFetch;
  }
});

const access: OrderAccess = {
  orderId: "order-1",
  accessToken: "access-token",
  expiresAt: "2026-10-05T12:00:00Z",
  accessExpiresAt: "2026-10-06T12:00:00Z",
  reservationId: "reservation-1",
  idempotencyKey: "idempotency-1",
  basketKey: "basket-1",
  reference: "TO-ORDER1",
  ticketIds: [],
};

test("retains paid order details when ticket listing fails, then loads tickets on retry", async () => {
  const originalFetch = globalThis.fetch;
  let ticketRequest = 0;
  globalThis.fetch = (async (input) => {
    if (String(input).endsWith("/tickets")) {
      ticketRequest += 1;
      if (ticketRequest === 1) return Response.json({ error: { code: "TEMPORARY", message: "Ticket API unavailable" } }, { status: 503 });
      return Response.json({ tickets: [{ id: "ticket-1" }] });
    }
    return Response.json({
      id: access.orderId,
      reference: access.reference,
      reservationId: access.reservationId,
      status: "PAID",
      expiresAt: access.expiresAt,
      subtotal: 10000,
      adminFee: 500,
      discount: 0,
      total: 10500,
      items: [{ tierId: "tier-1", name: "Festival", quantity: 1, unitPrice: 10000, lineTotal: 10000 }],
    });
  }) as typeof fetch;

  try {
    const failed = await loadBuyerOrderTickets(access);
    expect(failed.detail?.status).toBe("PAID");
    expect(failed.tickets).toEqual([]);
    expect(failed.error?.message).toBe("Ticket API unavailable");

    const retried = await loadBuyerOrderTickets(access);
    expect(retried.detail?.status).toBe("PAID");
    expect(retried.tickets).toHaveLength(1);
    expect(retried.error).toBeNull();
  } finally {
    removeOrderAccess(access.orderId);
    globalThis.fetch = originalFetch;
  }
});

import type { OrderDetail } from "./api.ts";
import { ApiError } from "./api.ts";
import { buyerOrderCategory, buyerOrderIssue, buyerOrderNeedsRecovery, canResumeOrderPayment, groupBuyerOrders, orderedTicketCount, type BuyerOrderTickets } from "./buyer-tickets.ts";

function buyerOrder(id: string, status: OrderDetail["status"] = "PAID", eligibility: Array<boolean | undefined> = [true]): BuyerOrderTickets {
  return {
    access: accessForOrder(id),
    detail: { ...detail(id, "PAID"), status, eventId: "same-concert", eventArtist: "Nusa Malam", eventVenue: "Istora", eventCity: "Jakarta", items: [{ tierId: "festival", name: "Festival", quantity: eligibility.length || 2, unitPrice: 100, lineTotal: 200 }], currentEvent: { id: "same-concert", status: "SCHEDULED", version: 0, salesPaused: false, startsAt: "2027-08-24T12:30:00Z", announcement: "", refundDeadline: null } },
    tickets: eligibility.map((usable, index) => ({ ...ticket, id: `${id}-${index}`, usable })),
    error: null,
  };
}

test("groups mixed orders by server eligibility, retaining separate purchases and partial use", () => {
  const active = buyerOrder("active", "PAID", [false, true]);
  const earlier = buyerOrder("earlier");
  earlier.detail!.currentEvent!.startsAt = "2026-01-01T12:30:00Z";
  const pending = buyerOrder("pending", "PENDING", []);
  const dueSoon = buyerOrder("due-soon", "PENDING", []);
  dueSoon.detail!.expiresAt = "2026-01-01T12:00:00Z";
  const history = [buyerOrder("used", "PAID", [false, false]), buyerOrder("expired", "EXPIRED", []), buyerOrder("cancelled", "CANCELLED", []), buyerOrder("refund", "REFUND_PENDING", []), buyerOrder("refunded", "REFUNDED", [])];
  history[2]!.detail!.createdAt = "2028-01-01T12:00:00Z";
  const postponed = buyerOrder("postponed", "PAID", [false]);
  postponed.detail!.currentEvent!.status = "POSTPONED";
  postponed.detail!.currentEvent!.startsAt = null;
  const groups = groupBuyerOrders([active, pending, ...history, earlier, dueSoon, postponed]);
  expect(groups.active.map((order) => order.access.orderId)).toEqual(["earlier", "active"]);
  expect(groups.pending.map((order) => order.access.orderId)).toEqual(["due-soon", "pending"]);
  expect(groups.history).toHaveLength(6);
  expect(groups.history[0]!.access.orderId).toBe("cancelled");
  expect(groups.review).toEqual([]);
  expect(orderedTicketCount(active.detail!)).toBe(2);
  expect(buyerOrderCategory(earlier)).toBe("active");
});

test("failed, missing, duplicated and unknown tickets stay outside categories until checked", () => {
  const failed = buyerOrder("failed"); failed.error = new Error("service details must stay private");
  const partial = buyerOrder("partial", "PAID", [true, false]); partial.tickets.pop();
  const unknown = buyerOrder("unknown", "PAID", [undefined]);
  const duplicate = buyerOrder("duplicate", "PAID", [false, false]); duplicate.tickets[1]!.id = duplicate.tickets[0]!.id;
  const empty = buyerOrder("empty", "PAID", []);
  const accessEnded = buyerOrder("access-ended"); accessEnded.detail = null; accessEnded.error = new ApiError("technical token", 410, "ACCESS_TOKEN_EXPIRED");
  const network = buyerOrder("network"); network.detail = null; network.error = new TypeError("Failed to fetch");
  expect(groupBuyerOrders([failed, partial, unknown, duplicate, empty, accessEnded, network]).review).toHaveLength(7);
  expect(buyerOrderNeedsRecovery(accessEnded)).toBe(true);
  expect(buyerOrderIssue(accessEnded)).toContain("Ini bukan berarti pesanan kedaluwarsa");
  expect(buyerOrderIssue(network)).toBe("Pesanan belum dapat dimuat. Coba lagi.");
  expect(buyerOrderIssue(failed)).not.toContain("service details");
});

test("confirmation and revoked eligibility move an order between categories", () => {
  const order = buyerOrder("transition", "PENDING", []);
  expect(buyerOrderCategory(order)).toBe("pending");
  order.detail!.status = "PAID";
  expect(buyerOrderCategory(order)).toBe("review");
  order.tickets = [true, false].map((usable, index) => ({ ...ticket, id: `transition-${index}`, usable }));
  expect(buyerOrderCategory(order)).toBe("active");
  order.tickets[0]!.usable = false;
  expect(buyerOrderCategory(order)).toBe("history");
});

test("payment continuation holds expired deadlines, uncertain confirmation and changed events", () => {
  const order = buyerOrder("recovered", "PENDING", []);
  delete order.access.idempotencyKey; delete order.access.basketKey;
  const beforeDeadline = Date.parse(order.detail!.expiresAt) - 1000;
  expect(canResumeOrderPayment(order.detail!, beforeDeadline)).toBe(true);
  expect(canResumeOrderPayment(order.detail!, beforeDeadline + 1000)).toBe(false);
  expect(order.detail!.status).toBe("PENDING");
  for (const status of ["SUCCEEDED"] as const) {
    order.detail!.payment = { id: "payment", method: "QRIS", amount: 110, status };
    expect(canResumeOrderPayment(order.detail!, beforeDeadline)).toBe(false);
  }
  order.detail!.payment!.status = "PENDING";
  expect(canResumeOrderPayment(order.detail!, beforeDeadline)).toBe(true);
  order.detail!.payment!.status = "FAILED";
  expect(canResumeOrderPayment(order.detail!, beforeDeadline)).toBe(true);
  order.detail!.currentEvent!.salesPaused = true;
  expect(canResumeOrderPayment(order.detail!, beforeDeadline)).toBe(false);
  order.detail!.status = "PAID";
  expect(canResumeOrderPayment(order.detail!, beforeDeadline)).toBe(false);
});
