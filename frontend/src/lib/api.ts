import { eventDate, type Concert, type Genre, type StageZone, type TicketStatus, type TicketTier } from "./concerts.ts";

export type ApiZone = { id: string; name: string; description: string };
export type ApiTicketTier = { id: string; name: string; zoneId: string; price: number; availableQuantity: number; maxPerOrder: number; benefit: string; gate: string; seating: "assigned" | "free-standing" };
export type ApiEvent = { id: string; artist: string; city: string; venue: string; address: string; startsAt: string; genre: string; status: string; image: string; description: string; lineup: string[]; price: number; zones: ApiZone[]; ticketTiers: ApiTicketTier[] };
export type ApiErrorBody = { error?: { code?: string; message?: string } };
export type ReservationItem = { tierId: string; name: string; quantity: number; unitPrice: number; lineTotal: number };
export type Reservation = { id: string; status: string; expiresAt: string; event: { id: string; artist: string }; items: ReservationItem[]; subtotal: number };

export class ApiError extends Error {
  code: string;
  status: number;
  constructor(message: string, status: number, code = "API_ERROR") { super(message); this.name = "ApiError"; this.status = status; this.code = code; }
}

async function request<T>(path: string, signal?: AbortSignal, init?: RequestInit): Promise<T> {
  let response: Response;
  try { response = await fetch(path, { ...init, signal, headers: { Accept: "application/json", ...init?.headers } }); }
  catch (error) { if (error instanceof DOMException && error.name === "AbortError") throw error; throw new ApiError("Tidak dapat terhubung ke server.", 0, "NETWORK_ERROR"); }
  if (!response.ok) {
    let body: ApiErrorBody = {};
    try { body = await response.json() as ApiErrorBody; } catch { /* malformed error body */ }
    throw new ApiError(body.error?.message ?? "Terjadi kesalahan pada server.", response.status, body.error?.code ?? "API_ERROR");
  }
  return response.json() as Promise<T>;
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
