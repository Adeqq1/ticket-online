<script lang="ts">
  import EventNotice from "../../components/EventNotice.svelte";
  import { emailAccessToken } from "../../lib/ticket-email-access.ts";
  import { onMount, tick } from "svelte";
  import { ApiError, getOrder, listOrderTickets, requestEventRefund, resendOrderEmail, type ApiTicket, type OrderDetail } from "../../lib/api.ts";
  import { buyerPaymentStatusLabel } from "../../lib/buyer-payment.ts";
  import { buyerOrderStatusLabel } from "../../lib/buyer-refund.ts";
  import { remainingReservationSeconds } from "../../lib/reservation.ts";
  import { refundStatusLabel } from "../../lib/buyer-refund.ts";
  import { loadBuyerOrderTickets } from "../../lib/buyer-tickets.ts";
  import { eventDate, formatRupiah } from "../../lib/concerts.ts";
  import { getOrderAccess, hasPersistentTicketAccess, saveOrderAccess, saveOrderTicketAccess } from "../../lib/order-access.ts";

  let { id }: { id: string } = $props();
  let detail = $state<OrderDetail | null>(null);
  let tickets = $state<ApiTicket[]>([]);
  let loading = $state(true);
  let error = $state("");
  let sending = $state(false);
  let resendMessage = $state("");
  let resendError = $state("");
  let refundBusy = $state(false);
  let refundMessage = $state("");
  let refundConfirmed = $state(false);
  let refundNeedsReview = $state(false);
  let refundResult = $state<HTMLParagraphElement>();
  let pageController: AbortController | undefined;
  let emailToken: string | null = null;
  let paymentBusy = $state(false);
  let paymentCheckError = $state("");
  let paymentMessage = $state("");
  let ticketError = $state("");
  let ticketAccessError = $state("");
  let paymentResult = $state<HTMLParagraphElement>();
  let isDesktop = $state(true);
  let remainingSeconds = $state(0);

  function updateCountdown() {
    remainingSeconds = detail?.status === "PENDING" ? remainingReservationSeconds(Date.parse(detail.expiresAt)) : 0;
  }

  async function load() {
    pageController?.abort();
    const controller = new AbortController(); pageController = controller;
    loading = true; error = "";
    try {
      let access = getOrderAccess(id);
      if (emailToken) {
        const result = await getOrder(id, emailToken, controller.signal);
        if (controller.signal.aborted) return;
        access = { orderId: id, accessToken: emailToken, reference: result.reference, reservationId: result.reservationId, expiresAt: result.expiresAt, accessExpiresAt: result.accessExpiresAt, ticketIds: [] };
        saveOrderAccess(access); emailToken = null;
      }
      if (!access) { error = "Akses pesanan tidak tersimpan. Pulihkan akses dengan email pembeli dan kode pesanan (reference)."; return; }
      const result = await loadBuyerOrderTickets(access, controller.signal);
      if (controller.signal.aborted) return;
      if (result.detail) { detail = result.detail; updateCountdown(); }
      tickets = result.tickets;
      ticketError = result.detail && result.error ? result.error.message : "";
      if (result.detail) {
        const saved = saveOrderTicketAccess(access, result.detail.accessExpiresAt, result.tickets.map((ticket) => ticket.id));
        ticketAccessError = saved ? "" : "Akses tiket belum tersimpan. Kode tiket terlihat selama halaman ini terbuka, tetapi tautan e-ticket memerlukan penyimpanan browser.";
      }
      if (result.error) error = result.error.message;
      return Boolean(result.detail);
    } catch (cause) {
      if (!controller.signal.aborted) error = cause instanceof Error ? cause.message : "Pesanan belum dapat dimuat.";
      return false;
    } finally { if (!controller.signal.aborted) loading = false; }
  }

  async function checkPaymentStatus() {
    const access = getOrderAccess(id);
    if (!access || paymentBusy) return;
    paymentBusy = true;
    paymentCheckError = "";
    paymentMessage = "";
    try {
      detail = await getOrder(id, access.accessToken);
      updateCountdown();
      paymentMessage = `Status pembayaran diperbarui: ${buyerPaymentStatusLabel(detail.status, detail.payment?.status)}.`;
    } catch (cause) {
      paymentCheckError = cause instanceof Error ? cause.message : "Terjadi gangguan jaringan.";
      paymentMessage = "Status pembayaran terbaru belum dapat diperiksa.";
    } finally {
      paymentBusy = false;
      await tick();
      paymentResult?.focus();
    }
  }

  async function reloadTickets() {
    const access = getOrderAccess(id);
    if (!access || paymentBusy || detail?.status !== "PAID") return;
    paymentBusy = true;
    ticketError = "";
    try {
      const expected = detail.items.reduce((count, item) => count + item.quantity, 0);
      tickets = await listOrderTickets(id, access.accessToken);
      if (tickets.length !== expected) ticketError = "Sebagian e-ticket belum dapat dimuat.";
      const saved = saveOrderTicketAccess(access, detail.accessExpiresAt, tickets.map((ticket) => ticket.id));
      ticketAccessError = saved ? "" : "Akses tiket belum tersimpan. Kode tiket terlihat selama halaman ini terbuka, tetapi tautan e-ticket memerlukan penyimpanan browser.";
    } catch (cause) {
      ticketError = cause instanceof Error ? cause.message : "Daftar tiket belum dapat dimuat.";
    } finally { paymentBusy = false; }
  }

  async function checkRefundStatus() {
    if (refundBusy) return;
    refundBusy = true;
    refundMessage = "";
    try {
      if (!await load()) {
        refundMessage = "Status belum dapat diperiksa. Coba periksa status lagi.";
        return;
      }
      if (detail?.refundRight?.requested || detail?.refund) {
        refundNeedsReview = false;
        refundMessage = "Status pengajuan diperbarui.";
      } else {
        refundNeedsReview = false;
        refundConfirmed = false;
        refundMessage = "Belum ada pengajuan tercatat. Periksa kembali hak dan batas pengajuan sebelum mengajukan.";
      }
    } finally {
      refundBusy = false;
      await tick();
      refundResult?.focus();
    }
  }

  async function requestRefund() {
    const access = getOrderAccess(id);
    if (!access || refundBusy || refundNeedsReview || !refundConfirmed || !detail?.refundRight || detail.refundRight.requested || detail.refund) return;
    if (detail.refundRight.deadline && Date.parse(detail.refundRight.deadline) <= Date.now()) {
      refundMessage = "Tenggat pengajuan refund telah berakhir. Periksa status pesanan untuk memastikan tidak ada pengajuan yang sudah tercatat.";
      return;
    }
    refundBusy = true; refundMessage = "";
    try {
      const result = await requestEventRefund(id, access.accessToken);
      if (result.status !== "REQUESTED") throw new ApiError("Hasil pengajuan belum dapat dipastikan.", 0, "UNKNOWN_OUTCOME");
      refundNeedsReview = true;
      refundMessage = "Pengajuan diterima. Dana belum dinyatakan kembali; periksa status pengembalian pada halaman ini.";
      await load();
      if (detail?.refundRight?.requested || detail?.refund) refundNeedsReview = false;
    } catch (cause) {
      refundNeedsReview = true;
      refundMessage = cause instanceof ApiError && cause.code === "UNKNOWN_OUTCOME"
        ? "Hasil pengajuan belum dapat dipastikan. Periksa status pesanan sebelum mencoba lagi."
        : cause instanceof Error ? cause.message : "Hasil pengajuan belum dapat dipastikan. Periksa status pesanan sebelum mencoba lagi.";
    }
    finally { refundBusy = false; await tick(); refundResult?.focus(); }
  }

  onMount(() => {
    const desktopViewport = window.matchMedia("(min-width: 768px)");
    const syncViewport = () => { isDesktop = desktopViewport.matches; };
    syncViewport();
    desktopViewport.addEventListener("change", syncViewport);
    const countdownTimer = window.setInterval(updateCountdown, 1_000);
    document.addEventListener("visibilitychange", updateCountdown);
    emailToken = emailAccessToken(window.location.hash);
    if (window.location.hash) history.replaceState(history.state, "", window.location.pathname + window.location.search);
    void load();
    return () => { pageController?.abort(); desktopViewport.removeEventListener("change", syncViewport); window.clearInterval(countdownTimer); document.removeEventListener("visibilitychange", updateCountdown); };
  });

  async function resend() {
    const access = getOrderAccess(id);
    if (!access || sending) return;
    sending = true;
    resendMessage = "";
    resendError = "";
    try { resendMessage = (await resendOrderEmail(id, access.accessToken)).message; }
    catch (value) {
      resendError = value instanceof ApiError && value.status === 429
        ? `Email baru saja dijadwalkan. Coba lagi dalam ${value.retryAfter ?? 60} detik.`
        : value instanceof ApiError ? value.message : "Email belum dapat dijadwalkan. Coba lagi.";
    } finally { sending = false; }
  }
