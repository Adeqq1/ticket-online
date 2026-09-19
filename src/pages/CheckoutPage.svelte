<script lang="ts">
  import { tick } from "svelte";
  import { eventDate, formatRupiah, getConcertById } from "../lib/concerts.ts";
  import NotFoundPanel from "../components/NotFoundPanel.svelte";
  import { ADMIN_FEE, checkoutLines, isValidEmail, isValidIdentity, isValidPhone, orderSubtotal, voucherDiscount, type Buyer } from "../lib/checkout.ts";
  import { createTicketSnapshot, ticketStorageKey } from "../lib/tickets.ts";
  let { id }: { id: string } = $props();
  const concert = $derived(getConcertById(id));
  const lines = $derived(concert ? checkoutLines(concert, new URLSearchParams(location.search)) : []);
  const subtotal = $derived(orderSubtotal(lines));
  let step = $state(1); let payment = $state(""); let voucherInput = $state(""); let voucher = $state(""); let completed = $state(false); let ticketId = $state(""); let reference = $state(""); let storageFailed = $state(false);
  let buyer = $state<Buyer>({ name: "", email: "", phone: "", identity: "" });
  let errors = $state<Record<keyof Buyer, string>>({ name: "", email: "", phone: "", identity: "" });
  let paymentError = $state("");
  const discount = $derived(voucherDiscount(subtotal, voucher));
  const total = $derived(subtotal + ADMIN_FEE - discount);
  async function goToStep(next: number) { step = next; await tick(); const name = next === 1 ? "buyer" : next === 2 ? "payment" : "confirmation"; const heading = document.querySelector<HTMLElement>(`#${name}-title`); heading?.focus(); heading?.scrollIntoView({ behavior: "smooth", block: "start" }); }
  function validate(name: keyof Buyer) {
    const value = buyer[name].trim(); buyer[name] = value;
    const valid = name === "name" ? value.length >= 2 && value.length <= 80 : name === "email" ? isValidEmail(value) : name === "phone" ? isValidPhone(value) : isValidIdentity(value);
    errors[name] = valid ? "" : name === "name" ? "Masukkan nama lengkap maksimal 80 karakter." : name === "email" ? "Masukkan alamat email yang valid." : name === "phone" ? "Masukkan nomor HP yang valid." : "Nomor identitas harus terdiri dari 12-20 angka.";
    return valid;
  }
  async function submitBuyer(event: SubmitEvent) { event.preventDefault(); const valid = (["name", "email", "phone", "identity"] as const).map(validate).every(Boolean); if (valid) await goToStep(2); else await tick().then(() => document.querySelector<HTMLInputElement>('[aria-invalid="true"]')?.focus()); }
  async function submitPayment(event: SubmitEvent) { event.preventDefault(); paymentError = payment ? "" : "Pilih metode pembayaran terlebih dahulu."; if (!payment) { await tick(); document.querySelector<HTMLInputElement>('input[name="payment"]')?.focus(); return; } await goToStep(3); }
  function applyVoucher(event: SubmitEvent) { event.preventDefault(); voucher = voucherInput; }
  async function completeOrder() {
    if (!concert) return;
    completed = true; ticketId = crypto.randomUUID(); reference = `TO-${crypto.randomUUID().replaceAll("-", "").slice(0, 10).toUpperCase()}`;
    try { localStorage.setItem(ticketStorageKey(ticketId), JSON.stringify(createTicketSnapshot(ticketId, reference, buyer.name, concert, lines))); }
    catch { storageFailed = true; }
    await tick(); document.querySelector<HTMLElement>("#success-title")?.focus();
  }
</script>

