import { expect, test } from "bun:test";
import { clearStaffSession, logoutStaffSession, readStaffSession, roleMatchesPage, saveStaffSession, staffHome, staffSessionKey } from "./staff-session.ts";

function memoryStorage(): Storage {
  const values = new Map<string, string>();
  return {
    get length() { return values.size; },
    key: (index) => [...values.keys()][index] ?? null,
    getItem: (key) => values.get(key) ?? null,
    setItem: (key, value) => { values.set(key, value); },
    removeItem: (key) => { values.delete(key); },
    clear: () => { values.clear(); },
  };
}

test("stores staff sessions separately and rejects malformed or expired data", () => {
  const storage = memoryStorage();
  const session = { accessToken: "s".repeat(43), expiresAt: new Date(Date.now() + 60_000).toISOString() };
  storage.setItem("ticket-online:order:order-id", "buyer-session");
  expect(saveStaffSession(session, storage)).toBe(true);
  expect(readStaffSession(storage)).toEqual(session);
  expect(storage.getItem(staffSessionKey)).toContain(session.accessToken);
  storage.setItem(staffSessionKey, JSON.stringify({ ...session, expiresAt: "2000-01-01T00:00:00Z" }));
  expect(readStaffSession(storage)).toBeNull();
  storage.setItem(staffSessionKey, "invalid");
  expect(readStaffSession(storage)).toBeNull();
  clearStaffSession(storage);
  expect(storage.getItem(staffSessionKey)).toBeNull();
  expect(storage.getItem("ticket-online:order:order-id")).toBe("buyer-session");
});

test("logout clears the tab token and reports the backend result truthfully", async () => {
  const originalFetch = globalThis.fetch;
  try {
    for (const scenario of [
      { status: 204, remote: "revoked" },
      { status: 401, remote: "invalid" },
    ] as const) {
      const storage = memoryStorage();
      const session = { accessToken: "s".repeat(43), expiresAt: new Date(Date.now() + 60_000).toISOString() };
      saveStaffSession(session, storage);
      storage.setItem("ticket-online:order:order-id", "buyer-session");
      globalThis.fetch = (async () => new Response(null, { status: scenario.status })) as unknown as typeof fetch;
      const outcome = await logoutStaffSession(session.accessToken, storage);
      expect(outcome).toEqual({ localCleared: true, remote: scenario.remote });
      expect(readStaffSession(storage)).toBeNull();
      expect(storage.getItem("ticket-online:order:order-id")).toBe("buyer-session");
    }
    const storage = memoryStorage();
    const session = { accessToken: "s".repeat(43), expiresAt: new Date(Date.now() + 60_000).toISOString() };
    saveStaffSession(session, storage);
    globalThis.fetch = (async () => { throw new TypeError("network disconnected"); }) as unknown as typeof fetch;
    expect(await logoutStaffSession(session.accessToken, storage)).toEqual({ localCleared: true, remote: "unconfirmed" });
    expect(readStaffSession(storage)).toBeNull();
  } finally { globalThis.fetch = originalFetch; }
});

test("routes valid staff sessions to their own role and rejects cross-role pages", () => {
  expect(staffHome("ADMIN")).toBe("/admin/staff");
  expect(staffHome("STAFF")).toBe("/admin/scan");
  expect(roleMatchesPage("staff", { role: "ADMIN" })).toBe(true);
  expect(roleMatchesPage("staff", { role: "STAFF" })).toBe(false);
  expect(roleMatchesPage("history", { role: "ADMIN" })).toBe(true);
  expect(roleMatchesPage("orders", { role: "ADMIN" })).toBe(true);
  expect(roleMatchesPage("orders", { role: "STAFF" })).toBe(false);
  expect(roleMatchesPage("history", { role: "STAFF" })).toBe(false);
  expect(roleMatchesPage("scan", { role: "STAFF" })).toBe(true);
  expect(roleMatchesPage("scan", { role: "ADMIN" })).toBe(false);
});
