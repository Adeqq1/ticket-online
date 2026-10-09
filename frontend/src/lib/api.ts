import { eventDate, type Concert, type EventState, type Genre, type StageZone, type TicketStatus, type TicketTier } from "./concerts.ts";
import type { Buyer } from "./checkout.ts";
import { isRecoveryResult } from "./ticket-recovery.ts";

export type ApiZone = { id: string; name: string; description: string };
export type ApiTicketTier = { id: string; name: string; zoneId: string; price: number; availableQuantity: number; maxPerOrder: number; benefit: string; gate: string; seating: "assigned" | "free-standing" };
export type AdminTicketTier = ApiTicketTier & { capacity: number; boundQuantity: number; gateLocked: boolean };
export type PublicationStatus = "DRAFT" | "PUBLISHED" | "ARCHIVED";
export type ApiEvent = { currentEvent?: EventState; id: string; artist: string; city: string; venue: string; address: string; startsAt: string; genre: string; status: string; publicationStatus: PublicationStatus; image: string; description: string; lineup: string[]; price: number; zones: ApiZone[]; ticketTiers: ApiTicketTier[]; scheduleLocked: boolean };
export type AdminApiEvent = Omit<ApiEvent, "ticketTiers"> & { ticketTiers: AdminTicketTier[]; locationLocked: boolean };
export type AdminEventInput = Pick<ApiEvent, "id" | "artist" | "city" | "venue" | "address" | "startsAt" | "genre" | "status" | "publicationStatus" | "image" | "description" | "lineup">;
export type ApiErrorBody = { error?: { code?: string; message?: string; expectedGate?: string }; status?: string; ticket?: CheckInTicket; checkedInAt?: string };
export type ReservationItem = { tierId: string; name: string; quantity: number; unitPrice: number; lineTotal: number };
export type Reservation = { id: string; status: string; expiresAt: string; event: { id: string; artist: string }; items: ReservationItem[]; subtotal: number; reference?: string };
export type OrderItem = ReservationItem;
export type OrderStatus = "PENDING" | "PAID" | "CANCELLED" | "EXPIRED" | "REFUND_PENDING" | "REFUNDED";
export type PaymentMethod = "QRIS" | "VIRTUAL_ACCOUNT" | "GOPAY";
export type PaymentStatus = "PENDING" | "FAILED" | "SUCCEEDED";
export type OrderAttendeeInput = { tierId: string; names: string[] };
export type CreateOrderRequest = { buyer: Buyer; attendees: OrderAttendeeInput[]; voucherCode?: string };
export type OrderBase = { id: string; reference: string; reservationId: string; status: OrderStatus; expiresAt: string; subtotal: number; adminFee: number; discount: number; total: number; items: OrderItem[] };
export type OrderResponse = OrderBase & { accessToken: string; accessExpiresAt: string | null };
export type OrderDetail = OrderBase & { buyer: Buyer; attendees: Array<{ tierId: string; ticketNumber: number; name: string }>; payment: PaymentSummary | null; createdAt: string; updatedAt: string; eventStartsAt: string; accessExpiresAt: string | null; currentEvent?: EventState; refundRight?: { requested: boolean; deadline: string | null } | null; refund?: { status: string; amount: number } | null };
export type PaymentSummary = { id: string; method: PaymentMethod; amount: number; status: PaymentStatus; paidAt?: string };
export type SimulatePaymentRequest = { method: PaymentMethod; result: PaymentStatus };
export type ApiTicket = { currentEvent?: EventState; usable?: boolean; id: string; code: string; attendeeName: string; orderReference: string; eventId: string; eventArtist: string; eventCity: string; eventVenue: string; eventAddress: string; eventStartsAt: string; tierName: string; gate: string; issuedAt: string };
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
export type TicketCheckInStatus = { status: "CHECKED_IN" | "NOT_CHECKED_IN"; ticket: CheckInTicket; orderStatus: OrderStatus; checkedInAt: string | null; currentEvent?: EventState; canCheckIn?: boolean };
export type CheckInOutcome = "CHECKED_IN" | "TICKET_ALREADY_USED" | "INVALID_REQUEST" | "TICKET_NOT_FOUND" | "ORDER_NOT_PAID" | "WRONG_GATE" | "FORBIDDEN" | "EVENT_CHANGED";
export type CheckInHistoryItem = { id: string; code: string | null; eventId: string | null; eventName: string | null; gate: string | null; staff: { id: string; name: string }; outcome: CheckInOutcome; recordedAt: string; checkedInAt: string | null };
export type CheckInHistoryEvent = { id: string; name: string; gates: string[] };
export type CheckInHistoryPage = { items: CheckInHistoryItem[]; nextCursor: string | null; filterOptions: CheckInHistoryEvent[] };
export type CheckInHistoryFilter = { eventId?: string; gate?: string; q?: string; beforeId?: string };
export type AdminOrder = { id: string; reference: string; status: OrderStatus; eventId: string; eventName: string; buyerName: string; createdAt: string; ticketCount: number; total: number };
export type AdminOrderItem = { tierId: string; name: string; quantity: number; unitPrice: number; lineTotal: number };
export type AdminOrderTicket = { id: string; code: string; attendeeName: string; tierName: string; gate: string; status: "CHECKED_IN" | "NOT_CHECKED_IN"; checkedInAt: string | null; checkedInBy: string | null };
export type AdminOrderDetail = AdminOrder & { buyer: { name: string; email: string; phone: string; identityMasked: string }; subtotal: number; adminFee: number; discount: number; expiresAt: string; items: AdminOrderItem[]; payment: { method: PaymentMethod; amount: number; status: PaymentStatus; paidAt: string | null } | null; tickets: AdminOrderTicket[]; currentEvent?: EventState; refund?: { status: string; amount: number; reason: string; manualReference: string; manualPaidAt: string | null } | null };
export type AdminOrderFilter = { q?: string; eventId?: string; status?: OrderStatus; dateFrom?: string; dateTo?: string; cursor?: string };
export type AdminOrderPage = { items: AdminOrder[]; nextCursor: string | null; filterOptions: { events: Array<{ id: string; name: string }> } };
export type AdminIssueAudit = { action: string; actorName: string; createdAt: string; data: Record<string, unknown> };
export type AdminPaymentCase = { id: string; status: "OPEN" | "RESOLVED"; orderId: string; reference: string; eventName: string; orderStatus: OrderStatus; paymentStatus: PaymentStatus; providerStatus: string; reason: string; amount: number; createdAt: string; updatedAt: string; lastCheckedAt: string | null; lastCheckError: string; checkInProgress: boolean };
export type AdminPaymentCaseDetail = AdminPaymentCase & { total: number; expiresAt: string; gatewayOrderId: string; canRecheck: boolean; canResolve: boolean; notes: AdminIssueAudit[]; history: AdminIssueAudit[] };
export type AdminEmailJob = { id: string; kind: "TICKETS" | "RECOVERY" | "REFUND" | "EVENT_CHANGE"; status: "FAILED" | "PENDING" | "SENT" | "PROCESSING"; reference: string; recipient: string; attempts: number; lastError: string; updatedAt: string; supersededBy: string | null };
export type AdminEmailDetail = AdminEmailJob & { orderStatus: string; canRetry: boolean; retryReason: string; retryJobId: string | null; history: AdminIssueAudit[] };
export type AdminWorkerStatus = { name: string; running: boolean; startedAt: string | null; lastFinishedAt: string | null; lastSuccessAt: string | null; lastFailureAt: string | null; consecutiveFailures: number };
export type AdminOperations = { collectedAt: string; api5xxLast5m: number; failedEmailJobs: number; oldestPendingEmailSeconds: number; openPaymentCases: number; openRefunds: number; heldTickets: number; pendingPayments: number; workers: AdminWorkerStatus[]; alerts: string[] };
export type AdminSalesAmounts = { successfulTransactions: number; paymentAmount: number; refundAmount: number; netAmount: number; unfinishedRefunds: number; openReconciliationCases: number };
export type AdminSalesReport = { period: { eventId: string | null; dateFrom: string; dateTo: string; timeZone: "Asia/Jakarta" }; summary: AdminSalesAmounts; daily: Array<{ date: string } & AdminSalesAmounts>; byEvent: Array<{ id: string; name: string } & AdminSalesAmounts>; filterOptions: { events: Array<{ id: string; name: string }> }; dataUpdatedAt: string };
export type AdminAttendanceAmounts = { capacity: number; available: number; issued: number; eligible: number; heldForRefund: number; checkedIn: number; attendanceRate: number | null };
export type AdminAttendanceReport = { event: { id: string; name: string }; gate: string | null; timeZone: "Asia/Jakarta"; summary: AdminAttendanceAmounts; byCategory: Array<{ id: number; name: string } & AdminAttendanceAmounts>; byGate: Array<{ gate: string | null } & AdminAttendanceAmounts>; hourly: Array<{ hour: string; checkedIn: number }>; filterOptions: { gates: string[] }; dataUpdatedAt: string };
export type ConversionStage = { detail: number; reservation: number; order: number; paymentStarted: number; paymentSucceeded: number };
export type ConversionBreakdown = { eventId?: string; eventName?: string; device?: string; total: ConversionStage; matured: ConversionStage; pendingObservation: number; lost: Record<string, number> };
export type ConversionReport = { period: { eventId: string; device: string; dateFrom: string; dateTo: string; timeZone: "Asia/Jakarta"; observationHours: number }; summary: ConversionBreakdown; byEvent: ConversionBreakdown[]; byDevice: ConversionBreakdown[]; blockers: Array<{ kind: string; reason: string; count: number }>; unattributedReservations: number | null; unattributedPayments: number | null; dataUpdatedAt: string };

