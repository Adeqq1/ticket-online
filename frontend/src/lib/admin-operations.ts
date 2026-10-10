import type { AdminIssueAudit, OrderStatus, PaymentStatus } from "./api.ts";

export function orderStatusLabel(status: OrderStatus | string) {
  if (!status) return "Tidak terkait pesanan";
  return ({ PENDING: "Menunggu pembayaran", PAID: "Lunas", CANCELLED: "Dibatalkan", EXPIRED: "Kedaluwarsa", REFUND_PENDING: "Refund diproses", REFUNDED: "Refund berhasil" } as Record<string, string>)[status] ?? "Status belum dikenali";
}

export function paymentStatusLabel(status: PaymentStatus | string) {
  return ({ PENDING: "Menunggu konfirmasi", FAILED: "Pembayaran gagal", SUCCEEDED: "Pembayaran berhasil" } as Record<string, string>)[status] ?? "Status belum dikenali";
}

export function providerStatusLabel(status: string) {
  return ({ pending: "Menunggu konfirmasi penyedia", settlement: "Pembayaran terkonfirmasi", capture: "Pembayaran ditangkap", authorize: "Pembayaran diotorisasi", expire: "Pembayaran kedaluwarsa", cancel: "Pembayaran dibatalkan", deny: "Pembayaran ditolak", refund: "Refund terkonfirmasi", refund_failed: "Refund gagal" } as Record<string, string>)[status.toLowerCase()] ?? (status ? "Status penyedia belum dikenali" : "Belum diperiksa");
}

export function refundStatus(status: string) {
  const values: Record<string, { label: string; next: string }> = {
    REQUESTED: { label: "Refund diajukan", next: "Menunggu pemrosesan penyedia pembayaran." },
    PROCESSING: { label: "Refund diproses", next: "Periksa kembali status setelah pemrosesan." },
    UNKNOWN: { label: "Hasil refund sedang diperiksa", next: "Jangan ajukan ulang; tunggu pemeriksaan status." },
    MANUAL_REQUIRED: { label: "Perlu transfer manual", next: "Catat bukti setelah transfer penuh berhasil." },
    FAILED: { label: "Refund belum berhasil", next: "Tinjau alasan kegagalan dan tindak lanjut yang tersedia." },
    SUCCEEDED: { label: "Refund berhasil", next: "Tidak ada tindakan lanjutan." },
  };
  return values[status] ?? { label: "Status refund belum dikenali", next: "Periksa detail teknis atau hubungi dukungan." };
}

export function caseStatus(status: string) {
  return status === "OPEN" ? "Perlu ditangani" : status === "RESOLVED" ? "Kasus ditutup" : "Status kasus belum dikenali";
}

export function emailStatus(status: string) {
  return ({ FAILED: "Pengiriman gagal", PENDING: "Menunggu pengiriman", SENT: "Terkirim", PROCESSING: "Sedang dikirim" } as Record<string, string>)[status] ?? "Status email belum dikenali";
}

export function emailKind(kind: string) {
  return ({ TICKETS: "E-ticket", RECOVERY: "Pemulihan pesanan", REFUND: "Pemberitahuan refund", EVENT_CHANGE: "Perubahan acara" } as Record<string, string>)[kind] ?? "Jenis email belum dikenali";
}

export function auditSummary(entry: AdminIssueAudit) {
  const data = entry.data;
  if (entry.action === "NOTE" || entry.action === "RESOLVE") return typeof data.note === "string" ? data.note : "Catatan tidak tersedia.";
  if (entry.action === "RECHECK") {
    if (data.result === "FAILED") return `Pemeriksaan gagal: ${String(data.reason ?? "alasan tidak tersedia")}`;
    if (typeof data.providerStatus === "string") return `Status penyedia: ${providerStatusLabel(data.providerStatus)}; pesanan ${orderStatusLabel(String(data.orderStatusBefore ?? ""))}, pembayaran ${paymentStatusLabel(String(data.paymentStatusBefore ?? ""))}.`;
    if (data.result === "STARTED") return "Pemeriksaan status dimulai.";
  }
  if (entry.action === "RETRY") return "Email dijadwalkan ulang.";
  return "Detail tindakan tersedia pada data teknis.";
}
