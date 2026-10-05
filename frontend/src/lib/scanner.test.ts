import { describe, expect, test } from "bun:test";
import jsQR from "jsqr";
import qrcode from "qrcode-generator";
import type { Staff } from "./api.ts";
import { checkTicketStatus, continueToNextScan, normalizeScanCode, parseCameraScanCode, submitScan, type ScannerState } from "./scanner.ts";

const assignment = { eventId: "nusa-malam", gate: "Gate B" };
const code = `ET-${"A".repeat(32)}`;
const state = (): ScannerState => ({ busy: false, status: "idle", resultTicket: null, expectedGate: "", checkedInAt: "", resultDetail: "", scanCount: 0, lastScan: "Belum ada scan", locked: false, lastAttempt: null, statusChecking: false, ticketStatus: null, statusCheckError: "" });
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

  test("decodes a ticket QR and rejects non-ticket QR values", () => {
    const matrix = qrcode(0, "M");
    matrix.addData(code);
    matrix.make();
    const scale = 5;
    const quiet = 8;
    const width = (matrix.getModuleCount() + quiet * 2) * scale;
    const pixels = new Uint8ClampedArray(width * width * 4).fill(255);
    for (let row = 0; row < matrix.getModuleCount(); row += 1) {
      for (let column = 0; column < matrix.getModuleCount(); column += 1) {
        if (!matrix.isDark(row, column)) continue;
        for (let y = 0; y < scale; y += 1) for (let x = 0; x < scale; x += 1) {
          const offset = (((row + quiet) * scale + y) * width + (column + quiet) * scale + x) * 4;
          pixels[offset] = 0; pixels[offset + 1] = 0; pixels[offset + 2] = 0;
        }
      }
    }
    expect(parseCameraScanCode(jsQR(pixels, width, width)?.data)).toBe(code);
    expect(parseCameraScanCode("https://example.com/ticket")).toBeNull();
    expect(parseCameraScanCode(null)).toBeNull();
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

  test("locks a completed attempt until the operator starts the next scan", async () => {
    const originalFetch = globalThis.fetch;
    let posts = 0;
    globalThis.fetch = (async (input) => String(input).endsWith("/me") ? jsonResponse({ staff: profile() }) : (posts += 1, jsonResponse(result, 201))) as typeof fetch;
    const current = state();
    try {
      await submitScan(current, "token", assignment, code, () => {}, () => {});
      await submitScan(current, "token", assignment, code, () => {}, () => {});
      expect(posts).toBe(1);
      expect(current.locked).toBe(true);
      expect(continueToNextScan(current, false)).toBe(true);
      expect(current).toMatchObject({ locked: false, status: "idle", lastAttempt: null });
    } finally { globalThis.fetch = originalFetch; }
  });

  test("checks the last unknown attempt without sending another check-in", async () => {
    const originalFetch = globalThis.fetch;
    const calls: Array<{ url: string; method: string }> = [];
    globalThis.fetch = (async (input, init) => {
      calls.push({ url: String(input), method: init?.method ?? "GET" });
      return jsonResponse({ status: "CHECKED_IN", ticket, orderStatus: "PAID", checkedInAt: result.checkedInAt });
    }) as typeof fetch;
    const current = state();
    current.locked = true;
    current.status = "unknown";
    current.lastAttempt = { ...assignment, code };
    try {
      await checkTicketStatus(current, "token", () => {});
      expect(current.ticketStatus).toMatchObject({ status: "CHECKED_IN", checkedInAt: result.checkedInAt });
      expect(continueToNextScan(current, false)).toBe(true);
      expect(calls).toEqual([{ url: `/api/v1/staff/ticket-status?code=${code}`, method: "GET" }]);
    } finally { globalThis.fetch = originalFetch; }
  });

  test("requires handling confirmation while an unknown result remains unresolved", () => {
    const current = state();
    current.locked = true;
    current.status = "unknown";
    expect(continueToNextScan(current, false)).toBe(false);
    expect(continueToNextScan(current, true)).toBe(true);
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

  test.each([123, {}, null, undefined])("keeps malformed successful ticket code %p unknown without retrying", async (code) => {
    const originalFetch = globalThis.fetch;
    let posts = 0;
    globalThis.fetch = (async (input) => {
      if (String(input).endsWith("/me")) return jsonResponse({ staff: profile() });
      posts += 1;
      return jsonResponse({ ...result, ticket: { ...ticket, code } }, 201);
    }) as typeof fetch;
    const current = state();
    try {
      await submitScan(current, "token", assignment, ticket.code, () => {}, () => {});
      expect(current).toMatchObject({ status: "unknown", busy: false, resultTicket: null, scanCount: 1 });
      expect(posts).toBe(1);
    } finally { globalThis.fetch = originalFetch; }
  });
});