export class ApiError extends Error {
  code: string;
  status: number;
  expectedGate?: string;
  checkedInAt?: string;
  ticket?: CheckInTicket;
  resultStatus?: "ALREADY_USED";
  retryAfter?: number;
  constructor(message: string, status: number, code = "API_ERROR", details: Pick<ApiErrorBody, "error" | "status" | "ticket" | "checkedInAt"> = {}) {
    super(message); this.name = "ApiError"; this.status = status; this.code = code;
    this.expectedGate = details.error?.expectedGate;
    this.checkedInAt = details.checkedInAt;
    this.ticket = details.ticket;
    this.resultStatus = details.status === "ALREADY_USED" ? details.status : undefined;
  }
}

async function request<T>(path: string, signal?: AbortSignal, init?: RequestInit, expectedStatus?: number): Promise<T> {
  let response: Response;
  try { response = await fetch(path, { ...init, signal, headers: { Accept: "application/json", ...init?.headers } }); }
  catch (error) { if (error instanceof DOMException && error.name === "AbortError") throw error; throw new ApiError("Tidak dapat terhubung ke server.", 0, "NETWORK_ERROR"); }
  if (response.ok && expectedStatus !== undefined && response.status !== expectedStatus) {
    throw new ApiError("Respons server tidak dapat dipastikan; hasil operasi belum diketahui.", 0, "UNKNOWN_OUTCOME");
  }
  if (!response.ok) {
    throw await apiResponseError(response);
  }
  if (response.status === 204) return undefined as T;
  try { return await response.json() as T; }
  catch (error) {
    if (error instanceof DOMException && error.name === "AbortError") throw error;
    throw new ApiError("Respons server terputus; hasil operasi belum diketahui.", 0, "UNKNOWN_OUTCOME");
  }
}

