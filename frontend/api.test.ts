import { expect, test } from "bun:test";
import { ApiError, addAdminPaymentCaseNote, checkInTicket, createAdminEvent, createOrder, createStaff, getAdminCheckInHistory, getAdminEmailJobs, getAdminOperations, getAdminSalesReport, getAdminOrder, getAdminOrders, getAdminPaymentCases, getAdminEvents, getEvent, getOrder, getStaffProfile, getStaffTicketStatus, getTicket, listOrderTickets, listStaff, loginStaff, logoutStaff, mapApiEvent, recheckAdminPaymentCase, replaceStaffAssignments, resetStaffPassword, retryAdminEmailJob, resolveAdminPaymentCase, simulatePayment, updateAdminEvent, updateStaff, type ApiEvent, type Staff } from "./src/lib/api.ts";

const apiEvent: ApiEvent = {
  id: "nusa-malam", artist: "Nusa Malam", city: "Jakarta", venue: "Ruang Selatan", address: "Jl. Musik Raya, Jakarta",
  startsAt: "2027-08-24T12:30:00Z", genre: "Indie", status: "Early Bird", publicationStatus: "PUBLISHED", scheduleLocked: false, image: "https://example.com/nusa.jpg",
  description: "Deskripsi", lineup: ["Nusa Malam"], price: 225000,
  zones: [{ id: "festival", name: "Festival", description: "Area umum" }],
  ticketTiers: [{ id: "festival", name: "Festival", zoneId: "festival", price: 225000, availableQuantity: 42, maxPerOrder: 6, benefit: "Area berdiri", gate: "Gate B", seating: "free-standing" }],
};

test("admin operations API sends the staff session to the protected endpoint", async () => {
  const originalFetch = globalThis.fetch;
  let request: { url: string; init?: RequestInit } | undefined;
  globalThis.fetch = (async (input: RequestInfo | URL, init?: RequestInit) => {
    request = { url: String(input), init };
    return new Response(JSON.stringify({ collectedAt: "2026-10-06T00:00:00Z", api5xxLast5m: 0, failedEmailJobs: 0, oldestPendingEmailSeconds: 0, openPaymentCases: 0, workers: [], alerts: [] }), { status: 200 });
  }) as typeof fetch;
  try {
    const result = await getAdminOperations("staff-token");
    expect(result.alerts).toEqual([]);
    expect(request?.url).toBe("/api/v1/admin/operations");
    expect(new Headers(request?.init?.headers).get("Authorization")).toBe("Bearer staff-token");
  } finally { globalThis.fetch = originalFetch; }
});

test("admin sales report API serializes filters and sends the staff session", async () => {
  const originalFetch = globalThis.fetch;
  let request: { url: string; init?: RequestInit } | undefined;
  globalThis.fetch = (async (input: RequestInfo | URL, init?: RequestInit) => {
    request = { url: String(input), init };
    return new Response(JSON.stringify({ period: {}, summary: {}, daily: [], byEvent: [], filterOptions: { events: [] }, dataUpdatedAt: "2026-10-08T00:00:00Z" }), { status: 200 });
  }) as typeof fetch;
  try {
    const result = await getAdminSalesReport("staff-token", { eventId: "nusa malam", dateFrom: "2026-10-01", dateTo: "2026-10-08" });
    expect(result.daily).toEqual([]);
    expect(request?.url).toBe("/api/v1/admin/reports/sales?eventId=nusa+malam&dateFrom=2026-10-01&dateTo=2026-10-08");
    expect(new Headers(request?.init?.headers).get("Authorization")).toBe("Bearer staff-token");
  } finally { globalThis.fetch = originalFetch; }
});

test("maps API catalog DTO to the frontend concert model", () => {
  const concert = mapApiEvent(apiEvent);
  expect(concert.date).toContain("Selasa, 24 Agustus 2027");
  expect(concert.ticketTiers[0]).toMatchObject({ stock: 42, maxPerOrder: 6, price: 225000 });
  expect(concert.ticketTiers[0]?.seating).toBe("free-standing");
});

