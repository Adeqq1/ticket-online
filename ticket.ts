import type { Concert, TicketTier } from "./concerts.ts";

export type TicketLine = { tierName: string; quantity: number; gate: string; allocation: string };
export type TicketSnapshot = { version: 1; id: string; reference: string; issuedAt: string; attendeeName: string; concert: Pick<Concert, "id" | "artist" | "venue" | "city" | "address" | "date" | "startsAt" | "image">; lines: TicketLine[] };

export function ticketStorageKey(id: string) { return `ticket-online:ticket:${id}`; }
export function ticketAllocation(tier: TicketTier, quantity: number, reference: string) {
  if (tier.seating === "free-standing") return "Berdiri bebas";
  const seed = [...reference].reduce((total, character) => total + character.charCodeAt(0), 0);
  const row = String.fromCharCode(65 + seed % 6);
  const start = 10 + seed % 70;
  return `Row ${row}, Kursi ${start}-${start + quantity - 1}`;
}
export function createTicketSnapshot(id: string, reference: string, attendeeName: string, concert: Concert, lines: Array<{ tier: TicketTier; quantity: number }>): TicketSnapshot {
  return { version: 1, id, reference, issuedAt: new Date().toISOString(), attendeeName, concert: { id: concert.id, artist: concert.artist, venue: concert.venue, city: concert.city, address: concert.address, date: concert.date, startsAt: concert.startsAt, image: concert.image }, lines: lines.map(({ tier, quantity }) => ({ tierName: tier.name, quantity, gate: tier.gate, allocation: ticketAllocation(tier, quantity, reference) })) };
}
export function parseTicketSnapshot(value: string | null): TicketSnapshot | null {
  try {
    const ticket = JSON.parse(value ?? "") as TicketSnapshot;
    if (ticket?.version !== 1 || !/^[\w-]{8,}$/.test(ticket.id) || !/^TO-[A-Z0-9]{10}$/.test(ticket.reference) || !ticket.attendeeName || !ticket.concert?.artist || !ticket.concert?.venue || !ticket.concert?.date || Number.isNaN(Date.parse(ticket.concert.startsAt)) || !Array.isArray(ticket.lines) || !ticket.lines.length || ticket.lines.some((line) => !line || typeof line !== "object" || !line.tierName || !line.gate || !line.allocation || !Number.isInteger(line.quantity) || line.quantity < 1)) return null;
    return ticket;
  } catch { return null; }
}
export function timeRemaining(startsAt: string, now = Date.now()) {
  const milliseconds = Date.parse(startsAt) - now;
  if (!Number.isFinite(milliseconds) || milliseconds <= 0) return { days: 0, hours: 0, minutes: 0, seconds: 0, started: true };
  const seconds = Math.floor(milliseconds / 1_000);
  return { days: Math.floor(seconds / 86_400), hours: Math.floor(seconds % 86_400 / 3_600), minutes: Math.floor(seconds % 3_600 / 60), seconds: seconds % 60, started: false };
}

