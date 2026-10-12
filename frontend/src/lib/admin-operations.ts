import type { AdminEmailDetail, AdminIssueAudit, OrderStatus, PaymentStatus } from "./api.ts";

export const adminDate = new Intl.DateTimeFormat("id-ID", { timeZone: "Asia/Jakarta", dateStyle: "medium" });
export const adminTime = new Intl.DateTimeFormat("id-ID", { timeZone: "Asia/Jakarta", hour: "2-digit", minute: "2-digit" });
export const adminNumber = new Intl.NumberFormat("id-ID");

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

export function emailFailureReason(reason: string) {
  const known: Record<string, string> = {
    "alamat penerima tidak valid": "Alamat penerima tidak dapat digunakan.",
    "tidak ada pesanan yang memenuhi syarat pemulihan": "Tidak ada pesanan yang memenuhi syarat untuk dipulihkan.",
    "permintaan pemulihan kedaluwarsa": "Permintaan pemulihan sudah kedaluwarsa.",
    "tiket tidak lagi aktif": "Tiket sudah tidak aktif.",
    "akses tiket telah kedaluwarsa": "Akses tiket sudah kedaluwarsa.",
    "tiket pesanan tidak tersedia": "Tiket untuk pesanan ini belum tersedia.",
    "pengiriman berhenti sebelum selesai": "Proses pengiriman berhenti sebelum selesai.",
    "konfigurasi pengiriman email tidak valid": "Pengaturan layanan email tidak valid. Hubungi pengelola layanan.",
    "SMTP koneksi gagal": "Koneksi ke layanan email gagal.",
    "SMTP autentikasi gagal": "Layanan email menolak autentikasi pengirim.",
    "SMTP penerima gagal": "Layanan email tidak menerima alamat tujuan pada percobaan ini.",
    "SMTP konfirmasi pesan gagal": "Konfirmasi dari layanan email tidak diterima. Pengiriman belum dapat dipastikan.",
  };
  return Object.hasOwn(known, reason) ? known[reason]! : "Pengiriman gagal; penyebab belum dapat dijelaskan. Periksa detail teknis.";
}

export function emailRetryActionLabel(detail: Pick<AdminEmailDetail, "kind" | "lastError">) {
  return detail.kind === "RECOVERY" && detail.lastError === "permintaan pemulihan kedaluwarsa" ? "Buat permintaan pemulihan baru" : "Kirim ulang email";
}

// Backend history is ordered by audit ID. A new RETRY proves scheduling, never delivery.
export function emailRetryConfirmed(detail: AdminEmailDetail, baseline: AdminIssueAudit[]) {
  const latest = detail.history.findLast((entry) => entry.action === "RETRY");
  if (!latest || baseline.some((entry) => entry.action === "RETRY" && entry.createdAt === latest.createdAt
    && entry.data.retryJobId === latest.data.retryJobId && entry.data.status === latest.data.status)) return false;
  const retryID = latest.data.retryJobId;
  if (typeof retryID !== "string" || !retryID) return false;
  if (retryID !== detail.id) return latest.data.status === "SUPERSEDED"
    && detail.supersededBy === retryID && detail.retryJobId === retryID;
  if (latest.data.status !== "PENDING" || !["PENDING", "PROCESSING", "SENT", "FAILED"].includes(detail.status)) return false;
  // The queue update precedes the audit in the same transaction. A later FAILED
  // must postdate that audit, so a stale failure read cannot unlock another retry.
  const preciseUTC = (value: string) => value.replace(/(?:\.(\d+))?Z$/, (_, fraction = "") => `.${fraction.padEnd(9, "0")}Z`);
  return detail.status !== "FAILED" || Number.isFinite(Date.parse(detail.updatedAt)) && Number.isFinite(Date.parse(latest.createdAt))
    && preciseUTC(detail.updatedAt) >= preciseUTC(latest.createdAt);
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
