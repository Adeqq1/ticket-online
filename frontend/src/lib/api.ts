import { eventDate, type Concert, type Genre, type StageZone, type TicketStatus, type TicketTier } from "./concerts.ts";
import type { Buyer } from "./checkout.ts";

export type ApiZone = { id: string; name: string; description: string };
export type ApiTicketTier = { id: string; name: string; zoneId: string; price: number; availableQuantity: number; maxPerOrder: number; benefit: string; gate: string; seating: "assigned" | "free-standing" };
export type ApiEvent = { id: string; artist: string; city: string; venue: string; address: string; startsAt: string; genre: string; status: string; image: string; description: string; lineup: string[]; price: number; zones: ApiZone[]; ticketTiers: ApiTicketTier[] };
export type ApiErrorBody = { error?: { code?: string; message?: string; expectedGate?: string }; status?: string; ticket?: CheckInTicket; checkedInAt?: string };
export type ReservationItem = { tierId: string; name: string; quantity: number; unitPrice: number; lineTotal: number };
export type Reservation = { id: string; status: string; expiresAt: string; event: { id: string; artist: string }; items: ReservationItem[]; subtotal: number; reference?: string };
export type OrderItem = ReservationItem;
export type OrderStatus = "PENDING" | "PAID" | "CANCELLED" | "EXPIRED";
export type PaymentMethod = "QRIS" | "VIRTUAL_ACCOUNT" | "GOPAY";
export type PaymentStatus = "FAILED" | "SUCCEEDED";
export type OrderAttendeeInput = { tierId: string; names: string[] };
export type CreateOrderRequest = { buyer: Buyer; attendees: OrderAttendeeInput[]; voucherCode?: string };
export type OrderBase = { id: string; reference: string; reservationId: string; status: OrderStatus; expiresAt: string; subtotal: number; adminFee: number; discount: number; total: number; items: OrderItem[] };
export type OrderResponse = OrderBase & { accessToken: string; accessExpiresAt: string };
export type OrderDetail = OrderBase & { buyer: Buyer; attendees: Array<{ tierId: string; ticketNumber: number; name: string }>; payment: PaymentSummary | null; createdAt: string; updatedAt: string; eventStartsAt: string; accessExpiresAt: string };
export type PaymentSummary = { id: string; method: PaymentMethod; amount: number; status: PaymentStatus; paidAt?: string };
export type SimulatePaymentRequest = { method: PaymentMethod; result: PaymentStatus };
export type ApiTicket = { id: string; code: string; attendeeName: string; orderReference: string; eventId: string; eventArtist: string; eventCity: string; eventVenue: string; eventAddress: string; eventStartsAt: string; tierName: string; gate: string; issuedAt: string };
export type PaymentResult = { id: string; orderId: string; orderStatus: "PENDING" | "PAID"; method: PaymentMethod; amount: number; status: PaymentStatus; paidAt?: string; tickets: ApiTicket[] };
export type StaffRole = "ADMIN" | "STAFF";
export type StaffAssignment = { eventId: string; gate: string };
export type Staff = { id: string; name: string; email: string; role: StaffRole; active: boolean; assignments: StaffAssignment[] };
export type StaffSession = { accessToken: string; expiresAt: string; staff: Staff };
export type StaffLoginRequest = { email: string; password: string };
export type CreateStaffRequest = { name: string; email: string; password: string; assignments: StaffAssignment[] };
export type UpdateStaffRequest = { name?: string; active?: boolean };
export type CheckInTicket = { id: string; code: string; attendeeName: string; tierName: string; eventId: string; gate: string };
export type CheckInResult = { status: "CHECKED_IN"; ticket: CheckInTicket; checkedInAt: string };
export type CheckInRequest = { eventId: string; gate: string; code: string };

export class ApiError extends Error {
  code: string;
  status: number;
  expectedGate?: string;
  checkedInAt?: string;
  ticket?: CheckInTicket;
  resultStatus?: "ALREADY_USED";
  constructor(message: string, status: number, code = "API_ERROR", details: Pick<ApiErrorBody, "error" | "status" | "ticket" | "checkedInAt"> = {}) {
    super(message); this.name = "ApiError"; this.status = status; this.code = code;
    this.expectedGate = details.error?.expectedGate;
    this.checkedInAt = details.checkedInAt;
    this.ticket = details.ticket;
    this.resultStatus = details.status === "ALREADY_USED" ? details.status : undefined;
  }
}

async function request<T>(path: string, signal?: AbortSignal, init?: RequestInit): Promise<T> {
  let response: Response;
  try { response = await fetch(path, { ...init, signal, headers: { Accept: "application/json", ...init?.headers } }); }
  catch (error) { if (error instanceof DOMException && error.name === "AbortError") throw error; throw new ApiError("Tidak dapat terhubung ke server.", 0, "NETWORK_ERROR"); }
  if (!response.ok) {
    let body: ApiErrorBody = {};
    try { body = await response.json() as ApiErrorBody; } catch { /* malformed error body */ }
    throw new ApiError(body.error?.message ?? "Terjadi kesalahan pada server.", response.status, body.error?.code ?? "API_ERROR", body);
  }
  if (response.status === 204) return undefined as T;
  try { return await response.json() as T; }
  catch (error) {
    if (error instanceof DOMException && error.name === "AbortError") throw error;
    throw new ApiError("Respons server terputus; hasil operasi belum diketahui.", 0, "UNKNOWN_OUTCOME");
  }
}

