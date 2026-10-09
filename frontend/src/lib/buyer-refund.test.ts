import { expect, test } from "bun:test";
import { buyerOrderStatusLabel, refundStatusLabel } from "./buyer-refund.ts";

test("maps backend refund and order statuses to buyer-facing labels", () => {
  expect(["REQUESTED", "PROCESSING", "UNKNOWN", "MANUAL_REQUIRED", "FAILED", "SUCCEEDED"].map((status) => refundStatusLabel(status))).toEqual([
    "Pengajuan tercatat", "Sedang diproses", "Hasil sedang diperiksa", "Ditangani tim", "Belum berhasil", "Pengembalian dikonfirmasi",
  ]);
  expect(refundStatusLabel(undefined, true)).toBe("Pengajuan tercatat");
  expect(refundStatusLabel("NEW_STATUS")).toBe("Status pengembalian belum dapat dipastikan");
  expect(buyerOrderStatusLabel("REFUND_PENDING")).toBe("Pengembalian belum dikonfirmasi");
  expect(buyerOrderStatusLabel("REFUNDED")).toBe("Pengembalian dikonfirmasi");
});