test("admin event APIs use the private admin routes and preserve publication status", async () => {
  const originalFetch = globalThis.fetch;
  const calls: Array<{ url: string; init?: RequestInit }> = [];
  globalThis.fetch = (async (input: RequestInfo | URL, init?: RequestInit) => {
    calls.push({ url: String(input), init });
    return new Response(JSON.stringify(calls.length === 1 ? { events: [apiEvent] } : apiEvent), { status: calls.length === 2 ? 201 : 200 });
  }) as typeof fetch;
  try {
    expect(await getAdminEvents("staff-token")).toHaveLength(1);
    await createAdminEvent("staff-token", { ...apiEvent, startsAt: "2027-08-24T12:30:00Z" });
    await updateAdminEvent("staff-token", apiEvent.id, { ...apiEvent, startsAt: "2027-08-24T12:30:00Z" });
    expect(calls.map((call) => call.url)).toEqual(["/api/v1/admin/events", "/api/v1/admin/events", "/api/v1/admin/events/nusa-malam"]);
    expect(calls.every((call) => new Headers(call.init?.headers).get("Authorization") === "Bearer staff-token")).toBe(true);
    expect(JSON.parse(String(calls[1]?.init?.body)).publicationStatus).toBe("PUBLISHED");
  } finally { globalThis.fetch = originalFetch; }
});

test("admin order APIs send filters and staff authorization to the admin endpoints", async () => {
  const originalFetch = globalThis.fetch;
  const calls: Array<{ url: string; init?: RequestInit }> = [];
  const page = { items: [], nextCursor: null, filterOptions: { events: [] } };
  globalThis.fetch = (async (input: RequestInfo | URL, init?: RequestInit) => {
    calls.push({ url: String(input), init });
    return new Response(JSON.stringify(page), { status: 200 });
  }) as typeof fetch;
  try {
    expect(await getAdminOrders("staff-token", { q: "TO-123", eventId: "nusa-malam", status: "PAID", dateFrom: "2026-10-06", dateTo: "2026-10-07", cursor: "next" })).toEqual(page);
    await getAdminOrder("staff-token", "0123456789abcdef0123456789abcdef");
    expect(calls.map((call) => call.url)).toEqual([
      "/api/v1/admin/orders?q=TO-123&eventId=nusa-malam&status=PAID&dateFrom=2026-10-06&dateTo=2026-10-07&cursor=next",
      "/api/v1/admin/orders/0123456789abcdef0123456789abcdef",
    ]);
    expect(calls.every((call) => new Headers(call.init?.headers).get("Authorization") === "Bearer staff-token")).toBe(true);
  } finally { globalThis.fetch = originalFetch; }
});

