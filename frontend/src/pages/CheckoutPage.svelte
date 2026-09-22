<script lang="ts">
  import { onMount, tick } from "svelte";
  import { createReservation, getEvent, getReservation, ApiError, type Reservation } from "../lib/api.ts";
  import { eventDate, formatRupiah, type Concert } from "../lib/concerts.ts";
  import NotFoundPanel from "../components/NotFoundPanel.svelte";
  import { ADMIN_FEE, checkoutLines, isValidEmail, isValidIdentity, isValidPhone, orderSubtotal, voucherDiscount, type Buyer } from "../lib/checkout.ts";
  import { createTicketSnapshot, loadTicketSnapshot, saveTicketSnapshot } from "../lib/tickets.ts";
  import Toast from "../components/Toast.svelte";
  import { concertTerms } from "../lib/terms.ts";
  import { RESERVATION_DURATION_MS, isReservationExpired, remainingReservationSeconds, reservationBasketKey, parseStoredReservation, serializeStoredReservation, type StoredReservation } from "../lib/reservation.ts";
  let { id }: { id: string } = $props();
  let concert = $state<Concert | undefined>();
  let loading = $state(true);
  let loadError = $state("");
  const lines = $derived(concert ? checkoutLines(concert, new URLSearchParams(location.search)) : []);
  let reservation = $state<Reservation | undefined>();
  let reservationError = $state("");
  let creatingReservation = $state(false);
  const subtotal = $derived(reservation?.subtotal ?? 0);
  let step = $state(1); let payment = $state(""); let voucherInput = $state(""); let voucher = $state(""); let completed = $state(false); let ticketId = $state(""); let reference = $state(""); let storageFailed = $state(false);
  let buyer = $state<Buyer>({ name: "", email: "", phone: "", identity: "" });
  let errors = $state<Record<keyof Buyer, string>>({ name: "", email: "", phone: "", identity: "" });
  let paymentError = $state(""); let termsDialog = $state<HTMLDialogElement>(); let expiryDialog = $state<HTMLDialogElement>(); let toast = $state<{ id: number; message: string; tone: "success" | "info" | "error" } | null>(null); let toastId = 0; let expiresAt = $state<number | null>(null); let reservationExpired = $state(false); let remainingSeconds = $state(RESERVATION_DURATION_MS / 1_000); let storedReservation = $state<StoredReservation | null>(null);
  const discount = $derived(voucherDiscount(subtotal, voucher));
  const total = $derived(subtotal + ADMIN_FEE - discount);
  const quantityMap = $derived(Object.fromEntries(lines.map((line) => [line.tier.id, line.quantity])));
  const reservationKey = $derived(concert ? reservationBasketKey(concert.id, quantityMap) : "");
  const completionKey = $derived(`${reservationKey}:completed`);
  async function goToStep(next: number) { step = next; await tick(); const name = next === 1 ? "buyer" : next === 2 ? "payment" : "confirmation"; const heading = document.querySelector<HTMLElement>(`#${name}-title`); heading?.focus(); heading?.scrollIntoView({ behavior: matchMedia("(prefers-reduced-motion: reduce)").matches ? "auto" : "smooth", block: "start" }); }
  function validate(name: keyof Buyer) {
    const value = buyer[name].trim(); buyer[name] = value;
    const valid = name === "name" ? value.length >= 2 && value.length <= 80 : name === "email" ? isValidEmail(value) : name === "phone" ? isValidPhone(value) : isValidIdentity(value);
    errors[name] = valid ? "" : name === "name" ? "Masukkan nama lengkap maksimal 80 karakter." : name === "email" ? "Masukkan alamat email yang valid." : name === "phone" ? "Masukkan nomor HP yang valid." : "Nomor identitas harus terdiri dari 12-20 angka.";
    return valid;
  }
  async function submitBuyer(event: SubmitEvent) { event.preventDefault(); if (checkExpiry()) return; const valid = (["name", "email", "phone", "identity"] as const).map(validate).every(Boolean); if (valid) await goToStep(2); else await tick().then(() => document.querySelector<HTMLInputElement>('[aria-invalid="true"]')?.focus()); }
  async function submitPayment(event: SubmitEvent) { event.preventDefault(); if (checkExpiry()) return; paymentError = payment ? "" : "Pilih metode pembayaran terlebih dahulu."; if (!payment) { await tick(); document.querySelector<HTMLInputElement>('input[name="payment"]')?.focus(); return; } await goToStep(3); }
  function showToast(message: string, tone: "success" | "info" | "error" = "info") { toast = { id: ++toastId, message, tone }; }
  function openTerms() { termsDialog?.showModal(); }
  function closeDialog(dialog: HTMLDialogElement) { dialog.close(); }
  function restartSelection() { try { sessionStorage.removeItem(reservationKey); sessionStorage.removeItem(completionKey); } catch { /* storage can be unavailable */ } }
  function recoverExpiredCheckout() { restartSelection(); if (concert) location.assign(`/konser/${concert.id}?${location.search.slice(1)}`); }
  function checkExpiry() {
    if (completed || expiresAt === null || !isReservationExpired(expiresAt)) return false;
    reservationExpired = true;
    try { sessionStorage.removeItem(reservationKey); } catch { /* storage can be unavailable */ }
    if (!expiryDialog?.open) expiryDialog?.showModal();
    return true;
  }
  function applyVoucher(event: SubmitEvent) { event.preventDefault(); if (reservationExpired || checkExpiry()) return; voucher = voucherInput; showToast(discount ? "Voucher diskon berhasil diterapkan." : "Kode voucher tidak valid.", discount ? "success" : "error"); }
  async function completeOrder() {
    if (!concert || reservationExpired || checkExpiry()) return;
    completed = true; ticketId = crypto.randomUUID(); reference = `TO-${crypto.randomUUID().replaceAll("-", "").slice(0, 10).toUpperCase()}`;
    storageFailed = !saveTicketSnapshot(createTicketSnapshot(ticketId, reference, buyer.name, concert, lines));
    if (!storageFailed) try { sessionStorage.setItem(completionKey, ticketId); } catch { /* storage can be unavailable */ }
    try { sessionStorage.removeItem(reservationKey); } catch { /* storage can be unavailable */ }
    await tick(); document.querySelector<HTMLElement>("#success-title")?.focus();
  }
  async function setupReservation(signal: AbortSignal) {
    if (!concert || !lines.length) return;
    creatingReservation = true;
    reservationError = "";
    let stored: StoredReservation | null = null;
    try { stored = parseStoredReservation(sessionStorage.getItem(reservationKey)); } catch { /* storage can be unavailable */ }
    let completedTicketId: string | null = null;
    try { completedTicketId = sessionStorage.getItem(completionKey); } catch { /* storage can be unavailable */ }
    if (completedTicketId && loadTicketSnapshot(completedTicketId)) { location.replace(`/tiket/${completedTicketId}`); return; }
    const basketKey = reservationKey.slice(reservationKey.lastIndexOf(":") + 1);
    const sameBasket = stored?.eventId === concert.id && stored.basketKey === basketKey;
    const idempotencyKey = sameBasket && stored ? stored.idempotencyKey : crypto.randomUUID().replaceAll("-", "");
    try {
      reservation = sameBasket && stored ? await getReservation(stored.reservationId, signal) : await createReservation(concert.id, lines.map((line) => ({ tierId: line.tier.id, quantity: line.quantity })), idempotencyKey, signal);
      storedReservation = { reservationId: reservation.id, idempotencyKey, eventId: concert.id, basketKey };
      try { sessionStorage.setItem(reservationKey, serializeStoredReservation(storedReservation)); } catch { storageFailed = true; }
      expiresAt = Date.parse(reservation.expiresAt);
    } catch (value) {
      reservationError = value instanceof ApiError ? value.message : "Reservasi belum dapat dibuat.";
      if (value instanceof ApiError && value.code === "RESERVATION_EXPIRED") reservationExpired = true;
      creatingReservation = false;
      return;
    }
    creatingReservation = false;
    const update = () => { remainingSeconds = remainingReservationSeconds(expiresAt ?? 0); if (remainingSeconds === 0) checkExpiry(); };
    update();
    const timer = window.setInterval(update, 1_000);
    const visibility = () => update();
    document.addEventListener("visibilitychange", visibility);
    return () => { window.clearInterval(timer); document.removeEventListener("visibilitychange", visibility); };
  }
  onMount(() => {
    const controller = new AbortController();
    let cleanupReservation: (() => void) | undefined;
    getEvent(id, controller.signal).then(async (event) => { if (!controller.signal.aborted) { concert = event; loading = false; cleanupReservation = await setupReservation(controller.signal); } }).catch((value) => { if (!controller.signal.aborted) { loadError = value instanceof ApiError && value.code === "EVENT_NOT_FOUND" ? "not-found" : value instanceof ApiError ? value.message : "Checkout belum dapat dimuat."; loading = false; } });
    return () => { controller.abort(); cleanupReservation?.(); };
  });
