import { expect, test } from "bun:test";
import { journeyId, recordConversion } from "./conversion.ts";

test("conversion journey is tab scoped and records only approved anonymous fields", async () => {
  const originalFetch = globalThis.fetch;
  const originalNow = Date.now;
  const originalStorage = Object.getOwnPropertyDescriptor(globalThis, "sessionStorage");
  const values = new Map<string, string>();
  Object.defineProperty(globalThis, "sessionStorage", { configurable: true, value: { getItem: (key: string) => values.get(key) ?? null, setItem: (key: string, value: string) => values.set(key, value) } });
  let sent: { url: string; body: Record<string, unknown> } | undefined;
  let now = originalNow();
  Date.now = () => now;
  globalThis.fetch = (async (input: RequestInfo | URL, init?: RequestInit) => {
    sent = { url: String(input), body: JSON.parse(String(init?.body)) as Record<string, unknown> };
    return new Response(null, { status: 204 });
  }) as typeof fetch;
  try {
    const first = journeyId("event-1");
    expect(first).toMatch(/^[a-f0-9]{32}$/);
    expect(journeyId("event-1")).toBe(first);
    expect(journeyId("event-2")).not.toBe(first);
    now += 31 * 60 * 1_000;
    const afterIdle = journeyId("event-1");
    expect(afterIdle).not.toBe(first);
    expect(recordConversion("event-1", "VALIDATION_FAILED", "BUYER_DATA")).toBe(afterIdle);
    await Promise.resolve();
    expect(sent?.url).toBe("/api/v1/conversion/events");
    expect(sent?.body).toEqual({ journeyId: afterIdle, eventId: "event-1", device: "unknown", kind: "VALIDATION_FAILED", reason: "BUYER_DATA" });
    expect(Object.keys(sent?.body ?? {}).sort()).toEqual(["device", "eventId", "journeyId", "kind", "reason"]);
  } finally {
    globalThis.fetch = originalFetch;
    Date.now = originalNow;
    if (originalStorage) Object.defineProperty(globalThis, "sessionStorage", originalStorage);
    else Reflect.deleteProperty(globalThis, "sessionStorage");
  }
});