async function apiResponseError(response: Response): Promise<ApiError> {
  let body: ApiErrorBody = {};
  try { body = await response.json() as ApiErrorBody; } catch { /* malformed error body */ }
  const error = new ApiError(body.error?.message ?? "Terjadi kesalahan pada server.", response.status, body.error?.code ?? "API_ERROR", body);
  const retryAfter = Number(response.headers.get("Retry-After"));
  if (Number.isInteger(retryAfter) && retryAfter > 0) error.retryAfter = retryAfter;
  return error;
}

async function getAdminReportCSV(accessToken: string, path: string, signal?: AbortSignal) {
  let response: Response;
  try {
    response = await fetch(path, { signal, headers: { Accept: "text/csv", ...privateHeaders(accessToken) } });
  } catch (error) {
    if (error instanceof DOMException && error.name === "AbortError") throw error;
    throw new ApiError("Tidak dapat terhubung ke server.", 0, "NETWORK_ERROR");
  }
  if (!response.ok) throw await apiResponseError(response);
  if (!response.headers.get("Content-Type")?.toLowerCase().startsWith("text/csv")) throw new ApiError("Respons ekspor CSV tidak valid.", 0, "INVALID_RESPONSE");
  const disposition = response.headers.get("Content-Disposition") ?? "";
  const filename = disposition.match(/filename="([A-Za-z0-9_.-]+)"/)?.[1] ?? "admin-report.csv";
  return { blob: await response.blob(), filename };
}