</script>

  <svelte:head><title>{!concert ? "Checkout tidak ditemukan | Tiket Online" : !lines.length ? "Keranjang kosong | Tiket Online" : `Checkout ${concert.artist} | Tiket Online`}</title><meta name="description" content={concert ? `Checkout tiket konser ${concert.artist} di Tiket Online.` : "Checkout tiket konser di Tiket Online."} /><meta name="robots" content="noindex" /></svelte:head>
{#if loading}
  <p class="shell" role="status" aria-live="polite">Memuat checkout...</p>
{:else if !concert || loadError === "not-found"}
  <NotFoundPanel className="checkout-missing" title="Konser tidak ditemukan." message="URL checkout ini tidak tepat." />
{:else if loadError}
  <section class="shell empty-state" role="alert"><p>{loadError}</p><button class="text-button" type="button" onclick={() => location.reload()}>Coba lagi</button></section>
{:else if !lines.length}
  <NotFoundPanel className="checkout-missing" kicker="Keranjang kosong" title="Pilih tiket terlebih dahulu." message="Belum ada tiket valid yang dapat dilanjutkan ke checkout ini." href={`/konser/${concert.id}`} link="Pilih tiket" />
{:else if reservationError}
  <section class="shell empty-state" role="alert"><p>{reservationError}</p><button class="text-button" type="button" onclick={() => location.reload()}>Coba lagi</button></section>
{:else if creatingReservation || !reservation}
  <p class="shell" role="status" aria-live="polite">Menahan tiket sementara...</p>
{:else}
   <section class="checkout shell"><a class="back-link" href={`/konser/${concert.id}?${location.search.slice(1)}`}>Kembali ke detail konser</a><div class="checkout-heading"><p class="checkout-kicker">Checkout aman</p><h1>Selesaikan pesananmu.</h1><p>{concert.artist} · {concert.date} · {concert.venue}</p></div>{#if !completed}<div class:reservation-warning={remainingSeconds <= 60} class="reservation-banner" role="timer"><span class="reservation-icon" aria-hidden="true">◷</span><span><b>Reservasi tiket sementara</b><small>Selesaikan pembayaran dalam {String(Math.floor(remainingSeconds / 60)).padStart(2, "0")}:{String(remainingSeconds % 60).padStart(2, "0")}</small></span></div>{/if}<ol class="checkout-steps" aria-label="Tahap checkout">{#each ["Data Diri", "Metode Bayar", "Konfirmasi"] as label, index}<li class:is-complete={index + 1 < step || completed} aria-current={index + 1 === step && !completed ? "step" : undefined}><span>{index + 1}</span><b>{label}</b></li>{/each}</ol>
    <div class="checkout-layout"><section class="checkout-panel">
       {#if completed}<section class="checkout-success"><p class="checkout-kicker">Pesanan berhasil</p><h2 id="success-title" tabindex="-1">E-ticket sudah dibuat.</h2><p>Konfirmasi demo telah disiapkan untuk {buyer.email}.</p><div class="booking-code"><span>Reference pesanan</span><strong>{reference}</strong></div>{#if storageFailed}<p class="success-note">Pesanan dan reference berhasil dibuat, tetapi e-ticket demo tidak dapat disimpan di browser ini.</p>{:else}<a class="button" href={`/tiket/${ticketId}`}>Lihat e-ticket</a>{/if}<a class="button button-secondary" href="/konser">Cari konser lain</a></section>
      {:else if step === 1}<form id="buyer-form" novalidate onsubmit={submitBuyer}><div class="step-title"><p>Langkah 1 dari 3</p><h2 id="buyer-title" tabindex="-1">Data pemesan</h2><span>Informasi ini digunakan untuk mengirim e-ticket.</span></div><div class="field-grid">{#each [{ name: "name", label: "Nama lengkap", type: "text" }, { name: "email", label: "Email", type: "email" }, { name: "phone", label: "No. HP", type: "tel" }, { name: "identity", label: "No. identitas", type: "text" }] as field}<label for={`buyer-${field.name}`}>{field.label}<input id={`buyer-${field.name}`} name={field.name} type={field.type} autocomplete={field.name === "name" ? "name" : field.name === "email" ? "email" : field.name === "phone" ? "tel" : "off"} inputmode={field.name === "phone" ? "tel" : field.name === "identity" ? "numeric" : undefined} maxlength={field.name === "name" ? 80 : undefined} bind:value={buyer[field.name as keyof Buyer]} aria-describedby={`${field.name}-error`} aria-invalid={Boolean(errors[field.name as keyof Buyer])} class:is-invalid={Boolean(errors[field.name as keyof Buyer])} onblur={() => validate(field.name as keyof Buyer)} required /><small id={`${field.name}-error`} aria-live="polite">{errors[field.name as keyof Buyer]}</small></label>{/each}</div><div class="step-actions"><span></span><button class="button" type="submit">Lanjut ke pembayaran</button></div></form>
       {:else if step === 2}<form id="payment-form" onsubmit={submitPayment}><div class="step-title"><p>Langkah 2 dari 3</p><h2 id="payment-title" tabindex="-1">Pilih metode bayar.</h2><span>Pembayaran di halaman ini hanya simulasi.</span></div><fieldset aria-describedby="payment-error" class:has-error={Boolean(paymentError)}><legend class="sr-only">Metode pembayaran</legend><div class="payment-options">{#each ["QRIS", "Virtual Account", "GoPay"] as method}<label class="payment-card"><input bind:group={payment} type="radio" name="payment" value={method} aria-describedby="payment-error" /><span class="payment-logo" class:gopay={method === "GoPay"}>{method}</span><span><b>{method}</b><small>Simulasi pembayaran</small></span></label>{/each}</div></fieldset><p id="payment-error" class="form-error" aria-live="polite">{paymentError}</p><div class="step-actions"><button class="text-button" type="button" onclick={() => goToStep(1)}>Kembali</button><button class="button" type="submit">Tinjau pesanan</button></div></form>
       {:else}<section id="confirmation"><div class="step-title"><p>Langkah 3 dari 3</p><h2 id="confirmation-title" tabindex="-1">Periksa sebelum memesan.</h2><span>Pastikan data dan pesananmu sudah benar.</span></div><div class="confirmation-block"><div><span>Pemesan</span><b>{buyer.name}</b><p>{buyer.email} · {buyer.phone}</p></div><button class="text-button" type="button" onclick={() => goToStep(1)}>Ubah data</button></div><div class="confirmation-block"><div><span>Pembayaran</span><b>{payment}</b></div><button class="text-button" type="button" onclick={() => goToStep(2)}>Ubah metode</button></div><p class="terms-trigger">Dengan melanjutkan, kamu menyetujui <button class="text-button" type="button" onclick={openTerms}>S&K Konser</button>.</p><div class="step-actions"><button class="text-button" type="button" onclick={() => goToStep(2)}>Kembali</button><button class="button" type="button" disabled={reservationExpired} onclick={completeOrder}>Buat pesanan</button></div></section>{/if}
       </section><aside class="order-summary"><h2>Ringkasan pesanan</h2><p class="summary-event">{concert.artist}</p><p>{eventDate(concert.startsAt)}</p>{#each reservation.items as line}<div class="summary-line"><span>{line.quantity}x {line.name}</span><b>{formatRupiah.format(line.lineTotal)}</b></div>{/each}<div class="summary-line"><span>Biaya admin</span><b>{formatRupiah.format(ADMIN_FEE)}</b></div>{#if discount}<div class="summary-line discount-row"><span>Diskon</span><b>-{formatRupiah.format(discount)}</b></div>{/if}<form class="voucher-form" onsubmit={applyVoucher}><label for="voucher">Kode promo<input bind:value={voucherInput} id="voucher" autocomplete="off" disabled={completed || reservationExpired} /></label><button class="button button-small" type="submit" disabled={completed || reservationExpired}>Gunakan</button></form>{#if voucher}<p class:is-valid={Boolean(discount)} class="voucher-message" aria-live="polite">{discount ? "Promo HEMAT10 diterapkan." : "Kode voucher tidak valid."}</p>{/if}<div class="grand-total"><span>Total</span><strong>{formatRupiah.format(total)}</strong></div><button class="terms-button" type="button" onclick={openTerms}>Lihat S&amp;K Konser</button></aside></div>
   </section>
   <dialog class="terms-dialog" bind:this={termsDialog} aria-labelledby="terms-title"><div class="dialog-heading"><p class="checkout-kicker">Informasi penting</p><h2 id="terms-title">Syarat &amp; ketentuan konser</h2></div><div class="terms-dialog-list">{#each concertTerms as term}<section><h3>{term.title}</h3><p>{term.body}</p></section>{/each}</div><form method="dialog" class="dialog-actions"><button class="button" type="submit">Tutup</button></form></dialog>
    <dialog class="expiry-dialog" bind:this={expiryDialog} aria-labelledby="expiry-title" oncancel={(event) => { event.preventDefault(); recoverExpiredCheckout(); }}><p class="checkout-kicker">Reservasi berakhir</p><h2 id="expiry-title">Waktu checkout habis.</h2><p>Waktu reservasi simulasi sudah selesai. Pilih tiket lagi untuk memulai sesi checkout baru.</p><button class="button" type="button" onclick={recoverExpiredCheckout}>Pilih tiket lagi</button></dialog>
    {#if toast}<Toast id={toast.id} message={toast.message} tone={toast.tone} onDismiss={() => toast = null} />{/if}
{/if}