export function mapApiEvent(event: ApiEvent): Concert {
  return {
    id: event.id, artist: event.artist, city: event.city, venue: event.venue, address: event.address,
    date: eventDate(event.startsAt), startsAt: event.startsAt, genre: event.genre as Genre,
    price: event.price, status: event.status as TicketStatus, image: event.image, description: event.description,
    lineup: event.lineup, zones: event.zones as StageZone[], ticketTiers: event.ticketTiers.map((tier) => ({ ...tier, stock: tier.availableQuantity, seating: tier.seating as TicketTier["seating"] })),
  };
}

export async function getEvents(signal?: AbortSignal): Promise<Concert[]> {
  const response = await request<{ events: ApiEvent[] }>("/api/v1/events", signal);
  return response.events.map(mapApiEvent);
}

export async function getEvent(id: string, signal?: AbortSignal): Promise<Concert> {
  return mapApiEvent(await request<ApiEvent>(`/api/v1/events/${encodeURIComponent(id)}`, signal));
}

export function createReservation(eventId: string, items: Array<{ tierId: string; quantity: number }>, idempotencyKey: string, signal?: AbortSignal) {
  return request<Reservation>("/api/v1/reservations", signal, { method: "POST", headers: { "Content-Type": "application/json", "Idempotency-Key": idempotencyKey }, body: JSON.stringify({ eventId, items }) });
}

export function getReservation(id: string, signal?: AbortSignal) { return request<Reservation>(`/api/v1/reservations/${encodeURIComponent(id)}`, signal); }

export function convertReservation(id: string, signal?: AbortSignal) { return request<Reservation>(`/api/v1/reservations/${encodeURIComponent(id)}/convert`, signal, { method: "POST" }); }

export function createOrder(reservationId: string, payload: CreateOrderRequest, idempotencyKey: string, signal?: AbortSignal) {
  return request<OrderResponse>(`/api/v1/reservations/${encodeURIComponent(reservationId)}/checkout`, signal, { method: "POST", headers: { "Content-Type": "application/json", "Idempotency-Key": idempotencyKey }, body: JSON.stringify(payload) });
}

function privateHeaders(accessToken: string): HeadersInit { return { Authorization: `Bearer ${accessToken}` }; }

export function getOrder(orderId: string, accessToken: string, signal?: AbortSignal) {
  return request<OrderDetail>(`/api/v1/orders/${encodeURIComponent(orderId)}`, signal, { headers: privateHeaders(accessToken) });
}

export function simulatePayment(orderId: string, payload: SimulatePaymentRequest, accessToken: string, signal?: AbortSignal) {
  return request<PaymentResult>(`/api/v1/orders/${encodeURIComponent(orderId)}/simulate-payment`, signal, { method: "POST", headers: { ...privateHeaders(accessToken), "Content-Type": "application/json" }, body: JSON.stringify(payload) });
}

export async function listOrderTickets(orderId: string, accessToken: string, signal?: AbortSignal) {
  return (await request<{ tickets: ApiTicket[] }>(`/api/v1/orders/${encodeURIComponent(orderId)}/tickets`, signal, { headers: privateHeaders(accessToken) })).tickets;
}

export function getTicket(ticketId: string, accessToken: string, signal?: AbortSignal) {
  return request<ApiTicket>(`/api/v1/tickets/${encodeURIComponent(ticketId)}`, signal, { headers: privateHeaders(accessToken) });
}

function staffHeaders(accessToken: string): HeadersInit { return { Authorization: `Bearer ${accessToken}` }; }
function staffJSON(accessToken: string, body: unknown): RequestInit { return { method: "POST", headers: { ...staffHeaders(accessToken), "Content-Type": "application/json" }, body: JSON.stringify(body) }; }

export function loginStaff(payload: StaffLoginRequest, signal?: AbortSignal) {
  return request<StaffSession>("/api/v1/staff/login", signal, { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(payload) });
}

export async function getStaffProfile(accessToken: string, signal?: AbortSignal) {
  return (await request<{ staff: Staff }>("/api/v1/staff/me", signal, { headers: staffHeaders(accessToken) })).staff;
}

export function logoutStaff(accessToken: string, signal?: AbortSignal) {
  return request<void>("/api/v1/staff/logout", signal, { method: "POST", headers: staffHeaders(accessToken) });
}

export async function listStaff(accessToken: string, signal?: AbortSignal) {
  return (await request<{ staff: Staff[] }>("/api/v1/admin/staff", signal, { headers: staffHeaders(accessToken) })).staff;
}

export function createStaff(accessToken: string, payload: CreateStaffRequest, signal?: AbortSignal) {
  return request<Staff>("/api/v1/admin/staff", signal, staffJSON(accessToken, payload));
}

export function updateStaff(accessToken: string, staffId: string, payload: UpdateStaffRequest, signal?: AbortSignal) {
  return request<void>(`/api/v1/admin/staff/${encodeURIComponent(staffId)}`, signal, { ...staffJSON(accessToken, payload), method: "PATCH" });
}

export function replaceStaffAssignments(accessToken: string, staffId: string, assignments: StaffAssignment[], signal?: AbortSignal) {
  return request<void>(`/api/v1/admin/staff/${encodeURIComponent(staffId)}/assignments`, signal, { ...staffJSON(accessToken, { assignments }), method: "PUT" });
}

export function resetStaffPassword(accessToken: string, staffId: string, password: string, signal?: AbortSignal) {
  return request<void>(`/api/v1/admin/staff/${encodeURIComponent(staffId)}/password`, signal, { ...staffJSON(accessToken, { password }), method: "PUT" });
}

export function checkInTicket(accessToken: string, payload: CheckInRequest, signal?: AbortSignal) {
  return request<CheckInResult>("/api/v1/staff/check-ins", signal, staffJSON(accessToken, payload));
}