export function saveAdminReportCSV(blob: Blob, filename: string) {
  const url = URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = url;
  link.download = filename;
  link.hidden = true;
  document.body.append(link);
  link.click();
  link.remove();
  setTimeout(() => URL.revokeObjectURL(url), 0);
}

export function mapApiEvent(event: ApiEvent): Concert {
  return {
    currentEvent: event.currentEvent, id: event.id, artist: event.artist, city: event.city, venue: event.venue, address: event.address,
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

export function getReservationEvent(reservationId: string, idempotencyKey: string, signal?: AbortSignal) {
  return request<ApiEvent>(`/api/v1/reservations/${encodeURIComponent(reservationId)}/event`, signal, { headers: { "Idempotency-Key": idempotencyKey } }).then(mapApiEvent);
}

export function getAdminEvents(accessToken: string, signal?: AbortSignal) {
  return request<{ events: AdminApiEvent[] }>("/api/v1/admin/events", signal, { headers: privateHeaders(accessToken) }).then((value) => value.events);
}

export function getAdminOrders(accessToken: string, filters: AdminOrderFilter = {}, signal?: AbortSignal) {
  const query = new URLSearchParams();
  for (const [key, value] of Object.entries(filters)) if (value) query.set(key, value);
  return request<AdminOrderPage>(`/api/v1/admin/orders${query.size ? `?${query}` : ""}`, signal, { headers: privateHeaders(accessToken) });
}

export function getAdminOrder(accessToken: string, orderId: string, signal?: AbortSignal) {
  return request<AdminOrderDetail>(`/api/v1/admin/orders/${encodeURIComponent(orderId)}`, signal, { headers: privateHeaders(accessToken) });
}
export function requestAdminRefund(accessToken: string, orderId: string, reason: string) {
  return adminIssueRequest<{ status: string; amount: number; reason: string }>(accessToken, `/api/v1/admin/orders/${encodeURIComponent(orderId)}/refund`, undefined, "POST", { reason });
}

function adminIssueRequest<T>(accessToken: string, path: string, signal?: AbortSignal, method = "GET", body?: unknown) {
  return request<T>(path, signal, { method, headers: { ...privateHeaders(accessToken), ...(body === undefined ? {} : { "Content-Type": "application/json" }) }, ...(body === undefined ? {} : { body: JSON.stringify(body) }) });
}
export function getAdminPaymentCases(accessToken: string, signal?: AbortSignal, cursor?: string) { const query = new URLSearchParams(); if (cursor) query.set("cursor", cursor); return adminIssueRequest<{ items: AdminPaymentCase[]; nextCursor: string | null }>(accessToken, `/api/v1/admin/payment-cases${query.size ? `?${query}` : ""}`, signal); }
export function getAdminPaymentCase(accessToken: string, id: string, signal?: AbortSignal) { return adminIssueRequest<AdminPaymentCaseDetail>(accessToken, `/api/v1/admin/payment-cases/${encodeURIComponent(id)}`, signal); }
export function recheckAdminPaymentCase(accessToken: string, id: string) { return adminIssueRequest<{ caseId: string; providerStatus: string }>(accessToken, `/api/v1/admin/payment-cases/${encodeURIComponent(id)}/recheck`, undefined, "POST"); }
export function addAdminPaymentCaseNote(accessToken: string, id: string, note: string) { return adminIssueRequest<void>(accessToken, `/api/v1/admin/payment-cases/${encodeURIComponent(id)}/notes`, undefined, "POST", { note }); }
export function resolveAdminPaymentCase(accessToken: string, id: string, note: string) { return adminIssueRequest<void>(accessToken, `/api/v1/admin/payment-cases/${encodeURIComponent(id)}/resolve`, undefined, "POST", { note }); }
export function getAdminEmailJobs(accessToken: string, signal?: AbortSignal, cursor?: string) { const query = new URLSearchParams(); if (cursor) query.set("cursor", cursor); return adminIssueRequest<{ items: AdminEmailJob[]; nextCursor: string | null }>(accessToken, `/api/v1/admin/email-jobs${query.size ? `?${query}` : ""}`, signal); }
export function getAdminEmailJob(accessToken: string, id: string, signal?: AbortSignal) { return adminIssueRequest<AdminEmailDetail>(accessToken, `/api/v1/admin/email-jobs/${encodeURIComponent(id)}`, signal); }
export function retryAdminEmailJob(accessToken: string, id: string) { return adminIssueRequest<{ jobId: string; retryJobId: string; status: string }>(accessToken, `/api/v1/admin/email-jobs/${encodeURIComponent(id)}/retry`, undefined, "POST"); }
export function getAdminOperations(accessToken: string, signal?: AbortSignal) { return adminIssueRequest<AdminOperations>(accessToken, "/api/v1/admin/operations", signal); }
export function getAdminSalesReport(accessToken: string, filters: { eventId?: string; dateFrom?: string; dateTo?: string } = {}, signal?: AbortSignal) {
  const query = new URLSearchParams();
  for (const [key, value] of Object.entries(filters)) if (value) query.set(key, value);
  return adminIssueRequest<AdminSalesReport>(accessToken, `/api/v1/admin/reports/sales${query.size ? `?${query}` : ""}`, signal);
}
export function exportAdminSalesReportCSV(accessToken: string, filters: { eventId?: string; dateFrom?: string; dateTo?: string } = {}, signal?: AbortSignal) {
  const query = new URLSearchParams();
  for (const [key, value] of Object.entries(filters)) if (value) query.set(key, value);
  return getAdminReportCSV(accessToken, `/api/v1/admin/reports/sales.csv${query.size ? `?${query}` : ""}`, signal);
}
export function getAdminAttendanceReport(accessToken: string, filters: { eventId: string; gate?: string } , signal?: AbortSignal) {
  const query = new URLSearchParams({ eventId: filters.eventId });
  if (filters.gate) query.set("gate", filters.gate);
  return adminIssueRequest<AdminAttendanceReport>(accessToken, `/api/v1/admin/reports/attendance?${query}`, signal);
}
export function getAdminConversionReport(accessToken: string, filters: { eventId?: string; device?: string; dateFrom?: string; dateTo?: string } = {}, signal?: AbortSignal) {
  const query = new URLSearchParams();
  for (const [key, value] of Object.entries(filters)) if (value) query.set(key, value);
  return adminIssueRequest<ConversionReport>(accessToken, `/api/v1/admin/reports/conversion${query.size ? `?${query}` : ""}`, signal);
}
export function exportAdminAttendanceReportCSV(accessToken: string, filters: { eventId: string; gate?: string }, signal?: AbortSignal) {
  const query = new URLSearchParams({ eventId: filters.eventId });
  if (filters.gate) query.set("gate", filters.gate);
  return getAdminReportCSV(accessToken, `/api/v1/admin/reports/attendance.csv?${query}`, signal);
}

export function createAdminEvent(accessToken: string, payload: AdminEventInput, signal?: AbortSignal) {
  return request<AdminApiEvent>("/api/v1/admin/events", signal, { method: "POST", headers: { ...privateHeaders(accessToken), "Content-Type": "application/json" }, body: JSON.stringify(payload) });
}

export function updateAdminEvent(accessToken: string, id: string, payload: AdminEventInput, signal?: AbortSignal) {
  return request<AdminApiEvent>(`/api/v1/admin/events/${encodeURIComponent(id)}`, signal, { method: "PUT", headers: { ...privateHeaders(accessToken), "Content-Type": "application/json" }, body: JSON.stringify(payload) });
}

export function createAdminZone(accessToken: string, eventId: string, payload: ApiZone) { return request<ApiZone>(`/api/v1/admin/events/${encodeURIComponent(eventId)}/zones`, undefined, { method: "POST", headers: { ...privateHeaders(accessToken), "Content-Type": "application/json" }, body: JSON.stringify(payload) }, 201); }
export function updateAdminZone(accessToken: string, eventId: string, zoneId: string, payload: Omit<ApiZone,"id">) { return request<ApiZone>(`/api/v1/admin/events/${encodeURIComponent(eventId)}/zones/${encodeURIComponent(zoneId)}`, undefined, { method: "PUT", headers: { ...privateHeaders(accessToken), "Content-Type": "application/json" }, body: JSON.stringify(payload) }); }
export type AdminTierInput = Omit<AdminTicketTier,"availableQuantity"|"boundQuantity"|"gateLocked">;
export function createAdminTier(accessToken: string, eventId: string, payload: AdminTierInput) { return request<AdminTicketTier>(`/api/v1/admin/events/${encodeURIComponent(eventId)}/ticket-tiers`, undefined, { method: "POST", headers: { ...privateHeaders(accessToken), "Content-Type": "application/json" }, body: JSON.stringify(payload) }, 201); }
export function updateAdminTier(accessToken: string, eventId: string, tierId: string, payload: Omit<AdminTierInput,"id">) { return request<AdminTicketTier>(`/api/v1/admin/events/${encodeURIComponent(eventId)}/ticket-tiers/${encodeURIComponent(tierId)}`, undefined, { method: "PUT", headers: { ...privateHeaders(accessToken), "Content-Type": "application/json" }, body: JSON.stringify(payload) }); }

export function createReservation(eventId: string, items: Array<{ tierId: string; quantity: number }>, idempotencyKey: string, signal?: AbortSignal, journeyId?: string) {
  return request<Reservation>("/api/v1/reservations", signal, { method: "POST", headers: { "Content-Type": "application/json", "Idempotency-Key": idempotencyKey, ...(journeyId ? { "X-Conversion-Journey": journeyId } : {}) }, body: JSON.stringify({ eventId, items }) });
}

export function getReservation(id: string, signal?: AbortSignal) { return request<Reservation>(`/api/v1/reservations/${encodeURIComponent(id)}`, signal); }

export function convertReservation(id: string, signal?: AbortSignal) { return request<Reservation>(`/api/v1/reservations/${encodeURIComponent(id)}/convert`, signal, { method: "POST" }); }

export function createOrder(reservationId: string, payload: CreateOrderRequest, idempotencyKey: string, signal?: AbortSignal, journeyId?: string) {
  return request<OrderResponse>(`/api/v1/reservations/${encodeURIComponent(reservationId)}/checkout`, signal, { method: "POST", headers: { "Content-Type": "application/json", "Idempotency-Key": idempotencyKey, ...(journeyId ? { "X-Conversion-Journey": journeyId } : {}) }, body: JSON.stringify(payload) });
}

function privateHeaders(accessToken: string): HeadersInit { return { Authorization: `Bearer ${accessToken}` }; }

export function getOrder(orderId: string, accessToken: string, signal?: AbortSignal) {
  return request<OrderDetail>(`/api/v1/orders/${encodeURIComponent(orderId)}`, signal, { headers: privateHeaders(accessToken) });
}

export function simulatePayment(orderId: string, payload: SimulatePaymentRequest, accessToken: string, signal?: AbortSignal) {
  return request<PaymentResult>(`/api/v1/orders/${encodeURIComponent(orderId)}/simulate-payment`, signal, { method: "POST", headers: { ...privateHeaders(accessToken), "Content-Type": "application/json" }, body: JSON.stringify(payload) });
}

export function createSnapPayment(orderId: string, method: PaymentMethod, returnUrl: string, accessToken: string, signal?: AbortSignal) {
  return request<{ redirectUrl: string }>(`/api/v1/orders/${encodeURIComponent(orderId)}/payments`, signal, { method: "POST", headers: { ...privateHeaders(accessToken), "Content-Type": "application/json" }, body: JSON.stringify({ method, returnUrl }) });
}

export async function listOrderTickets(orderId: string, accessToken: string, signal?: AbortSignal) {
  return (await request<{ tickets: ApiTicket[] }>(`/api/v1/orders/${encodeURIComponent(orderId)}/tickets`, signal, { headers: privateHeaders(accessToken) })).tickets;
}

export function getTicket(ticketId: string, accessToken: string, signal?: AbortSignal) {
  return request<ApiTicket>(`/api/v1/tickets/${encodeURIComponent(ticketId)}`, signal, { headers: privateHeaders(accessToken) });
}

export function requestTicketRecovery(email: string, reference: string, signal?: AbortSignal) {
  return request<{ message: string }>("/api/v1/ticket-recovery", signal, { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ email, reference }) }, 202);
}

