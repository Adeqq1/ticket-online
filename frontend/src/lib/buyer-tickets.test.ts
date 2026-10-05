import { expect, test } from "bun:test";
import { loadBuyerOrderTickets } from "./buyer-tickets.ts";
import type { OrderAccess } from "./order-access.ts";

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
    globalThis.fetch = originalFetch;
  }
});
