import { expect, test } from "bun:test";
import { getConcertById } from "./concerts.ts";
import { createTicketSnapshot, parseTicketSnapshot, ticketAllocation, timeRemaining } from "./ticket.ts";

test("creates minimal ticket snapshot with stable allocations", () => {
  const concert = getConcertById("nusa-malam")!;
  const tier = concert.ticketTiers.find((item) => item.id === "tribune")!;
  const snapshot = createTicketSnapshot("a1b2c3d4-e5f6", "TO-ABCDEFGHIJ", "Ayu Pertiwi", concert, [{ tier, quantity: 2 }]);
  expect(snapshot.lines[0]).toEqual({ tierName: "Tribune", quantity: 2, gate: "Gate C", allocation: ticketAllocation(tier, 2, "TO-ABCDEFGHIJ") });
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
  expect(timeRemaining("2027-01-02T00:00:00Z", Date.parse("2027-01-01T00:00:00Z"))).toMatchObject({ days: 1, hours: 0, started: false });
  expect(timeRemaining("2027-01-01T00:00:00Z", Date.parse("2027-01-01T00:00:00Z")).started).toBe(true);
});
