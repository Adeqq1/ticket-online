import { expect, test } from "bun:test";
import { hasPersistentTicketAccess, type OrderAccess } from "./order-access.ts";

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
    if (original) Object.defineProperty(globalThis, "localStorage", original);
    else Reflect.deleteProperty(globalThis, "localStorage");
  }
});
