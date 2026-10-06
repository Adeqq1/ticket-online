<script lang="ts">
  import { onMount } from "svelte";
  import { ApiError, resendOrderEmail, type ApiTicket, type OrderDetail } from "../lib/api.ts";
  import { loadBuyerOrderTickets } from "../lib/buyer-tickets.ts";
  import { eventDate, formatRupiah } from "../lib/concerts.ts";
  import { getOrderAccess, saveOrderAccess } from "../lib/order-access.ts";

  let { id }: { id: string } = $props();
  let detail = $state<OrderDetail | null>(null);
  let tickets = $state<ApiTicket[]>([]);
  let loading = $state(true);
  let error = $state("");
  let sending = $state(false);
  let resendMessage = $state("");
  let resendError = $state("");

  onMount(() => {
    const controller = new AbortController();
    const access = getOrderAccess(id);
    if (!access) {
      error = "Akses pesanan tidak tersimpan di browser ini. Pulihkan tiket dengan email dan reference pesanan.";
      loading = false;
      return;
    }
    loadBuyerOrderTickets(access, controller.signal).then((result) => {
      if (controller.signal.aborted) return;
      detail = result.detail;
      tickets = result.tickets;
      if (result.tickets.length) saveOrderAccess({ ...access, ticketIds: [...new Set([...access.ticketIds, ...result.tickets.map((ticket) => ticket.id)])] });
      if (result.error) error = result.error instanceof ApiError && result.error.code === "ACCESS_TOKEN_EXPIRED" ? "Akses pesanan ini sudah kedaluwarsa." : result.error.message;
    }).finally(() => { if (!controller.signal.aborted) loading = false; });
    return () => controller.abort();
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
    {#if detail}<p>{detail.status === "PAID" ? "Pembayaran dikonfirmasi." : detail.status === "REFUND_PENDING" ? "Refund sedang diproses. Tiket tidak dapat digunakan selama pemeriksaan." : detail.status === "REFUNDED" ? "Refund berhasil. Tiket pesanan ini sudah tidak berlaku." : "Pesanan belum lunas."} Akses berlaku sampai {eventDate(detail.accessExpiresAt)}.</p>{/if}
  </header>

  {#if loading}
    <p role="status">Memuat pesanan...</p>
  {:else if !detail}
    <div class="recovery-panel" role="alert"><p>{error || "Pesanan tidak dapat dimuat."}</p><a class="button" href="/pulihkan-tiket">Pulihkan tiket</a></div>
  {:else}
    {#if error}<p class="legacy-ticket-note" role="alert">{error}</p>{/if}
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
      <div class="recovery-panel" role="status"><h2>{detail.status === "REFUNDED" ? "Refund selesai" : "Refund sedang diproses"}</h2><p>{detail.status === "REFUNDED" ? "E-ticket tidak lagi dapat digunakan." : "Kami sedang mengonfirmasi refund dengan penyedia pembayaran. Periksa kembali halaman ini untuk pembaruan."}</p></div>
    {/if}
    {#if detail.status === "PAID"}
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
