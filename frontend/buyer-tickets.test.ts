import { expect, test } from "bun:test";
import { loadBuyerOrderTickets, uniqueBuyerTickets } from "./src/lib/buyer-tickets.ts";
import type { OrderAccess } from "./src/lib/order-access.ts";

const access = (orderId: string): OrderAccess => ({ orderId, accessToken: `${orderId}-token`, expiresAt: "2027-08-24T12:30:00Z", accessExpiresAt: "2027-08-25T00:00:00Z", reservationId: "reservation-id", idempotencyKey: "stable-key", basketKey: "basket", reference: orderId, ticketIds: [] });
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
    const results = await Promise.all(["paid-order", "pending-order", "failed-order", "partial-order"].map((id) => loadBuyerOrderTickets(access(id))));
    expect(results.map(({ error, tickets }) => [Boolean(error), tickets.length])).toEqual([[false, 1], [false, 0], [true, 0], [true, 0]]);
    expect(uniqueBuyerTickets(results)).toEqual([ticket]);
    expect(calls).not.toContain("/api/v1/orders/pending-order/tickets");
    expect(calls).toContain("/api/v1/orders/failed-order");
  } finally { globalThis.fetch = originalFetch; }
});
