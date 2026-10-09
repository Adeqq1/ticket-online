import { expect, test } from "bun:test";
import { getOrderAccess, getOrderAccessForTicket, hasPersistentOrderAccess, hasPersistentTicketAccess, listOrderAccess, parseOrderAccess, removeOrderAccess, saveOrderAccess, type OrderAccess } from "./order-access.ts";

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
    removeOrderAccess(record.orderId);
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
    removeOrderAccess(record.orderId);
    if (original) Object.defineProperty(globalThis, "localStorage", original);
    else delete (globalThis as { localStorage?: Storage }).localStorage;
  }
});

test("a ticket link requires that exact ticket ID in persistent order access", () => {
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

  const record: OrderAccess = {
    orderId: "order-persisted",
    accessToken: "secret-token",
    expiresAt: "2026-10-05T12:00:00Z",
    accessExpiresAt: "2026-10-06T12:00:00Z",
    reservationId: "reservation-1",
    idempotencyKey: "key-1",
    basketKey: "basket-1",
    reference: "TO-ORDER1",
    ticketIds: [],
  };

  try {
    values.set(`ticket-online:order:${record.orderId}`, JSON.stringify(record));
    expect(hasPersistentTicketAccess(record.orderId, "ticket-1")).toBe(false);
    values.set(`ticket-online:order:${record.orderId}`, JSON.stringify({ ...record, ticketIds: ["ticket-1"] }));
    expect(hasPersistentTicketAccess(record.orderId, "ticket-1")).toBe(true);
    expect(hasPersistentTicketAccess("another-order", "ticket-1")).toBe(false);
  } finally {
    removeOrderAccess(record.orderId);
    if (original) Object.defineProperty(globalThis, "localStorage", original);
    else Reflect.deleteProperty(globalThis, "localStorage");
  }
});

test("tab storage preserves order recovery when local storage is unavailable", () => {
  const originalLocal = Object.getOwnPropertyDescriptor(globalThis, "localStorage");
  const originalSession = Object.getOwnPropertyDescriptor(globalThis, "sessionStorage");
  const values = new Map<string, string>();
  const session = {
    getItem: (key: string) => values.get(key) ?? null,
    setItem: (key: string, value: string) => values.set(key, value),
    removeItem: (key: string) => values.delete(key),
    key: (index: number) => [...values.keys()][index] ?? null,
    get length() { return values.size; },
  };
  Object.defineProperty(globalThis, "localStorage", { configurable: true, value: { setItem() { throw new Error("blocked"); }, getItem() { throw new Error("blocked"); } } });
  Object.defineProperty(globalThis, "sessionStorage", { configurable: true, value: session });
  const record: OrderAccess = {
    orderId: "order-session-fallback", accessToken: "secret-token", expiresAt: "2026-10-05T12:00:00Z",
    accessExpiresAt: "2026-10-06T12:00:00Z", reservationId: "reservation-1", idempotencyKey: "key-1",
    basketKey: "basket-1", reference: "TO-ORDER1", ticketIds: ["ticket-1"],
  };
  try {
    expect(saveOrderAccess(record)).toBe(true);
    expect(hasPersistentOrderAccess(record.orderId)).toBe(true);
    expect(hasPersistentTicketAccess(record.orderId, "ticket-1")).toBe(true);
    expect(listOrderAccess()).toContainEqual(record);
    Object.defineProperty(globalThis, "sessionStorage", { configurable: true, value: { setItem() { throw new Error("blocked"); }, getItem() { throw new Error("blocked"); } } });
    const volatile = { ...record, orderId: "order-memory-only" };
    expect(saveOrderAccess(volatile)).toBe(false);
    expect(hasPersistentOrderAccess(volatile.orderId)).toBe(false);
  } finally {
    removeOrderAccess(record.orderId);
    removeOrderAccess("order-memory-only");
    values.clear();
    if (originalLocal) Object.defineProperty(globalThis, "localStorage", originalLocal); else Reflect.deleteProperty(globalThis, "localStorage");
    if (originalSession) Object.defineProperty(globalThis, "sessionStorage", originalSession); else Reflect.deleteProperty(globalThis, "sessionStorage");
  }
});
