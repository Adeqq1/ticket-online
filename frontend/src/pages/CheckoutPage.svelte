<script lang="ts">
  import { onMount, tick } from "svelte";
  import { createOrder, createReservation, createSnapPayment, getEvent, getOrder, getReservation, listOrderTickets, simulatePayment, ApiError, type ApiTicket, type CreateOrderRequest, type OrderBase, type OrderDetail, type OrderResponse, type PaymentMethod, type Reservation } from "../lib/api.ts";
  import { eventDate, formatRupiah, type Concert } from "../lib/concerts.ts";
  import NotFoundPanel from "../components/NotFoundPanel.svelte";
  import { ADMIN_FEE, checkoutLines, isValidEmail, isValidIdentity, isValidPhone, voucherDiscount, type Buyer } from "../lib/checkout.ts";
  import { listOrderAccess, saveOrderAccess, type OrderAccess } from "../lib/order-access.ts";
  import { findOrderForActiveReservation } from "../lib/checkout-recovery.ts";
  import Toast from "../components/Toast.svelte";
  import { concertTerms } from "../lib/terms.ts";
  import { RESERVATION_DURATION_MS, isReservationExpired, remainingReservationSeconds, reservationBasketKey, parseCheckoutAttempt, parseStoredReservation, serializeStoredReservation, type StoredReservation } from "../lib/reservation.ts";
  let { id }: { id: string } = $props();
  let concert = $state<Concert | undefined>();
  let loading = $state(true);
  let loadError = $state("");
  const lines = $derived(concert ? checkoutLines(concert, new URLSearchParams(location.search)) : []);
  let reservation = $state<Reservation | undefined>();
  let reservationError = $state("");
  let creatingReservation = $state(false);
  let activeOrder = $state<OrderBase | undefined>(); let orderAccess = $state<OrderAccess | null>(null); let paymentStatus = $state(""); let checkoutUncertain = $state(false); let paymentUncertain = $state(false); let pendingCheckoutPayload = $state<CreateOrderRequest | null>(null); let restartCheckoutRequired = $state(false);
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
  const checkoutAttemptKey = $derived(`${reservationKey}:checkout-attempt`);
  const simulationEnabled = import.meta.env.VITE_ENABLE_PAYMENT_SIMULATION === "true";
  async function goToStep(next: number) { step = next; await tick(); const name = next === 1 ? "buyer" : next === 2 ? "payment" : "confirmation"; const heading = document.querySelector<HTMLElement>(`#${name}-title`); heading?.focus(); heading?.scrollIntoView({ behavior: matchMedia("(prefers-reduced-motion: reduce)").matches ? "auto" : "smooth", block: "start" }); }
  function validate(name: keyof Buyer) {
    const value = buyer[name].trim(); buyer[name] = value;
    const valid = name === "name" ? value.length >= 2 && value.length <= 80 : name === "email" ? isValidEmail(value) : name === "phone" ? isValidPhone(value) : isValidIdentity(value);
    errors[name] = valid ? "" : name === "name" ? "Masukkan nama lengkap maksimal 80 karakter." : name === "email" ? "Masukkan alamat email yang valid." : name === "phone" ? "Masukkan nomor HP yang valid." : "Nomor identitas harus terdiri dari 12-20 angka.";
    return valid;
  }
  function setAttendeeName(tierId: string, index: number, value: string) { if (checkoutUncertain || activeOrder) return; const names = [...(attendeeNames[tierId] ?? [])]; names[index] = value; attendeeNames[tierId] = names; }
  async function submitBuyer(event: SubmitEvent) { event.preventDefault(); if (checkoutUncertain || activeOrder || checkExpiry()) return; const valid = (["name", "email", "phone", "identity"] as const).map(validate).every(Boolean); const attendeesValid = lines.every((line) => { const names = attendeeNames[line.tier.id] ?? []; const messages = names.map((name) => name.trim().length >= 2 && name.trim().length <= 80 ? "" : "Nama peserta harus 2–80 karakter."); attendeeErrors[line.tier.id] = messages; return names.length === line.quantity && messages.every((error) => !error); }); if (valid && attendeesValid) await goToStep(2); else await tick().then(() => document.querySelector<HTMLInputElement>('[aria-invalid="true"]')?.focus()); }
  async function submitPayment(event: SubmitEvent) { event.preventDefault(); if (checkExpiry()) return; paymentError = payment ? "" : "Pilih metode pembayaran terlebih dahulu."; if (!payment) { await tick(); document.querySelector<HTMLInputElement>('input[name="payment"]')?.focus(); return; } await goToStep(3); }
  function showToast(message: string, tone: "success" | "info" | "error" = "info") { toast = { id: ++toastId, message, tone }; }
  function openTerms() { termsDialog?.showModal(); }
  function closeDialog(dialog: HTMLDialogElement) { dialog.close(); }
  function restartSelection() { try { sessionStorage.removeItem(reservationKey); sessionStorage.removeItem(completionKey); sessionStorage.removeItem(checkoutAttemptKey); } catch { /* storage can be unavailable */ } }
  function startOver() { restartSelection(); if (concert) location.assign(`/konser/${concert.id}?${location.search.slice(1)}`); }
  function buyAgain() { restartSelection(); if (concert) location.assign(`/konser/${concert.id}`); }
  function recoverExpiredCheckout() { restartSelection(); if (concert) location.assign(`/konser/${concert.id}?${location.search.slice(1)}`); }
  function checkExpiry() {
    if (completed || checkoutUncertain || paymentUncertain || activeOrder?.status === "PAID" || expiresAt === null || !isReservationExpired(expiresAt)) return false;
    reservationExpired = true;
    try { sessionStorage.removeItem(reservationKey); } catch { /* storage can be unavailable */ }
    if (!expiryDialog?.open) expiryDialog?.showModal();
    return true;
  }
  function applyVoucher(event: SubmitEvent) { event.preventDefault(); if (activeOrder || checkoutUncertain || reservationExpired || checkExpiry()) return; voucher = voucherInput.trim().toUpperCase(); showToast(voucher === "HEMAT10" ? "Estimasi diskon HEMAT10 diterapkan." : "Kode voucher akan divalidasi saat pesanan dibuat.", voucher === "HEMAT10" ? "success" : "info"); }
  let expiryTimer: number | undefined;
  let expiryVisibility: (() => void) | undefined;
  let updateExpiryTimer: () => void = () => {};
  function startExpiryTimer() {
    const update = () => { remainingSeconds = remainingReservationSeconds(expiresAt ?? 0); if (remainingSeconds === 0) checkExpiry(); };
    updateExpiryTimer = update;
    update();
    if (expiryTimer !== undefined) return () => {};
    expiryTimer = window.setInterval(update, 1_000);
    expiryVisibility = update;
    document.addEventListener("visibilitychange", update);
    return () => { if (expiryTimer !== undefined) window.clearInterval(expiryTimer); if (expiryVisibility) document.removeEventListener("visibilitychange", expiryVisibility); expiryTimer = undefined; expiryVisibility = undefined; };
  }
  function saveIssuedTickets(tickets: ApiTicket[]) {
    if (!concert || !orderAccess) return;
    issuedTickets = tickets;
    ticketIds = tickets.map((ticket) => ticket.id);
    orderAccess = { ...orderAccess, ticketIds };
    if (!saveOrderAccess(orderAccess)) storageFailed = true;
  }
  function clearCheckoutAttempt() {
    pendingCheckoutPayload = null;
    checkoutUncertain = false;
    try { sessionStorage.removeItem(checkoutAttemptKey); } catch { storageFailed = true; }
  }
  function persistOrderAccessRecord(order: OrderResponse, idempotencyKey: string) {
    orderAccess = { orderId: order.id, accessToken: order.accessToken, expiresAt: order.expiresAt, accessExpiresAt: order.accessExpiresAt, reservationId: order.reservationId, idempotencyKey, basketKey: reservationKey, reference: order.reference, ticketIds: [] };
    const saved = saveOrderAccess(orderAccess);
    if (saved) clearCheckoutAttempt();
    else { storageFailed = true; showToast("Akses pesanan hanya tersimpan selama halaman ini terbuka.", "error"); }
    return saved;
  }
  async function restoreOrder(detail: OrderDetail, access: OrderAccess, signal?: AbortSignal) {
    if (!concert) return;
    checkoutUncertain = false;
    pendingCheckoutPayload = null;
    orderAccess = access; activeOrder = detail; reference = detail.reference; buyer = detail.buyer;
    attendeeNames = Object.fromEntries(lines.map((line) => [line.tier.id, detail.attendees.filter((attendee) => attendee.tierId === line.tier.id).map((attendee) => attendee.name)]));
    reservation = { id: detail.reservationId, status: detail.status, expiresAt: detail.expiresAt, event: { id: concert.id, artist: concert.artist }, items: detail.items, subtotal: detail.subtotal };
    storedReservation = { reservationId: detail.reservationId, idempotencyKey: access.idempotencyKey, eventId: concert.id, basketKey: reservationKey.slice(reservationKey.lastIndexOf(":") + 1) };
    expiresAt = Date.parse(detail.expiresAt);
    updateExpiryTimer();
    step = 3; payment = detail.payment?.method ?? "QRIS"; paymentStatus = detail.payment?.status ?? "";
    reservationExpired = detail.status === "EXPIRED" || detail.status === "CANCELLED";
    if (detail.status === "PAID") {
      try { const tickets = await listOrderTickets(detail.id, access.accessToken, signal); saveIssuedTickets(tickets); completed = tickets.length === detail.items.reduce((count, item) => count + item.quantity, 0) && tickets.length > 0; if (!completed) paymentError = "Sebagian e-ticket belum dapat dimuat. Coba muat ulang."; }
      catch (value) { if (!signal?.aborted) paymentError = value instanceof ApiError ? value.message : "Daftar tiket belum dapat dimuat."; }
    }
  }
  async function refreshOrderStatus() {
    if (!activeOrder || !orderAccess || converting) return;
    converting = true;
    try { await restoreOrder(await getOrder(activeOrder.id, orderAccess.accessToken), orderAccess); paymentUncertain = false; }
    catch (value) { paymentError = value instanceof ApiError ? value.message : "Status order belum dapat dimuat. Coba lagi."; }
    finally { converting = false; }
  }
  async function completeOrder(result: "SUCCEEDED" | "FAILED" = "SUCCEEDED") {
    if (!concert || !reservation || converting || (reservationExpired && !activeOrder && !checkoutUncertain) || (!checkoutUncertain && !paymentUncertain && checkExpiry())) return;
    converting = true;
    try {
      if (!activeOrder) {
        if (!storedReservation) throw new Error("Data idempotensi reservasi tidak tersedia");
        const payload = pendingCheckoutPayload ?? { buyer: { ...buyer }, attendees: lines.map((line) => ({ tierId: line.tier.id, names: (attendeeNames[line.tier.id] ?? []).map((name) => name.trim()) })), ...(voucher ? { voucherCode: voucher } : {}) };
        pendingCheckoutPayload = payload;
        checkoutUncertain = true;
        try { sessionStorage.setItem(checkoutAttemptKey, JSON.stringify(payload)); }
        catch { storageFailed = true; showToast("Data retry hanya tersimpan selama halaman ini terbuka.", "error"); }
        const created = await createOrder(reservation.id, payload, storedReservation.idempotencyKey);
        activeOrder = created;
        persistOrderAccessRecord(created, storedReservation.idempotencyKey);
        reference = created.reference;
        expiresAt = Date.parse(created.expiresAt);
        updateExpiryTimer();
      }
      if (!orderAccess || !activeOrder) throw new Error("Akses order tidak tersedia di memori browser");
      if (activeOrder.status === "PAID") {
        const tickets = await listOrderTickets(activeOrder.id, orderAccess.accessToken);
        saveIssuedTickets(tickets);
        if (tickets.length !== activeOrder.items.reduce((count, item) => count + item.quantity, 0)) throw new Error("Sebagian e-ticket belum tersedia. Coba muat ulang.");
        completed = true;
        paymentStatus = "SUCCEEDED";
        return;
      }
      if (paymentUncertain) {
        const detail = await getOrder(activeOrder.id, orderAccess.accessToken);
        await restoreOrder(detail, orderAccess);
        paymentUncertain = false;
        if (detail.status === "PAID") {
          if (!completed) paymentError = "Pembayaran terkonfirmasi, tetapi tiket belum dapat dimuat. Coba muat ulang.";
          return;
        }
        if (detail.status === "EXPIRED" || detail.status === "CANCELLED") {
          reservationExpired = true;
          paymentError = "Order sudah kedaluwarsa atau dibatalkan.";
          return;
        }
      }
      if (simulationEnabled) {
        paymentUncertain = true;
        const resultPayment = await simulatePayment(activeOrder.id, { method: payment || "QRIS", result }, orderAccess.accessToken);
        paymentUncertain = false;
        paymentStatus = resultPayment.status;
        if (resultPayment.orderStatus === "PAID") activeOrder = { ...activeOrder, status: "PAID" };
        if (resultPayment.status === "FAILED") paymentError = "Pembayaran simulasi gagal. Coba bayar kembali untuk menyelesaikan pesanan.";
        else if (resultPayment.orderStatus === "PAID") {
          const tickets = resultPayment.tickets.length ? resultPayment.tickets : await listOrderTickets(activeOrder.id, orderAccess.accessToken);
          saveIssuedTickets(tickets);
          if (tickets.length !== activeOrder.items.reduce((count, item) => count + item.quantity, 0)) throw new Error("Sebagian e-ticket belum dapat dimuat. Coba muat ulang halaman.");
          completed = true;
          paymentError = "";
          await tick(); document.querySelector<HTMLElement>("#success-title")?.focus();
        } else paymentError = "Server belum mengonfirmasi pembayaran. Periksa status order sebelum mencoba lagi.";
      } else {
        paymentUncertain = true;
        const { redirectUrl } = await createSnapPayment(activeOrder.id, payment || "QRIS", window.location.href, orderAccess.accessToken);
        window.location.assign(redirectUrl);
      }
    } catch (value) {
      if (!activeOrder && value instanceof ApiError && value.status === 422) clearCheckoutAttempt();
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
    const basketKey = reservationKey.slice(reservationKey.lastIndexOf(":") + 1);
    const savedOrder = findOrderForActiveReservation(listOrderAccess(), stored, reservationKey);
    if (savedOrder) {
      try {
        const detail = await getOrder(savedOrder.orderId, savedOrder.accessToken, signal);
        if (signal.aborted) return;
        if (["PAID", "EXPIRED", "CANCELLED"].includes(detail.status)) {
          try { sessionStorage.removeItem(reservationKey); sessionStorage.removeItem(checkoutAttemptKey); } catch { storageFailed = true; }
          stored = null;
        } else {
          await restoreOrder(detail, savedOrder, signal);
          creatingReservation = false;
          return startExpiryTimer();
        }
      } catch (value) {
        if (signal.aborted) return;
        reservationError = value instanceof ApiError ? value.message : "Pesanan sebelumnya belum dapat dimuat.";
        creatingReservation = false;
        return;
      }
    }
    let rawCheckoutAttempt: string | null = null;
    try { rawCheckoutAttempt = sessionStorage.getItem(checkoutAttemptKey); } catch { storageFailed = true; }
    if (rawCheckoutAttempt !== null) {
      pendingCheckoutPayload = parseCheckoutAttempt(rawCheckoutAttempt, quantityMap);
      if (!pendingCheckoutPayload) {
        reservationError = "Data checkout tersimpan tidak cocok dengan keranjang ini. Mulai checkout baru untuk melanjutkan.";
        restartCheckoutRequired = true;
        creatingReservation = false;
        return;
      }
      checkoutUncertain = true;
    }
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
    if (pendingCheckoutPayload) {
      buyer = pendingCheckoutPayload.buyer;
      voucher = pendingCheckoutPayload.voucherCode ?? "";
      voucherInput = voucher;
      attendeeNames = Object.fromEntries(pendingCheckoutPayload.attendees.map((attendee) => [attendee.tierId, attendee.names]));
      try {
        const created = await createOrder(reservation.id, pendingCheckoutPayload, idempotencyKey, signal);
        if (signal.aborted) return;
        activeOrder = created;
        persistOrderAccessRecord(created, idempotencyKey);
        const detail = await getOrder(created.id, created.accessToken, signal);
        if (signal.aborted) return;
        await restoreOrder(detail, orderAccess!, signal);
        creatingReservation = false;
        return startExpiryTimer();
      } catch (value) {
        if (signal.aborted) return;
        if (value instanceof ApiError && value.status === 422) {
          clearCheckoutAttempt();
          paymentError = value.message;
          step = 1;
          creatingReservation = false;
          return startExpiryTimer();
        }
        paymentError = value instanceof ApiError ? value.message : "Checkout sebelumnya belum dapat dipastikan. Coba lagi.";
        step = 3;
        creatingReservation = false;
        return startExpiryTimer();
      }
    }
    checkoutUncertain = false;
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
{:else if reservationError && restartCheckoutRequired}
  <section class="shell empty-state" role="alert"><p>{reservationError}</p><button class="button" type="button" onclick={startOver}>Pilih tiket lagi</button></section>
{:else if reservationError}
  <section class="shell empty-state" role="alert"><p>{reservationError}</p><button class="text-button" type="button" onclick={() => location.reload()}>Coba lagi</button></section>
{:else if creatingReservation || !reservation}
  <p class="shell" role="status" aria-live="polite">Menahan tiket sementara...</p>
{:else}
   <section class="checkout shell"><a class="back-link" href={`/konser/${concert.id}?${location.search.slice(1)}`}>Kembali ke detail konser</a><div class="checkout-heading"><p class="checkout-kicker">Checkout aman</p><h1>Selesaikan pesananmu.</h1><p>{concert.artist} · {concert.date} · {concert.venue}</p></div>{#if !completed}<div class:reservation-warning={remainingSeconds <= 60} class="reservation-banner" role="timer"><span class="reservation-icon" aria-hidden="true">◷</span><span><b>{activeOrder ? "Pesanan menunggu pembayaran" : "Reservasi tiket sementara"}</b><small>Selesaikan pembayaran dalam {String(Math.floor(remainingSeconds / 60)).padStart(2, "0")}:{String(remainingSeconds % 60).padStart(2, "0")}</small></span></div>{/if}<ol class="checkout-steps" aria-label="Tahap checkout">{#each ["Data Diri", "Metode Bayar", "Konfirmasi"] as label, index}<li class:is-complete={index + 1 < step || completed} aria-current={index + 1 === step && !completed ? "step" : undefined}><span>{index + 1}</span><b>{label}</b></li>{/each}</ol>
    <div class="checkout-layout"><section class="checkout-panel">
       {#if completed}
         <section class="checkout-success">
           <p class="checkout-kicker">Pesanan berhasil</p>
           <h2 id="success-title" tabindex="-1">E-ticket sudah dibuat.</h2>
           <p>Pembayaran dikonfirmasi untuk {buyer.email}.</p>
           <div class="booking-code"><span>Reference pesanan</span><strong>{reference}</strong></div>
           {#if storageFailed}
             <p class="success-note">Akses tiket hanya tersedia selama halaman checkout ini terbuka. Simpan atau cetak kode setiap peserta sebelum meninggalkan halaman.</p>
             {#each issuedTickets as ticket}
               <article class="memory-ticket">
                 <b>{ticket.attendeeName} · {ticket.tierName} · {ticket.gate}</b>
                 <span>{ticket.eventArtist} · {ticket.eventVenue}</span>
                 <strong>{ticket.code}</strong>
               </article>
             {/each}
           {:else}
             {#each ticketIds as ticketId}
               <a class="button" href={`/tiket/${encodeURIComponent(ticketId)}`}>Lihat e-ticket</a>
             {/each}
           {/if}
           <button class="button button-secondary" type="button" onclick={buyAgain}>Beli tiket lagi</button>
           <a class="button button-secondary" href="/konser">Cari konser lain</a>
         </section>
      {:else if step === 1}<form id="buyer-form" novalidate onsubmit={submitBuyer}><div class="step-title"><p>Langkah 1 dari 3</p><h2 id="buyer-title" tabindex="-1">Data pemesan</h2><span>Data order disimpan pada browser yang sama; e-ticket belum dikirim lewat email.</span></div><div class="field-grid">{#each [{ name: "name", label: "Nama lengkap", type: "text" }, { name: "email", label: "Email", type: "email" }, { name: "phone", label: "No. HP", type: "tel" }, { name: "identity", label: "No. identitas", type: "text" }] as field}<label for={`buyer-${field.name}`}>{field.label}<input id={`buyer-${field.name}`} name={field.name} type={field.type} autocomplete={field.name === "name" ? "name" : field.name === "email" ? "email" : field.name === "phone" ? "tel" : "off"} inputmode={field.name === "phone" ? "tel" : field.name === "identity" ? "numeric" : undefined} maxlength={field.name === "name" ? 80 : undefined} disabled={Boolean(activeOrder) || checkoutUncertain} bind:value={buyer[field.name as keyof Buyer]} aria-describedby={`${field.name}-error`} aria-invalid={Boolean(errors[field.name as keyof Buyer])} class:is-invalid={Boolean(errors[field.name as keyof Buyer])} onblur={() => validate(field.name as keyof Buyer)} required /><small id={`${field.name}-error`} aria-live="polite">{errors[field.name as keyof Buyer]}</small></label>{/each}</div><section class="attendee-fields" aria-label="Nama pemegang tiket"><h3>Nama pemegang tiket</h3>{#each lines as line}<fieldset><legend>{line.quantity} tiket {line.tier.name}</legend>{#each attendeeNames[line.tier.id] ?? [] as _name, index}<label for={`attendee-${line.tier.id}-${index}`}>Peserta {index + 1}<input id={`attendee-${line.tier.id}-${index}`} value={attendeeNames[line.tier.id]?.[index] ?? ""} oninput={(event) => setAttendeeName(line.tier.id, index, event.currentTarget.value)} maxlength="80" autocomplete="off" disabled={Boolean(activeOrder) || checkoutUncertain} aria-invalid={Boolean(attendeeErrors[line.tier.id]?.[index])} required /><small class="form-error">{attendeeErrors[line.tier.id]?.[index] ?? ""}</small></label>{/each}</fieldset>{/each}</section><div class="step-actions"><span></span><button class="button" type="submit">Lanjut ke pembayaran</button></div></form>
      {:else if step === 2}<form id="payment-form" onsubmit={submitPayment}><div class="step-title"><p>Langkah 2 dari 3</p><h2 id="payment-title" tabindex="-1">Pilih metode bayar.</h2><span>{simulationEnabled ? "Mode pengembangan: pembayaran disimulasikan." : "Pembayaran diproses aman melalui Midtrans sandbox."}</span></div><fieldset aria-describedby="payment-error" class:has-error={Boolean(paymentError)}><legend class="sr-only">Metode pembayaran</legend><div class="payment-options">{#each [{ id: "QRIS", label: "QRIS" }, { id: "VIRTUAL_ACCOUNT", label: "Virtual Account" }, { id: "GOPAY", label: "GoPay" }] as method}<label class="payment-card"><input bind:group={payment} type="radio" name="payment" value={method.id} aria-describedby="payment-error" disabled={Boolean(activeOrder) || checkoutUncertain} /><span class="payment-logo" class:gopay={method.id === "GOPAY"}>{method.label}</span><span><b>{method.label}</b><small>{simulationEnabled ? "Simulasi" : "Pembayaran sandbox"}</small></span></label>{/each}</div></fieldset><p id="payment-error" class="form-error" aria-live="polite">{paymentError}</p><div class="step-actions"><button class="text-button" type="button" disabled={Boolean(activeOrder) || checkoutUncertain} onclick={() => goToStep(1)}>Kembali</button><button class="button" type="submit">Tinjau pesanan</button></div></form>
       {:else}<section id="confirmation"><div class="step-title"><p>Langkah 3 dari 3</p><h2 id="confirmation-title" tabindex="-1">Periksa sebelum memesan.</h2><span>Pastikan data dan pesananmu sudah benar.</span></div><div class="confirmation-block"><div><span>Pemesan</span><b>{buyer.name}</b><p>{buyer.email} · {buyer.phone}</p></div>{#if !activeOrder && !checkoutUncertain}<button class="text-button" type="button" onclick={() => goToStep(1)}>Ubah data</button>{/if}</div><div class="confirmation-block"><div><span>Pembayaran</span><b>{paymentLabel}</b></div>{#if !activeOrder && !checkoutUncertain}<button class="text-button" type="button" onclick={() => goToStep(2)}>Ubah metode</button>{/if}</div><p class="terms-trigger">Dengan melanjutkan, kamu menyetujui <button class="text-button" type="button" onclick={openTerms}>S&K Konser</button>.</p><p class="form-error" aria-live="polite">{paymentError}</p>{#if paymentStatus === "FAILED"}<p role="status">Pembayaran gagal. Order tetap tersimpan dan kamu dapat mencoba lagi.</p>{/if}{#if activeOrder && activeOrder.status === "PENDING"}<p role="status">{paymentStatus === "PENDING" ? "Menunggu konfirmasi pembayaran dari Midtrans." : "Pesanan siap dibayar."}</p><button class="text-button" type="button" disabled={converting} onclick={refreshOrderStatus}>Periksa status pembayaran</button>{/if}<div class="step-actions"><button class="text-button" type="button" disabled={checkoutUncertain} onclick={() => goToStep(2)}>Kembali</button>{#if !completed}<button class="button" type="button" disabled={converting || (reservationExpired && !paymentUncertain)} onclick={() => completeOrder("SUCCEEDED")}>{converting ? "Memproses..." : checkoutUncertain ? "Coba ulang checkout" : paymentUncertain ? "Periksa status pembayaran" : activeOrder ? activeOrder.status === "PAID" ? "Muat e-ticket lagi" : paymentStatus === "FAILED" ? "Coba bayar lagi" : "Lanjutkan pembayaran" : "Buat pesanan dan bayar"}</button>{/if}</div>{#if simulationEnabled && !activeOrder && !checkoutUncertain}<button class="text-button" type="button" disabled={reservationExpired || converting} onclick={() => completeOrder("FAILED")}>Simulasikan pembayaran gagal</button>{/if}</section>{/if}
       </section><aside class="order-summary"><h2>Ringkasan pesanan</h2><p class="summary-event">{concert.artist}</p><p>{eventDate(concert.startsAt)}</p>{#each reservation.items as line}<div class="summary-line"><span>{line.quantity}x {line.name}</span><b>{formatRupiah.format(line.lineTotal)}</b></div>{/each}<div class="summary-line"><span>Biaya admin</span><b>{formatRupiah.format(activeOrder?.adminFee ?? ADMIN_FEE)}</b></div>{#if discount}<div class="summary-line discount-row"><span>Diskon</span><b>-{formatRupiah.format(discount)}</b></div>{/if}<form class="voucher-form" onsubmit={applyVoucher}><label for="voucher">Kode promo<input bind:value={voucherInput} id="voucher" autocomplete="off" disabled={Boolean(activeOrder) || checkoutUncertain || reservationExpired} /></label><button class="button button-small" type="submit" disabled={Boolean(activeOrder) || checkoutUncertain || reservationExpired}>Gunakan</button></form>{#if voucher}<p class:is-valid={Boolean(discount)} class="voucher-message" aria-live="polite">{activeOrder ? "Voucher tercatat pada pesanan." : discount ? "Estimasi promo HEMAT10 diterapkan." : "Kode voucher akan divalidasi server."}</p>{/if}<div class="grand-total"><span>{activeOrder ? "Total" : "Estimasi total"}</span><strong>{formatRupiah.format(total)}</strong></div>{#if !activeOrder && !checkoutUncertain}<small>Nominal akhir ditetapkan server setelah pesanan dibuat.</small>{/if}<button class="terms-button" type="button" onclick={openTerms}>Lihat S&amp;K Konser</button></aside></div>
   </section>
   <dialog class="terms-dialog" bind:this={termsDialog} aria-labelledby="terms-title"><div class="dialog-heading"><p class="checkout-kicker">Informasi penting</p><h2 id="terms-title">Syarat &amp; ketentuan konser</h2></div><div class="terms-dialog-list">{#each concertTerms as term}<section><h3>{term.title}</h3><p>{term.body}</p></section>{/each}</div><form method="dialog" class="dialog-actions"><button class="button" type="submit">Tutup</button></form></dialog>
    <dialog class="expiry-dialog" bind:this={expiryDialog} aria-labelledby="expiry-title" oncancel={(event) => { event.preventDefault(); recoverExpiredCheckout(); }}><p class="checkout-kicker">Reservasi berakhir</p><h2 id="expiry-title">Waktu checkout habis.</h2><p>Waktu reservasi simulasi sudah selesai. Pilih tiket lagi untuk memulai sesi checkout baru.</p><button class="button" type="button" onclick={recoverExpiredCheckout}>Pilih tiket lagi</button></dialog>
    {#if toast}<Toast id={toast.id} message={toast.message} tone={toast.tone} onDismiss={() => toast = null} />{/if}
{/if}