export function verifyTicketRecovery(token: string, signal?: AbortSignal) {
  return request<unknown>("/api/v1/ticket-recovery/verify", signal, { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ token }) }, 200).then((result) => {
    if (!isRecoveryResult(result)) throw new ApiError("Respons pemulihan tidak lengkap; hasil belum diketahui.", 0, "UNKNOWN_OUTCOME");
    return result;
  });
}

export function resendOrderEmail(orderId: string, accessToken: string, signal?: AbortSignal) {
  return request<{ message: string }>(`/api/v1/orders/${encodeURIComponent(orderId)}/resend-email`, signal, { method: "POST", headers: privateHeaders(accessToken) }, 202);
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
  return request<void>("/api/v1/staff/logout", signal, { method: "POST", headers: staffHeaders(accessToken) }, 204);
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
  return request<unknown>("/api/v1/staff/check-ins", signal, staffJSON(accessToken, payload), 201).then((result) => {
    if (!isCheckInResult(result, payload)) throw new ApiError("Respons check-in tidak lengkap; hasil belum diketahui.", 0, "UNKNOWN_OUTCOME");
    return result;
  });
}

export function getStaffTicketStatus(accessToken: string, code: string, signal?: AbortSignal) {
  return request<unknown>(`/api/v1/staff/ticket-status?code=${encodeURIComponent(code)}`, signal, { headers: staffHeaders(accessToken) }).then((result) => {
    if (!isTicketCheckInStatus(result)) throw new ApiError("Respons status tiket tidak valid.", 0, "INVALID_RESPONSE");
    return result;
  });
}

