import { expect, test } from "bun:test";
import { getOrderAccess, getOrderAccessForTicket, listOrderAccess, parseOrderAccess, saveOrderAccess } from "./src/lib/order-access.ts";

const record = { orderId: "order-1", accessToken: "private-token", expiresAt: "2027-08-24T12:40:00Z", accessExpiresAt: "2027-08-25T00:00:00Z", reservationId: "reservation-1", idempotencyKey: "stable-key", basketKey: "festival=1", reference: "TO-ABCDEFGHIJ", ticketIds: ["ticket-1"] };

test("persists order access in localStorage and indexes tickets without putting tokens in URLs", () => {
  const original = Object.getOwnPropertyDescriptor(globalThis, "localStorage");
  const values = new Map<string, string>();
  const storage = { get length() { return values.size; }, key: (index: number) => [...values.keys()][index] ?? null, getItem: (key: string) => values.get(key) ?? null, setItem: (key: string, value: string) => { values.set(key, value); }, removeItem: (key: string) => { values.delete(key); }, clear: () => values.clear() } as Storage;
  Object.defineProperty(globalThis, "localStorage", { configurable: true, value: storage });
  try {
    expect(saveOrderAccess(record)).toBe(true);
    expect(getOrderAccess("order-1")).toEqual(record);
    expect(getOrderAccessForTicket("ticket-1")).toEqual(record);
    expect(listOrderAccess()).toEqual([record]);
    expect([...values.values()].join()).toContain("private-token");
    expect(parseOrderAccess(JSON.stringify({ ...record, expiresAt: "invalid" }))).toBeNull();
  } finally {
    if (original) Object.defineProperty(globalThis, "localStorage", original);
    else delete (globalThis as { localStorage?: Storage }).localStorage;
  }
});

test("keeps order access in memory when localStorage cannot persist it", () => {
  const original = Object.getOwnPropertyDescriptor(globalThis, "localStorage");
  const unavailable = { get length() { throw new Error("blocked"); }, key: () => null, getItem: () => null, setItem() { throw new Error("blocked"); }, removeItem() {}, clear() {} } as unknown as Storage;
  Object.defineProperty(globalThis, "localStorage", { configurable: true, value: unavailable });
  try {
    expect(saveOrderAccess(record)).toBe(false);
    expect(getOrderAccess("order-1")).toEqual(record);
    expect(listOrderAccess().find(({ orderId }) => orderId === "order-1")).toEqual(record);
  } finally {
    if (original) Object.defineProperty(globalThis, "localStorage", original);
    else delete (globalThis as { localStorage?: Storage }).localStorage;
  }
});
