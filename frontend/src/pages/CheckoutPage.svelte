<script lang="ts">
  import { onMount, tick } from "svelte";
  import { createOrder, createReservation, getEvent, getOrder, getReservation, listOrderTickets, simulatePayment, ApiError, type ApiTicket, type OrderBase, type PaymentMethod, type Reservation } from "../lib/api.ts";
  import { eventDate, formatRupiah, type Concert } from "../lib/concerts.ts";
  import NotFoundPanel from "../components/NotFoundPanel.svelte";
  import { ADMIN_FEE, checkoutLines, isValidEmail, isValidIdentity, isValidPhone, voucherDiscount, type Buyer } from "../lib/checkout.ts";
  import { createTicketSnapshot, loadTicketSnapshot, saveTicketSnapshot } from "../lib/tickets.ts";
  import { listOrderAccess, removeOrderAccess, saveOrderAccess, type OrderAccess } from "../lib/order-access.ts";
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
  let activeOrder = $state<OrderBase | undefined>(); let orderAccess = $state<OrderAccess | null>(null); let paymentStatus = $state("");
  let issuedTickets = $state<ApiTicket[]>([]);
  const subtotal = $derived(activeOrder?.subtotal ?? reservation?.subtotal ?? 0);
  let step = $state(1); let payment = $state<PaymentMethod | "">(""); let voucherInput = $state(""); let voucher = $state(""); let completed = $state(false); let ticketIds = $state<string[]>([]); let reference = $state(""); let storageFailed = $state(false);
  let buyer = $state<Buyer>({ name: "", email: "", phone: "", identity: "" });
  let attendeeNames = $state<Record<string, string[]>>({}); let attendeeErrors = $state<Record<string, string[]>>({});
  let errors = $state<Record<keyof Buyer, string>>({ name: "", email: "", phone: "", identity: "" });
  let paymentError = $state(""); let termsDialog = $state<HTMLDialogElement>(); let expiryDialog = $state<HTMLDialogElement>(); let toast = $state<{ id: number; message: string; tone: "success" | "info" | "error" } | null>(null); let toastId = 0; let expiresAt = $state<number | null>(null); let reservationExpired = $state(false); let remainingSeconds = $state(RESERVATION_DURATION_MS / 1_000); let storedReservation = $state<StoredReservation | null>(null); let converting = $state(false);
  const discount = $derived(activeOrder?.discount ?? voucherDiscount(subtotal, voucher));
  const total = $derived(activeOrder?.total ?? subtotal + ADMIN_FEE - discount);
  const paymentLabel = $derived(payment === "VIRTUAL_ACCOUNT" ? "Virtual Account" : payment === "GOPAY" ? "GoPay" : payment);
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
  function setAttendeeName(tierId: string, index: number, value: string) { const names = [...(attendeeNames[tierId] ?? [])]; names[index] = value; attendeeNames[tierId] = names; }
  async function submitBuyer(event: SubmitEvent) { event.preventDefault(); if (checkExpiry()) return; const valid = (["name", "email", "phone", "identity"] as const).map(validate).every(Boolean); const attendeesValid = lines.every((line) => { const names = attendeeNames[line.tier.id] ?? []; const messages = names.map((name) => name.trim().length >= 2 && name.trim().length <= 80 ? "" : "Nama peserta harus 2–80 karakter."); attendeeErrors[line.tier.id] = messages; return names.length === line.quantity && messages.every((error) => !error); }); if (valid && attendeesValid) await goToStep(2); else await tick().then(() => document.querySelector<HTMLInputElement>('[aria-invalid="true"]')?.focus()); }
  async function submitPayment(event: SubmitEvent) { event.preventDefault(); if (checkExpiry()) return; paymentError = payment ? "" : "Pilih metode pembayaran terlebih dahulu."; if (!payment) { await tick(); document.querySelector<HTMLInputElement>('input[name="payment"]')?.focus(); return; } await goToStep(3); }
  function showToast(message: string, tone: "success" | "info" | "error" = "info") { toast = { id: ++toastId, message, tone }; }
  function openTerms() { termsDialog?.showModal(); }
  function closeDialog(dialog: HTMLDialogElement) { dialog.close(); }
  function restartSelection() { try { sessionStorage.removeItem(reservationKey); sessionStorage.removeItem(completionKey); } catch { /* storage can be unavailable */ } }
  function recoverExpiredCheckout() { restartSelection(); if (activeOrder) removeOrderAccess(activeOrder.id); if (concert) location.assign(`/konser/${concert.id}?${location.search.slice(1)}`); }
  function checkExpiry() {
    if (completed || activeOrder?.status === "PAID" || expiresAt === null || !isReservationExpired(expiresAt)) return false;
    reservationExpired = true;
    try { sessionStorage.removeItem(reservationKey); } catch { /* storage can be unavailable */ }
    if (!expiryDialog?.open) expiryDialog?.showModal();
    return true;
  }
  function applyVoucher(event: SubmitEvent) { event.preventDefault(); if (activeOrder || reservationExpired || checkExpiry()) return; voucher = voucherInput.trim().toUpperCase(); showToast(voucher === "HEMAT10" ? "Estimasi diskon HEMAT10 diterapkan." : "Kode voucher akan divalidasi saat pesanan dibuat.", voucher === "HEMAT10" ? "success" : "info"); }
  function startExpiryTimer() {
    const update = () => { remainingSeconds = remainingReservationSeconds(expiresAt ?? 0); if (remainingSeconds === 0) checkExpiry(); };
    update();
    const timer = window.setInterval(update, 1_000);
    const visibility = () => update();
    document.addEventListener("visibilitychange", visibility);
    return () => { window.clearInterval(timer); document.removeEventListener("visibilitychange", visibility); };
  }
  function saveIssuedTickets(tickets: ApiTicket[]) {
    if (!concert || !orderAccess) return;
    issuedTickets = tickets;
    ticketIds = tickets.map((ticket) => ticket.id);
    orderAccess = { ...orderAccess, ticketIds };
    if (!saveOrderAccess(orderAccess)) storageFailed = true;
    for (const ticket of tickets) {
      const tier = concert.ticketTiers.find((item) => item.name === ticket.tierName);
      if (tier && !saveTicketSnapshot({ ...createTicketSnapshot(ticket.id, ticket.orderReference, ticket.attendeeName, concert, [{ tier, quantity: 1 }]), issuedAt: ticket.issuedAt })) storageFailed = true;
    }
  }
  async function completeOrder(result: "SUCCEEDED" | "FAILED" = "SUCCEEDED") {
    if (!concert || !reservation || converting || (reservationExpired && !activeOrder) || checkExpiry()) return;
    converting = true;
    try {
      if (!activeOrder) {
        if (!storedReservation) throw new Error("Data idempotensi reservasi tidak tersedia");
        const payload = { buyer, attendees: lines.map((line) => ({ tierId: line.tier.id, names: (attendeeNames[line.tier.id] ?? []).map((name) => name.trim()) })), ...(voucher ? { voucherCode: voucher } : {}) };
        const created = await createOrder(reservation.id, payload, storedReservation.idempotencyKey);
        activeOrder = created;
        orderAccess = { orderId: created.id, accessToken: created.accessToken, expiresAt: created.expiresAt, accessExpiresAt: created.accessExpiresAt, reservationId: created.reservationId, idempotencyKey: storedReservation.idempotencyKey, basketKey: reservationKey, reference: created.reference, ticketIds: [] };
        if (!saveOrderAccess(orderAccess)) { storageFailed = true; showToast("Akses pesanan hanya tersimpan selama halaman ini terbuka.", "error"); }
        reference = created.reference;
        expiresAt = Date.parse(created.expiresAt);
        startExpiryTimer();
      }
      if (!orderAccess || !activeOrder) throw new Error("Akses order tidak tersedia di memori browser");
      const resultPayment = await simulatePayment(activeOrder.id, { method: payment || "QRIS", result }, orderAccess.accessToken);
      paymentStatus = resultPayment.status;
      if (resultPayment.orderStatus === "PAID") activeOrder = { ...activeOrder, status: "PAID" };
      if (resultPayment.status === "FAILED") paymentError = "Pembayaran simulasi gagal. Coba bayar kembali untuk menyelesaikan pesanan.";
      else {
        const tickets = resultPayment.tickets.length ? resultPayment.tickets : await listOrderTickets(activeOrder.id, orderAccess.accessToken);
        saveIssuedTickets(tickets);
        if (!tickets.length) throw new Error("Pembayaran berhasil, tetapi tiket belum dapat dimuat. Coba muat ulang halaman.");
        completed = true;
        paymentError = "";
        await tick(); document.querySelector<HTMLElement>("#success-title")?.focus();
      }
    } catch (value) {
      if (value instanceof ApiError && ["RESERVATION_EXPIRED", "ORDER_NOT_PAYABLE"].includes(value.code)) {
        reservationExpired = true;
        if (!expiryDialog?.open) expiryDialog?.showModal();
      } else if (value instanceof ApiError && value.code === "RESERVATION_CANCELLED") reservationError = value.message;
      else if (value instanceof ApiError && value.code === "INVALID_VOUCHER") paymentError = value.message;
      else paymentError = value instanceof ApiError ? value.message : "Pesanan belum dapat dibuat. Coba lagi.";
    } finally { converting = false; }
  }
  async function setupReservation(signal: AbortSignal) {
    if (!concert || !lines.length) return;
    creatingReservation = true;
    reservationError = "";
    let stored: StoredReservation | null = null;
    try { stored = parseStoredReservation(sessionStorage.getItem(reservationKey)); } catch { /* storage can be unavailable */ }
    let completedTicketId: string | null = null;
    try { completedTicketId = sessionStorage.getItem(completionKey); } catch { /* storage can be unavailable */ }
    const basketKey = reservationKey.slice(reservationKey.lastIndexOf(":") + 1);
    for (const savedOrder of listOrderAccess()) if (savedOrder.basketKey === reservationKey) {
      try {
        const detail = await getOrder(savedOrder.orderId, savedOrder.accessToken, signal);
        if (signal.aborted) return;
        orderAccess = savedOrder; activeOrder = detail; reference = detail.reference; buyer = detail.buyer;
        attendeeNames = Object.fromEntries(lines.map((line) => [line.tier.id, detail.attendees.filter((attendee) => attendee.tierId === line.tier.id).map((attendee) => attendee.name)]));
        reservation = { id: detail.reservationId, status: detail.status, expiresAt: detail.expiresAt, event: { id: concert.id, artist: concert.artist }, items: detail.items, subtotal: detail.subtotal };
        storedReservation = { reservationId: detail.reservationId, idempotencyKey: savedOrder.idempotencyKey, eventId: concert.id, basketKey };
        expiresAt = Date.parse(detail.expiresAt);
        step = 3; payment = detail.payment?.method ?? "QRIS"; paymentStatus = detail.payment?.status ?? "";
        if (detail.payment?.status === "SUCCEEDED") {
          try { const tickets = await listOrderTickets(detail.id, savedOrder.accessToken, signal); saveIssuedTickets(tickets); completed = tickets.length > 0; }
          catch (value) { paymentError = value instanceof ApiError ? value.message : "Daftar tiket belum dapat dimuat."; }
        }
        creatingReservation = false;
        return startExpiryTimer();
      } catch (value) {
        if (signal.aborted) return;
        reservationError = value instanceof ApiError ? value.message : "Pesanan sebelumnya belum dapat dimuat.";
        creatingReservation = false;
        return;
      }
    }
    if (completedTicketId && loadTicketSnapshot(completedTicketId)) { location.replace(`/tiket/${completedTicketId}`); return; }
    const sameBasket = stored?.eventId === concert.id && stored.basketKey === basketKey;
    const idempotencyKey = sameBasket && stored ? stored.idempotencyKey : crypto.randomUUID().replaceAll("-", "");
    const payload = { eventId: concert.id, items: lines.map((line) => ({ tierId: line.tier.id, quantity: line.quantity })) };
    if (!sameBasket || !stored) {
      stored = { idempotencyKey, eventId: concert.id, basketKey };
      try { sessionStorage.setItem(reservationKey, serializeStoredReservation(stored)); } catch { storageFailed = true; }
    }
    try {
      reservation = sameBasket && stored?.reservationId ? await getReservation(stored.reservationId, signal) : await createReservation(payload.eventId, payload.items, idempotencyKey, signal);
      if (reservation.event.id !== concert.id || reservation.items.length !== lines.length || reservation.items.some((item) => item.quantity !== quantityMap[item.tierId] || !quantityMap[item.tierId])) throw new ApiError("Data reservasi tidak cocok dengan keranjang.", 409, "RESERVATION_MISMATCH");
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
    attendeeNames = Object.fromEntries(lines.map((line) => [line.tier.id, Array.from({ length: line.quantity }, () => "")]));
    return startExpiryTimer();
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
  {:else if loadError}
    {#if loadError === "not-found"}<NotFoundPanel className="checkout-missing" title="Konser tidak ditemukan." message="URL checkout ini tidak tepat." />{:else}<section class="shell empty-state" role="alert"><p>{loadError}</p><button class="text-button" type="button" onclick={() => location.reload()}>Coba lagi</button></section>{/if}
  {:else if !concert}
   <NotFoundPanel className="checkout-missing" title="Konser tidak ditemukan." message="URL checkout ini tidak tepat." />
{:else if !lines.length}
  <NotFoundPanel className="checkout-missing" kicker="Keranjang kosong" title="Pilih tiket terlebih dahulu." message="Belum ada tiket valid yang dapat dilanjutkan ke checkout ini." href={`/konser/${concert.id}`} link="Pilih tiket" />
{:else if reservationError}
  <section class="shell empty-state" role="alert"><p>{reservationError}</p><button class="text-button" type="button" onclick={() => location.reload()}>Coba lagi</button></section>
{:else if creatingReservation || !reservation}
  <p class="shell" role="status" aria-live="polite">Menahan tiket sementara...</p>
{:else}
   <section class="checkout shell"><a class="back-link" href={`/konser/${concert.id}?${location.search.slice(1)}`}>Kembali ke detail konser</a><div class="checkout-heading"><p class="checkout-kicker">Checkout aman</p><h1>Selesaikan pesananmu.</h1><p>{concert.artist} · {concert.date} · {concert.venue}</p></div>{#if !completed}<div class:reservation-warning={remainingSeconds <= 60} class="reservation-banner" role="timer"><span class="reservation-icon" aria-hidden="true">◷</span><span><b>Reservasi tiket sementara</b><small>Selesaikan pembayaran dalam {String(Math.floor(remainingSeconds / 60)).padStart(2, "0")}:{String(remainingSeconds % 60).padStart(2, "0")}</small></span></div>{/if}<ol class="checkout-steps" aria-label="Tahap checkout">{#each ["Data Diri", "Metode Bayar", "Konfirmasi"] as label, index}<li class:is-complete={index + 1 < step || completed} aria-current={index + 1 === step && !completed ? "step" : undefined}><span>{index + 1}</span><b>{label}</b></li>{/each}</ol>
    <div class="checkout-layout"><section class="checkout-panel">
       {#if completed}<section class="checkout-success"><p class="checkout-kicker">Pesanan berhasil</p><h2 id="success-title" tabindex="-1">E-ticket sudah dibuat.</h2><p>Pembayaran dikonfirmasi untuk {buyer.email}.</p><div class="booking-code"><span>Reference pesanan</span><strong>{reference}</strong></div>{#if storageFailed}<p class="success-note">Akses tiket hanya tersimpan selama halaman checkout ini terbuka.</p>{#each issuedTickets as ticket}<article class="memory-ticket"><b>{ticket.attendeeName} · {ticket.tierName}</b><span>{ticket.eventArtist} · {ticket.eventVenue}</span><span>{ticket.code}</span></article>{/each}{:else}{#each ticketIds as id}<a class="button" href={`/tiket/${encodeURIComponent(id)}`}>Lihat e-ticket</a>{/each}{/if}<a class="button button-secondary" href="/konser">Cari konser lain</a></section>
      {:else if step === 1}<form id="buyer-form" novalidate onsubmit={submitBuyer}><div class="step-title"><p>Langkah 1 dari 3</p><h2 id="buyer-title" tabindex="-1">Data pemesan</h2><span>Informasi ini digunakan untuk mengirim e-ticket.</span></div><div class="field-grid">{#each [{ name: "name", label: "Nama lengkap", type: "text" }, { name: "email", label: "Email", type: "email" }, { name: "phone", label: "No. HP", type: "tel" }, { name: "identity", label: "No. identitas", type: "text" }] as field}<label for={`buyer-${field.name}`}>{field.label}<input id={`buyer-${field.name}`} name={field.name} type={field.type} autocomplete={field.name === "name" ? "name" : field.name === "email" ? "email" : field.name === "phone" ? "tel" : "off"} inputmode={field.name === "phone" ? "tel" : field.name === "identity" ? "numeric" : undefined} maxlength={field.name === "name" ? 80 : undefined} disabled={Boolean(activeOrder)} bind:value={buyer[field.name as keyof Buyer]} aria-describedby={`${field.name}-error`} aria-invalid={Boolean(errors[field.name as keyof Buyer])} class:is-invalid={Boolean(errors[field.name as keyof Buyer])} onblur={() => validate(field.name as keyof Buyer)} required /><small id={`${field.name}-error`} aria-live="polite">{errors[field.name as keyof Buyer]}</small></label>{/each}</div><section class="attendee-fields" aria-label="Nama pemegang tiket"><h3>Nama pemegang tiket</h3>{#each lines as line}<fieldset><legend>{line.quantity} tiket {line.tier.name}</legend>{#each attendeeNames[line.tier.id] ?? [] as _name, index}<label for={`attendee-${line.tier.id}-${index}`}>Peserta {index + 1}<input id={`attendee-${line.tier.id}-${index}`} value={attendeeNames[line.tier.id]?.[index] ?? ""} oninput={(event) => setAttendeeName(line.tier.id, index, event.currentTarget.value)} maxlength="80" autocomplete="off" disabled={Boolean(activeOrder)} aria-invalid={Boolean(attendeeErrors[line.tier.id]?.[index])} required /><small class="form-error">{attendeeErrors[line.tier.id]?.[index] ?? ""}</small></label>{/each}</fieldset>{/each}</section><div class="step-actions"><span></span><button class="button" type="submit">Lanjut ke pembayaran</button></div></form>
       {:else if step === 2}<form id="payment-form" onsubmit={submitPayment}><div class="step-title"><p>Langkah 2 dari 3</p><h2 id="payment-title" tabindex="-1">Pilih metode bayar.</h2><span>Pembayaran di halaman ini hanya simulasi.</span></div><fieldset aria-describedby="payment-error" class:has-error={Boolean(paymentError)}><legend class="sr-only">Metode pembayaran</legend><div class="payment-options">{#each [{ id: "QRIS", label: "QRIS" }, { id: "VIRTUAL_ACCOUNT", label: "Virtual Account" }, { id: "GOPAY", label: "GoPay" }] as method}<label class="payment-card"><input bind:group={payment} type="radio" name="payment" value={method.id} aria-describedby="payment-error" disabled={Boolean(activeOrder)} /><span class="payment-logo" class:gopay={method.id === "GOPAY"}>{method.label}</span><span><b>{method.label}</b><small>Simulasi pembayaran</small></span></label>{/each}</div></fieldset><p id="payment-error" class="form-error" aria-live="polite">{paymentError}</p><div class="step-actions"><button class="text-button" type="button" disabled={Boolean(activeOrder)} onclick={() => goToStep(1)}>Kembali</button><button class="button" type="submit">Tinjau pesanan</button></div></form>
       {:else}<section id="confirmation"><div class="step-title"><p>Langkah 3 dari 3</p><h2 id="confirmation-title" tabindex="-1">Periksa sebelum memesan.</h2><span>Pastikan data dan pesananmu sudah benar.</span></div><div class="confirmation-block"><div><span>Pemesan</span><b>{buyer.name}</b><p>{buyer.email} · {buyer.phone}</p></div>{#if !activeOrder}<button class="text-button" type="button" onclick={() => goToStep(1)}>Ubah data</button>{/if}</div><div class="confirmation-block"><div><span>Pembayaran</span><b>{paymentLabel}</b></div>{#if !activeOrder}<button class="text-button" type="button" onclick={() => goToStep(2)}>Ubah metode</button>{/if}</div><p class="terms-trigger">Dengan melanjutkan, kamu menyetujui <button class="text-button" type="button" onclick={openTerms}>S&K Konser</button>.</p><p class="form-error" aria-live="polite">{paymentError}</p>{#if paymentStatus === "FAILED"}<p role="status">Pembayaran gagal. Order tetap tersimpan dan kamu dapat mencoba lagi.</p>{/if}<div class="step-actions"><button class="text-button" type="button" onclick={() => goToStep(2)}>Kembali</button>{#if !completed}<button class="button" type="button" disabled={reservationExpired || converting} onclick={() => completeOrder("SUCCEEDED")}>{converting ? "Memproses..." : activeOrder ? paymentStatus === "FAILED" ? "Coba bayar lagi" : "Muat e-ticket lagi" : "Buat pesanan dan bayar"}</button>{/if}</div>{#if !activeOrder}<button class="text-button" type="button" disabled={reservationExpired || converting} onclick={() => completeOrder("FAILED")}>Simulasikan pembayaran gagal</button>{/if}</section>{/if}
       </section><aside class="order-summary"><h2>Ringkasan pesanan</h2><p class="summary-event">{concert.artist}</p><p>{eventDate(concert.startsAt)}</p>{#each reservation.items as line}<div class="summary-line"><span>{line.quantity}x {line.name}</span><b>{formatRupiah.format(line.lineTotal)}</b></div>{/each}<div class="summary-line"><span>Biaya admin</span><b>{formatRupiah.format(activeOrder?.adminFee ?? ADMIN_FEE)}</b></div>{#if discount}<div class="summary-line discount-row"><span>Diskon</span><b>-{formatRupiah.format(discount)}</b></div>{/if}<form class="voucher-form" onsubmit={applyVoucher}><label for="voucher">Kode promo<input bind:value={voucherInput} id="voucher" autocomplete="off" disabled={Boolean(activeOrder) || reservationExpired} /></label><button class="button button-small" type="submit" disabled={Boolean(activeOrder) || reservationExpired}>Gunakan</button></form>{#if voucher}<p class:is-valid={Boolean(discount)} class="voucher-message" aria-live="polite">{activeOrder ? "Voucher tercatat pada pesanan." : discount ? "Estimasi promo HEMAT10 diterapkan." : "Kode voucher akan divalidasi server."}</p>{/if}<div class="grand-total"><span>{activeOrder ? "Total" : "Estimasi total"}</span><strong>{formatRupiah.format(total)}</strong></div>{#if !activeOrder}<small>Nominal akhir ditetapkan server setelah pesanan dibuat.</small>{/if}<button class="terms-button" type="button" onclick={openTerms}>Lihat S&amp;K Konser</button></aside></div>
   </section>
   <dialog class="terms-dialog" bind:this={termsDialog} aria-labelledby="terms-title"><div class="dialog-heading"><p class="checkout-kicker">Informasi penting</p><h2 id="terms-title">Syarat &amp; ketentuan konser</h2></div><div class="terms-dialog-list">{#each concertTerms as term}<section><h3>{term.title}</h3><p>{term.body}</p></section>{/each}</div><form method="dialog" class="dialog-actions"><button class="button" type="submit">Tutup</button></form></dialog>
    <dialog class="expiry-dialog" bind:this={expiryDialog} aria-labelledby="expiry-title" oncancel={(event) => { event.preventDefault(); recoverExpiredCheckout(); }}><p class="checkout-kicker">Reservasi berakhir</p><h2 id="expiry-title">Waktu checkout habis.</h2><p>Waktu reservasi simulasi sudah selesai. Pilih tiket lagi untuk memulai sesi checkout baru.</p><button class="button" type="button" onclick={recoverExpiredCheckout}>Pilih tiket lagi</button></dialog>
    {#if toast}<Toast id={toast.id} message={toast.message} tone={toast.tone} onDismiss={() => toast = null} />{/if}
{/if}
