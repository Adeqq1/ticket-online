import { expect, test } from "bun:test";
import { scanTicket, type ScanTicket } from "./src/lib/scanner.ts";
import { createTicketSnapshot, listTicketSnapshots, saveTicketSnapshot } from "./src/lib/tickets.ts";
import { getConcertById } from "./src/lib/concerts.ts";

const tickets = (): ScanTicket[] => [{ id: "TO-ALPHA-2027", attendee: "Ayu Pertiwi", ticketType: "Festival", event: "Nusa Malam", gate: "Gate A", isUsed: false }];

test("accepts a valid ticket once, then rejects it as used", () => {
  const state = tickets();
  expect(scanTicket(state, " to-alpha-2027 ", "Gate A")).toMatchObject({ status: "valid", ticket: { isUsed: true } });
  expect(scanTicket(state, "TO-ALPHA-2027", "Gate A")).toMatchObject({ status: "used", ticket: { attendee: "Ayu Pertiwi" } });
  expect(scanTicket(state, "unknown", "Gate A")).toEqual({ status: "not-found", ticket: null });
});

test("rejects another gate without consuming the ticket", () => {
  const ticket = tickets()[0]!;
  const state: ScanTicket[] = [{ ...ticket, gate: "Gate C" }];
  expect(scanTicket(state, ticket.id, "Gate A")).toMatchObject({ status: "wrong-gate", ticket: { isUsed: false } });
  expect(scanTicket(state, ticket.id, "Gate C")).toMatchObject({ status: "valid", ticket: { isUsed: true } });
});

test("scans a validated checkout reference and rejects its second scan", () => {
  const concert = getConcertById("nusa-malam")!;
  const snapshot = createTicketSnapshot("checkout-ticket", "TO-ABCDEFGHIJ", "Dewi Lestari", concert, [{ tier: concert.ticketTiers[0]!, quantity: 1 }]);
  const storage = new Map<string, string>();
  const memoryStorage = { get length() { return storage.size; }, key: (index: number) => [...storage.keys()][index] ?? null, getItem: (key: string) => storage.get(key) ?? null, setItem: (key: string, value: string) => { storage.set(key, value); }, removeItem: (key: string) => { storage.delete(key); }, clear: () => storage.clear() } as Storage;
  saveTicketSnapshot(snapshot, memoryStorage, null);
  const [ticket] = listTicketSnapshots([memoryStorage]).map((saved) => ({ id: saved.reference, attendee: saved.attendeeName, ticketType: saved.lines[0]!.tierName, event: saved.concert.artist, gate: saved.lines[0]!.gate, isUsed: false }));
  expect(scanTicket([ticket!], snapshot.reference, "Gate A").status).toBe("valid");
  expect(scanTicket([ticket!], snapshot.reference, "Gate A").status).toBe("used");
});
