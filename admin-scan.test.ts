import { expect, test } from "bun:test";
import { scanTicket, type ScanTicket } from "./src/lib/scanner.ts";

const tickets = (): ScanTicket[] => [{ id: "TO-ALPHA-2027", attendee: "Ayu Pertiwi", ticketType: "Festival", event: "Nusa Malam", gate: "Gate A", isUsed: false }];

test("accepts a valid ticket once, then rejects it as used", () => {
  const state = tickets();
  expect(scanTicket(state, " to-alpha-2027 ")).toMatchObject({ status: "valid", ticket: { isUsed: true } });
  expect(scanTicket(state, "TO-ALPHA-2027")).toMatchObject({ status: "used", ticket: { attendee: "Ayu Pertiwi" } });
  expect(scanTicket(state, "unknown")).toEqual({ status: "not-found", ticket: null });
});