export function getAdminCheckInHistory(accessToken: string, filters: CheckInHistoryFilter = {}, signal?: AbortSignal) {
  const query = new URLSearchParams();
  for (const key of ["eventId", "gate", "q", "beforeId"] as const) {
    const value = filters[key];
    if (value) query.set(key, value);
  }
  return request<unknown>(`/api/v1/admin/check-ins${query.size ? `?${query}` : ""}`, signal, { headers: staffHeaders(accessToken) }).then((result) => {
    if (!isCheckInHistoryPage(result)) throw new ApiError("Respons riwayat check-in tidak valid.", 0, "INVALID_RESPONSE");
    return result;
  });
}

function isCheckInHistoryPage(value: unknown): value is CheckInHistoryPage {
  if (!value || typeof value !== "object") return false;
  const page = value as Partial<CheckInHistoryPage>;
  const outcomes: CheckInOutcome[] = ["CHECKED_IN", "TICKET_ALREADY_USED", "INVALID_REQUEST", "TICKET_NOT_FOUND", "ORDER_NOT_PAID", "WRONG_GATE", "FORBIDDEN", "EVENT_CHANGED"];
  return Array.isArray(page.items) && (page.nextCursor === null || (typeof page.nextCursor === "string" && /^[1-9]\d*$/.test(page.nextCursor))) && Array.isArray(page.filterOptions) &&
    page.items.every((item) => Boolean(item && typeof item.id === "string" && /^\d+$/.test(item.id) &&
      (item.code === null || (typeof item.code === "string" && /^ET-[0-9A-F]{32}$/.test(item.code))) &&
      (item.eventId === null || typeof item.eventId === "string") && (item.eventName === null || typeof item.eventName === "string") &&
      (item.gate === null || typeof item.gate === "string") && typeof item.staff?.id === "string" && typeof item.staff.name === "string" &&
      outcomes.includes(item.outcome) && typeof item.recordedAt === "string" && Number.isFinite(Date.parse(item.recordedAt)) &&
      ((item.outcome === "CHECKED_IN" || item.outcome === "TICKET_ALREADY_USED")
        ? typeof item.checkedInAt === "string" && Number.isFinite(Date.parse(item.checkedInAt))
        : item.checkedInAt === null))) &&
    page.filterOptions.every((event) => Boolean(event && typeof event.id === "string" && typeof event.name === "string" && Array.isArray(event.gates) && event.gates.every((gate) => typeof gate === "string")));
}

