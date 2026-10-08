<script lang="ts">
  import { onMount } from "svelte";
  import { eventDate } from "../../lib/concerts.ts";
  import Countdown from "../../components/ticket/Countdown.svelte";
  import { ApiError, getTicket, type ApiTicket } from "../../lib/api.ts";
  import { getOrderAccessForTicket } from "../../lib/order-access.ts";
  import { emailAccessToken, getEmailTicketAccess, saveEmailTicketAccess } from "../../lib/ticket-email-access.ts";
  import { ticketQrSource } from "../../lib/ticket-qr.ts";
  import Toast from "../../components/Toast.svelte";

  let { id }: { id: string } = $props();
  let ticket = $state<ApiTicket | null>(null);
  let loading = $state(true);
  let error = $state<unknown>(null);
  let retrying = $state(false);
  let pendingEmailToken: string | null = null;
  let toast = $state<{ id: number; message: string; tone: "success" | "error" } | null>(null);
  let toastId = 0;
  let controller: AbortController | undefined;
  const qrSource = $derived(ticketQrSource(ticket?.code));
  const errorMessage = $derived(error instanceof ApiError && error.code === "ACCESS_TOKEN_EXPIRED"
    ? "Akses tiket pada browser ini sudah kedaluwarsa."
    : error instanceof ApiError && error.code === "TICKET_NOT_FOUND"
      ? "E-ticket tidak ditemukan."
      : error instanceof ApiError && error.status === 404
        ? "Order atau e-ticket tidak ditemukan. Periksa akses yang tersimpan di browser ini."
        : error instanceof ApiError && error.status === 400
          ? "ID tiket tidak valid."
          : error instanceof ApiError && error.status === 0
            ? "Server belum dapat dihubungi. Periksa koneksi, lalu coba lagi."
            : error instanceof Error ? error.message : "E-ticket belum dapat dimuat.");

  async function load() {
    controller?.abort();
    controller = new AbortController();
    loading = true;
    error = null;
    const access = getOrderAccessForTicket(id);
    const emailToken = pendingEmailToken ?? getEmailTicketAccess(id);
    const token = emailToken ?? access?.accessToken;
    if (!token) {
      error = new Error("Akses privat tiket tidak ditemukan di browser ini.");
      loading = false;
      return;
    }
    try {
      ticket = await getTicket(id, token, controller.signal);
      if (emailToken) {
        saveEmailTicketAccess(id, emailToken);
        pendingEmailToken = null;
      }
    }
    catch (value) { if (!controller.signal.aborted) error = value; }
    finally { if (!controller.signal.aborted) loading = false; }
  }

  onMount(() => {
    pendingEmailToken = emailAccessToken(window.location.hash);
    if (window.location.hash) history.replaceState(history.state, "", window.location.pathname + window.location.search);
    void load();
    return () => controller?.abort();
  });

  async function copyCode() {
    if (!ticket) return;
    try {
      await navigator.clipboard.writeText(ticket.code);
      toast = { id: ++toastId, message: "Kode e-ticket disalin.", tone: "success" };
    } catch { toast = { id: ++toastId, message: "Kode tidak dapat disalin otomatis. Pilih dan salin kodenya secara manual.", tone: "error" }; }
  }
</script>

<svelte:head><title>{ticket ? `${ticket.eventArtist} | E-Ticket Tiket Online` : "E-ticket | Tiket Online"}</title><meta name="description" content={ticket ? `E-ticket digital untuk konser ${ticket.eventArtist}.` : "E-ticket digital Tiket Online."} /><meta name="robots" content="noindex" /></svelte:head>
{#if loading}
  <p class="shell" role="status">Memuat e-ticket...</p>
{:else if !ticket}
  <section class="ticket-missing shell"><p class="ticket-kicker">E-ticket belum tersedia</p><h1>Tiket tidak dapat dimuat.</h1><p role="status">{errorMessage}</p>{#if error instanceof ApiError || error instanceof Error}<button class="button" type="button" disabled={retrying} onclick={async () => { retrying = true; await load(); retrying = false; }}>{retrying ? "Memuat..." : "Coba lagi"}</button>{/if}<a class="button button-secondary" href="/tiket-saya">Buka Tiket Saya</a></section>
{:else}
  <section class="ticket-page shell">
    <header><p class="ticket-kicker">E-ticket digital</p><h1>Tiketmu sudah siap.</h1><p>Tunjukkan kode e-ticket ini kepada petugas di gate yang tertera.</p></header>
    <article class="boarding-pass" aria-labelledby="ticket-title">
      <section class="pass-main"><div class="pass-top"><span class="pass-label">Tiket Online</span><span class="pass-status">E-ticket</span></div><h2 id="ticket-title">{ticket.eventArtist}</h2><p class="pass-venue">{ticket.eventVenue}, {ticket.eventCity}</p>
        <div class="pass-grid"><div class="pass-row"><span>Pengunjung</span><b>{ticket.attendeeName}</b></div><div class="pass-row"><span>Tanggal event</span><b>{eventDate(ticket.eventStartsAt)}</b></div><div class="pass-row"><span>Lokasi</span><b>{ticket.eventVenue}, {ticket.eventCity}</b></div><div class="pass-row"><span>Alamat</span><b>{ticket.eventAddress}</b></div><div class="pass-row"><span>Jenis tiket</span><b>{ticket.tierName}</b></div><div class="pass-row"><span>Gate</span><b>{ticket.gate}</b></div><div class="pass-row"><span>Reference pesanan</span><b>{ticket.orderReference}</b></div><div class="pass-row"><span>Diterbitkan</span><b>{eventDate(ticket.issuedAt)}</b></div></div>
      </section>
      <aside class="pass-stub"><div><span class="pass-label">Kode e-ticket</span>{#if qrSource}<img class="qr-code" src={qrSource} alt={`QR code untuk kode e-ticket ${ticket.code}`} />{:else}<p role="status">QR tidak tersedia. Gunakan kode e-ticket di bawah.</p>{/if}<p class="ticket-code">{ticket.code}</p><button class="button button-secondary" type="button" onclick={copyCode}>Salin kode</button></div><Countdown startsAt={ticket.eventStartsAt} /></aside>
    </article>
    <div class="ticket-actions"><button class="button" type="button" onclick={() => window.print()}>Cetak / Simpan PDF</button><a class="button button-secondary" href="/tiket-saya">Tiket Saya</a></div>
    <p class="ticket-caption">Simpan email ini untuk membuka e-ticket kembali. Tunjukkan QR atau kode e-ticket kepada petugas di gate.</p>
    {#if toast}<Toast id={toast.id} message={toast.message} tone={toast.tone} onDismiss={() => toast = null} />{/if}
  </section>
{/if}
