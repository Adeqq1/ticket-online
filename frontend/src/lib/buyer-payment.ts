import type { OrderStatus, PaymentStatus } from "./api.ts";

export function buyerPaymentStatusLabel(orderStatus: OrderStatus, paymentStatus?: PaymentStatus | null) {
  if (orderStatus === "EXPIRED") return "Pesanan kedaluwarsa";
  if (orderStatus === "CANCELLED") return "Pesanan dibatalkan";
  if (orderStatus === "PAID" || paymentStatus === "SUCCEEDED") return "Pembayaran dikonfirmasi";
  if (orderStatus === "PENDING" && paymentStatus === "FAILED") return "Pembayaran gagal";
  if (orderStatus === "PENDING" && paymentStatus === "PENDING") return "Menunggu konfirmasi pembayaran";
  if (orderStatus === "PENDING") return "Belum dibayar";
  return "Pembayaran dikonfirmasi";
}

export function canContinuePayment(orderStatus: OrderStatus) {
  return orderStatus === "PENDING";
}