function isTicketCheckInStatus(value: unknown): value is TicketCheckInStatus {
  if (!value || typeof value !== "object") return false;
  const result = value as Partial<TicketCheckInStatus>;
  const ticket = result.ticket;
  if (!ticket || typeof ticket !== "object" || !["PENDING", "PAID", "CANCELLED", "EXPIRED", "REFUND_PENDING", "REFUNDED"].includes(result.orderStatus ?? "")) return false;
  const checkedIn = result.status === "CHECKED_IN";
  if ((!checkedIn && result.status !== "NOT_CHECKED_IN") || (checkedIn && (typeof result.checkedInAt !== "string" || !Number.isFinite(Date.parse(result.checkedInAt)))) || (!checkedIn && result.checkedInAt !== null)) return false;
  const candidate = ticket as Partial<CheckInTicket>;
  return typeof candidate.id === "string" && /^[0-9a-f]{32}$/.test(candidate.id) && typeof candidate.code === "string" &&
    candidate.code === `ET-${candidate.id.toUpperCase()}` && typeof candidate.attendeeName === "string" && Boolean(candidate.attendeeName.trim()) &&
    typeof candidate.tierName === "string" && Boolean(candidate.tierName.trim()) && typeof candidate.eventId === "string" &&
    typeof candidate.gate === "string" && Boolean(candidate.gate.trim());
}

