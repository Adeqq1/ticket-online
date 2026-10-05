import { expect, test } from "bun:test";
import { ApiError, createOrder, getEvent, getOrder, getTicket, listOrderTickets, mapApiEvent, simulatePayment, type ApiEvent } from "./src/lib/api.ts";

const apiEvent: ApiEvent = {
  id: "nusa-malam", artist: "Nusa Malam", city: "Jakarta", venue: "Ruang Selatan", address: "Jl. Musik Raya, Jakarta",
  startsAt: "2027-08-24T12:30:00Z", genre: "Indie", status: "Early Bird", image: "https://example.com/nusa.jpg",
  description: "Deskripsi", lineup: ["Nusa Malam"], price: 225000,
  zones: [{ id: "festival", name: "Festival", description: "Area umum" }],
  ticketTiers: [{ id: "festival", name: "Festival", zoneId: "festival", price: 225000, availableQuantity: 42, maxPerOrder: 6, benefit: "Area berdiri", gate: "Gate B", seating: "free-standing" }],
};

test("maps API catalog DTO to the frontend concert model", () => {
  const concert = mapApiEvent(apiEvent);
  expect(concert.date).toContain("Selasa, 24 Agustus 2027");
  expect(concert.ticketTiers[0]).toMatchObject({ stock: 42, maxPerOrder: 6, price: 225000 });
  expect(concert.ticketTiers[0]?.seating).toBe("free-standing");
});

test("parses backend error responses without treating them as success", async () => {
  const originalFetch = globalThis.fetch;
  globalThis.fetch = (async () => new Response(JSON.stringify({ error: { code: "EVENT_NOT_FOUND", message: "Event tidak ditemukan" } }), { status: 404 })) as unknown as typeof fetch;
  try {
    await expect(getEvent("unknown")).rejects.toEqual(expect.objectContaining({ code: "EVENT_NOT_FOUND", status: 404, message: "Event tidak ditemukan" } satisfies Partial<ApiError>));
  } finally {
    globalThis.fetch = originalFetch;
  }
});

test("uses checkout idempotency and Bearer auth for order, payment, and ticket APIs", async () => {
  const originalFetch = globalThis.fetch;
  const calls: Array<{ input: RequestInfo | URL; init?: RequestInit }> = [];
  const orderResponse = { id: "order-id", reference: "ORD-1", reservationId: "reservation-id", status: "PENDING", expiresAt: "2027-08-24T12:30:00Z", subtotal: 450000, adminFee: 7500, discount: 45000, total: 412500, items: [], accessToken: "secret", accessExpiresAt: "2027-08-25T00:00:00Z" };
  const ticket = { id: "ticket-id", code: "TICKET-1", attendeeName: "Peserta Satu", orderReference: "ORD-1", eventId: "nusa-malam", eventArtist: "Nusa Malam", eventCity: "Jakarta", eventVenue: "Ruang Selatan", eventAddress: "Jl. Musik Raya", eventStartsAt: "2027-08-24T12:30:00Z", tierName: "Festival", gate: "Gate B", issuedAt: "2027-08-24T10:00:00Z" };
  const responses = [orderResponse, { ...orderResponse, payment: null, buyer: { name: "Pembeli", email: "buyer@example.com", phone: "+6281234567890", identity: "123456789012" }, attendees: [], createdAt: "2027-08-24T10:00:00Z", updatedAt: "2027-08-24T10:00:00Z", eventStartsAt: "2027-08-24T12:30:00Z" }, { id: "payment-id", orderId: "order-id", orderStatus: "PENDING", method: "QRIS", amount: 412500, status: "FAILED", tickets: [] }, { tickets: [ticket] }, ticket];
  const statuses = [201, 200, 201, 200, 200];
  globalThis.fetch = (async (input: RequestInfo | URL, init?: RequestInit) => {
    calls.push({ input, init });
    return new Response(JSON.stringify(responses.shift()), { status: statuses.shift(), headers: { "Content-Type": "application/json" } });
  }) as typeof fetch;
  try {
    const signal = new AbortController().signal;
    const payload = { buyer: { name: "Pembeli", email: "buyer@example.com", phone: "+6281234567890", identity: "123456789012" }, attendees: [{ tierId: "tier-a", names: ["Peserta Satu", "Peserta Dua"] }, { tierId: "tier-b", names: ["Peserta Tiga"] }], voucherCode: "HEMAT10" };
    expect(await createOrder("reservation-id", payload, "reservation-key-123456", signal)).toMatchObject({ id: "order-id", accessToken: "secret" });
    expect(await getOrder("order-id", "secret", signal)).toMatchObject({ payment: null, reservationId: "reservation-id" });
    expect(await simulatePayment("order-id", { method: "QRIS", result: "FAILED" }, "secret", signal)).toMatchObject({ status: "FAILED", tickets: [] });
    expect(await listOrderTickets("order-id", "secret", signal)).toEqual([ticket]);
    expect(await getTicket("ticket-id", "secret", signal)).toEqual(ticket);
    expect(calls.map(({ input }) => String(input))).toEqual([
      "/api/v1/reservations/reservation-id/checkout",
      "/api/v1/orders/order-id",
      "/api/v1/orders/order-id/simulate-payment",
      "/api/v1/orders/order-id/tickets",
      "/api/v1/tickets/ticket-id",
    ]);
    expect(new Headers(calls[0]?.init?.headers).get("Idempotency-Key")).toBe("reservation-key-123456");
    expect(JSON.parse(String(calls[0]?.init?.body))).toEqual(payload);
    for (const call of calls.slice(1)) expect(new Headers(call.init?.headers).get("Authorization")).toBe("Bearer secret");
    expect(calls.every((call) => call.init?.signal === signal)).toBe(true);
  } finally {
    globalThis.fetch = originalFetch;
  }
});

test("preserves checkout voucher errors from the backend", async () => {
  const originalFetch = globalThis.fetch;
  globalThis.fetch = (async () => new Response(JSON.stringify({ error: { code: "INVALID_VOUCHER", message: "Kode voucher tidak valid" } }), { status: 422 })) as unknown as typeof fetch;
  try {
    await expect(createOrder("reservation-id", { buyer: { name: "Pembeli", email: "buyer@example.com", phone: "08123456789", identity: "123456789012" }, attendees: [], voucherCode: "SALAH" }, "reservation-key-123456")).rejects.toEqual(expect.objectContaining({ code: "INVALID_VOUCHER", status: 422 } satisfies Partial<ApiError>));
  } finally {
    globalThis.fetch = originalFetch;
  }
});

test("surfaces wrong and expired private ticket access responses", async () => {
  const originalFetch = globalThis.fetch;
  const errors = [
    { status: 404, code: "NOT_FOUND", message: "Data tidak ditemukan" },
    { status: 410, code: "ACCESS_TOKEN_EXPIRED", message: "Token akses sudah kedaluwarsa" },
  ];
  globalThis.fetch = (async () => {
    const error = errors.shift()!;
    return new Response(JSON.stringify({ error: { code: error.code, message: error.message } }), { status: error.status });
  }) as unknown as typeof fetch;
  try {
    await expect(getTicket("ticket-id", "wrong-token")).rejects.toEqual(expect.objectContaining({ code: "NOT_FOUND", status: 404 } satisfies Partial<ApiError>));
    await expect(getTicket("ticket-id", "expired-token")).rejects.toEqual(expect.objectContaining({ code: "ACCESS_TOKEN_EXPIRED", status: 410 } satisfies Partial<ApiError>));
  } finally {
    globalThis.fetch = originalFetch;
  }
});