<svelte:head><title>{!concert ? "Checkout tidak ditemukan | Tiket Online" : !lines.length ? "Keranjang kosong | Tiket Online" : `Checkout ${concert.artist} | Tiket Online`}</title><meta name="description" content={concert ? `Checkout tiket konser ${concert.artist} di Tiket Online.` : "Checkout tiket konser di Tiket Online."} /></svelte:head>
{#if !concert}
  <NotFoundPanel className="checkout-missing" title="Konser tidak ditemukan." message="URL checkout ini tidak tepat." />
{:else if !lines.length}
  <NotFoundPanel className="checkout-missing" kicker="Keranjang kosong" title="Pilih tiket terlebih dahulu." message="Belum ada tiket valid yang dapat dilanjutkan ke checkout ini." href={`/konser/${concert.id}`} link="Pilih tiket" />
{:else}
  <section class="checkout shell"><a class="back-link" href={`/konser/${concert.id}?${location.search.slice(1)}`}>Kembali ke detail konser</a><div class="checkout-heading"><p class="checkout-kicker">Checkout aman</p><h1>Selesaikan pesananmu.</h1><p>{concert.artist} · {concert.date} · {concert.venue}</p></div><ol class="checkout-steps" aria-label="Tahap checkout">{#each ["Data Diri", "Metode Bayar", "Konfirmasi"] as label, index}<li class:is-complete={index + 1 < step || completed} aria-current={index + 1 === step && !completed ? "step" : undefined}><span>{index + 1}</span><b>{label}</b></li>{/each}</ol>
    <div class="checkout-layout"><section class="checkout-panel">
       {#if completed}<section class="checkout-success"><p class="checkout-kicker">Pesanan berhasil</p><h2 id="success-title" tabindex="-1">E-ticket sudah dibuat.</h2><p>Konfirmasi demo telah disiapkan untuk {buyer.email}.</p><div class="booking-code"><span>Reference pesanan</span><strong>{reference}</strong></div>{#if storageFailed}<p class="success-note">Pesanan dan reference berhasil dibuat, tetapi e-ticket demo tidak dapat disimpan di browser ini.</p>{:else}<a class="button" href={`/tiket/${ticketId}`}>Lihat e-ticket</a>{/if}<a class="button button-secondary" href="/konser">Cari konser lain</a></section>
      {:else if step === 1}<form id="buyer-form" novalidate onsubmit={submitBuyer}><div class="step-title"><p>Langkah 1 dari 3</p><h2 id="buyer-title" tabindex="-1">Data pemesan</h2><span>Informasi ini digunakan untuk mengirim e-ticket.</span></div><div class="field-grid">{#each [{ name: "name", label: "Nama lengkap", type: "text" }, { name: "email", label: "Email", type: "email" }, { name: "phone", label: "No. HP", type: "tel" }, { name: "identity", label: "No. identitas", type: "text" }] as field}<label for={`buyer-${field.name}`}>{field.label}<input id={`buyer-${field.name}`} name={field.name} type={field.type} autocomplete={field.name === "name" ? "name" : field.name === "email" ? "email" : field.name === "phone" ? "tel" : "off"} inputmode={field.name === "phone" ? "tel" : field.name === "identity" ? "numeric" : undefined} maxlength={field.name === "name" ? 80 : undefined} bind:value={buyer[field.name as keyof Buyer]} aria-describedby={`${field.name}-error`} aria-invalid={Boolean(errors[field.name as keyof Buyer])} class:is-invalid={Boolean(errors[field.name as keyof Buyer])} onblur={() => validate(field.name as keyof Buyer)} required /><small id={`${field.name}-error`} aria-live="polite">{errors[field.name as keyof Buyer]}</small></label>{/each}</div><div class="step-actions"><span></span><button class="button" type="submit">Lanjut ke pembayaran</button></div></form>
       {:else if step === 2}<form id="payment-form" onsubmit={submitPayment}><div class="step-title"><p>Langkah 2 dari 3</p><h2 id="payment-title" tabindex="-1">Pilih metode bayar.</h2><span>Pembayaran di halaman ini hanya simulasi.</span></div><fieldset aria-describedby="payment-error" class:has-error={Boolean(paymentError)}><legend class="sr-only">Metode pembayaran</legend><div class="payment-options">{#each ["QRIS", "Virtual Account", "GoPay"] as method}<label class="payment-card"><input bind:group={payment} type="radio" name="payment" value={method} aria-describedby="payment-error" /><span class="payment-logo" class:gopay={method === "GoPay"}>{method}</span><span><b>{method}</b><small>Simulasi pembayaran</small></span></label>{/each}</div></fieldset><p id="payment-error" class="form-error" aria-live="polite">{paymentError}</p><div class="step-actions"><button class="text-button" type="button" onclick={() => goToStep(1)}>Kembali</button><button class="button" type="submit">Tinjau pesanan</button></div></form>
       {:else}<section id="confirmation"><div class="step-title"><p>Langkah 3 dari 3</p><h2 id="confirmation-title" tabindex="-1">Periksa sebelum memesan.</h2><span>Pastikan data dan pesananmu sudah benar.</span></div><div class="confirmation-block"><div><span>Pemesan</span><b>{buyer.name}</b><p>{buyer.email} · {buyer.phone}</p></div><button class="text-button" type="button" onclick={() => goToStep(1)}>Ubah data</button></div><div class="confirmation-block"><div><span>Pembayaran</span><b>{payment}</b></div><button class="text-button" type="button" onclick={() => goToStep(2)}>Ubah metode</button></div><div class="step-actions"><button class="text-button" type="button" onclick={() => goToStep(2)}>Kembali</button><button class="button" type="button" onclick={completeOrder}>Buat pesanan</button></div></section>{/if}
     </section><aside class="order-summary"><h2>Ringkasan pesanan</h2><p class="summary-event">{concert.artist}</p><p>{eventDate(concert.startsAt)}</p>{#each lines as line}<div class="summary-line"><span>{line.quantity}x {line.tier.name}</span><b>{formatRupiah.format(line.tier.price * line.quantity)}</b></div>{/each}<div class="summary-line"><span>Biaya admin</span><b>{formatRupiah.format(ADMIN_FEE)}</b></div>{#if discount}<div class="summary-line discount-row"><span>Diskon</span><b>-{formatRupiah.format(discount)}</b></div>{/if}<form class="voucher-form" onsubmit={applyVoucher}><label for="voucher">Kode promo<input bind:value={voucherInput} id="voucher" autocomplete="off" disabled={completed} /></label><button class="button button-small" type="submit" disabled={completed}>Gunakan</button></form>{#if voucher}<p class:is-valid={Boolean(discount)} class="voucher-message" aria-live="polite">{discount ? "Promo HEMAT10 diterapkan." : "Kode promo tidak valid."}</p>{/if}<div class="grand-total"><span>Total</span><strong>{formatRupiah.format(total)}</strong></div></aside></div>
  </section>
{/if}
