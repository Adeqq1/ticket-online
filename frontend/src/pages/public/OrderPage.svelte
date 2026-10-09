<script lang="ts">
  import EventNotice from "../../components/EventNotice.svelte";
  import { emailAccessToken } from "../../lib/ticket-email-access.ts";
  import { onMount, tick } from "svelte";
  import { ApiError, getOrder, listOrderTickets, requestEventRefund, resendOrderEmail, type ApiTicket, type OrderDetail } from "../../lib/api.ts";
  import { buyerPaymentStatusLabel } from "../../lib/buyer-payment.ts";
  import { refundStatusLabel } from "../../lib/buyer-refund.ts";
  import { loadBuyerOrderTickets } from "../../lib/buyer-tickets.ts";
  import { eventDate, formatRupiah } from "../../lib/concerts.ts";
  import { getOrderAccess, saveOrderAccess } from "../../lib/order-access.ts";

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
  let paymentResult = $state<HTMLParagraphElement>();

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
      if (result.detail) detail = result.detail;
      tickets = result.tickets;
      ticketError = result.detail && result.error ? result.error.message : "";
      if (result.detail) saveOrderAccess({ ...access, accessExpiresAt: result.detail.accessExpiresAt, ticketIds: [...new Set([...access.ticketIds, ...result.tickets.map((ticket) => ticket.id)])] });
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
    emailToken = emailAccessToken(window.location.hash);
    if (window.location.hash) history.replaceState(history.state, "", window.location.pathname + window.location.search);
    void load();
    return () => pageController?.abort();
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
  <header class="my-tickets-heading">
    <p class="ticket-kicker">Detail pesanan</p>
    <h1>{detail ? detail.reference : "Pesanan"}</h1>
    {#if detail}<p>{detail.status === "REFUND_PENDING" ? "Pengembalian belum dikonfirmasi. Tiket tidak dapat digunakan selama pemeriksaan." : detail.status === "REFUNDED" ? "Pengembalian dana dikonfirmasi. Tiket pesanan ini sudah tidak berlaku." : buyerPaymentStatusLabel(detail.status, detail.payment?.status)} {detail.accessExpiresAt ? `Akses berlaku sampai ${eventDate(detail.accessExpiresAt)}.` : "Akses tetap tersedia selama perubahan acara atau pengembalian belum selesai."}</p>{/if}
  </header>

  {#if loading}
    <p role="status">Memuat pesanan...</p>
  {:else if !detail}
    <div class="recovery-panel" role="alert"><p>{error || "Pesanan tidak dapat dimuat."}</p><a class="button" href="/pulihkan-tiket">Pulihkan tiket</a></div>
  {:else}
    {#if error}<p class="legacy-ticket-note" role="alert">{error}</p>{/if}
    <section class="recovery-panel" aria-labelledby="payment-status-title">
      <h2 id="payment-status-title">Status pembayaran</h2>
      <p>{buyerPaymentStatusLabel(detail.status, detail.payment?.status)}</p>
      {#if buyerPaymentStatusLabel(detail.status, detail.payment?.status) === "Menunggu konfirmasi pembayaran"}<p>Pembayaran belum dikonfirmasi. Tiket tersedia setelah pembayaran dikonfirmasi.</p>{/if}
      {#if paymentCheckError}<p role="alert">Status pembayaran terbaru belum dapat diperiksa. {paymentCheckError} Status terakhir yang berhasil dibaca: {buyerPaymentStatusLabel(detail.status, detail.payment?.status)}.</p>{/if}
      <p role="status" aria-live="polite" tabindex="-1" bind:this={paymentResult}>{paymentBusy ? "Memeriksa status pembayaran…" : paymentMessage}</p>
      {#if detail.status === "PENDING"}<button class="button" type="button" disabled={paymentBusy} onclick={checkPaymentStatus}>{paymentBusy ? "Memeriksa…" : "Periksa status pembayaran"}</button>{/if}
    </section>
    <EventNotice state={detail.currentEvent} />
    <button class="text-button" type="button" onclick={() => load()} disabled={refundBusy}>Muat ulang informasi pesanan</button>
    <div class="recovery-panel">
      <h2>Ringkasan pesanan</h2>
      <ul class="recovery-codes" aria-label="Rincian pesanan">
        {#each detail.items as item (item.tierId)}<li><span>{item.name} × {item.quantity}</span><span>{formatRupiah.format(item.lineTotal)}</span></li>{/each}
        <li><b>Total</b><b>{formatRupiah.format(detail.total)}</b></li>
      </ul>
    </div>
    {#if tickets.length}
      <ul class="my-order-list" aria-label="E-ticket pesanan">
        {#each tickets as ticket (ticket.id)}
          <li><article class="my-order-card"><div><b>{ticket.attendeeName}</b><p>{ticket.tierName} · Gate {ticket.gate} · {ticket.code}</p></div><a class="button button-secondary" href={`/tiket/${encodeURIComponent(ticket.id)}`}>Buka e-ticket</a></article></li>
        {/each}
      </ul>
    {/if}
    {#if detail.status === "PAID" && (!tickets.length || ticketError)}<section class="recovery-panel"><h2>Tiket</h2><p>{ticketError || "Tiket belum dimuat."}</p><button class="button button-secondary" type="button" disabled={paymentBusy} onclick={reloadTickets}>{paymentBusy ? "Memuat…" : "Muat ulang tiket"}</button></section>{/if}
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
    {#if detail.status === "PAID" && detail.currentEvent?.startsAt !== null && !detail.refundRight?.requested}
      <div class="recovery-panel">
        <h2>Kirim ulang email</h2>
        <p>Email tiket dikirim ke alamat pembeli yang tersimpan pada pesanan.</p>
        <button class="button" type="button" disabled={sending} aria-busy={sending} onclick={resend}>{sending ? "Menjadwalkan..." : "Kirim ulang email"}</button>
        <p role="status" aria-live="polite">{resendMessage}</p>
        <p class="form-error" aria-live="polite">{resendError}</p>
      </div>
    {/if}
  {/if}
  <p class="recovery-footer"><a class="text-button" href="/tiket-saya">Kembali ke Tiket Saya</a></p>
</section>
