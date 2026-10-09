import { expect, test } from "bun:test";
import { buyerPaymentStatusLabel, canContinuePayment, checkoutTicketsComplete, showCheckoutPaymentCountdown } from "./buyer-payment.ts";

test("resolves payment labels from backend order and payment status", () => {
  expect(buyerPaymentStatusLabel("PENDING", null)).toBe("Belum dibayar");
  expect(buyerPaymentStatusLabel("PENDING", "PENDING")).toBe("Menunggu konfirmasi pembayaran");
  expect(buyerPaymentStatusLabel("PAID", null)).toBe("Pembayaran dikonfirmasi");
  const failed = buyerPaymentStatusLabel("PENDING", "FAILED");
  expect(failed).toBe("Pembayaran gagal");
  const succeeded = buyerPaymentStatusLabel("PENDING", "SUCCEEDED");
  expect(succeeded).toBe("Pembayaran dikonfirmasi");
  expect(buyerPaymentStatusLabel("EXPIRED", "SUCCEEDED")).toBe("Pesanan kedaluwarsa");
  expect(buyerPaymentStatusLabel("CANCELLED", "SUCCEEDED")).toBe("Pesanan dibatalkan");
  expect(buyerPaymentStatusLabel("REFUNDED", null)).toBe("Pembayaran dikonfirmasi");
  expect(canContinuePayment("PENDING")).toBe(true);
  expect(canContinuePayment("EXPIRED")).toBe(false);
  expect(canContinuePayment("CANCELLED")).toBe(false);
});

test("only a fully loaded PAID order completes checkout or keeps the payment countdown", () => {
  expect(checkoutTicketsComplete("PAID", 2, 2)).toBe(true);
  expect(checkoutTicketsComplete("PAID", 1, 2)).toBe(false);
  expect(checkoutTicketsComplete("REFUND_PENDING", 2, 2)).toBe(false);
  expect(checkoutTicketsComplete("REFUNDED", 2, 2)).toBe(false);
  expect(showCheckoutPaymentCountdown(false, "PENDING")).toBe(true);
  expect(showCheckoutPaymentCountdown(false, "PAID")).toBe(false);
  expect(showCheckoutPaymentCountdown(false, "REFUNDED")).toBe(false);
  expect(showCheckoutPaymentCountdown(false)).toBe(true);
});