test("admin issue APIs use protected routes and send required case notes", async () => {
  const originalFetch = globalThis.fetch;
  const calls: Array<{ url: string; init?: RequestInit }> = [];
  globalThis.fetch = (async (input: RequestInfo | URL, init?: RequestInit) => {
    calls.push({ url: String(input), init });
    return new Response(calls.length <= 2 ? JSON.stringify({ items: [], nextCursor: null }) : calls.length === 6 ? JSON.stringify({ jobId: "a", retryJobId: "b", status: "PENDING" }) : calls.length === 3 ? JSON.stringify({ caseId: "1", providerStatus: "settlement" }) : null, { status: calls.length === 4 || calls.length === 5 ? 204 : 200 });
  }) as typeof fetch;
  try {
    await getAdminPaymentCases("staff-token"); await getAdminEmailJobs("staff-token", undefined, "next");
    await recheckAdminPaymentCase("staff-token", "1"); await addAdminPaymentCaseNote("staff-token", "1", "dicek");
    await resolveAdminPaymentCase("staff-token", "1", "selesai"); await retryAdminEmailJob("staff-token", "0123456789abcdef0123456789abcdef");
    expect(calls.map((call) => call.url)).toEqual([
      "/api/v1/admin/payment-cases", "/api/v1/admin/email-jobs?cursor=next",
      "/api/v1/admin/payment-cases/1/recheck", "/api/v1/admin/payment-cases/1/notes",
      "/api/v1/admin/payment-cases/1/resolve", "/api/v1/admin/email-jobs/0123456789abcdef0123456789abcdef/retry",
    ]);
    expect(JSON.parse(String(calls[3]?.init?.body))).toEqual({ note: "dicek"});
    expect(JSON.parse(String(calls[4]?.init?.body))).toEqual({ note: "selesai"});
    expect(calls.every((call) => new Headers(call.init?.headers).get("Authorization") === "Bearer staff-token")).toBe(true);
  } finally { globalThis.fetch = originalFetch; }
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

test("calls staff APIs with staff bearer auth and handles empty 204 responses", async () => {
  const originalFetch = globalThis.fetch;
  const calls: Array<{ url: string; init?: RequestInit }> = [];
  const staff: Staff = { id: "a".repeat(32), name: "Petugas", email: "staff@example.com", role: "STAFF", active: true, assignments: [{ eventId: "nusa-malam", gate: "Gate B" }] };
  const responses: unknown[] = [
    { accessToken: "staff-token", expiresAt: "2027-08-24T20:00:00Z", staff },
    { staff }, { staff: [staff] }, staff, null, null, null, null, { status: "CHECKED_IN", checkedInAt: "2027-08-24T12:30:00Z", ticket: { id: "a".repeat(32), code: `ET-${"A".repeat(32)}`, attendeeName: "Peserta", tierName: "Festival", eventId: "nusa-malam", gate: "Gate B" } },
  ];
  globalThis.fetch = (async (input: RequestInfo | URL, init?: RequestInit) => {
    calls.push({ url: String(input), init });
    const body = responses.shift();
    return body === null ? new Response(null, { status: 204 }) : new Response(JSON.stringify(body), { status: calls.length === 1 || calls.length === 4 || calls.length === 9 ? 201 : 200, headers: { "Content-Type": "application/json" } });
  }) as typeof fetch;
  try {
    const session = await loginStaff({ email: "staff@example.com", password: "long enough password" });
    expect(session.accessToken).toBe("staff-token");
    expect(await getStaffProfile(session.accessToken)).toEqual(staff);
    expect(await listStaff(session.accessToken)).toEqual([staff]);
    expect(await createStaff(session.accessToken, { name: "Petugas", email: "staff@example.com", password: "long enough password", assignments: staff.assignments })).toEqual(staff);
    expect(await updateStaff(session.accessToken, staff.id, { active: false })).toBeUndefined();
    expect(await replaceStaffAssignments(session.accessToken, staff.id, [])).toBeUndefined();
    expect(await resetStaffPassword(session.accessToken, staff.id, "another long password")).toBeUndefined();
    expect(await logoutStaff(session.accessToken)).toBeUndefined();
    expect((await checkInTicket(session.accessToken, { eventId: "nusa-malam", gate: "Gate B", code: `ET-${"A".repeat(32)}` })).status).toBe("CHECKED_IN");
    expect(calls.map(({ url }) => url)).toEqual([
      "/api/v1/staff/login", "/api/v1/staff/me", "/api/v1/admin/staff", "/api/v1/admin/staff",
      `/api/v1/admin/staff/${staff.id}`, `/api/v1/admin/staff/${staff.id}/assignments`,
      `/api/v1/admin/staff/${staff.id}/password`, "/api/v1/staff/logout", "/api/v1/staff/check-ins",
    ]);
    expect(calls.map(({ init }) => init?.method ?? "GET")).toEqual(["POST", "GET", "GET", "POST", "PATCH", "PUT", "PUT", "POST", "POST"]);
    for (const call of calls.slice(1)) expect(new Headers(call.init?.headers).get("Authorization")).toBe("Bearer staff-token");
    expect(new Headers(calls[0]?.init?.headers).get("Authorization")).toBeNull();
    expect(JSON.parse(String(calls[5]?.init?.body))).toEqual({ assignments: [] });
    expect(JSON.parse(String(calls[8]?.init?.body))).toEqual({ eventId: "nusa-malam", gate: "Gate B", code: `ET-${"A".repeat(32)}` });
  } finally { globalThis.fetch = originalFetch; }
});

test("preserves expected gate and previous check-in details from server errors", async () => {
  const originalFetch = globalThis.fetch;
  const errors = [
    { error: { code: "WRONG_GATE", message: "Gate lain", expectedGate: "Gate C" } },
    { error: { code: "TICKET_ALREADY_USED", message: "Sudah digunakan" }, status: "ALREADY_USED", ticket: { id: "a".repeat(32), code: `ET-${"A".repeat(32)}`, attendeeName: "Peserta", tierName: "Festival", eventId: "nusa-malam", gate: "Gate B" }, checkedInAt: "2027-08-24T12:30:00Z" },
  ];
  globalThis.fetch = (async () => new Response(JSON.stringify(errors.shift()), { status: 409 })) as unknown as typeof fetch;
  try {
    await expect(checkInTicket("staff-token", { eventId: "nusa-malam", gate: "Gate B", code: `ET-${"A".repeat(32)}` })).rejects.toMatchObject({ code: "WRONG_GATE", expectedGate: "Gate C" });
    await expect(checkInTicket("staff-token", { eventId: "nusa-malam", gate: "Gate B", code: `ET-${"A".repeat(32)}` })).rejects.toMatchObject({ code: "TICKET_ALREADY_USED", resultStatus: "ALREADY_USED", checkedInAt: "2027-08-24T12:30:00Z", ticket: { attendeeName: "Peserta" } });
  } finally { globalThis.fetch = originalFetch; }
});

test("reads ticket check-in status with staff authentication and no mutations", async () => {
  const originalFetch = globalThis.fetch;
  const calls: Array<{ url: string; method: string; headers: Headers }> = [];
  const code = `ET-${"A".repeat(32)}`;
  globalThis.fetch = (async (input, init) => {
    calls.push({ url: String(input), method: init?.method ?? "GET", headers: new Headers(init?.headers) });
    return Response.json({ status: "CHECKED_IN", ticket: { id: "a".repeat(32), code, attendeeName: "Peserta", tierName: "Festival", eventId: "nusa-malam", gate: "Gate B" }, orderStatus: "PAID", checkedInAt: "2027-08-24T12:30:00Z" });
  }) as typeof fetch;
  try {
    await expect(getStaffTicketStatus("staff-token", code)).resolves.toMatchObject({ status: "CHECKED_IN", checkedInAt: "2027-08-24T12:30:00Z" });
    expect(calls).toEqual([{ url: `/api/v1/staff/ticket-status?code=${code}`, method: "GET", headers: expect.any(Headers) }]);
    expect(calls[0]?.headers.get("Authorization")).toBe("Bearer staff-token");
    expect(JSON.stringify(calls[0])).not.toContain("accessToken");
  } finally { globalThis.fetch = originalFetch; }
});

test("loads admin check-in history with literal filters, cursor, and staff authentication", async () => {
  const originalFetch = globalThis.fetch;
  const calls: Array<{ url: string; method: string; headers: Headers }> = [];
  globalThis.fetch = (async (input, init) => {
    calls.push({ url: String(input), method: init?.method ?? "GET", headers: new Headers(init?.headers) });
    return Response.json({ items: [{ id: "23", code: `ET-${"A".repeat(32)}`, eventId: "nusa-malam", eventName: "Nusa Malam", gate: "Gate B", staff: { id: "b".repeat(32), name: "Petugas" }, outcome: "CHECKED_IN", recordedAt: "2026-10-05T12:30:00Z", checkedInAt: "2026-10-05T12:30:00Z" }], nextCursor: "23", filterOptions: [{ id: "nusa-malam", name: "Nusa Malam", gates: ["Gate B"] }] });
  }) as typeof fetch;
  try {
    await expect(getAdminCheckInHistory("admin-token", { eventId: "nusa malam", gate: "Gate B", q: "abcd", beforeId: "45" })).resolves.toMatchObject({ nextCursor: "23", items: [{ outcome: "CHECKED_IN" }] });
    expect(calls[0]?.url).toBe("/api/v1/admin/check-ins?eventId=nusa+malam&gate=Gate+B&q=abcd&beforeId=45");
    expect(calls[0]?.method).toBe("GET");
    expect(calls[0]?.headers.get("Authorization")).toBe("Bearer admin-token");
  } finally { globalThis.fetch = originalFetch; }
});

test("rejects malformed check-in history responses", async () => {
  const originalFetch = globalThis.fetch;
  globalThis.fetch = (async () => Response.json({ items: [{ id: "1", outcome: "UNKNOWN" }], nextCursor: null, filterOptions: [] })) as unknown as typeof fetch;
  try { await expect(getAdminCheckInHistory("admin-token")).rejects.toMatchObject({ code: "INVALID_RESPONSE" }); }
  finally { globalThis.fetch = originalFetch; }
});

test("marks interrupted operation responses as unknown without retrying", async () => {
  const originalFetch = globalThis.fetch;
  let calls = 0;
  globalThis.fetch = (async () => { calls += 1; return { ok: true, status: 201, json: async () => { throw new TypeError("connection closed"); } } as unknown as Response; }) as unknown as typeof fetch;
  try {
    await expect(checkInTicket("staff-token", { eventId: "nusa-malam", gate: "Gate B", code: `ET-${"A".repeat(32)}` })).rejects.toMatchObject({ code: "UNKNOWN_OUTCOME", status: 0 });
    expect(calls).toBe(1);
  } finally { globalThis.fetch = originalFetch; }
});

test("rejects incomplete successful check-in responses as unknown", async () => {
  const originalFetch = globalThis.fetch;
  globalThis.fetch = (async () => new Response(JSON.stringify({ status: "CHECKED_IN", ticket: {}, checkedInAt: "2026-10-05T12:00:00Z" }), { status: 201 })) as unknown as typeof fetch;
  try {
    await expect(checkInTicket("staff-token", { eventId: "nusa-malam", gate: "Gate B", code: `ET-${"A".repeat(32)}` })).rejects.toMatchObject({ code: "UNKNOWN_OUTCOME", status: 0 });
  } finally { globalThis.fetch = originalFetch; }
});

test.each([123, {}, null, undefined])("rejects malformed successful ticket code %p as unknown", async (code) => {
  const originalFetch = globalThis.fetch;
  const payload = { eventId: "nusa-malam", gate: "Gate B", code: `ET-${"A".repeat(32)}` };
  const result = { status: "CHECKED_IN", checkedInAt: "2026-10-05T12:00:00Z", ticket: { id: "a".repeat(32), code, attendeeName: "Peserta", tierName: "Festival", eventId: payload.eventId, gate: payload.gate } };
  let calls = 0;
  globalThis.fetch = (async () => { calls += 1; return new Response(JSON.stringify(result), { status: 201 }); }) as unknown as typeof fetch;
  try {
    const error = await checkInTicket("staff-token", payload).catch((cause: unknown) => cause);
    expect(error).toBeInstanceOf(ApiError);
    expect(error).toMatchObject({ code: "UNKNOWN_OUTCOME", status: 0 });
    expect(calls).toBe(1);
  } finally { globalThis.fetch = originalFetch; }
});

test("requires the backend's 204 confirmation for staff logout", async () => {
  const originalFetch = globalThis.fetch;
  globalThis.fetch = (async () => new Response(null, { status: 200 })) as unknown as typeof fetch;
  try {
    await expect(logoutStaff("staff-token")).rejects.toMatchObject({ code: "UNKNOWN_OUTCOME", status: 0 });
  } finally { globalThis.fetch = originalFetch; }
});
