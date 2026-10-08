import { expect, test } from "bun:test";
import { salesChartPaths } from "./admin-sales-chart.ts";

test("sales chart handles flat, single-day, and negative net values", () => {
  expect(salesChartPaths([{ paymentAmount: 0, refundAmount: 0, netAmount: 0 }])).toMatchObject({
    payments: "M374.0,156.0", refunds: "M374.0,156.0", net: "M374.0,156.0", min: 0, max: 1, zeroY: 156,
    points: { payments: { x: 374, y: 156 }, refunds: { x: 374, y: 156 }, net: { x: 374, y: 156 } },
  });
  expect(salesChartPaths([{ paymentAmount: 100, refundAmount: 25, netAmount: 75 }]).points).toEqual({
    payments: { x: 374, y: 12 }, refunds: { x: 374, y: 120 }, net: { x: 374, y: 48 },
  });
  const chart = salesChartPaths([
    { paymentAmount: 100, refundAmount: 0, netAmount: 100 },
    { paymentAmount: 0, refundAmount: 150, netAmount: -150 },
  ]);
  expect(chart.min).toBe(-150);
  expect(chart.zeroY).toBeLessThan(156);
  expect(chart.net).toContain("L708.0,156.0");
  expect(chart.points).toBeNull();
});