</script>

<svelte:head>
  <title>{detail ? `Pesanan ${detail.reference}` : "Pesanan"} | Tiket Online</title>
  <meta name="robots" content="noindex" />
</svelte:head>

<section class="my-tickets shell">
    <header class="my-tickets-heading order-heading">
    <p class="ticket-kicker">Detail pesanan</p>
      <h1>Pesananmu</h1>
      {#if detail}<p class="order-reference">Kode pesanan <strong>{detail.reference}</strong></p>{/if}
  </header>

  {#if loading}
    <p role="status">Memuat pesanan...</p>
  {:else if !detail}
    <div class="recovery-panel" role="alert"><p>{error || "Pesanan tidak dapat dimuat."}</p><a class="button" href="/pulihkan-tiket">Pulihkan tiket</a></div>
  {:else}
    <section class="recovery-panel" aria-labelledby="payment-status-title">
      <h2 id="payment-status-title">Status pesanan</h2>
      <p class="order-status order-status-{detail.status.toLowerCase()}">{buyerOrderStatusLabel(detail.status)}</p>
      {#if detail.payment}<p>{detail.status === "PENDING" && detail.payment.status === "SUCCEEDED" ? "Pembayaran diterima; pesanan menunggu konfirmasi." : `Pembayaran: ${buyerPaymentStatusLabel(detail.status, detail.payment.status)}.`}</p>{/if}
      {#if detail.status === "PENDING"}<p>Pembayaran {detail.payment?.status === "SUCCEEDED" ? "diterima; status pesanan masih menunggu konfirmasi." : "belum dikonfirmasi. Tiket tersedia setelah pembayaran dikonfirmasi."}</p><p class:reservation-warning={remainingSeconds <= 60} class="order-payment-deadline" role="timer">{remainingSeconds ? `Selesaikan pembayaran dalam ${String(Math.floor(remainingSeconds / 60)).padStart(2, "0")}:${String(remainingSeconds % 60).padStart(2, "0")}` : "Batas pembayaran lewat. Periksa status pesanan untuk memastikan hasilnya."}</p>{/if}
      {#if detail.status === "REFUND_PENDING"}<p>Tiket seluruh order tidak dapat digunakan selama pengembalian diperiksa.</p>{:else if detail.status === "REFUNDED"}<p>Pengembalian dana dikonfirmasi. Tiket pesanan ini sudah tidak berlaku.</p>{:else if detail.currentEvent?.status === "CANCELLED"}<p>Acara dibatalkan. Tiket tidak berlaku untuk masuk.</p>{/if}
      {#if detail.accessExpiresAt}<p>Akses pesanan berlaku sampai {eventDate(detail.accessExpiresAt)}.</p>{:else}<p>Akses dipertahankan selama perubahan acara atau pengembalian belum selesai.</p>{/if}
      {#if paymentCheckError}<p role="alert">Status pembayaran terbaru belum dapat diperiksa. {paymentCheckError} Status terakhir yang berhasil dibaca: {detail.status === "PENDING" && detail.payment?.status === "SUCCEEDED" ? "Pesanan masih menunggu konfirmasi." : buyerPaymentStatusLabel(detail.status, detail.payment?.status)}.</p>{/if}
      <p role="status" aria-live="polite" tabindex="-1" bind:this={paymentResult}>{paymentBusy ? "Memeriksa status pembayaran…" : paymentMessage}</p>
      {#if detail.status === "PENDING"}<button class="button" type="button" disabled={paymentBusy} onclick={checkPaymentStatus}>{paymentBusy ? "Memeriksa…" : "Periksa status pembayaran"}</button>{/if}
      {#if detail.status === "CANCELLED" || detail.status === "EXPIRED"}<a class="button" href="/konser">Cari tiket lagi</a>{/if}
    </section>
    <EventNotice state={detail.currentEvent} />
    <button class="text-button" type="button" onclick={() => load()} disabled={refundBusy}>Muat ulang informasi pesanan</button>
    {#if error}<p class="legacy-ticket-note" role="alert">{error}</p>{/if}
    <section class="recovery-panel order-ticket-section" aria-labelledby="order-tickets-title">
      <h2 id="order-tickets-title">Tiket</h2>
      {#if tickets.length}
      <ul class="my-order-list" aria-label="E-ticket pesanan">
        {#each tickets as ticket (ticket.id)}
          {@const persistent = hasPersistentTicketAccess(id, ticket.id)}
          <li><article class="my-order-card"><div><b>{ticket.attendeeName}</b><p>{ticket.tierName} · Gate {ticket.gate} · {ticket.code}</p>{#if detail.status === "REFUND_PENDING" || detail.status === "REFUNDED"}<small class="ticket-inactive">Tiket tidak dapat digunakan selama pengembalian dana.</small>{:else if detail.currentEvent?.status === "CANCELLED"}<small class="ticket-inactive">Acara dibatalkan; tiket tidak berlaku untuk masuk.</small>{:else if detail.currentEvent?.salesPaused}<small>Tiket dipertahankan sampai informasi acara tersedia.</small>{/if}</div>{#if persistent}<a class="button button-secondary" href={`/tiket/${encodeURIComponent(ticket.id)}`}>Buka e-ticket</a>{:else}<span>Kode tiket tersedia selama halaman ini terbuka.</span>{/if}</article></li>
        {/each}
      </ul>
      {:else if detail.status === "PAID"}<p>{ticketError || "Tiket belum dimuat."}</p><button class="button button-secondary" type="button" disabled={paymentBusy} onclick={reloadTickets}>{paymentBusy ? "Memuat…" : "Muat ulang tiket"}</button>
      {:else if detail.status === "PENDING"}<p>Tiket akan tersedia setelah pembayaran dikonfirmasi.</p>
      {:else}<p>Tidak ada tiket yang dapat digunakan untuk pesanan ini.</p>{/if}
      {#if ticketAccessError}<p class="legacy-ticket-note" role="status">{ticketAccessError}</p>{/if}
      {#if detail.status === "PAID" && detail.currentEvent?.startsAt !== null && !detail.refundRight?.requested}
        <div class="order-resend"><h3>Kirim ulang email</h3><p>Email tiket dikirim ke alamat pembeli yang tersimpan pada pesanan.</p><button class="button" type="button" disabled={sending} aria-busy={sending} onclick={resend}>{sending ? "Menjadwalkan..." : "Kirim ulang email"}</button><p role="status" aria-live="polite">{resendMessage}</p><p class="form-error" aria-live="polite">{resendError}</p></div>
      {/if}
    </section>
    <section class="recovery-panel" aria-labelledby="refund-right-title">
      <h2 id="refund-right-title">Hak refund</h2>
      {#if detail.refundRight}<p>Anda berhak meminta refund penuh untuk seluruh order: {formatRupiah.format(detail.total)}, termasuk biaya admin sebesar {formatRupiah.format(detail.adminFee)}.</p>
      {:else if detail.refund}<p>Refund untuk order ini sudah ditangani backend dengan nominal {formatRupiah.format(detail.refund.amount)}.</p>
      {:else}<p>Hak refund perubahan acara belum tersedia untuk order ini.</p>{/if}
      <p>Setelah pengajuan tercatat, tiket seluruh order tidak dapat digunakan. Pengajuan belum berarti dana sudah kembali.</p>
      {#if detail.refundRight && !detail.refundRight.requested && !detail.refund && !refundNeedsReview}
        <label><input type="checkbox" bind:checked={refundConfirmed} disabled={refundBusy} /> Saya memahami refund mencakup seluruh order dan semua tiket akan dinonaktifkan.</label>
        <button class="button" type="button" disabled={refundBusy || !refundConfirmed || Boolean(detail.refundRight.deadline && Date.parse(detail.refundRight.deadline) <= Date.now())} onclick={requestRefund}>{refundBusy ? "Mengajukan…" : "Ajukan refund penuh"}</button>
      {/if}
    </section>
    <section class="recovery-panel" aria-labelledby="refund-deadline-title">
      <h2 id="refund-deadline-title">Batas pengajuan</h2>
      {#if refundNeedsReview}<p>Hasil pengajuan sedang diperiksa; jangan mengajukan lagi sebelum status dipastikan.</p>
      {:else if detail.refundRight?.requested || detail.refund}<p>Pengajuan sudah tercatat; tenggat pengajuan tidak lagi berlaku.</p>
      {:else if detail.refundRight && !detail.refundRight.deadline}<p>Tanpa tenggat selama penundaan acara.</p>
      {:else if detail.refundRight?.deadline && Date.parse(detail.refundRight.deadline) > Date.now()}<p>Ajukan sebelum {eventDate(detail.refundRight.deadline)}.</p>
      {:else if detail.refundRight?.deadline}<p>Tenggat berakhir pada {eventDate(detail.refundRight.deadline)}.</p>
      {:else}<p>Tidak ada tenggat refund untuk order ini.</p>{/if}
    </section>
    {#if detail.refundRight || detail.refund || detail.status === "REFUND_PENDING" || detail.status === "REFUNDED"}
      <section class="recovery-panel" aria-labelledby="refund-status-title">
        <h2 id="refund-status-title">Status pengembalian dana</h2>
        {#if refundNeedsReview}
          <p>Hasil pengajuan belum dipastikan. Periksa status dari backend sebelum mencoba mengajukan lagi.</p>
          <button class="button button-secondary" type="button" disabled={refundBusy} onclick={checkRefundStatus}>{refundBusy ? "Memeriksa…" : "Periksa status"}</button>
        {:else if detail.refund || detail.refundRight?.requested || detail.status === "REFUND_PENDING" || detail.status === "REFUNDED"}
          <p>{refundStatusLabel(detail.refund?.status ?? (detail.status === "REFUNDED" ? "SUCCEEDED" : detail.status === "REFUND_PENDING" ? "PROCESSING" : undefined), detail.refundRight?.requested)}</p>
          <p>Nominal pengembalian: {formatRupiah.format(detail.refund?.amount ?? detail.total)}.</p>
          {#if detail.refund?.status === "MANUAL_REQUIRED" || detail.refund?.status === "FAILED"}<p>Tunggu penanganan tim untuk pembaruan berikutnya.</p>{/if}
          {#if detail.refund?.status !== "SUCCEEDED" && detail.status !== "REFUNDED"}
            <button class="button button-secondary" type="button" disabled={refundBusy} onclick={checkRefundStatus}>{refundBusy ? "Memeriksa…" : "Periksa status"}</button>
          {/if}
        {:else}<p>Pengajuan refund belum tercatat.</p>{/if}
      </section>
    {/if}
    <p role="status" aria-live="polite" tabindex="-1" bind:this={refundResult}>{refundMessage}</p>
    <details class="recovery-panel order-breakdown" open={isDesktop}>
      <summary><span>Rincian pesanan</span><strong>Total {formatRupiah.format(detail.total)}</strong></summary>
      <ul class="recovery-codes" aria-label="Rincian item pesanan">
        {#each detail.items as item (item.tierId)}<li><span>{item.name} × {item.quantity}</span><span>{formatRupiah.format(item.lineTotal)}</span></li>{/each}
        <li><span>Subtotal</span><span>{formatRupiah.format(detail.subtotal)}</span></li>
        <li><span>Biaya admin</span><span>{formatRupiah.format(detail.adminFee)}</span></li>
        {#if detail.discount}<li><span>Diskon</span><span>-{formatRupiah.format(detail.discount)}</span></li>{/if}
        <li><b>Total</b><b>{formatRupiah.format(detail.total)}</b></li>
      </ul>
    </details>
  {/if}
  <p class="recovery-footer"><a class="text-button" href="/tiket-saya">Kembali ke Tiket Saya</a></p>
</section>
