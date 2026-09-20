<script lang="ts">
  import { eventDate } from "../lib/concerts.ts";
  import DemoCodeCanvas from "../components/ticket/DemoCodeCanvas.svelte";
  import Countdown from "../components/ticket/Countdown.svelte";
  import { loadTicketSnapshot } from "../lib/tickets.ts";
  import Toast from "../components/Toast.svelte";
  let { id }: { id: string } = $props();
  const ticket = $derived(loadTicketSnapshot(id));
  let status = $state("Kode dan QR pada halaman ini adalah visual demo, tidak dapat digunakan untuk masuk event.");
  let toast = $state<{ message: string; tone: "success" | "info" | "error" } | null>(null);
  async function copyTicketLink() {
    try { await navigator.clipboard.writeText(location.href); toast = { message: "Link tiket disalin. Tautan hanya dapat dibuka di browser ini.", tone: "success" }; }
    catch { toast = { message: "Link tiket tidak dapat disalin otomatis.", tone: "error" }; }
  }
</script>

<svelte:head><title>{ticket ? `${ticket.concert.artist} | E-Ticket Tiket Online` : "E-ticket tidak tersedia | Tiket Online"}</title><meta name="description" content={ticket ? `E-ticket digital untuk konser ${ticket.concert.artist}.` : "E-ticket digital Tiket Online."} /><meta name="robots" content="noindex" /></svelte:head>
{#if !ticket}
  <section class="ticket-missing shell"><p class="ticket-kicker">E-ticket tidak tersedia</p><h1>Tiket tidak ditemukan.</h1><p>Tiket demo ini mungkin sudah dihapus dari penyimpanan browser.</p><a class="button" href="/tiket-saya">Buka dompet tiket</a></section>
{:else}
  <section class="ticket-page shell"><header><p class="ticket-kicker">E-ticket digital</p><h1>Tiketmu sudah siap.</h1><p>Gunakan reference di bawah sebagai bukti pembelian demo.</p></header><article class="boarding-pass" aria-labelledby="ticket-title"><section class="pass-main"><div class="pass-top"><span class="pass-label">Tiket Online</span><span class="pass-status">E-ticket</span></div><h2 id="ticket-title">{ticket.concert.artist}</h2><p class="pass-venue">{ticket.concert.venue}, {ticket.concert.city}</p><div class="pass-grid"><div class="pass-row"><span>Pengunjung</span><b>{ticket.attendeeName}</b></div><div class="pass-row"><span>Tanggal event</span><b>{eventDate(ticket.concert.startsAt)}</b></div><div class="pass-row"><span>Lokasi</span><b>{ticket.concert.venue}, {ticket.concert.city}</b></div><div class="pass-row"><span>Alamat</span><b>{ticket.concert.address}</b></div></div><div class="ticket-lines">{#each ticket.lines as line}<div class="ticket-line"><span>{line.quantity}x {line.tierName}</span><b>{line.gate}</b><p>{line.allocation}</p></div>{/each}</div></section><aside class="pass-stub"><div><DemoCodeCanvas reference={ticket.reference} /><div class="barcode" aria-hidden="true"></div><small>Reference demo</small><p class="ticket-reference">{ticket.reference}</p></div><Countdown startsAt={ticket.concert.startsAt} /></aside></article><div class="ticket-actions"><button class="button" type="button" onclick={() => window.print()}>Cetak / Simpan PDF</button><button class="button button-secondary" type="button" onclick={copyTicketLink}>Salin link tiket</button><button class="button button-secondary" type="button" onclick={() => status = "Simulasi: e-ticket akan dikirim ke email pemesan."}>Kirim ke email</button></div><p class="ticket-caption" aria-live="polite">{status}</p>{#if toast}<Toast message={toast.message} tone={toast.tone} onDismiss={() => toast = null} />{/if}</section>
{/if}
