<script lang="ts">
  import { onMount } from "svelte";
  import { eventDate } from "../lib/concerts.ts";
  import DemoCodeCanvas from "../components/ticket/DemoCodeCanvas.svelte";
  import Countdown from "../components/ticket/Countdown.svelte";
  import { loadTicketSnapshot } from "../lib/tickets.ts";
  import { getTicket, ApiError, type ApiTicket } from "../lib/api.ts";
  import { getOrderAccessForTicket } from "../lib/order-access.ts";
  import Toast from "../components/Toast.svelte";
  let { id }: { id: string } = $props();
  const cachedTicket = $derived(loadTicketSnapshot(id));
  let ticket = $state<ApiTicket | null>(null);
  let loading = $state(true);
  let apiError = $state("");
  const artist = $derived(ticket?.eventArtist ?? cachedTicket?.concert.artist ?? "");
  const venue = $derived(ticket?.eventVenue ?? cachedTicket?.concert.venue ?? "");
  const city = $derived(ticket?.eventCity ?? cachedTicket?.concert.city ?? "");
  const address = $derived(ticket?.eventAddress ?? cachedTicket?.concert.address ?? "");
  const startsAt = $derived(ticket?.eventStartsAt ?? cachedTicket?.concert.startsAt ?? "");
  const reference = $derived(ticket?.orderReference ?? cachedTicket?.reference ?? "");
  const attendee = $derived(ticket?.attendeeName ?? cachedTicket?.attendeeName ?? "");
  const tierName = $derived(ticket?.tierName ?? cachedTicket?.lines[0]?.tierName ?? "");
  const gate = $derived(ticket?.gate ?? cachedTicket?.lines[0]?.gate ?? "");
  onMount(() => {
    const controller = new AbortController();
    const access = getOrderAccessForTicket(id);
    if (!access) { apiError = "Akses privat tiket tidak ditemukan di browser ini."; loading = false; return () => controller.abort(); }
    getTicket(id, access.accessToken, controller.signal).then((result) => { ticket = result; }).catch((error) => { if (!controller.signal.aborted) apiError = error instanceof ApiError ? error.message : "E-ticket belum dapat dimuat."; }).finally(() => { if (!controller.signal.aborted) loading = false; });
    return () => controller.abort();
  });
  let status = $state("Kode dan QR pada halaman ini adalah visual demo, tidak dapat digunakan untuk masuk event.");
  let toast = $state<{ id: number; message: string; tone: "success" | "info" | "error" } | null>(null);
  let toastId = 0;
  async function copyTicketLink() {
    try { await navigator.clipboard.writeText(location.href); toast = { id: ++toastId, message: "Link tiket disalin. Tautan hanya dapat dibuka di browser ini.", tone: "success" }; }
    catch { toast = { id: ++toastId, message: "Link tiket tidak dapat disalin otomatis.", tone: "error" }; }
  }
</script>

<svelte:head><title>{artist ? `${artist} | E-Ticket Tiket Online` : "E-ticket | Tiket Online"}</title><meta name="description" content={artist ? `E-ticket digital untuk konser ${artist}.` : "E-ticket digital Tiket Online."} /><meta name="robots" content="noindex" /></svelte:head>
{#if loading}
  <p class="shell" role="status">Memuat e-ticket...</p>
{:else if !cachedTicket && !ticket}
  <section class="ticket-missing shell"><p class="ticket-kicker">E-ticket tidak tersedia</p><h1>Tiket tidak ditemukan.</h1><p>Tiket demo ini mungkin sudah dihapus dari penyimpanan browser.</p><a class="button" href="/tiket-saya">Buka dompet tiket</a></section>
{:else}
   <section class="ticket-page shell"><header><p class="ticket-kicker">E-ticket digital</p><h1>Tiketmu sudah siap.</h1><p>Gunakan reference di bawah sebagai bukti pembelian.</p></header>{#if apiError}<p class="success-note" role="status">{apiError} Menampilkan salinan yang tersimpan di browser.</p>{/if}<article class="boarding-pass" aria-labelledby="ticket-title"><section class="pass-main"><div class="pass-top"><span class="pass-label">Tiket Online</span><span class="pass-status">E-ticket</span></div><h2 id="ticket-title">{artist}</h2><p class="pass-venue">{venue}, {city}</p><div class="pass-grid"><div class="pass-row"><span>Pengunjung</span><b>{attendee}</b></div><div class="pass-row"><span>Tanggal event</span><b>{eventDate(startsAt)}</b></div><div class="pass-row"><span>Lokasi</span><b>{venue}, {city}</b></div><div class="pass-row"><span>Alamat</span><b>{address}</b></div></div><div class="ticket-lines"><div class="ticket-line"><span>{tierName}</span><b>{gate}</b></div></div></section><aside class="pass-stub"><div><DemoCodeCanvas reference={reference} /><div class="barcode" aria-hidden="true"></div><small>Reference</small><p class="ticket-reference">{reference}</p></div><Countdown startsAt={startsAt} /></aside></article><div class="ticket-actions"><button class="button" type="button" onclick={() => window.print()}>Cetak / Simpan PDF</button><button class="button button-secondary" type="button" onclick={copyTicketLink}>Salin link tiket</button><button class="button button-secondary" type="button" onclick={() => status = "Simulasi: e-ticket akan dikirim ke email pemesan."}>Kirim ke email</button></div><p class="ticket-caption" aria-live="polite">{status}</p>{#if toast}<Toast id={toast.id} message={toast.message} tone={toast.tone} onDismiss={() => toast = null} />{/if}</section>
{/if}
