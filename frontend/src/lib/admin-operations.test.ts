import { describe, expect, it } from "bun:test";
import { auditSummary, caseStatus, emailKind, emailStatus, orderStatusLabel, paymentStatusLabel, providerStatusLabel, refundStatus } from "./admin-operations.ts";

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
