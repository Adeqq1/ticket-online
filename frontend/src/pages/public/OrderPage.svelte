<script lang="ts">
  import EventNotice from "../../components/EventNotice.svelte";
  import { emailAccessToken } from "../../lib/ticket-email-access.ts";
  import { onMount, tick } from "svelte";
  import { ApiError, getOrder, requestEventRefund, resendOrderEmail, type ApiTicket, type OrderDetail } from "../../lib/api.ts";
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
  let refundResult = $state<HTMLParagraphElement>();
  let pageController: AbortController | undefined;
  let emailToken: string | null = null;

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
      if (!access) { error = "Akses pesanan tidak tersimpan. Pulihkan akses dengan email dan reference pesanan."; return; }
      const result = await loadBuyerOrderTickets(access, controller.signal);
      if (controller.signal.aborted) return;
      detail = result.detail; tickets = result.tickets;
      if (result.detail) saveOrderAccess({ ...access, accessExpiresAt: result.detail.accessExpiresAt, ticketIds: [...new Set([...access.ticketIds, ...result.tickets.map((ticket) => ticket.id)])] });
      if (result.error) error = result.error.message;
    } catch (cause) {
      if (!controller.signal.aborted) error = cause instanceof Error ? cause.message : "Pesanan belum dapat dimuat.";
    } finally { if (!controller.signal.aborted) loading = false; }
  }

  async function requestRefund() {
    const access = getOrderAccess(id);
    if (!access || refundBusy || !refundConfirmed) return;
    refundBusy = true; refundMessage = "";
    try {
      await requestEventRefund(id, access.accessToken);
      refundMessage = "Permintaan refund penuh diterima. Periksa halaman ini untuk progresnya.";
      await load();
    } catch (cause) { refundMessage = cause instanceof Error ? cause.message : "Hasil permintaan belum pasti. Muat ulang pesanan sebelum mencoba lagi."; }
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
    {#if detail}<p>{detail.status === "PAID" ? "Pembayaran dikonfirmasi." : detail.status === "REFUND_PENDING" ? "Refund sedang diproses. Tiket tidak dapat digunakan selama pemeriksaan." : detail.status === "REFUNDED" ? "Refund berhasil. Tiket pesanan ini sudah tidak berlaku." : "Pesanan belum lunas."} {detail.accessExpiresAt ? `Akses berlaku sampai ${eventDate(detail.accessExpiresAt)}.` : "Akses tetap tersedia selama perubahan acara atau refund belum selesai."}</p>{/if}
  </header>

  {#if loading}
    <p role="status">Memuat pesanan...</p>
  {:else if !detail}
    <div class="recovery-panel" role="alert"><p>{error || "Pesanan tidak dapat dimuat."}</p><a class="button" href="/pulihkan-tiket">Pulihkan tiket</a></div>
  {:else}
    {#if error}<p class="legacy-ticket-note" role="alert">{error}</p>{/if}
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
    {#if detail.status === "REFUND_PENDING" || detail.status === "REFUNDED"}
      <div class="recovery-panel" role="status"><h2>{detail.status === "REFUNDED" ? "Refund selesai" : "Refund sedang diproses"}</h2><p>{detail.status === "REFUNDED" ? "E-ticket tidak lagi dapat digunakan." : "Pengembalian sedang ditangani. Periksa kembali halaman ini untuk pembaruan."}</p></div>
    {/if}
    {#if detail.refundRight}
     <section class="recovery-panel" aria-label="Hak refund perubahan acara">
      <h2>Refund penuh perubahan acara</h2>
      <p>Nominal: {formatRupiah.format(detail.total)}, termasuk biaya admin.</p>
      {#if detail.refundRight.requested}<p>Permintaan sudah tercatat. {detail.refund?.status === "MANUAL_REQUIRED" ? "Tim sedang menangani pengembalian melalui kanal manual." : detail.refund?.status === "SUCCEEDED" ? "Pengembalian berhasil." : "Pengembalian sedang diproses."}</p>
      {:else if !detail.refundRight.deadline || Date.parse(detail.refundRight.deadline)>Date.now()}
       <p>{detail.refundRight.deadline ? `Ajukan sebelum ${eventDate(detail.refundRight.deadline)}.` : "Permintaan tersedia selama jadwal pengganti belum ditetapkan."}</p>
       <label><input type="checkbox" bind:checked={refundConfirmed} disabled={refundBusy} /> Saya memilih refund seluruh order; tiket akan dinonaktifkan.</label>
       <button class="button" type="button" disabled={refundBusy || !refundConfirmed} onclick={requestRefund}>{refundBusy ? "Mengajukan…" : "Ajukan refund penuh"}</button>
      {:else}<p>Tenggat permintaan refund telah berakhir.</p>{/if}
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
