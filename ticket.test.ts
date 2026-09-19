import { expect, test } from "bun:test";
import { eventDate, getConcertById } from "./src/lib/concerts.ts";
import { createTicketSnapshot, listTicketSnapshots, parseTicketSnapshot, saveTicketSnapshot, ticketAllocation, timeRemaining } from "./src/lib/tickets.ts";

class FakeStorage implements Storage {
  private values = new Map<string, string>();
  get length() { return this.values.size; }
  clear() { this.values.clear(); }
  getItem(key: string) { return this.values.get(key) ?? null; }
  key(index: number) { return [...this.values.keys()][index] ?? null; }
  removeItem(key: string) { this.values.delete(key); }
  setItem(key: string, value: string) { this.values.set(key, value); }
}

test("creates minimal ticket snapshot with stable allocations", () => {
  const concert = getConcertById("nusa-malam")!;
  const tier = concert.ticketTiers.find((item) => item.id === "tribune")!;
  const snapshot = createTicketSnapshot("a1b2c3d4-e5f6", "TO-ABCDEFGHIJ", "Ayu Pertiwi", concert, [{ tier, quantity: 2 }]);
  expect(snapshot.lines[0]).toEqual({ tierName: "Tribune", quantity: 2, gate: "Gate C", allocation: ticketAllocation(tier, 2, "TO-ABCDEFGHIJ") });
  expect(eventDate(concert.startsAt)).toContain("Selasa, 24 Agustus 2027");
  expect(JSON.stringify(snapshot)).not.toContain("identity");
  expect(JSON.stringify(snapshot)).not.toContain("payment");
  expect(ticketAllocation(concert.ticketTiers.find((item) => item.id === "festival")!, 2, "TO-ABCDEFGHIJ")).toBe("Berdiri bebas");
});

test("validates stored snapshots and calculates countdown boundaries", () => {
  const concert = getConcertById("nusa-malam")!;
  const snapshot = createTicketSnapshot("a1b2c3d4-e5f6", "TO-ABCDEFGHIJ", "Ayu Pertiwi", concert, [{ tier: concert.ticketTiers[0]!, quantity: 1 }]);
  expect(parseTicketSnapshot(JSON.stringify(snapshot))?.reference).toBe("TO-ABCDEFGHIJ");
  expect(parseTicketSnapshot("{bad json")).toBeNull();
  expect(parseTicketSnapshot(JSON.stringify({ ...snapshot, version: 2 }))).toBeNull();
  expect(parseTicketSnapshot(JSON.stringify({ ...snapshot, attendeeName: [] }))).toBeNull();
  expect(parseTicketSnapshot(JSON.stringify({ ...snapshot, concert: { ...snapshot.concert, city: {} } }))).toBeNull();
  expect(parseTicketSnapshot(JSON.stringify({ ...snapshot, issuedAt: "invalid" }))).toBeNull();
  expect(parseTicketSnapshot(JSON.stringify({ ...snapshot, lines: [{ ...snapshot.lines[0], quantity: 7 }] }))).toBeNull();
  expect(timeRemaining("2027-01-02T00:00:00Z", Date.parse("2027-01-01T00:00:00Z"))).toMatchObject({ days: 1, hours: 0, started: false });
  expect(timeRemaining("2027-01-01T00:00:00Z", Date.parse("2027-01-01T00:00:00Z")).started).toBe(true);
});

test("falls back to session storage when local storage cannot write", () => {
  const concert = getConcertById("nusa-malam")!;
  const snapshot = createTicketSnapshot("fallback-ticket", "TO-ABCDEFGHIJ", "Ayu Pertiwi", concert, [{ tier: concert.ticketTiers[0]!, quantity: 1 }]);
  const local = new FakeStorage();
  const session = new FakeStorage();
  const blockedLocal = { ...local, setItem() { throw new Error("quota"); } } as unknown as Storage;
  expect(saveTicketSnapshot(snapshot, blockedLocal, session)).toBe(true);
  expect(session.getItem("ticket-online:ticket:fallback-ticket")).toBe(JSON.stringify(snapshot));
});

test("lists and migrates valid session tickets without overwriting local tickets", () => {
  const concert = getConcertById("nusa-malam")!;
  const sessionTicket = createTicketSnapshot("session-ticket", "TO-ABCDEFGHIJ", "Ayu Pertiwi", concert, [{ tier: concert.ticketTiers[0]!, quantity: 1 }]);
  const local = new FakeStorage();
  const session = new FakeStorage();
  session.setItem("ticket-online:ticket:session-ticket", JSON.stringify(sessionTicket));
  expect(listTicketSnapshots([local, session]).map((ticket) => ticket.id)).toEqual(["session-ticket"]);
  expect(local.getItem("ticket-online:ticket:session-ticket")).toBe(JSON.stringify(sessionTicket));

  const localTicket = createTicketSnapshot("local-ticket", "TO-ZYXWVUTSRQ", "Ayu Pertiwi", concert, [{ tier: concert.ticketTiers[0]!, quantity: 1 }]);
  local.setItem("ticket-online:ticket:local-ticket", JSON.stringify(localTicket));
  const newerSessionTicket = { ...localTicket, attendeeName: "Session copy" };
  session.setItem("ticket-online:ticket:local-ticket", JSON.stringify(newerSessionTicket));
  expect(listTicketSnapshots([local, session]).find((ticket) => ticket.id === "local-ticket")?.attendeeName).toBe("Ayu Pertiwi");
});
