<script lang="ts">
  import { eventDate, type EventState } from "../lib/concerts.ts";
  import { eventStatus } from "../lib/event-changes.ts";
  let { state }: { state?: EventState } = $props();
</script>

{#if state && state.version > 0}
  <section class="event-notice" aria-label="Informasi terbaru acara" role="status">
    <h2>{eventStatus(state)}</h2>
    <p>{state.announcement}</p>
    {#if state.status === "RESCHEDULED"}<p>Jadwal terbaru: <strong>{eventDate(state.startsAt)}</strong>. QR tetap sama; tiket yang sudah dipakai tetap terpakai.</p>
    {:else if state.status === "POSTPONED"}<p>Jadwal pengganti belum diumumkan. Penjualan dan check-in dihentikan sementara.</p>
    {:else if state.status === "CANCELLED"}<p>Tiket tidak berlaku untuk masuk. Semua order lunas berhak refund penuh.</p>{/if}
    {#if state.salesPaused && state.status === "RESCHEDULED"}<p>Penjualan akan dibuka setelah transaksi lama selesai ditangani.</p>{/if}
  </section>
{/if}

<style>
  .event-notice { border: 1px solid currentColor; border-radius: .5rem; padding: 1rem; margin-block: 1rem; }
  h2 { font-size: 1.1rem; margin: 0 0 .5rem; }
  p { margin: .5rem 0; }
</style>
