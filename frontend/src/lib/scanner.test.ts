import { describe, expect, test } from "bun:test";
import type { Staff } from "./api.ts";
import { normalizeScanCode, submitScan, type ScannerState } from "./scanner.ts";

const assignment = { eventId: "nusa-malam", gate: "Gate B" };
const code = `ET-${"A".repeat(32)}`;
const state = (): ScannerState => ({ busy: false, status: "idle", resultTicket: null, expectedGate: "", checkedInAt: "", resultDetail: "", scanCount: 0, lastScan: "Belum ada scan" });
const profile = (assignments = [assignment]): Staff => ({ id: "b".repeat(32), name: "Petugas", email: "staff@example.com", role: "STAFF", active: true, assignments });
const ticket = { id: "a".repeat(32), code, attendeeName: "Peserta", tierName: "Festival", eventId: assignment.eventId, gate: assignment.gate };
const result = { status: "CHECKED_IN", ticket, checkedInAt: "2026-10-05T12:00:00Z" };

function jsonResponse(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });
}

describe("scanner operations", () => {
  test("trims surrounding whitespace and normalizes case", () => {
    expect(normalizeScanCode("  et-0123abcd  ")).toBe("ET-0123ABCD");
  });

  test("submits exactly once after refreshing the session assignment", async () => {
    const originalFetch = globalThis.fetch;
    const calls: Array<{ url: string; method: string }> = [];
    globalThis.fetch = (async (input, init) => {
      calls.push({ url: String(input), method: init?.method ?? "GET" });
      return calls.length === 1 ? jsonResponse({ staff: profile() }) : jsonResponse(result, 201);
    }) as typeof fetch;
    const current = state();
    let updated = false;
    try {
      await submitScan(current, "token", assignment, ` ${code.toLowerCase()} `, () => { updated = true; }, () => {});
      expect(current).toMatchObject({ busy: false, status: "valid", resultTicket: ticket, checkedInAt: result.checkedInAt, scanCount: 1 });
      expect(updated).toBe(true);
      expect(calls).toEqual([
        { url: "/api/v1/staff/me", method: "GET" },
        { url: "/api/v1/staff/check-ins", method: "POST" },
      ]);
    } finally { globalThis.fetch = originalFetch; }
  });

  test("does not check in when the selected assignment has been removed", async () => {
    const originalFetch = globalThis.fetch;
    const calls: string[] = [];
    globalThis.fetch = (async (input) => { calls.push(String(input)); return jsonResponse({ staff: profile([]) }); }) as typeof fetch;
    const current = state();
    try {
      await submitScan(current, "token", assignment, code, () => {}, () => {});
      expect(current.status).toBe("unavailable");
      expect(current.resultDetail).toContain("sudah berubah");
      expect(calls).toEqual(["/api/v1/staff/me"]);
    } finally { globalThis.fetch = originalFetch; }
  });

  test("maps server rejection codes to the gate result", async () => {
    const originalFetch = globalThis.fetch;
    const cases = [
      { status: 409, body: { error: { code: "TICKET_ALREADY_USED", message: "Sudah digunakan" }, status: "ALREADY_USED", ticket, checkedInAt: result.checkedInAt }, expected: "used", time: result.checkedInAt },
      { status: 409, body: { error: { code: "WRONG_GATE", message: "Gate lain", expectedGate: "Gate C" } }, expected: "wrong-gate", gate: "Gate C" },
      { status: 404, body: { error: { code: "TICKET_NOT_FOUND", message: "Tidak ditemukan" } }, expected: "not-found" },
      { status: 409, body: { error: { code: "ORDER_NOT_PAID", message: "Belum dibayar" } }, expected: "unpaid" },
      { status: 403, body: { error: { code: "FORBIDDEN", message: "Akses ditolak" } }, expected: "denied" },
    ] as const;
    try {
      for (const item of cases) {
        let calls = 0;
        globalThis.fetch = (async () => ++calls === 1 ? jsonResponse({ staff: profile() }) : jsonResponse(item.body, item.status)) as unknown as typeof fetch;
        const current = state();
        await submitScan(current, "token", assignment, code, () => {}, () => {});
        expect(current.status).toBe(item.expected);
        if ("gate" in item) expect(current.expectedGate).toBe(item.gate);
        if ("time" in item) expect(current.checkedInAt).toBe(item.time);
      }
    } finally { globalThis.fetch = originalFetch; }
  });

  test("shows unknown outcome after a disconnected check-in and never retries", async () => {
    const originalFetch = globalThis.fetch;
    let calls = 0;
    globalThis.fetch = (async (input) => {
      calls += 1;
      if (String(input).endsWith("/me")) return jsonResponse({ staff: profile() });
      throw new TypeError("connection closed");
    }) as typeof fetch;
    const current = state();
    try {
      await submitScan(current, "token", assignment, code, () => {}, () => {});
      expect(current.status).toBe("unknown");
      expect(current.scanCount).toBe(1);
      expect(calls).toBe(2);
    } finally { globalThis.fetch = originalFetch; }
  });

  test("ignores a second submit while the first check-in is pending", async () => {
    const originalFetch = globalThis.fetch;
    let release!: (response: Response) => void;
    const checkInResponse = new Promise<Response>((resolve) => { release = resolve; });
    let checkInCalls = 0;
    globalThis.fetch = (async (input) => {
      if (String(input).endsWith("/me")) return jsonResponse({ staff: profile() });
      checkInCalls += 1;
      return checkInResponse;
    }) as typeof fetch;
    const current = state();
    try {
      const first = submitScan(current, "token", assignment, code, () => {}, () => {});
      for (let tries = 0; tries < 10 && checkInCalls === 0; tries += 1) await Promise.resolve();
      await submitScan(current, "token", assignment, code, () => {}, () => {});
      expect(checkInCalls).toBe(1);
      expect(current.scanCount).toBe(1);
      release(jsonResponse(result, 201));
      await first;
      expect(current.status).toBe("valid");
    } finally { globalThis.fetch = originalFetch; }
  });
});
