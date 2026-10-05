import { expect, test } from "bun:test";
import jsQR from "jsqr";
import { readFileSync } from "node:fs";
import { ticketQrSource } from "./ticket-qr.ts";

const code = `ET-${"A1B2".repeat(8)}`;

function rasterizeTicketSVG(source: string, width: number) {
  const svg = decodeURIComponent(source.slice(source.indexOf(",") + 1));
  const viewBox = svg.match(/viewBox="0 0 (\d+) (\d+)"/);
  const path = svg.match(/<path d="([^"]+)"/);
  if (!viewBox || !path) throw new Error("ticket QR SVG is missing its view box or modules");
  const ratio = width / Number(viewBox[1]);
  const pixels = new Uint8ClampedArray(width * width * 4).fill(255);
  for (const match of path[1]!.matchAll(/M(\d+),(\d+)l/g)) {
    const x = Number(match[1]) * ratio;
    const y = Number(match[2]) * ratio;
    const right = (Number(match[1]) + 6) * ratio;
    const bottom = (Number(match[2]) + 6) * ratio;
    for (let row = Math.round(y); row < Math.round(bottom); row++) {
      for (let column = Math.round(x); column < Math.round(right); column++) {
        const offset = (row * width + column) * 4;
        pixels[offset] = 0;
        pixels[offset + 1] = 0;
        pixels[offset + 2] = 0;
      }
    }
  }
  return { svg, width, pixels };
}

test("e-ticket SVG QR decodes at screen and printed sizes", () => {
  const source = ticketQrSource(code);
  expect(source?.startsWith("data:image/svg+xml,")).toBe(true);
  const screen = rasterizeTicketSVG(source!, 164);
  const print = rasterizeTicketSVG(source!, 484);
  expect(screen.svg).toContain('fill="white"');
  expect(jsQR(screen.pixels, screen.width, screen.width, { inversionAttempts: "dontInvert" })?.data).toBe(code);
  expect(jsQR(print.pixels, print.width, print.width, { inversionAttempts: "dontInvert" })?.data).toBe(code);
  expect(screen.svg).not.toContain("token");
  expect(screen.svg).not.toContain("@example.com");

  const css = readFileSync(new URL("../../ticket.css", import.meta.url), "utf8");
  expect(css).toMatch(/@media print\s*\{\s*\.qr-code\s*\{[^}]*width:\s*45mm[^}]*height:\s*45mm/);
  expect(css).toMatch(/\.qr-code\s*\{[^}]*background:\s*#fff/);
});

test("e-ticket QR source rejects non-canonical values and private URL payloads", () => {
  expect(ticketQrSource(code.toLowerCase())).toBeNull();
  expect(ticketQrSource(`https://example.test/ticket?token=${"x".repeat(32)}`)).toBeNull();
  expect(ticketQrSource(null)).toBeNull();
});
