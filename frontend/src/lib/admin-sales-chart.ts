export type SalesSeries = { paymentAmount: number; refundAmount: number; netAmount: number };

export function salesChartPaths(days: SalesSeries[], width = 720, height = 180) {
  const padding = { left: 40, right: 12, top: 12, bottom: 24 };
  const values = days.flatMap((day) => [day.paymentAmount, day.refundAmount, day.netAmount]);
  let minimum = Math.min(0, ...values);
  let maximum = Math.max(0, ...values);
  if (minimum === maximum) maximum = 1;
  const x = (index: number) => padding.left + (days.length <= 1 ? (width - padding.left - padding.right) / 2 : index * (width - padding.left - padding.right) / (days.length - 1));
  const y = (value: number) => padding.top + (maximum - value) * (height - padding.top - padding.bottom) / (maximum - minimum);
  const path = (key: keyof SalesSeries) => days.map((day, index) => `${index ? "L" : "M"}${x(index).toFixed(1)},${y(day[key]).toFixed(1)}`).join(" ");
  const singleDay = days.length === 1 ? days[0] : undefined;
  const points = singleDay ? {
    payments: { x: x(0), y: y(singleDay.paymentAmount) },
    refunds: { x: x(0), y: y(singleDay.refundAmount) },
    net: { x: x(0), y: y(singleDay.netAmount) },
  } : null;
  return {
    payments: path("paymentAmount"), refunds: path("refundAmount"), net: path("netAmount"),
    zeroY: y(0), points,
    min: minimum,
    max: maximum,
  };
}