if (typeof document !== "undefined") {
  const app = document.querySelector<HTMLDivElement>("#ticket-app");
  if (app) {
    const id = location.pathname.split("/").at(-1) ?? "";
    let storedTicket: string | null = null;
    try { storedTicket = sessionStorage.getItem(ticketStorageKey(id)); } catch { /* Storage can be unavailable in private or restricted contexts. */ }
    const ticket = parseTicketSnapshot(storedTicket);
    if (!ticket || ticket.id !== id) {
    app.innerHTML = `<section class="ticket-missing shell"><p class="ticket-kicker">E-ticket tidak tersedia</p><h1>Buka dari tab checkout.</h1><p>Tiket demo ini hanya tersedia pada tab tempat checkout dibuat dan tidak dapat dibuka sebagai tautan publik.</p><a class="button" href="/konser">Kembali ke konser</a></section>`;
    } else {
    document.title = `${ticket.concert.artist} | E-Ticket Tiket Online`;
    app.innerHTML = `<section class="ticket-page shell"><header><p class="ticket-kicker">E-ticket digital</p><h1>Tiketmu sudah siap.</h1><p>Gunakan reference di bawah sebagai bukti pembelian demo.</p></header><article class="boarding-pass" aria-labelledby="ticket-title"><section class="pass-main"><div class="pass-top"><span class="pass-label">Tiket Online</span><span class="pass-status">E-ticket</span></div><h2 id="ticket-title"></h2><p class="pass-venue"></p><div class="pass-grid"><div class="pass-row"><span>Pengunjung</span><b data-attendee></b></div><div class="pass-row"><span>Tanggal event</span><b data-date></b></div><div class="pass-row"><span>Lokasi</span><b data-venue></b></div><div class="pass-row"><span>Alamat</span><b data-address></b></div></div><div class="ticket-lines" data-lines></div></section><aside class="pass-stub"><div><canvas class="qr-code" width="180" height="180" aria-hidden="true"></canvas><div class="barcode" aria-hidden="true"></div><small>Reference demo</small><p class="ticket-reference"></p></div><div class="countdown"><span class="pass-label">Event dimulai dalam</span><strong data-countdown aria-live="off"></strong><span class="sr-only" data-countdown-accessible aria-live="polite"></span></div></aside></article><div class="ticket-actions"><button class="button" type="button" id="print-ticket">Cetak / Simpan PDF</button><button class="button button-secondary" type="button" id="email-ticket">Kirim ke email</button></div><p class="ticket-caption" id="action-status" aria-live="polite">Kode dan QR pada halaman ini adalah visual demo, tidak dapat digunakan untuk masuk event.</p></section>`;
    const setText = (selector: string, value: string) => { app.querySelector<HTMLElement>(selector)!.textContent = value; };
    setText("#ticket-title", ticket.concert.artist);
    setText(".pass-venue", `${ticket.concert.venue}, ${ticket.concert.city}`);
    setText("[data-attendee]", ticket.attendeeName);
    setText("[data-date]", `${ticket.concert.date}, 19.30 WIB`);
    setText("[data-venue]", `${ticket.concert.venue}, ${ticket.concert.city}`);
    setText("[data-address]", ticket.concert.address);
    setText(".ticket-reference", ticket.reference);
    const lines = app.querySelector<HTMLElement>("[data-lines]")!;
    ticket.lines.forEach((line) => { const row = document.createElement("div"); row.className = "ticket-line"; const name = document.createElement("span"); name.textContent = `${line.quantity}x ${line.tierName}`; const gate = document.createElement("b"); gate.textContent = line.gate; const allocation = document.createElement("p"); allocation.textContent = line.allocation; row.append(name, gate, allocation); lines.append(row); });
    const canvas = app.querySelector<HTMLCanvasElement>(".qr-code")!;
    const context = canvas.getContext("2d")!;
    context.fillStyle = "#fff";
    context.fillRect(0, 0, canvas.width, canvas.height);
    const cells = 25;
    const size = canvas.width / cells;
    const seed = [...ticket.reference].reduce((total, character) => total * 31 + character.charCodeAt(0), 7) >>> 0;
    const isFinder = (x: number, y: number) => [[0, 0], [cells - 7, 0], [0, cells - 7]].some(([left, top]) => x >= left && x < left + 7 && y >= top && y < top + 7);
    for (let y = 0; y < cells; y++) for (let x = 0; x < cells; x++) { let on = ((seed + x * 17 + y * 31 + x * y) % 5) < 2; if (isFinder(x, y)) { const left = x < 7 ? 0 : cells - 7; const top = y < 7 ? 0 : cells - 7; const dx = x - left; const dy = y - top; on = dx === 0 || dx === 6 || dy === 0 || dy === 6 || (dx >= 2 && dx <= 4 && dy >= 2 && dy <= 4); } if (on) { context.fillStyle = "#17211f"; context.fillRect(x * size, y * size, Math.ceil(size), Math.ceil(size)); } }
    const countdown = app.querySelector<HTMLElement>("[data-countdown]")!;
    const accessibleCountdown = app.querySelector<HTMLElement>("[data-countdown-accessible]")!;
    const startsAt = ticket.concert.startsAt;
    let lastMinute = "";
    function updateCountdown() { const remaining = timeRemaining(startsAt); const text = remaining.started ? "Event telah dimulai" : `${remaining.days} hari · ${String(remaining.hours).padStart(2, "0")} jam · ${String(remaining.minutes).padStart(2, "0")} menit · ${String(remaining.seconds).padStart(2, "0")} detik`; countdown.textContent = text; const minuteText = remaining.started ? text : `${remaining.days} hari, ${remaining.hours} jam, dan ${remaining.minutes} menit`; if (minuteText !== lastMinute) { accessibleCountdown.textContent = minuteText; lastMinute = minuteText; } return remaining.started; }
    const timer = updateCountdown() ? undefined : window.setInterval(() => { if (updateCountdown() && timer) window.clearInterval(timer); }, 1_000);
    window.addEventListener("pagehide", () => { if (timer) window.clearInterval(timer); }, { once: true });
    app.querySelector<HTMLButtonElement>("#print-ticket")!.addEventListener("click", () => window.print());
    app.querySelector<HTMLButtonElement>("#email-ticket")!.addEventListener("click", () => { app.querySelector<HTMLElement>("#action-status")!.textContent = "Simulasi: e-ticket akan dikirim ke email pemesan."; });
    }
  }
}
