<script lang="ts">
 import EventNotice from "../../components/EventNotice.svelte";
  import { onMount, onDestroy, tick } from "svelte";
  import { createOrder, createReservation, createSnapPayment, getEvent, getReservationEvent, getOrder, getReservation, listOrderTickets, simulatePayment, ApiError, type ApiTicket, type CreateOrderRequest, type OrderBase, type OrderDetail, type OrderResponse, type PaymentMethod, type PaymentStatus, type Reservation } from "../../lib/api.ts";
  import { eventDate, formatRupiah, type Concert } from "../../lib/concerts.ts";
  import NotFoundPanel from "../../components/NotFoundPanel.svelte";
  import { quoteCheckout, type CheckoutQuote } from "../../lib/api.ts";
  import { checkoutLines, isValidEmail, isValidPhone, type Buyer } from "../../lib/checkout.ts";
  import { hasPersistentOrderAccess, listOrderAccess, saveOrderAccess, type CheckoutOrderAccess } from "../../lib/order-access.ts";
  import { findOrderForActiveReservation, orderMatchesReservation, restoredReservationQuantities } from "../../lib/checkout-recovery.ts";
  import Toast from "../../components/Toast.svelte";
  import { concertTerms } from "../../lib/terms.ts";
  import { buyerPaymentStatusLabel, checkoutTicketsComplete, showCheckoutPaymentCountdown } from "../../lib/buyer-payment.ts";
  import { buyerOrderStatusLabel, refundStatusLabel } from "../../lib/buyer-refund.ts";
  import { canResumeOrderPayment } from "../../lib/buyer-tickets.ts";
  import { isReservationExpired, remainingReservationSeconds, reservationBasketKey, parseCheckoutAttempt, parseStoredReservation, serializeStoredReservation, type StoredReservation } from "../../lib/reservation.ts";
  import { journeyId, recordConversion, trackConversionActivity } from "../../lib/conversion.ts";
  let { id }: { id: string } = $props();
  let isDesktop = $state(true);
  let copyDialog = $state<HTMLDialogElement>();
  let copyScope = $state<"first" | "all">("all");
  let totalChange = $state<{ previous: number | null; current: number } | null>(null);
  let totalChangeAccepted = $state(false);
  let actionBarHeight = $state(0);
  let keyboardInset = $state(0);
  let concert = $state<Concert | undefined>();
  let restoredReservation = $state<Reservation | null>(null);
  let loading = $state(true);
  let loadError = $state("");
  const lines = $derived.by(() => {
    if (!concert) return [];
    const currentConcert = concert;
    if (!restoredReservation) return checkoutLines(currentConcert, new URLSearchParams(location.search));
    return restoredReservation.items.flatMap((item) => {
      const tier = currentConcert.ticketTiers.find((candidate) => candidate.id === item.tierId);
      return tier ? [{ tier: { ...tier, name: item.name, price: item.unitPrice }, quantity: item.quantity }] : [];
    });
  });
  let reservation = $state<Reservation | undefined>();
  let reservationError = $state("");
  let creatingReservation = $state(false);
  let activeOrder = $state<OrderBase | undefined>(); let orderAccess = $state<CheckoutOrderAccess | null>(null); let paymentStatus = $state<PaymentStatus | null>(null); let checkoutUncertain = $state(false); let paymentUncertain = $state(false); let pendingCheckoutPayload = $state<CreateOrderRequest | null>(null); let restartCheckoutRequired = $state(false);
  let issuedTickets = $state<ApiTicket[]>([]); let ticketsLoading = $state(false); let paymentCheckMessage = $state(""); let orderRefundStatus = $state<string | undefined>();
  const subtotal = $derived(activeOrder?.subtotal ?? reservation?.subtotal ?? 0);
  let step = $state(1); let payment = $state<PaymentMethod | "">(""); let voucherInput = $state(""); let voucher = $state(""); let completed = $state(false); let ticketIds = $state<string[]>([]); let reference = $state(""); let storageFailed = $state(false);
  let buyer = $state<Buyer>({ name: "", email: "", phone: "" });
  let attendeeNames = $state<Record<string, string[]>>({}); let attendeeErrors = $state<Record<string, string[]>>({});
  let errors = $state<Record<"name" | "email" | "phone", string>>({ name: "", email: "", phone: "" });
  let paymentError = $state(""); let paymentCheckError = $state(""); let ticketError = $state(""); let statusResult = $state<HTMLParagraphElement>(); let termsDialog = $state<HTMLDialogElement>(); let expiryDialog = $state<HTMLDialogElement>(); let toast = $state<{ id: number; message: string; tone: "success" | "info" | "error" } | null>(null); let toastId = 0; let expiresAt = $state<number | null>(null); let reservationExpired = $state(false); let remainingSeconds = $state(0); let storedReservation = $state<StoredReservation | null>(null); let converting = $state(false);
  let checkedDeadline: number | null = null;
  const expiryWarning = $derived(!showCheckoutPaymentCountdown(completed, activeOrder?.status, paymentStatus) ? "" : remainingSeconds === 0 ? activeOrder || checkoutUncertain || paymentUncertain ? "Batas waktu lewat. Periksa status pembayaran sebelum melanjutkan." : "Waktu reservasi habis. Pilih tiket kembali untuk memeriksa ketersediaannya." : remainingSeconds <= 60 ? "Waktu tersisa kurang dari satu menit. Segera selesaikan checkout dan pembayaran." : "");
  let quote = $state<CheckoutQuote | null>(null);
  let quoteLoading = $state(false);
  let quoteError = $state("");
  let quoteRequest: AbortController | undefined;
  const discount = $derived(activeOrder?.discount ?? quote?.discount);
  const total = $derived(activeOrder?.total ?? quote?.total);
  async function refreshQuote(code = voucher, signal?: AbortSignal) {
    if (!reservation || !storedReservation || activeOrder || checkoutUncertain) return;
    quoteRequest?.abort();
    const controller = new AbortController(); quoteRequest = controller;
    const abort = () => controller.abort();
    signal?.addEventListener("abort", abort, { once: true });
    if (signal?.aborted) controller.abort();
    quoteLoading = true; quoteError = ""; quote = null;
    try {
      const result = await quoteCheckout(reservation.id, storedReservation.idempotencyKey, code, controller.signal);
      if (controller.signal.aborted) return;
      if (result.subtotal !== reservation.subtotal) throw new Error("Rincian biaya tidak cocok dengan reservasi.");
      quote = result; voucher = code;
    } catch (error) {
      if (!controller.signal.aborted) quoteError = error instanceof ApiError && error.code === "INVALID_VOUCHER" ? "Kode promo tidak berlaku. Periksa kode atau hapus untuk melanjutkan." : "Biaya belum dapat dimuat. Coba lagi.";
    } finally {
      signal?.removeEventListener("abort", abort);
      if (quoteRequest === controller) { quoteLoading = false; quoteRequest = undefined; }
    }
  }
  onDestroy(() => quoteRequest?.abort());
  const paymentLabel = $derived(payment === "VIRTUAL_ACCOUNT" ? "Virtual Account" : payment === "GOPAY" ? "GoPay" : payment);
  const activeOrderPaymentLabel = $derived(activeOrder ? activeOrder.status === "PENDING" && paymentStatus === "SUCCEEDED" ? "Pembayaran diterima; status pesanan masih menunggu konfirmasi." : buyerPaymentStatusLabel(activeOrder.status, paymentStatus) : "");
  const quantityMap = $derived(Object.fromEntries(lines.map((line) => [line.tier.id, line.quantity])));
  const reservationKey = $derived(concert ? reservationBasketKey(concert.id, quantityMap) : "");
  const completionKey = $derived(`${reservationKey}:completed`);
  const checkoutAttemptKey = $derived(`${reservationKey}:checkout-attempt`);
  const simulationEnabled = import.meta.env.VITE_ENABLE_PAYMENT_SIMULATION === "true";
  function savedReservationForEvent(): StoredReservation | null {
    try {
      const quantities = Object.fromEntries([...new URLSearchParams(location.search)].flatMap(([tierId, raw]) => {
        if (!/^[1-9]\d*$/.test(raw)) return [];
        const quantity = Number(raw);
        return Number.isSafeInteger(quantity) ? [[tierId, quantity]] : [];
      }));
      const key = reservationBasketKey(id, quantities);
      if (!key.endsWith(":")) {
        const saved = parseStoredReservation(sessionStorage.getItem(key));
        if (saved?.eventId === id && saved.basketKey === key.slice(key.lastIndexOf(":") + 1) && saved.reservationId) return saved;
      }
    } catch { /* browser storage may be unavailable */ }
    return null;
  }
  async function goToStep(next: number) { step = next; await tick(); const name = next === 1 ? "buyer" : next === 2 ? "payment" : "confirmation"; const heading = document.querySelector<HTMLElement>(`#${name}-title`); heading?.focus(); (heading?.closest(".step-title") ?? heading)?.scrollIntoView({ behavior: matchMedia("(prefers-reduced-motion: reduce)").matches ? "auto" : "smooth", block: "start" }); }
  function validate(name: "name" | "email" | "phone") {
    const value = buyer[name].trim(); buyer[name] = value;
    const valid = name === "name" ? value.length >= 2 && value.length <= 80 : name === "email" ? isValidEmail(value) : isValidPhone(value);
    errors[name] = valid ? "" : name === "name" ? "Masukkan nama lengkap maksimal 80 karakter." : name === "email" ? "Masukkan alamat email yang valid." : "Masukkan nomor HP yang valid.";
    return valid;
  }
  function setAttendeeName(tierId: string, index: number, value: string) { if (checkoutUncertain || activeOrder) return; const names = [...(attendeeNames[tierId] ?? [])]; names[index] = value; attendeeNames[tierId] = names; }
  function copyBuyerName(scope: "first" | "all", confirmed = false) {
    if (activeOrder || checkoutUncertain || checkExpiry()) return;
    if (!validate("name")) { document.querySelector<HTMLInputElement>("#name")?.focus(); return; }
    const firstTier = lines[0]?.tier.id;
    const targets = lines.flatMap((line) => (attendeeNames[line.tier.id] ?? []).map((name, index) => ({ tierId: line.tier.id, index, name }))).filter((item) => scope === "all" || item.tierId === firstTier && item.index === 0);
    if (!confirmed && targets.some((item) => item.name.trim() && item.name.trim() !== buyer.name)) {
      copyScope = scope;
      copyDialog?.showModal();
      return;
    }
    for (const item of targets) setAttendeeName(item.tierId, item.index, buyer.name);
    attendeeErrors = {};
    copyDialog?.close();
  }
  async function submitBuyer(event: SubmitEvent) { event.preventDefault(); if (checkoutUncertain || activeOrder || checkExpiry()) return; const valid = (["name", "email", "phone"] as const).map(validate).every(Boolean); const attendeesValid = lines.every((line) => { const names = attendeeNames[line.tier.id] ?? []; const messages = names.map((name) => name.trim().length >= 2 && name.trim().length <= 80 ? "" : "Nama peserta harus 2–80 karakter."); attendeeErrors[line.tier.id] = messages; return names.length === line.quantity && messages.every((error) => !error); }); if (valid && attendeesValid) await goToStep(2); else { recordConversion(id, "VALIDATION_FAILED", valid ? "ATTENDEE_DATA" : "BUYER_DATA"); await tick().then(() => document.querySelector<HTMLInputElement>('[aria-invalid="true"]')?.focus()); } }
  async function submitPayment(event: SubmitEvent) { event.preventDefault(); if (checkExpiry()) return; paymentError = payment ? "" : "Pilih metode pembayaran terlebih dahulu."; if (!payment) { await tick(); document.querySelector<HTMLInputElement>('input[name="payment"]')?.focus(); return; } await goToStep(3); }
  function showToast(message: string, tone: "success" | "info" | "error" = "info") { toast = { id: ++toastId, message, tone }; }
  function openTerms() { termsDialog?.showModal(); }
  function closeDialog(dialog: HTMLDialogElement) { dialog.close(); }
  function restartSelection() { try { sessionStorage.removeItem(reservationKey); sessionStorage.removeItem(completionKey); sessionStorage.removeItem(checkoutAttemptKey); } catch { /* storage can be unavailable */ } }
  function startOver() { restartSelection(); if (concert) location.assign(`/konser/${concert.id}?${location.search.slice(1)}`); }
  function buyAgain() { restartSelection(); if (concert) location.assign(`/konser/${concert.id}`); }
  function recoverExpiredCheckout() { restartSelection(); if (concert) location.assign(`/konser/${concert.id}?${location.search.slice(1)}`); }
  function checkExpiry() {
    if (completed || activeOrder && activeOrder.status !== "PENDING" || expiresAt === null || !isReservationExpired(expiresAt)) return false;
    if (activeOrder || checkoutUncertain || paymentUncertain || converting) {
      if (activeOrder && !converting && !creatingReservation && checkedDeadline !== expiresAt) {
        checkedDeadline = expiresAt;
        void refreshOrderStatus();
      }
      return true;
    }
    reservationExpired = true;
    if (!expiryDialog?.open) expiryDialog?.showModal();
    return true;
  }
  async function applyVoucher(event: SubmitEvent) { event.preventDefault(); if (activeOrder || checkoutUncertain || reservationExpired || quoteLoading || checkExpiry()) return; await refreshQuote(voucherInput.trim().toUpperCase()); }
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
    else { storageFailed = true; showToast("Akses pesanan belum tersimpan di perangkat ini. Pembayaran ditahan sampai akses dapat disimpan.", "error"); }
    return saved;
  }
  async function restoreOrder(detail: OrderDetail, access: CheckoutOrderAccess, signal?: AbortSignal, loadTickets = true) {
    if (!concert) return;
    checkoutUncertain = false;
    pendingCheckoutPayload = null;
    orderAccess = access; activeOrder = detail; reference = detail.reference; buyer = detail.buyer;
    voucher = detail.voucher?.code ?? ""; voucherInput = voucher;
    orderRefundStatus = detail.refund?.status;
    attendeeNames = Object.fromEntries(lines.map((line) => [line.tier.id, detail.attendees.filter((attendee) => attendee.tierId === line.tier.id).map((attendee) => attendee.name)]));
    reservation = { id: detail.reservationId, status: detail.status, expiresAt: detail.expiresAt, event: { id: concert.id, artist: concert.artist }, items: detail.items, subtotal: detail.subtotal };
    storedReservation = { reservationId: detail.reservationId, idempotencyKey: access.idempotencyKey, eventId: concert.id, basketKey: reservationKey.slice(reservationKey.lastIndexOf(":") + 1) };
    expiresAt = Date.parse(detail.expiresAt);
    updateExpiryTimer();
    step = 3; payment = detail.payment?.method ?? (payment || "QRIS"); paymentStatus = detail.payment?.status ?? null;
    paymentUncertain = false;
    reservationExpired = detail.status === "EXPIRED" || detail.status === "CANCELLED";
    if (detail.status === "PAID" && loadTickets) {
      await reloadTickets(detail, access, signal);
    }
  }
  async function reloadTickets(detail = activeOrder, access = orderAccess, signal?: AbortSignal, focusOnSuccess = false) {
    if (ticketsLoading || !detail || !access || detail.status !== "PAID") return;
    ticketsLoading = true;
    ticketError = "";
    try {
      const tickets = await listOrderTickets(detail.id, access.accessToken, signal);
      saveIssuedTickets(tickets);
      completed = checkoutTicketsComplete(detail.status, tickets.length, detail.items.reduce((count, item) => count + item.quantity, 0));
      if (!completed) ticketError = "Sebagian e-ticket belum dapat dimuat.";
      else if (focusOnSuccess) { await tick(); document.querySelector<HTMLElement>("#success-title")?.focus(); }
    } catch (value) {
      if (!signal?.aborted) ticketError = value instanceof ApiError ? value.message : "Daftar tiket belum dapat dimuat.";
    } finally { ticketsLoading = false; }
  }
  async function refreshOrderStatus() {
    if (!activeOrder || !orderAccess || converting) return;
    converting = true;
    paymentCheckError = "";
    paymentCheckMessage = "";
    try {
      await restoreOrder(await getOrder(activeOrder.id, orderAccess.accessToken), orderAccess, undefined, false);
      paymentCheckMessage = `Status pembayaran diperbarui: ${activeOrderPaymentLabel}.`;
    } catch {
      paymentCheckError = "Pesanan belum dapat dimuat. Coba periksa lagi.";
      paymentCheckMessage = "Status pembayaran terbaru belum dapat diperiksa.";
    } finally { converting = false; await tick(); statusResult?.focus(); }
  }
  async function completeOrder(result: "SUCCEEDED" | "FAILED" = "SUCCEEDED") {
    if (!concert || !reservation || converting || paymentUncertain || (reservationExpired && !activeOrder && !checkoutUncertain) || (!checkoutUncertain && checkExpiry())) return;
    converting = true;
    try {
      if (!activeOrder) {
        if (!checkoutUncertain && (!quote || quoteLoading)) { paymentError = "Muat rincian biaya sebelum membuat pesanan."; return; }
        if (!storedReservation) throw new Error("Data idempotensi reservasi tidak tersedia");
        const payload = pendingCheckoutPayload ?? { buyer: { name: buyer.name, email: buyer.email, phone: buyer.phone }, attendees: lines.map((line) => ({ tierId: line.tier.id, names: (attendeeNames[line.tier.id] ?? []).map((name) => name.trim()) })), ...(voucher ? { voucherCode: voucher } : {}) };
        pendingCheckoutPayload = payload;
        checkoutUncertain = true;
        try { sessionStorage.setItem(checkoutAttemptKey, JSON.stringify(payload)); }
        catch { storageFailed = true; showToast("Data pemeriksaan pesanan belum tersimpan. Jangan tutup halaman sampai status pesanan dapat dipastikan.", "error"); }
        const created = await createOrder(reservation.id, payload, storedReservation.idempotencyKey, undefined, journeyId(concert.id));
        activeOrder = created;
        persistOrderAccessRecord(created, storedReservation.idempotencyKey);
        reference = created.reference;
        expiresAt = Date.parse(created.expiresAt);
        updateExpiryTimer();
        if (!quote || created.total !== quote.total) {
          totalChange = { previous: quote?.total ?? null, current: created.total };
          totalChangeAccepted = false;
          paymentError = quote ? "Nominal berubah sejak pratinjau. Periksa total pesanan lalu lanjutkan pembayaran." : "Pesanan dipulihkan. Periksa rincian dan total akhir lalu lanjutkan pembayaran.";
          return;
        }
      }
      if (!orderAccess || !activeOrder) throw new Error("Akses pesanan belum tersedia. Coba lagi.");
      if (totalChange && !totalChangeAccepted) {
        paymentError = "Konfirmasikan total pesanan sebelum melanjutkan pembayaran.";
        await tick(); document.querySelector<HTMLInputElement>("#accept-total")?.focus();
        return;
      }
      const latest = await getOrder(activeOrder.id, orderAccess.accessToken);
      activeOrder = latest; paymentStatus = latest.payment?.status ?? null;
      expiresAt = Date.parse(latest.expiresAt); updateExpiryTimer();
      if (latest.payment?.method) payment = latest.payment.method;
      if (activeOrder.status !== "PENDING") return;
      if (!canResumeOrderPayment(latest)) {
        paymentError = "Pembayaran ditahan. Periksa status pembayaran untuk melanjutkan.";
        return;
      }
      if (simulationEnabled) {
        paymentUncertain = true;
        const resultPayment = await simulatePayment(activeOrder.id, { method: payment || "QRIS", result }, orderAccess.accessToken);
        paymentUncertain = false;
        paymentStatus = resultPayment.status;
        if (resultPayment.orderStatus === "PAID") activeOrder = { ...activeOrder, status: "PAID" };
        if (resultPayment.status === "FAILED") paymentError = "Pembayaran simulasi gagal. Coba bayar kembali untuk menyelesaikan pesanan.";
        else if (resultPayment.orderStatus === "PAID") {
          if (resultPayment.tickets.length) saveIssuedTickets(resultPayment.tickets);
          await reloadTickets(activeOrder, orderAccess, undefined, true);
          paymentError = "";
        } else paymentError = "Pembayaran belum dikonfirmasi. Periksa status pembayaran sebelum mencoba lagi.";
      } else {
        const reservationForReturn = {
          reservationId: activeOrder.reservationId,
          idempotencyKey: orderAccess.idempotencyKey,
          eventId: concert.id,
          basketKey: reservationKey.slice(reservationKey.lastIndexOf(":") + 1),
        };
        try {
          sessionStorage.setItem(reservationKey, serializeStoredReservation(reservationForReturn));
          storedReservation = parseStoredReservation(sessionStorage.getItem(reservationKey));
        } catch { storedReservation = null; }
        if (!hasPersistentOrderAccess(activeOrder.id)) {
          saveOrderAccess(orderAccess);
        }
        if (!storedReservation || storedReservation.reservationId !== activeOrder.reservationId || !hasPersistentOrderAccess(activeOrder.id)) {
          storageFailed = true;
          paymentError = "Akses pesanan belum tersimpan di perangkat ini. Izinkan penyimpanan perangkat lalu coba lagi. Pembayaran belum dimulai.";
          return;
        }
        storageFailed = false;
        paymentUncertain = true;
        const { redirectUrl } = await createSnapPayment(activeOrder.id, payment || "QRIS", window.location.href, orderAccess.accessToken);
        window.location.assign(redirectUrl);
      }
    } catch (value) {
      if (value instanceof ApiError && (value.status === 0 || value.status >= 500) && value.code !== "PAYMENT_PROVIDER_ERROR") recordConversion(id, "SERVICE_FAILURE", "SERVICE");
      if (!activeOrder && value instanceof ApiError && value.status === 422) clearCheckoutAttempt();
      if (value instanceof ApiError && ["RESERVATION_EXPIRED", "ORDER_NOT_PAYABLE"].includes(value.code)) {
        reservationExpired = true;
        if (!activeOrder && !checkoutUncertain && !expiryDialog?.open) expiryDialog?.showModal();
        if (activeOrder) paymentError = "Pembayaran belum dapat dilanjutkan. Periksa status pembayaran.";
      } else if (value instanceof ApiError && value.code === "RESERVATION_CANCELLED") reservationError = value.message;
      else if (value instanceof ApiError && value.code === "INVALID_VOUCHER") paymentError = value.message;
      else { paymentError = paymentUncertain ? "Hasil pembayaran belum dapat dipastikan. Periksa status pembayaran sebelum mencoba lagi." : checkoutUncertain ? "Hasil pembuatan pesanan belum dapat dipastikan. Periksa status pesanan sebelum melanjutkan." : "Pesanan belum dapat diproses. Coba lagi."; if (activeOrder && !(value instanceof ApiError && (value.status === 0 || value.status >= 500))) paymentUncertain = false; }
    } finally { converting = false; }
  }
  async function setupReservation(signal: AbortSignal, recoveryHint: StoredReservation | null = null) {
    if (!concert) return;
    if (recoveryHint?.reservationId) {
      creatingReservation = true;
      reservationError = "";
      try {
        const confirmed = await getReservation(recoveryHint.reservationId, signal);
        if (signal.aborted) return;
        const quantities = restoredReservationQuantities(concert, id, new URLSearchParams(location.search), confirmed);
        const confirmedKey = quantities ? reservationBasketKey(concert.id, quantities) : "";
        const confirmedBasket = confirmedKey.slice(confirmedKey.lastIndexOf(":") + 1);
        if (confirmed.id !== recoveryHint.reservationId || !quantities || recoveryHint.eventId !== concert.id || recoveryHint.basketKey !== confirmedBasket) {
          throw new ApiError("Data reservasi tidak cocok dengan konser dan keranjang ini. Mulai checkout dari pilihan tiket yang tersimpan.", 409, "RESERVATION_MISMATCH");
        }
        restoredReservation = confirmed;
        reservation = confirmed;
        storedReservation = recoveryHint;
        await tick();
        if (reservationKey !== confirmedKey) throw new ApiError("Keranjang checkout berubah saat pemulihan reservasi.", 409, "RESERVATION_MISMATCH");
        creatingReservation = false;
        return setupReservation(signal);
      } catch (value) {
        if (signal.aborted) return;
        reservationError = value instanceof ApiError ? value.message : "Reservasi sebelumnya belum dapat dimuat.";
        restartCheckoutRequired = value instanceof ApiError && value.code === "RESERVATION_MISMATCH";
        creatingReservation = false;
        return;
      }
    }
    if (!lines.length) return;
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
        if (!reservation || !orderMatchesReservation(detail, reservation)) throw new ApiError("Pesanan tidak cocok dengan data tiket yang dipilih. Coba pilih tiket kembali.", 409, "RESERVATION_MISMATCH");
        await restoreOrder(detail, savedOrder, signal);
        if (detail.status === "PENDING") totalChange = { previous: null, current: detail.total };
        creatingReservation = false;
        return startExpiryTimer();
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
      reservation = sameBasket && stored?.reservationId ? await getReservation(stored.reservationId, signal) : await createReservation(payload.eventId, payload.items, idempotencyKey, signal, journeyId(concert.id));
      if (reservation.event.id !== concert.id || reservation.items.length !== lines.length || reservation.items.some((item) => item.quantity !== quantityMap[item.tierId] || !quantityMap[item.tierId])) throw new ApiError("Data reservasi tidak cocok dengan keranjang.", 409, "RESERVATION_MISMATCH");
      storedReservation = { reservationId: reservation.id, idempotencyKey, eventId: concert.id, basketKey };
      try { sessionStorage.setItem(reservationKey, serializeStoredReservation(storedReservation)); } catch { storageFailed = true; }
      expiresAt = Date.parse(reservation.expiresAt);
    } catch (value) {
      reservationError = value instanceof ApiError && value.code === "RESERVATION_EXPIRED" ? "" : value instanceof ApiError ? value.message : "Reservasi belum dapat dibuat.";
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
        const created = await createOrder(reservation.id, pendingCheckoutPayload, idempotencyKey, signal, journeyId(concert.id));
        if (signal.aborted) return;
        activeOrder = created;
        persistOrderAccessRecord(created, idempotencyKey);
        const detail = await getOrder(created.id, created.accessToken, signal);
        if (signal.aborted) return;
        await restoreOrder(detail, orderAccess!, signal);
        if (detail.status === "PENDING") totalChange = { previous: null, current: detail.total };
        creatingReservation = false;
        return startExpiryTimer();
      } catch (value) {
        if (signal.aborted) return;
        if (value instanceof ApiError && value.status === 422) {
          clearCheckoutAttempt();
          await refreshQuote("", signal);
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
    if (!isReservationExpired(expiresAt ?? 0)) await refreshQuote("", signal);
    attendeeNames = Object.fromEntries(lines.map((line) => [line.tier.id, Array.from({ length: line.quantity }, () => "")]));
    return startExpiryTimer();
  }
  onMount(() => {
    const controller = new AbortController();
    const stopTracking = trackConversionActivity(id);
    const desktopViewport = window.matchMedia("(min-width: 768px)");
    const syncViewport = () => { isDesktop = desktopViewport.matches; };
    syncViewport();
    desktopViewport.addEventListener("change", syncViewport);
    const syncKeyboard = () => {
      const viewport = window.visualViewport;
      const inset = viewport ? Math.max(0, window.innerHeight - viewport.height - viewport.offsetTop) : 0;
      keyboardInset = inset > 100 ? inset : 0;
    };
    window.visualViewport?.addEventListener("resize", syncKeyboard);
    window.visualViewport?.addEventListener("scroll", syncKeyboard);
    let cleanupReservation: (() => void) | undefined;
    const recoveryHint = savedReservationForEvent();
    (async () => {
      try {
        let event: Concert;
        try { event = await getEvent(id, controller.signal); }
        catch (cause) {
          if (!(cause instanceof ApiError && cause.code === "EVENT_NOT_FOUND" && recoveryHint?.reservationId)) throw cause;
          event = await getReservationEvent(recoveryHint.reservationId, recoveryHint.idempotencyKey, controller.signal);
        }
        if (!controller.signal.aborted) { concert = event; loading = false; recordConversion(event.id, "CHECKOUT_STARTED"); cleanupReservation = await setupReservation(controller.signal, recoveryHint); }
      } catch (value) {
        if (!controller.signal.aborted) { loadError = value instanceof ApiError && value.code === "EVENT_NOT_FOUND" ? "not-found" : value instanceof ApiError ? value.message : "Checkout belum dapat dimuat."; loading = false; }
      }
    })();
    return () => { controller.abort(); stopTracking(); cleanupReservation?.(); desktopViewport.removeEventListener("change", syncViewport); window.visualViewport?.removeEventListener("resize", syncKeyboard); window.visualViewport?.removeEventListener("scroll", syncKeyboard); };
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
{:else if reservationExpired && !activeOrder && !checkoutUncertain}
  <section class="shell empty-state" role="status"><p>Waktu reservasi habis. Pilih tiket kembali untuk memeriksa ketersediaannya.</p><button class="button" type="button" onclick={recoverExpiredCheckout}>Pilih tiket kembali</button></section>
{:else if reservationError && restartCheckoutRequired}
  <section class="shell empty-state" role="alert"><p>{reservationError}</p><button class="button" type="button" onclick={startOver}>Pilih tiket lagi</button></section>
{:else if reservationError}
  <section class="shell empty-state" role="alert"><p>{reservationError}</p><button class="text-button" type="button" onclick={() => location.reload()}>Coba lagi</button></section>
{:else if creatingReservation || !reservation}
  <p class="shell" role="status" aria-live="polite">Menahan tiket sementara...</p>
{:else}
   {#snippet primaryAction()}
  {#if step === 1 && !activeOrder}
    <button class="button" type="submit" form="buyer-form" disabled={converting || reservationExpired || checkoutUncertain}>Lanjut ke pembayaran</button>
  {:else if step === 2 && !activeOrder}
    <button class="button" type="submit" form="payment-form" disabled={converting || reservationExpired || checkoutUncertain}>Tinjau pesanan</button>
  {:else if step === 3 && !activeOrder}
    <button class="button" type="button" disabled={converting || reservationExpired || paymentUncertain || (!checkoutUncertain && (!quote || quoteLoading)) || Boolean(totalChange && !totalChangeAccepted)} onclick={() => completeOrder("SUCCEEDED")}>{converting ? "Memeriksa..." : checkoutUncertain ? "Periksa status pesanan" : "Buat pesanan dan bayar"}</button>
  {:else if activeOrder?.status === "PENDING"}
    {#if paymentUncertain || checkoutUncertain || paymentStatus === "SUCCEEDED" || remainingSeconds === 0 || paymentCheckError}
      <button class="button" type="button" disabled={converting} onclick={refreshOrderStatus}>{converting ? "Memeriksa..." : "Periksa status pembayaran"}</button>
    {:else}
      <button class="button" type="button" disabled={converting || Boolean(totalChange && !totalChangeAccepted)} onclick={() => completeOrder("SUCCEEDED")}>{converting ? "Memeriksa..." : paymentStatus === "FAILED" ? "Coba bayar lagi" : "Lanjutkan pembayaran"}</button>
    {/if}
  {:else if activeOrder?.status === "PAID" && !completed}
    <button class="button" type="button" disabled={ticketsLoading} onclick={() => reloadTickets()}>{ticketsLoading ? "Memuat..." : "Muat ulang e-ticket"}</button>
  {/if}
{/snippet}

<section class="checkout shell" style={`--checkout-actionbar-height:${actionBarHeight}px;--checkout-keyboard-inset:${keyboardInset}px`} onfocusin={(event) => { const field = event.target; if (field instanceof HTMLInputElement || field instanceof HTMLSelectElement) requestAnimationFrame(() => field.scrollIntoView({ block: "nearest", behavior: matchMedia("(prefers-reduced-motion: reduce)").matches ? "auto" : "smooth" })); }}>
  <EventNotice state={concert.currentEvent} />
  <a class="back-link" href={`/konser/${concert.id}?${location.search.slice(1)}`}>Kembali ke detail konser</a>
  <div class="checkout-heading">
    <p class="checkout-kicker">Checkout aman</p>
    <h1>Selesaikan pesananmu.</h1>
    <p>{concert.artist} · {eventDate(concert.currentEvent?.startsAt ?? concert.startsAt)} · {concert.venue}</p>
  </div>
  {#if showCheckoutPaymentCountdown(completed, activeOrder?.status, paymentStatus)}
    <div class:reservation-warning={remainingSeconds <= 60} class="reservation-banner" role="timer" aria-live="off">
      <span class="reservation-icon" aria-hidden="true">◷</span>
      <span><b>{activeOrder ? "Batas pembayaran" : "Tiket ditahan selama"} {String(Math.floor(remainingSeconds / 60)).padStart(2, "0")}:{String(remainingSeconds % 60).padStart(2, "0")}</b><small>{activeOrder ? "Membuat pesanan tidak menambah waktu." : "Batas ini mencakup pengisian data hingga pembayaran."}</small></span>
    </div>
  {/if}
  <p class="expiry-warning" role="status" aria-live="polite">{expiryWarning}</p>
  <ol class="checkout-steps" aria-label="Tahap checkout">
    {#each ["Data pemesan", "Metode pembayaran", "Konfirmasi"] as label, index}
      <li class:is-complete={index + 1 < step || completed} aria-current={index + 1 === step && !completed ? "step" : undefined}><span>{index + 1}</span><b>{label}</b></li>
    {/each}
  </ol>

  <div class="checkout-layout">
    <section class="checkout-panel" aria-label="Langkah checkout">
      {#if completed}
        <section class="checkout-success">
          <p class="checkout-kicker">Pesanan berhasil</p>
          <h2 id="success-title" tabindex="-1">E-ticket sudah dibuat.</h2>
          <p>Pembayaran dikonfirmasi. Pengiriman email mungkin membutuhkan waktu. Buka Tiket Saya atau gunakan tautan email konfirmasi untuk mengakses e-ticket.</p>
          <div class="booking-code"><span>Nomor pesanan</span><strong>{reference}</strong></div>
          {#if storageFailed}
            <p class="success-note">Akses belum tersimpan di perangkat ini. Simpan kode tiket berikut selagi halaman terbuka, lalu gunakan tautan email konfirmasi atau Pulihkan tiket.</p>
            {#each issuedTickets as ticket}<article class="memory-ticket"><b>{ticket.attendeeName} · {ticket.tierName} · {ticket.gate}</b><span>{ticket.eventArtist} · {ticket.eventVenue}</span><strong>{ticket.code}</strong></article>{/each}
            <a class="button button-secondary" href="/pulihkan-tiket">Pulihkan tiket</a>
          {:else}
            <a class="button button-secondary" href="/tiket-saya">Tiket saya</a>
            {#each ticketIds as ticketId}<a class="button" href={`/tiket/${encodeURIComponent(ticketId)}`}>Lihat e-ticket</a>{/each}
          {/if}
        </section>
      {:else if step === 1}
        <form id="buyer-form" novalidate onsubmit={submitBuyer}>
          <div class="step-title"><p>Langkah 1 dari 3</p><h2 id="buyer-title" tabindex="-1">Data pemesan</h2><span>Gunakan email aktif untuk menerima konfirmasi dan mengakses e-ticket setelah pembayaran berhasil.</span></div>
          <div class="field-grid buyer-fields">
            <label for="name">Nama pemesan<input id="name" type="text" autocomplete="name" maxlength="80" bind:value={buyer.name} required disabled={Boolean(activeOrder) || checkoutUncertain} aria-describedby="name-error" aria-invalid={Boolean(errors.name)} class:is-invalid={Boolean(errors.name)} onblur={() => validate("name")} /><small id="name-error" aria-live="polite">{errors.name}</small></label>
            <label for="email">Email aktif<input id="email" type="email" autocomplete="email" maxlength="254" bind:value={buyer.email} required disabled={Boolean(activeOrder) || checkoutUncertain} aria-describedby="email-hint email-error" aria-invalid={Boolean(errors.email)} class:is-invalid={Boolean(errors.email)} onblur={() => validate("email")} /><span id="email-hint" class="field-hint">Konfirmasi pesanan dan akses e-ticket dikirim ke email ini. Tiket juga tersedia di Tiket Saya.</span><small id="email-error" aria-live="polite">{errors.email}</small></label>
            <label for="phone">Nomor HP<input id="phone" type="tel" inputmode="tel" autocomplete="tel" maxlength="20" bind:value={buyer.phone} required disabled={Boolean(activeOrder) || checkoutUncertain} aria-describedby="phone-error" aria-invalid={Boolean(errors.phone)} class:is-invalid={Boolean(errors.phone)} onblur={() => validate("phone")} /><small id="phone-error" aria-live="polite">{errors.phone}</small></label>
          </div>
          <section class="attendee-fields" aria-label="Nama pemegang tiket">
            <h3>Nama pemegang tiket</h3>
            <p class="field-hint">Isi nama untuk setiap tiket. Nama akan tercetak pada e-ticket dan boleh sama.</p>
            {#if !activeOrder && !checkoutUncertain}
              <div class="attendee-copy-actions" aria-label="Salin nama pemesan">
                <button type="button" class="attendee-copy-button" aria-label="Gunakan nama pemesan untuk tiket pertama" title="Gunakan nama pemesan untuk tiket pertama" onclick={() => copyBuyerName("first")}>
                  <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><circle cx="12" cy="8" r="3.25" /><path d="M5.5 20v-1.25a6.5 6.5 0 0 1 13 0V20" /></svg>
                </button>
                <button type="button" class="attendee-copy-button" aria-label="Gunakan nama pemesan untuk semua tiket" title="Gunakan nama pemesan untuk semua tiket" onclick={() => copyBuyerName("all")}>
                  <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><circle cx="9" cy="8" r="3" /><path d="M3 20v-1.25a6 6 0 0 1 12 0V20m1.5-14.5a3 3 0 0 1 0 5.8m2.5 3a5.5 5.5 0 0 1 2 4.25V20" /></svg>
                </button>
              </div>
            {/if}
            {#each lines as line, lineIndex}
              {@const firstNumber = lines.slice(0, lineIndex).reduce((count, previous) => count + previous.quantity, 0) + 1}
              <fieldset><legend>{line.tier.name} · {line.quantity} tiket</legend>
                {#each attendeeNames[line.tier.id] ?? [] as _name, index}
                  {@const number = firstNumber + index}
                  <label for={`attendee-${line.tier.id}-${index}`}>Nama peserta {number}<input id={`attendee-${line.tier.id}-${index}`} value={attendeeNames[line.tier.id]?.[index] ?? ""} oninput={(event) => setAttendeeName(line.tier.id, index, event.currentTarget.value)} maxlength="80" autocomplete="off" disabled={Boolean(activeOrder) || checkoutUncertain} aria-describedby={`attendee-error-${line.tier.id}-${index}`} aria-invalid={Boolean(attendeeErrors[line.tier.id]?.[index])} required /><small id={`attendee-error-${line.tier.id}-${index}`} class="form-error">{attendeeErrors[line.tier.id]?.[index] ?? ""}</small></label>
                {/each}
              </fieldset>
            {/each}
          </section>
          <div class="step-actions"><span></span>{@render primaryAction()}</div>
        </form>
      {:else if step === 2}
        <form id="payment-form" onsubmit={submitPayment}>
          <div class="step-title"><p>Langkah 2 dari 3</p><h2 id="payment-title" tabindex="-1">Metode pembayaran</h2><span>Pilih kanal pembayaran yang tersedia. Kamu akan melanjutkan pembayaran melalui penyedia pembayaran.</span></div>
          <fieldset aria-describedby="payment-error" class:has-error={Boolean(paymentError)}><legend class="sr-only">Metode pembayaran</legend><div class="payment-options">{#each [{ id: "QRIS", label: "QRIS" }, { id: "VIRTUAL_ACCOUNT", label: "Virtual Account" }, { id: "GOPAY", label: "GoPay" }] as method}<label class="payment-card"><input bind:group={payment} type="radio" name="payment" value={method.id} aria-describedby="payment-error" disabled={Boolean(activeOrder) || checkoutUncertain} /><span><b>{method.label}</b><small>{simulationEnabled ? "Simulasi" : "Lanjut melalui penyedia pembayaran"}</small></span></label>{/each}</div></fieldset>
          <p id="payment-error" class="form-error" role="status" aria-live="polite">{paymentError}</p>
          <div class="step-actions"><button class="text-button" type="button" disabled={Boolean(activeOrder) || checkoutUncertain || reservationExpired} onclick={() => goToStep(1)}>Kembali</button>{@render primaryAction()}</div>
        </form>
      {:else}
        <section id="confirmation" aria-labelledby="confirmation-title">
          <div class="step-title"><p>Langkah 3 dari 3</p><h2 id="confirmation-title" tabindex="-1">Tinjau pesanan</h2><span>Periksa detail dan total sebelum pembayaran.</span></div>
          <div class="confirmation-block"><div><span>Konser dan jadwal</span><b>{concert.artist}</b><p>{eventDate(concert.currentEvent?.startsAt ?? concert.startsAt)} · {concert.venue}, {concert.city}</p></div></div>
          {#each lines as line, lineIndex}
            {@const firstNumber = lines.slice(0, lineIndex).reduce((count, previous) => count + previous.quantity, 0) + 1}
            <div class="confirmation-block"><div><span>{line.tier.name} · {line.quantity} tiket</span>{#each attendeeNames[line.tier.id] ?? [] as attendeeName, index}<p>Peserta {firstNumber + index}: {attendeeName}</p>{/each}</div>{#if !activeOrder && !checkoutUncertain}<button class="text-button" type="button" onclick={() => goToStep(1)}>Ubah data</button>{/if}</div>
          {/each}
          <div class="confirmation-block"><div><span>Email penerima</span><b>{buyer.email}</b></div>{#if !activeOrder && !checkoutUncertain}<button class="text-button" type="button" onclick={() => goToStep(1)}>Ubah data</button>{/if}</div>
          <div class="confirmation-block"><div><span>Metode pembayaran</span><b>{paymentLabel || "Belum dipilih"}</b></div>{#if !activeOrder && !checkoutUncertain}<button class="text-button" type="button" onclick={() => goToStep(2)}>Ubah metode</button>{/if}</div>
          <div class="confirmation-block confirmation-total"><div><span>{activeOrder ? "Total pesanan" : "Estimasi total"}</span><b>{total === undefined ? "Belum tersedia" : formatRupiah.format(total)}</b>{#if totalChange}<p>{#if totalChange.previous === null}Total pesanan dipulihkan dari server. Pastikan nominalnya sebelum membayar.{:else}Total berubah dari {formatRupiah.format(totalChange.previous)} menjadi {formatRupiah.format(totalChange.current)}. Konfirmasi perubahan ini untuk melanjutkan.{/if}</p>{/if}</div></div>
          {#if totalChange && !totalChangeAccepted && !paymentUncertain}<label class="total-confirmation" for="accept-total"><input id="accept-total" type="checkbox" bind:checked={totalChangeAccepted} />Saya telah memeriksa dan menyetujui total pesanan ini.</label>{/if}
          {#if activeOrder?.status === "PENDING"}<section class="recovery-panel" aria-labelledby="checkout-payment-status"><h3 id="checkout-payment-status">Status pembayaran</h3><p>{activeOrderPaymentLabel}</p>{#if paymentStatus === "SUCCEEDED"}<p>Pembayaran diterima dan status pesanan masih menunggu konfirmasi. Tiket akan tersedia setelah status lunas dikonfirmasi.</p>{:else if paymentStatus === "FAILED"}<p>Pembayaran sebelumnya gagal. Pesanan tetap menunggu pembayaran.</p>{/if}{#if paymentCheckError}<p role="alert">{paymentCheckError} Periksa status lagi sebelum mencoba pembayaran.</p>{/if}</section>{/if}
          {#if activeOrder?.status === "EXPIRED" || activeOrder?.status === "CANCELLED"}<section class="recovery-panel" aria-live="polite"><h3>{activeOrder.status === "EXPIRED" ? "Pesanan kedaluwarsa" : "Pesanan dibatalkan"}</h3><p>Pilih tiket kembali untuk memeriksa ketersediaannya.</p><a class="button button-secondary" href={`/konser/${concert.id}`}>Pilih tiket kembali</a></section>{/if}
          <nav class="purchase-help" aria-label="Bantuan dan ketentuan sebelum konfirmasi"><a class="text-button" href="/panduan" target="_blank" rel="noopener">Bantuan <span class="sr-only">(tab baru)</span></a><button class="text-button" type="button" onclick={openTerms}>Ketentuan pembelian</button><a class="text-button" href="/panduan#refund" target="_blank" rel="noopener">Kebijakan refund <span class="sr-only">(tab baru)</span></a></nav>
          {#if !activeOrder && !checkoutUncertain}<p class="terms-trigger">Dengan melanjutkan, kamu menyetujui <button class="text-button" type="button" onclick={openTerms}>S&amp;K konser</button>.</p>{/if}
          <p id="checkout-payment-error" class="form-error" role="alert" aria-live="polite">{paymentError}</p>
          <div class="step-actions"><button class="text-button" type="button" disabled={checkoutUncertain || Boolean(activeOrder)} onclick={() => goToStep(2)}>Kembali</button>{@render primaryAction()}</div>
          {#if activeOrder?.status === "PENDING" && !paymentUncertain}<button class="text-button" type="button" disabled={converting} onclick={refreshOrderStatus}>{converting ? "Memeriksa..." : "Periksa status pembayaran"}</button>{/if}
        </section>
      {/if}
    </section>

    <aside class="order-summary" aria-label="Ringkasan biaya pesanan">
      <h2>Ringkasan pesanan</h2>
      <div class="summary-total"><span>{activeOrder ? "Total pesanan" : "Estimasi total"}</span><strong>{total === undefined ? "Belum tersedia" : formatRupiah.format(total)}</strong></div>
      <p class="summary-subtotal-note">Biaya admin dihitung saat checkout.</p>
      <p class="summary-event">{concert.artist}</p><p>{eventDate(concert.currentEvent?.startsAt ?? concert.startsAt)} · {concert.venue}, {concert.city}</p>
      <details class="summary-breakdown" open={isDesktop}>
        <summary>Rincian biaya</summary>
        {#each reservation.items as line}<div class="summary-line"><span>{line.quantity}× {line.name}</span><b>{formatRupiah.format(line.lineTotal)}</b></div>{/each}
        <div class="summary-line"><span>Subtotal</span><b>{formatRupiah.format(subtotal)}</b></div>
        <div class="summary-line"><span>Biaya admin</span><b>{activeOrder || quote ? formatRupiah.format(activeOrder?.adminFee ?? quote!.adminFee) : "Belum tersedia"}</b></div>
        {#if discount}<div class="summary-line discount-row"><span>Diskon</span><b>-{formatRupiah.format(discount)}</b></div>{/if}
        {#if !activeOrder && !checkoutUncertain}<form class="voucher-form" onsubmit={applyVoucher}><label for="voucher">Kode promo<input bind:value={voucherInput} id="voucher" autocomplete="off" disabled={reservationExpired || quoteLoading} /></label><button class="button button-small" type="submit" disabled={reservationExpired || quoteLoading}>Gunakan</button></form>{/if}
        {#if quoteLoading}<p role="status">Memuat rincian biaya…</p>{/if}{#if quoteError}<p role="alert">{quoteError}</p><button class="button button-small" type="button" onclick={() => refreshQuote(voucherInput.trim().toUpperCase())} disabled={quoteLoading || reservationExpired}>Muat ulang biaya</button>{/if}
        {#if voucher}<p class:is-valid={Boolean(discount)} class="voucher-message" aria-live="polite">{activeOrder ? "Voucher tercatat pada pesanan." : discount ? `Estimasi voucher ${voucher} diterapkan.` : "Voucher belum diterapkan."}</p>{/if}
      </details>
      <button class="terms-button" type="button" onclick={openTerms}>Lihat S&amp;K Konser</button>
    </aside>
  </div>
  {#if simulationEnabled && !activeOrder && !checkoutUncertain}<button class="text-button" type="button" disabled={reservationExpired || converting} onclick={() => completeOrder("FAILED")}>Simulasikan pembayaran gagal</button>{/if}
  <div class="checkout-mobile-actions" bind:clientHeight={actionBarHeight}>
    {#if step === 1 && !activeOrder}<span>Est. total <b>{total === undefined ? "Belum tersedia" : formatRupiah.format(total)}</b></span>{:else if step >= 2 && !completed}<span>{activeOrder ? "Total pesanan" : "Est. total"} <b>{total === undefined ? "Belum tersedia" : formatRupiah.format(total)}</b></span>{/if}
    {@render primaryAction()}
  </div>
</section>
<dialog class="terms-dialog copy-name-dialog" bind:this={copyDialog} aria-labelledby="copy-name-title">
  <div class="dialog-heading"><p class="checkout-kicker">Nama peserta</p><h2 id="copy-name-title">Ganti nama tiket?</h2></div>
  <p>Nama pada tiket yang sudah diisi akan diganti dengan nama pemesan. Perubahan hanya berlaku untuk pengisian ini.</p>
  <div class="dialog-actions"><button class="text-button" type="button" onclick={() => copyDialog?.close()}>Batal</button><button class="button" type="button" onclick={() => copyBuyerName(copyScope, true)}>Ganti nama</button></div>
</dialog>
<dialog class="terms-dialog" bind:this={termsDialog} aria-labelledby="terms-title"><div class="dialog-heading"><p class="checkout-kicker">Informasi penting</p><h2 id="terms-title">Syarat &amp; ketentuan konser</h2></div><div class="terms-dialog-list">{#each concertTerms as term}<section><h3>{term.title}</h3><p>{term.body}</p></section>{/each}</div><form method="dialog" class="dialog-actions"><button class="button" type="submit">Tutup</button></form></dialog>
    <dialog class="expiry-dialog" bind:this={expiryDialog} aria-labelledby="expiry-title"><p class="checkout-kicker">Reservasi berakhir</p><h2 id="expiry-title">Waktu reservasi habis.</h2><p>Waktu reservasi habis. Pilih tiket kembali untuk memeriksa ketersediaannya.</p><button class="button" type="button" onclick={recoverExpiredCheckout}>Pilih tiket kembali</button></dialog>
    {#if toast}<Toast id={toast.id} message={toast.message} tone={toast.tone} onDismiss={() => toast = null} />{/if}
{/if}

<style>
  .checkout { padding-bottom: calc(var(--checkout-actionbar-height, 0px) + 24px + env(safe-area-inset-bottom, 0px)); }
  .checkout-heading { margin-block: 18px 22px; }
  .checkout-steps { gap: 18px; margin-bottom: 22px; }
  .checkout-panel { grid-column: 1; grid-row: 1; }
  .order-summary { grid-column: 2; grid-row: 1; }
  .summary-total { display: flex; align-items: baseline; justify-content: space-between; gap: 12px; margin: 12px 0 18px; padding: 14px 0; border-block: 1px solid var(--line); }
  .summary-total strong { font-size: 24px; text-align: right; }
  .summary-subtotal-note, .field-hint { color: var(--muted); font-size: 13px; line-height: 1.5; }
  .summary-subtotal-note { margin: 12px 0 0; }
  .field-grid label { align-content: start; }
  .field-grid small:empty, .form-error:empty { display: none; }
  .field-hint { font-weight: 400; }
  .attendee-fields { margin-top: 24px; }
  .attendee-fields > p { margin: 0; }
  .attendee-copy-actions { display: flex; flex-wrap: wrap; gap: 8px; }
  .attendee-copy-button { display: grid; width: 44px; min-width: 44px; min-height: 44px; place-items: center; border: 1px solid var(--line); border-radius: 50%; background: var(--surface); color: var(--accent); cursor: pointer; }
  .attendee-copy-button:hover { border-color: var(--accent); background: color-mix(in srgb, var(--accent) 8%, var(--surface)); }
  .attendee-copy-button:focus-visible { outline: 2px solid var(--accent); outline-offset: 3px; }
  .attendee-fields fieldset { margin: 0; }
  .step-title { margin-bottom: 18px; }
  .confirmation-total { font-size: 18px; font-weight: 700; }
  .confirmation-total p { max-width: 460px; margin: 8px 0 0; color: var(--muted); font-size: 14px; font-weight: 400; }
  .total-confirmation { display: flex; align-items: flex-start; gap: 10px; margin-block: 14px; font-weight: 600; }
  .total-confirmation input { flex: 0 0 auto; width: 20px; height: 20px; accent-color: var(--accent); }
  .expiry-warning { max-width: 720px; margin: 0 0 12px; color: var(--status-warning); font-weight: 700; line-height: 1.5; }
  .expiry-warning:empty { display: none; }
  .checkout-mobile-actions { display: none; }
  .copy-name-dialog > p { line-height: 1.5; }
  .copy-name-dialog .dialog-actions { justify-content: flex-end; gap: 12px; }
  @media (max-width: 767px) {
    .checkout { padding-bottom: calc(var(--checkout-actionbar-height, 76px) + 28px + env(safe-area-inset-bottom, 0px)); }
    .checkout-heading { margin-block: 18px 20px; }
    .checkout-steps { gap: 8px; margin-bottom: 18px; }
    .checkout-layout { grid-template-columns: minmax(0, 1fr); gap: 20px; }
    .checkout .checkout-panel { grid-column: 1; grid-row: 2; }
    .checkout .order-summary { grid-column: 1; grid-row: 1; }
    .checkout-mobile-actions { position: fixed; z-index: 10; right: 0; bottom: calc(var(--checkout-keyboard-inset, 0px) + env(safe-area-inset-bottom, 0px)); left: 0; display: grid; grid-template-columns: minmax(0, .8fr) minmax(0, 1.2fr); align-items: center; gap: 12px; border-top: 1px solid var(--line); padding: 10px 16px; background: var(--surface); box-shadow: 0 -5px 16px color-mix(in srgb, var(--ink) 10%, transparent); }
    .checkout-mobile-actions > span { display: grid; min-width: 0; gap: 3px; color: var(--muted); font-size: 12px; }
    .checkout-mobile-actions > span b { overflow-wrap: anywhere; color: var(--ink); font-size: 14px; }
    .checkout-mobile-actions > .button { width: 100%; min-width: 0; min-height: 48px; padding-inline: 8px; white-space: normal; }
    .checkout-mobile-actions > .button:disabled { opacity: .58; }
    .checkout .summary-breakdown > summary { min-height: 44px; display: flex; align-items: center; color: var(--accent); font-weight: 700; cursor: pointer; }
    .checkout .summary-breakdown > summary::-webkit-details-marker { display: none; }
    .checkout .summary-breakdown:not([open]) > :not(summary) { display: none; }
    .checkout .summary-breakdown .voucher-form { padding-block: 12px; }
    .checkout .order-summary { padding: 12px 14px; }
    .checkout .order-summary h2 { margin-bottom: 4px; font-size: 20px; }
    .checkout .order-summary > .summary-event, .checkout .order-summary > p:nth-of-type(2) { display: none; }
    .checkout .confirmation-block { gap: 12px; padding-block: 14px; }
    .checkout .field-grid input, .checkout .attendee-fields input { scroll-margin-block: calc(var(--checkout-actionbar-height, 76px) + env(safe-area-inset-top, 0px) + 16px); }
    .checkout .step-title { scroll-margin-top: calc(env(safe-area-inset-top, 0px) + 16px); }
    .checkout .checkout-panel .step-actions > .button { display: none; }
  }
</style>
