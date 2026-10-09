<script lang="ts">
  import { tick } from "svelte";
  import { ApiError, commitEventChange, getEventChanges, previewEventChange, type AdminApiEvent, type EventChange, type EventChangeInput, type EventChangePreview } from "../../lib/api.ts";
  import { eventDate, formatRupiah } from "../../lib/concerts.ts";
  import { eventStatus, wibDateTime } from "../../lib/event-changes.ts";
  let { event, accessToken, onSaved, onUnauthorized }: { event: AdminApiEvent; accessToken: string; onSaved: () => Promise<void>; onUnauthorized: () => void } = $props();
  let action = $state<EventChangeInput["action"]>("POSTPONED");
  let reason = $state("");
  let announcement = $state("");
  let startsAt = $state("");
  let deadline = $state("");
  let preview = $state<EventChangePreview | null>(null);
  let input: EventChangeInput | null = null;
  let key = "";
  let confirmed = $state(false);
  let busy = $state(false);
  let uncertain = $state(false);
  let message = $state("");
  let history = $state<EventChange[]>([]);
  let historyError = $state("");
  let historyBusy = $state(false);
  let impactHeading = $state<HTMLHeadingElement>();
  let resultMessage = $state<HTMLParagraphElement>();
  $effect(() => {
    const id = event.id;
    const controller = new AbortController();
    history = []; historyError = ""; historyBusy = true;
    getEventChanges(accessToken, id, controller.signal).then((value) => { if (!controller.signal.aborted) history = value; }).catch((cause) => { if (!controller.signal.aborted) historyError = cause instanceof Error ? cause.message : "Riwayat belum dapat dimuat."; }).finally(() => { if (!controller.signal.aborted) historyBusy = false; });
    return () => controller.abort();
  });
  function edited() { if (!uncertain) { preview = null; input = null; confirmed = false; message = ""; } }
  function fail(cause: unknown) {
    if (cause instanceof ApiError && cause.status === 401) onUnauthorized();
    message = cause instanceof Error ? cause.message : "Perubahan belum dapat diproses.";
  }
  async function prepare(e: SubmitEvent) {
    e.preventDefault(); if (busy || uncertain) return;
    busy = true; message = ""; preview = null; confirmed = false;
    const draft: EventChangeInput = { action, reason: reason.trim(), announcement: announcement.trim(), startsAt: action === "RESCHEDULED" ? wibDateTime(startsAt) : null, refundDeadline: action === "RESCHEDULED" ? wibDateTime(deadline) : null, expectedVersion: event.currentEvent?.version ?? 0, snapshot: "" };
    try { preview = await previewEventChange(accessToken, event.id, draft); input = { ...draft, snapshot: preview.snapshot }; key = crypto.randomUUID(); }
    catch (cause) { fail(cause); }
    finally { busy = false; await tick(); (preview ? impactHeading : resultMessage)?.focus(); }
  }
  async function apply() {
    if (!input || !confirmed || busy) return;
    busy = true; message = "";
    let saved = false;
    try {
      await commitEventChange(accessToken, event.id, input, key);
      saved = true;
      uncertain = false; preview = null; input = null; confirmed = false;
      message = "Keputusan tersimpan. Reservasi, pembayaran, pemberitahuan, dan refund diproses bertahap.";
      await onSaved();
      history = await getEventChanges(accessToken, event.id);
    } catch (cause) {
      uncertain = !saved && cause instanceof ApiError && (cause.status === 0 || cause.status >= 500);
      if (!uncertain) { preview = null; input = null; confirmed = false; }
      fail(cause);
      if (saved) message = `Keputusan tersimpan; progres belum dapat dimuat. ${message}`;
    } finally { busy = false; await tick(); resultMessage?.focus(); }
  }
  async function refresh() {
    if (busy) return;
    busy = true;
    try { await onSaved(); history = await getEventChanges(accessToken, event.id); }
    catch (cause) { fail(cause); }
    finally { busy = false; }
  }
</script>

