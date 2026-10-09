import { expect, test } from "bun:test";
import { canBuy, currentStart } from "./event-changes.ts";
import { parseOrderAccess } from "./order-access.ts";
import { isRecoveryResult } from "./ticket-recovery.ts";
import type { EventState } from "./concerts.ts";

test("old schedules never replace an unknown effective date and held access remains recoverable", () => {
  const state: EventState = { id: "event", status: "POSTPONED", version: 1, salesPaused: true, startsAt: null, announcement: "Ditunda", refundDeadline: null };
  expect(canBuy(state)).toBe(false);
  expect(currentStart({ currentEvent: state, eventStartsAt: "2026-10-09T12:00:00Z" })).toBeNull();
  expect(canBuy({ ...state, status: "RESCHEDULED", salesPaused: false })).toBe(true);
  const access = { orderId: "a".repeat(32), reservationId: "b".repeat(32), accessToken: "c".repeat(43), reference: "TO-" + "d".repeat(20), expiresAt: "2026-10-09T12:00:00Z", accessExpiresAt: null, ticketIds: [], changedEvent: true };
  expect(parseOrderAccess(JSON.stringify(access))?.accessExpiresAt).toBeNull();
  expect(isRecoveryResult(access)).toBe(true);
  expect(isRecoveryResult({ ...access, changedEvent: false })).toBe(false);
  expect(isRecoveryResult({ ...access, accessExpiresAt: "invalid" })).toBe(false);
});
