import { describe, expect, it } from "bun:test";
import { auditSummary, emailRetryActionLabel, emailFailureReason, emailRetryConfirmed, caseStatus, emailKind, emailStatus, orderStatusLabel, paymentStatusLabel, providerStatusLabel, refundStatus } from "./admin-operations.ts";

describe("admin operations labels", () => {
  it("maps known operational statuses and safely labels unknown ones", () => {
    expect(orderStatusLabel("REFUND_PENDING")).toBe("Refund diproses");
    expect(orderStatusLabel("FUTURE")).toBe("Status belum dikenali");
    expect(paymentStatusLabel("SUCCEEDED")).toBe("Pembayaran berhasil");
    expect(providerStatusLabel("settlement")).toBe("Pembayaran terkonfirmasi");
    expect(providerStatusLabel("future_code")).toBe("Status penyedia belum dikenali");
    expect(refundStatus("UNKNOWN").next).toContain("Jangan ajukan ulang");
    expect(caseStatus("OPEN")).toBe("Perlu ditangani");
    expect(emailStatus("FAILED")).toBe("Pengiriman gagal");
    expect(emailKind("TICKETS")).toBe("E-ticket");
  });

  it("summarizes audit payloads without claiming unavailable before and after values", () => {
    expect(auditSummary({ action: "NOTE", actorName: "Admin", createdAt: "2026-01-01", data: { note: "Sudah dicek" } })).toBe("Sudah dicek");
    expect(auditSummary({ action: "RECHECK", actorName: "Admin", createdAt: "2026-01-01", data: { providerStatus: "settlement", orderStatusBefore: "PENDING", paymentStatusBefore: "PENDING" } })).toContain("Pembayaran terkonfirmasi");
    expect(auditSummary({ action: "FUTURE", actorName: "Admin", createdAt: "2026-01-01", data: {} })).toBe("Detail tindakan tersedia pada data teknis.");
  });
});


describe("email failure and retry evidence", () => {
  const old = { action: "RETRY", actorName: "Admin", createdAt: "2026-10-11T01:00:00.000001Z", data: { retryJobId: "job", status: "PENDING" } };
  const fresh = { ...old, createdAt: "2026-10-11T01:00:00.000002Z" };
  const detail = { id: "job", kind: "TICKETS" as const, status: "PENDING" as const, reference: "TO-TEST", recipient: "test@example.test", attempts: 0, lastError: "", updatedAt: "2026-10-11T01:00:01Z", supersededBy: null, orderStatus: "PAID", canRetry: false, retryReason: "", retryJobId: null, history: [old, fresh] };
  it("labels recovery replacement separately from resending an email", () => {
    expect(emailRetryActionLabel({ kind: "RECOVERY", lastError: "permintaan pemulihan kedaluwarsa" })).toBe("Buat permintaan pemulihan baru");
    expect(emailRetryActionLabel({ kind: "RECOVERY", lastError: "SMTP koneksi gagal" })).toBe("Kirim ulang email");
    expect(emailRetryActionLabel({ kind: "TICKETS", lastError: "permintaan pemulihan kedaluwarsa" })).toBe("Kirim ulang email");
  });
  it("translates only known reasons and keeps unknown failures neutral", () => {
    expect(emailFailureReason("SMTP koneksi gagal")).toContain("Koneksi");
    expect(emailFailureReason("permintaan pemulihan kedaluwarsa")).toContain("kedaluwarsa");
    expect(emailFailureReason("unexpected SMTP custom error")).toContain("penyebab belum dapat dijelaskan");
    expect(emailFailureReason("")).toContain("detail teknis");
    expect(emailFailureReason("__proto__")).toContain("penyebab belum dapat dijelaskan");
  });
  it("requires fresh RETRY evidence for same-ID jobs, including after the worker finishes", () => {
    for (const status of ["PENDING", "PROCESSING", "SENT", "FAILED"] as const) expect(emailRetryConfirmed({ ...detail, status }, [old])).toBe(true);
    expect(emailRetryConfirmed({ ...detail, history: [old] }, [old])).toBe(false);
    expect(emailRetryConfirmed({ ...detail, history: [] }, [])).toBe(false);
    expect(emailRetryConfirmed({ ...detail, updatedAt: "2026-10-11T01:00:00.000001Z" }, [old])).toBe(true);
    expect(emailRetryConfirmed({ ...detail, status: "FAILED", updatedAt: "2026-10-11T01:00:00.000001Z" }, [old])).toBe(false);
    expect(emailRetryConfirmed({ ...detail, status: "FAILED", updatedAt: "2026-10-11T01:00:00.000003Z" }, [old])).toBe(true);
    expect(emailRetryConfirmed({ ...detail, history: [{ ...fresh, action: "NOTE" }] }, [old])).toBe(false);
    expect(emailRetryConfirmed({ ...detail, history: [{ ...fresh, data: {} }] }, [old])).toBe(false);
    expect(emailRetryConfirmed({ ...detail, status: "FAILED", updatedAt: "2026-10-10T01:00:00Z" }, [old])).toBe(false);
  });
  it("requires the replacement pointer to match new SUPERSEDED history", () => {
    const replacement = { ...fresh, data: { retryJobId: "replacement", status: "SUPERSEDED" } };
    const replaced = { ...detail, status: "FAILED" as const, supersededBy: "replacement", retryJobId: "replacement", history: [old, replacement] };
    expect(emailRetryConfirmed(replaced, [old])).toBe(true);
    expect(emailRetryConfirmed(replaced, [old, replacement])).toBe(false);
    expect(emailRetryConfirmed({ ...replaced, retryJobId: "other" }, [old])).toBe(false);
    expect(emailRetryConfirmed({ ...replaced, history: [old] }, [old])).toBe(false);
  });
});
