import { expect, test } from "bun:test";
import { findOrderForActiveReservation } from "./checkout-recovery.ts";
import { getOrderAccessForTicket, hasPersistentTicketAccess, parseOrderAccess, removeOrderAccess, saveOrderAccess } from "./order-access.ts";
import { isRecoveryResult, recoveredOrderAccess, recoveryToken, type RecoveryResult } from "./ticket-recovery.ts";

const result: RecoveryResult = {
  orderId: "a".repeat(32), accessToken: "B".repeat(43), reference: `TO-${"c".repeat(20)}`, reservationId: "d".repeat(32),
  expiresAt: "2026-10-06T10:00:00Z", accessExpiresAt: "2026-10-08T17:00:00Z", ticketIds: ["e".repeat(32)],
};

test("recovery token is read once from the fragment", () => {
  const token = "A".repeat(43);
  expect(recoveryToken(`#token=${token}`)).toBe(token);
  expect(recoveryToken(`#token=${token}&token=${token}`)).toBeNull();
  expect(recoveryToken("#token=short")).toBeNull();
  expect(recoveryToken(`#access_token=${token}`)).toBeNull();
});

test("verify response is validated before it becomes order access", () => {
  expect(isRecoveryResult(result)).toBe(true);
  for (const broken of [{ ...result, orderId: "x" }, { ...result, accessToken: "" }, { ...result, ticketIds: [] }, { ...result, ticketIds: ["../x"] }, { ...result, accessExpiresAt: "soon" }, null]) {
    expect(isRecoveryResult(broken)).toBe(false);
  }
});

test("recovered record round-trips storage and is never used by checkout", () => {
  const record = recoveredOrderAccess(result);
  expect(record).not.toHaveProperty("idempotencyKey");
  expect(parseOrderAccess(JSON.stringify(record))).toEqual(record);
  expect(parseOrderAccess(JSON.stringify({ ...record, idempotencyKey: "only-one-field" }))).toBeNull();
  expect(findOrderForActiveReservation([record], { reservationId: result.reservationId, idempotencyKey: "k", eventId: "e", basketKey: "b" }, "b")).toBeNull();
});

test("a recovered order is usable on a browser with empty storage", () => {
  const original = Object.getOwnPropertyDescriptor(globalThis, "localStorage");
  const values = new Map<string, string>();
  Object.defineProperty(globalThis, "localStorage", {
    configurable: true,
    value: {
      getItem: (key: string) => values.get(key) ?? null,
      setItem: (key: string, value: string) => values.set(key, value),
      removeItem: (key: string) => values.delete(key),
      key: (index: number) => [...values.keys()][index] ?? null,
      get length() { return values.size; },
    },
  });
  try {
    const record = recoveredOrderAccess(result);
    expect(saveOrderAccess(record)).toBe(true);
    expect(values.size).toBe(1);
    expect(hasPersistentTicketAccess(result.orderId, result.ticketIds[0]!)).toBe(true);
    expect(getOrderAccessForTicket(result.ticketIds[0]!)?.accessToken).toBe(result.accessToken);
    expect(getOrderAccessForTicket("f".repeat(32))).toBeNull();
  } finally {
    removeOrderAccess(result.orderId);
    if (original) Object.defineProperty(globalThis, "localStorage", original);
    else Reflect.deleteProperty(globalThis, "localStorage");
  }
});
