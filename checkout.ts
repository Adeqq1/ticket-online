import { formatRupiah, getConcertById, parseQuantities, type Concert, type TicketTier } from "./concerts.ts";

export type CheckoutLine = { tier: TicketTier; quantity: number };
export type Buyer = { name: string; email: string; phone: string; identity: string };
export const ADMIN_FEE = 7_500;

export function checkoutLines(concert: Concert, params: URLSearchParams): CheckoutLine[] {
  const quantities = parseQuantities(concert, params);
  return concert.ticketTiers.flatMap((tier) => quantities[tier.id] ? [{ tier, quantity: quantities[tier.id] }] : []);
}

export function orderSubtotal(lines: CheckoutLine[]) { return lines.reduce((total, { tier, quantity }) => total + tier.price * quantity, 0); }
export function voucherDiscount(subtotal: number, code: string) { return code.trim().toUpperCase() === "HEMAT10" ? Math.floor(subtotal * .1) : 0; }
export function isValidEmail(value: string) { return /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(value); }
export function isValidPhone(value: string) { return /^\+?\d{9,15}$/.test(value.replace(/[ -]/g, "")); }
export function isValidIdentity(value: string) { return /^\d{12,20}$/.test(value); }

if (typeof document !== "undefined") {
  const app = document.querySelector<HTMLDivElement>("#checkout-app");
  const concert = getConcertById(location.pathname.split("/").at(-1));
  if (!app) throw new Error("Checkout container tidak ditemukan.");
  if (!concert) {
    app.innerHTML = `<section class="checkout-missing shell"><h1>Konser tidak ditemukan.</h1><p>URL checkout ini tidak tepat.</p><a class="button" href="/konser">Kembali ke konser</a></section>`;
  } else {
    const lines = checkoutLines(concert, new URLSearchParams(location.search));
    if (!lines.length) {
      app.innerHTML = `<section class="checkout-missing shell"><p class="checkout-kicker">Keranjang kosong</p><h1>Pilih tiket terlebih dahulu.</h1><p>Belum ada tiket valid yang dapat dilanjutkan ke checkout ini.</p><a class="button" href="/konser/${concert.id}">Pilih tiket</a></section>`;
    } else {
      document.title = `Checkout ${concert.artist} | Tiket Online`;
      let step = 1;
      let payment = "";
      let voucher = "";
      let completed = false;
      const buyer: Buyer = { name: "", email: "", phone: "", identity: "" };
      const subtotal = orderSubtotal(lines);
      const total = () => subtotal + ADMIN_FEE - voucherDiscount(subtotal, voucher);
      const ticketText = lines.map(({ tier, quantity }) => `${quantity}x ${tier.name}`).join(", ");
      app.innerHTML = `<section class="checkout shell"><a class="back-link" href="/konser/${concert.id}?${location.search.slice(1)}">Kembali ke detail konser</a><div class="checkout-heading"><p class="checkout-kicker">Checkout aman</p><h1>Selesaikan pesananmu.</h1><p>${concert.artist} · ${concert.date} · ${concert.venue}</p></div><ol class="checkout-steps" aria-label="Tahap checkout"><li data-step="1" aria-current="step"><span>1</span><b>Data Diri</b></li><li data-step="2"><span>2</span><b>Metode Bayar</b></li><li data-step="3"><span>3</span><b>Konfirmasi</b></li></ol><div class="checkout-layout"><section class="checkout-panel"><form id="buyer-form" novalidate><div class="step-title"><p>Langkah 1 dari 3</p><h2 id="buyer-title" tabindex="-1">Data pemesan</h2><span>Informasi ini digunakan untuk mengirim e-ticket.</span></div><div class="field-grid"><label for="buyer-name">Nama lengkap<input id="buyer-name" name="name" autocomplete="name" aria-describedby="name-error" required /><small id="name-error" data-error="name" aria-live="polite"></small></label><label for="buyer-email">Email<input id="buyer-email" name="email" type="email" autocomplete="email" aria-describedby="email-error" required /><small id="email-error" data-error="email" aria-live="polite"></small></label><label for="buyer-phone">No. HP<input id="buyer-phone" name="phone" inputmode="tel" autocomplete="tel" aria-describedby="phone-error" required /><small id="phone-error" data-error="phone" aria-live="polite"></small></label><label for="buyer-identity">No. identitas<input id="buyer-identity" name="identity" inputmode="numeric" aria-describedby="identity-error" required /><small id="identity-error" data-error="identity" aria-live="polite"></small></label></div><div class="step-actions"><span></span><button class="button" type="submit">Lanjut ke pembayaran</button></div></form><form id="payment-form" hidden><div class="step-title"><p>Langkah 2 dari 3</p><h2 id="payment-title" tabindex="-1">Pilih metode bayar</h2><span>Pilih salah satu metode untuk pesanan ini.</span></div><fieldset aria-describedby="payment-error"><legend class="sr-only">Metode pembayaran</legend><div class="payment-options"><label class="payment-card"><input type="radio" name="payment" value="BCA Virtual Account" /><span class="payment-logo bca">BCA</span><span><b>BCA Virtual Account</b><small>Verifikasi otomatis</small></span></label><label class="payment-card"><input type="radio" name="payment" value="Mandiri Virtual Account" /><span class="payment-logo mandiri">mandiri</span><span><b>Mandiri Virtual Account</b><small>Verifikasi otomatis</small></span></label><label class="payment-card"><input type="radio" name="payment" value="QRIS" /><span class="payment-logo qris">QRIS</span><span><b>QRIS</b><small>Scan dari aplikasi pembayaran</small></span></label><label class="payment-card"><input type="radio" name="payment" value="GoPay" /><span class="payment-logo gopay">GoPay</span><span><b>GoPay</b><small>Bayar dari aplikasi GoPay</small></span></label></div><small class="form-error" id="payment-error" aria-live="polite"></small></fieldset><div class="step-actions"><button class="text-button" type="button" data-back="1">Kembali</button><button class="button" type="submit">Tinjau pesanan</button></div></form><section id="confirmation" hidden><div class="step-title"><p>Langkah 3 dari 3</p><h2 id="confirmation-title" tabindex="-1">Periksa pesanan</h2><span>Pastikan data dan metode pembayaran sudah sesuai.</span></div><div class="confirmation-block"><div><b data-buyer-name></b><span data-buyer-contact></span></div><button class="text-button" type="button" data-edit="1">Ubah</button></div><div class="confirmation-block"><div><b data-payment></b><span>Metode pembayaran terpilih</span></div><button class="text-button" type="button" data-edit="2">Ubah</button></div><div class="step-actions"><button class="text-button" type="button" data-back="2">Kembali</button><button class="button" type="button" id="place-order">Buat pesanan</button></div></section><section id="success" class="checkout-success" tabindex="-1" hidden><p class="checkout-kicker">Pesanan dibuat</p><h2 id="success-title" tabindex="-1">Sampai jumpa di konser.</h2><p>E-ticket untuk <b>${ticketText}</b> akan dikirim ke <b data-success-email></b>.</p><div class="booking-code"><span>Kode booking</span><strong id="booking-code"></strong></div><p class="success-note">Ini adalah simulasi frontend. Tidak ada pembayaran atau reservasi stok yang diproses.</p><a class="button" href="/konser">Jelajahi konser</a></section></section><aside class="order-summary"><div class="summary-concert"><img src="${concert.image}" alt="Poster contoh konser ${concert.artist}" width="90" height="112" /><div><p>${concert.status}</p><h2>${concert.artist}</h2><span>${concert.date}</span><span>${concert.venue}, ${concert.city}</span></div></div><div class="summary-lines">${lines.map(({ tier, quantity }) => `<div><span>${quantity}x ${tier.name}</span><b>${formatRupiah.format(tier.price * quantity)}</b></div>`).join("")}</div><form class="voucher-form" id="voucher-form"><label for="voucher">Kode promo</label><div><input id="voucher" name="voucher" placeholder="Contoh: HEMAT10" /><button type="submit">Pakai</button></div><small id="voucher-message"></small></form><div class="summary-total"><div><span>Subtotal</span><b>${formatRupiah.format(subtotal)}</b></div><div><span>Biaya admin</span><b>${formatRupiah.format(ADMIN_FEE)}</b></div><div id="discount-row" hidden><span>Diskon</span><b id="discount-value"></b></div><div class="grand-total"><span>Total</span><strong id="grand-total">${formatRupiah.format(total())}</strong></div></div></aside></div></section>`;

      const buyerForm = document.querySelector<HTMLFormElement>("#buyer-form")!;
      const paymentForm = document.querySelector<HTMLFormElement>("#payment-form")!;
      const confirmation = document.querySelector<HTMLElement>("#confirmation")!;
      const success = document.querySelector<HTMLElement>("#success")!;
      const voucherForm = document.querySelector<HTMLFormElement>("#voucher-form")!;
      let finalTotal = 0;
      function setStep(next: number) {
        step = next;
        buyerForm.hidden = step !== 1 || completed;
        paymentForm.hidden = step !== 2 || completed;
        confirmation.hidden = step !== 3 || completed;
        document.querySelectorAll<HTMLElement>("[data-step]").forEach((item) => { const value = Number(item.dataset.step); item.toggleAttribute("aria-current", value === step && !completed); item.classList.toggle("is-complete", value < step || completed); });
        const target = completed ? success : document.querySelector<HTMLElement>(`#${step === 1 ? "buyer" : step === 2 ? "payment" : "confirmation"}-title`);
        target?.focus();
        target?.scrollIntoView({ behavior: "smooth", block: "start" });
      }
      function setError(name: keyof Buyer, valid: boolean, message: string) {
        const input = buyerForm.elements.namedItem(name) as HTMLInputElement;
        const error = document.querySelector<HTMLElement>(`[data-error="${name}"]`);
        input.setAttribute("aria-invalid", String(!valid));
        input.classList.toggle("is-invalid", !valid);
        if (error) error.textContent = valid ? "" : message;
        return valid;
      }
      function validateField(name: keyof Buyer) {
        buyer.name = (buyerForm.elements.namedItem("name") as HTMLInputElement).value.trim();
        buyer.email = (buyerForm.elements.namedItem("email") as HTMLInputElement).value.trim();
        buyer.phone = (buyerForm.elements.namedItem("phone") as HTMLInputElement).value.trim();
        buyer.identity = (buyerForm.elements.namedItem("identity") as HTMLInputElement).value.trim();
        if (name === "name") return setError(name, buyer.name.length >= 2, "Masukkan nama lengkap.");
        if (name === "email") return setError(name, isValidEmail(buyer.email), "Masukkan alamat email yang valid.");
        if (name === "phone") return setError(name, isValidPhone(buyer.phone), "Masukkan nomor HP yang valid.");
        return setError(name, isValidIdentity(buyer.identity), "Nomor identitas harus terdiri dari 12-20 angka.");
      }
      function validateBuyer() { return (["name", "email", "phone", "identity"] as const).map(validateField).every(Boolean); }
      buyerForm.addEventListener("submit", (event) => { event.preventDefault(); if (validateBuyer()) setStep(2); else buyerForm.querySelector<HTMLInputElement>("[aria-invalid=\"true\"]")?.focus(); });
      buyerForm.querySelectorAll<HTMLInputElement>("input").forEach((input) => input.addEventListener("blur", () => validateField(input.name as keyof Buyer)));
      paymentForm.addEventListener("submit", (event) => { event.preventDefault(); payment = (paymentForm.elements.namedItem("payment") as RadioNodeList).value; const error = document.querySelector<HTMLElement>("#payment-error")!; const fieldset = paymentForm.querySelector("fieldset")!; error.textContent = payment ? "" : "Pilih metode pembayaran terlebih dahulu."; fieldset.setAttribute("aria-invalid", String(!payment)); if (!payment) { paymentForm.querySelector<HTMLInputElement>("input[type=radio]")?.focus(); return; } document.querySelector<HTMLElement>("[data-buyer-name]")!.textContent = buyer.name; document.querySelector<HTMLElement>("[data-buyer-contact]")!.textContent = `${buyer.email} · ${buyer.phone}`; document.querySelector<HTMLElement>("[data-payment]")!.textContent = payment; setStep(3); });
      document.querySelectorAll<HTMLButtonElement>("[data-back], [data-edit]").forEach((button) => button.addEventListener("click", () => setStep(Number(button.dataset.back ?? button.dataset.edit))));
      voucherForm.addEventListener("submit", (event) => { event.preventDefault(); voucher = document.querySelector<HTMLInputElement>("#voucher")!.value; const discount = voucherDiscount(subtotal, voucher); const message = document.querySelector<HTMLElement>("#voucher-message")!; message.textContent = discount ? "Promo HEMAT10 diterapkan." : "Kode promo tidak valid."; message.classList.toggle("is-valid", Boolean(discount)); document.querySelector<HTMLElement>("#discount-row")!.hidden = !discount; document.querySelector<HTMLElement>("#discount-value")!.textContent = `-${formatRupiah.format(discount)}`; document.querySelector<HTMLElement>("#grand-total")!.textContent = formatRupiah.format(total()); });
      document.querySelector<HTMLButtonElement>("#place-order")!.addEventListener("click", () => { finalTotal = total(); completed = true; voucherForm.querySelectorAll<HTMLInputElement | HTMLButtonElement>("input, button").forEach((control) => { control.disabled = true; }); success.hidden = false; document.querySelector<HTMLElement>("[data-success-email]")!.textContent = buyer.email; document.querySelector<HTMLElement>("#booking-code")!.textContent = `TO-${concert.id.slice(0, 3).toUpperCase()}-${String(finalTotal).slice(-5)}`; document.querySelector<HTMLElement>("#grand-total")!.textContent = formatRupiah.format(finalTotal); setStep(3); });
    }
  }
}
