import type { Concert, TicketTier } from "./concerts.ts";

export type TicketLine = { tierName: string; quantity: number; gate: string; allocation: string };
export type TicketSnapshot = { version: 1; id: string; reference: string; issuedAt: string; attendeeName: string; concert: Pick<Concert, "id" | "artist" | "venue" | "city" | "address" | "startsAt" | "image">; lines: TicketLine[] };

export function ticketStorageKey(id: string) { return `ticket-online:ticket:${id}`; }
const ticketKeyPrefix = "ticket-online:ticket:";
function getStorage(name: "localStorage" | "sessionStorage") { try { return globalThis[name]; } catch { return null; } }
function readStorage(storage: Storage | null, key: string) { try { return storage?.getItem(key) ?? null; } catch { return null; } }
export function loadTicketSnapshot(id: string) {
  const key = ticketStorageKey(id);
  const localStorage = getStorage("localStorage");
  const sessionStorage = getStorage("sessionStorage");
  const local = parseTicketSnapshot(readStorage(localStorage, key));
  if (local?.id === id) return local;
  const session = parseTicketSnapshot(readStorage(sessionStorage, key));
  if (session?.id !== id) return null;
  try { localStorage?.setItem(key, JSON.stringify(session)); } catch { /* localStorage may be unavailable */ }
  return session;
}
export function listTicketSnapshots() {
  const tickets = new Map<string, TicketSnapshot>();
  for (const storage of [getStorage("localStorage"), getStorage("sessionStorage")]) {
    if (!storage) continue;
    try {
      for (let index = 0; index < storage.length; index += 1) {
        const key = storage.key(index);
        if (!key?.startsWith(ticketKeyPrefix)) continue;
        const ticket = parseTicketSnapshot(storage.getItem(key));
        if (ticket?.id === key.slice(ticketKeyPrefix.length)) tickets.set(ticket.id, ticket);
      }
    } catch { /* storage access can fail in private browsing */ }
  }
  return [...tickets.values()].sort((a, b) => Date.parse(a.concert.startsAt) - Date.parse(b.concert.startsAt));
}
export function ticketAllocation(tier: TicketTier, quantity: number, reference: string) {
  if (tier.seating === "free-standing") return "Berdiri bebas";
  const seed = [...reference].reduce((total, character) => total + character.charCodeAt(0), 0);
  const row = String.fromCharCode(65 + seed % 6);
  const start = 10 + seed % 70;
  return `Row ${row}, Kursi ${start}-${start + quantity - 1}`;
}
export function createTicketSnapshot(id: string, reference: string, attendeeName: string, concert: Concert, lines: Array<{ tier: TicketTier; quantity: number }>): TicketSnapshot {
  return { version: 1, id, reference, issuedAt: new Date().toISOString(), attendeeName, concert: { id: concert.id, artist: concert.artist, venue: concert.venue, city: concert.city, address: concert.address, startsAt: concert.startsAt, image: concert.image }, lines: lines.map(({ tier, quantity }) => ({ tierName: tier.name, quantity, gate: tier.gate, allocation: ticketAllocation(tier, quantity, reference) })) };
}
export function parseTicketSnapshot(value: string | null): TicketSnapshot | null {
  try {
    const ticket = JSON.parse(value ?? "") as TicketSnapshot;
    const text = (candidate: unknown): candidate is string => typeof candidate === "string" && candidate.trim().length > 0;
    const concert = ticket?.concert;
    if (ticket?.version !== 1 || !text(ticket.id) || !/^[\w-]{8,}$/.test(ticket.id) || !text(ticket.reference) || !/^TO-[A-Z0-9]{10}$/.test(ticket.reference) || !text(ticket.issuedAt) || !Number.isFinite(Date.parse(ticket.issuedAt)) || !text(ticket.attendeeName) || !text(concert?.id) || !text(concert?.artist) || !text(concert?.venue) || !text(concert?.city) || !text(concert?.address) || !text(concert?.startsAt) || !Number.isFinite(Date.parse(concert.startsAt)) || !text(concert?.image) || !Array.isArray(ticket.lines) || !ticket.lines.length || ticket.lines.some((line) => !line || typeof line !== "object" || !text(line.tierName) || !text(line.gate) || !text(line.allocation) || !Number.isInteger(line.quantity) || line.quantity < 1 || line.quantity > 6)) return null;
    return ticket;
  } catch { return null; }
}
export function timeRemaining(startsAt: string, now = Date.now()) {
  const milliseconds = Date.parse(startsAt) - now;
  if (!Number.isFinite(milliseconds) || milliseconds <= 0) return { days: 0, hours: 0, minutes: 0, seconds: 0, started: true };
  const seconds = Math.floor(milliseconds / 1_000);
  return { days: Math.floor(seconds / 86_400), hours: Math.floor(seconds % 86_400 / 3_600), minutes: Math.floor(seconds % 3_600 / 60), seconds: seconds % 60, started: false };
}
