export function refundStatusLabel(status?: string, requested = false) {
  if (!status && requested) return "Pengajuan tercatat";
  return ({
    REQUESTED: "Pengajuan tercatat",
    PROCESSING: "Sedang diproses",
    UNKNOWN: "Hasil sedang diperiksa",
    MANUAL_REQUIRED: "Ditangani tim",
    FAILED: "Belum berhasil",
    SUCCEEDED: "Pengembalian dikonfirmasi",
  } as Record<string, string>)[status ?? ""] ?? "Status pengembalian belum dapat dipastikan";
}

export function buyerOrderStatusLabel(status: string) {
  return ({
    PAID: "Dibayar",
    PENDING: "Menunggu pembayaran",
    EXPIRED: "Kedaluwarsa",
    CANCELLED: "Dibatalkan",
    REFUND_PENDING: "Pengembalian belum dikonfirmasi",
    REFUNDED: "Pengembalian dikonfirmasi",
  } as Record<string, string>)[status] ?? "Status pesanan belum dapat dipastikan";
}
