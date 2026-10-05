import qrcode from "qrcode-generator";

export function ticketQrSource(code: unknown): string | null {
  if (typeof code !== "string" || !/^ET-[0-9A-F]{32}$/.test(code)) return null;
  try {
    const qr = qrcode(0, "M");
    qr.addData(code);
    qr.make();
    return `data:image/svg+xml,${encodeURIComponent(qr.createSvgTag({ cellSize: 6, margin: 24, scalable: true }))}`;
  } catch {
    return null;
  }
}