<section class="event-change-panel" aria-labelledby="event-change-title" aria-busy={busy}>
  <h2 id="event-change-title">Pembatalan dan perubahan jadwal</h2>
  <p>{eventStatus(event.currentEvent) || "Acara terjadwal"} · {eventDate(event.currentEvent?.startsAt ?? event.startsAt)}</p>
  <p>Refund penuh mencakup total order. QR tetap sama pada jadwal baru; check-in lama tetap tercatat dan tiket terpakai tidak dapat digunakan lagi.</p>
  {#if event.currentEvent?.status !== "CANCELLED"}
    <form class="staff-form" onsubmit={prepare} oninput={edited}>
      <fieldset disabled={busy || uncertain}>
        <label>Keputusan<select bind:value={action}><option value="POSTPONED">Tunda tanpa tanggal</option><option value="RESCHEDULED">Tetapkan jadwal baru</option><option value="CANCELLED">Batalkan acara</option></select></label>
        <label>Alasan untuk audit<textarea bind:value={reason} minlength="3" maxlength="500" required></textarea></label>
        <label>Pengumuman untuk pembeli<textarea bind:value={announcement} minlength="3" maxlength="2000" required></textarea></label>
        {#if action === "RESCHEDULED"}<div class="event-form-row"><label>Jadwal baru (WIB)<input type="datetime-local" bind:value={startsAt} required /></label><label>Tenggat permintaan refund (WIB)<input type="datetime-local" bind:value={deadline} required /></label></div><p>Tenggat harus sebelum jadwal baru dan tidak boleh memendekkan tenggat yang sudah diberikan.</p>{/if}
        <button class="staff-secondary-button" type="submit">{busy ? "Menyiapkan…" : "Pratinjau dampak"}</button>
      </fieldset>
    </form>
  {/if}
  {#if preview}
    <div class="impact" aria-label="Pratinjau dampak keputusan">
      <h3 tabindex="-1" bind:this={impactHeading}>Dampak keputusan</h3>
      <dl><dt>Reservasi aktif</dt><dd>{preview.impact.activeReservations}</dd><dt>Order belum lunas</dt><dd>{preview.impact.pendingOrders}</dd><dt>Pembayaran berjalan</dt><dd>{preview.impact.pendingPayments}</dd><dt>Order lunas</dt><dd>{preview.impact.paidOrders}</dd><dt>Tiket terbit / check-in</dt><dd>{preview.impact.issuedTickets} / {preview.impact.checkIns}</dd><dt>Refund sudah tercatat</dt><dd>{preview.impact.existingRefunds}</dd><dt>Estimasi melalui provider</dt><dd>{formatRupiah.format(preview.impact.automaticAmount)}</dd><dt>Estimasi penanganan manual</dt><dd>{formatRupiah.format(preview.impact.manualAmount)}</dd></dl>
      <p>Nilai di atas adalah potensi refund. Pembatalan memproses semuanya; penundaan/perubahan jadwal menunggu permintaan pembeli. Kemampuan provider diperiksa kembali saat diproses.</p>
      <label class="confirm"><input type="checkbox" bind:checked={confirmed} disabled={busy || uncertain} /> Saya sudah memeriksa dampak dan pengumuman pembeli.</label>
      <button class="scan-submit" type="button" disabled={busy || !confirmed} onclick={apply}>{busy ? "Menyimpan…" : uncertain ? "Periksa dan ulangi keputusan yang sama" : "Terapkan keputusan"}</button>
      {#if uncertain}<p role="alert">Hasil penyimpanan belum pasti. Ulangi dengan keputusan yang sama untuk memeriksa hasil tanpa menggandakan tindakan.</p>{/if}
    </div>
  {/if}
  <p role="status" aria-live="polite" tabindex="-1" bind:this={resultMessage}>{message}</p>
  <div class="staff-panel-heading"><h3>Riwayat keputusan</h3><button class="staff-text-button" type="button" disabled={busy || uncertain} onclick={refresh}>Muat ulang progres</button></div>
  {#if historyBusy}<p role="status">Memuat riwayat…</p>{/if}
  {#if historyError}<p role="alert">{historyError}</p>{/if}
  {#each history as change (change.id)}
    <article class="change-history"><h4>#{change.version} · {change.action}</h4><p>{eventDate(change.createdAt)} · Admin {change.staffId}</p><p>{change.reason}</p><p>{change.announcement}</p>{#if change.startsAt}<p>Jadwal pada keputusan ini: {eventDate(change.startsAt)}</p>{/if}<p>Order menunggu penanganan: {change.remainingOrders} · Gangguan: {change.errors}</p></article>
  {/each}
</section>

<style>
  .event-change-panel { margin-block: 2rem; padding-top: 1.5rem; border-top: 1px solid currentColor; }
  fieldset { padding: 0; border: 0; display: grid; gap: 1rem; }
  .impact, .change-history { padding: 1rem; margin-block: 1rem; border: 1px solid currentColor; border-radius: .5rem; }
  dl { display: grid; grid-template-columns: 1fr auto; gap: .5rem 1rem; }
  dd { margin: 0; text-align: right; }
  .confirm { display: flex; align-items: start; gap: .5rem; margin-bottom: 1rem; }
  .confirm input { width: auto; }
</style>