function isCheckInResult(value: unknown, request: CheckInRequest): value is CheckInResult {
  if (!value || typeof value !== "object") return false;
  const result = value as Partial<CheckInResult>;
  const ticket = result.ticket;
  if (result.status !== "CHECKED_IN" || typeof result.checkedInAt !== "string" || !Number.isFinite(Date.parse(result.checkedInAt)) || !ticket || typeof ticket !== "object") return false;
  const candidate = ticket as Partial<CheckInTicket>;
  const ticketID = request.code.trim().toLowerCase().match(/^et-([0-9a-f]{32})$/)?.[1];
  return Boolean(ticketID && candidate.id === ticketID && typeof candidate.code === "string" && candidate.code.toUpperCase() === request.code.trim().toUpperCase() &&
    candidate.eventId === request.eventId && candidate.gate === request.gate &&
    typeof candidate.attendeeName === "string" && candidate.attendeeName.trim() &&
    typeof candidate.tierName === "string" && candidate.tierName.trim());
}

export type EventChangeInput = { action: "POSTPONED" | "RESCHEDULED" | "CANCELLED"; reason: string; announcement: string; startsAt: string | null; refundDeadline: string | null; expectedVersion: number; snapshot: string };
export type EventChangeImpact = { activeReservations: number; pendingOrders: number; pendingPayments: number; paidOrders: number; issuedTickets: number; checkIns: number; existingRefunds: number; automaticAmount: number; manualAmount: number };
export type EventChangePreview = { event: EventState; snapshot: string; impact: EventChangeImpact };
export type EventChange = { id: string; version: number; action: EventChangeInput["action"]; reason: string; announcement: string; previousStartsAt: string | null; startsAt: string | null; refundDeadline: string | null; staffId: string; createdAt: string; remainingOrders: number; errors: number };
export function previewEventChange(token: string, id: string, input: EventChangeInput, signal?: AbortSignal) {
  return request<EventChangePreview>(`/api/v1/admin/events/${encodeURIComponent(id)}/changes/preview`, signal, staffJSON(token, input));
}
export function commitEventChange(token: string, id: string, input: EventChangeInput, key: string) {
  const init = staffJSON(token, input);
  return request<{ id: string; version: number }>(`/api/v1/admin/events/${encodeURIComponent(id)}/changes`, undefined, { ...init, headers: { ...init.headers, "Idempotency-Key": key } });
}
export function getEventChanges(token: string, id: string, signal?: AbortSignal) {
  return request<EventChange[]>(`/api/v1/admin/events/${encodeURIComponent(id)}/changes`, signal, { headers: staffHeaders(token) });
}
export function requestEventRefund(id: string, token: string) {
  return request<{ status: string }>(`/api/v1/orders/${encodeURIComponent(id)}/refund-request`, undefined, { method: "POST", headers: privateHeaders(token) });
}
export function completeManualRefund(token: string, id: string, input: { reference: string; paidAt: string; note: string }) {
  return request<{ status: string; amount: number }>(`/api/v1/admin/orders/${encodeURIComponent(id)}/refund/manual`, undefined, staffJSON(token, input));
}
