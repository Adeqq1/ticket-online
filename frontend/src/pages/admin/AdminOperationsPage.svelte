<script lang="ts">
  import AdminLayout from "../../layouts/AdminLayout.svelte";
  import { onDestroy, onMount } from "svelte";
  import { ApiError, getAdminOperations, type AdminOperations } from "../../lib/api.ts";
  import AdminReportDetails from "../../components/admin/AdminReportDetails.svelte";
  import { MediaQuery } from "svelte/reactivity";
  import { adminDate, adminNumber } from "../../lib/admin-operations.ts";

  const mobile = new MediaQuery("(max-width: 767px)");
  const workerTime = new Intl.DateTimeFormat("id-ID", { timeZone: "Asia/Jakarta", timeStyle: "medium" });

  let { accessToken, onUnauthorized, onLogout, loggingOut }: { accessToken: string; onUnauthorized: () => void; onLogout: () => void; loggingOut: boolean } = $props();
  let data = $state<AdminOperations | null>(null); let error = $state(""); let loading = $state(true); let updating = $state(false); let request: AbortController | undefined; let timer: ReturnType<typeof setInterval> | undefined; let generation = 0;
  async function load() {
    request?.abort(); const controller = new AbortController(); request = controller; const current = ++generation; error = ""; updating = Boolean(data); loading = !data;
    try { const latest = await getAdminOperations(accessToken, controller.signal); if (current === generation) data = latest; }
    catch (cause) { if (current !== generation || cause instanceof DOMException && cause.name === "AbortError") return; if (cause instanceof ApiError && cause.status === 401) onUnauthorized(); else error = cause instanceof ApiError ? cause.message : "Status operasional belum dapat dimuat."; }
    finally { if (current === generation) { loading = false; updating = false; if (request === controller) request = undefined; } }
  }
  function time(value: string | null) { return value ? new Date(value).toLocaleString("id-ID", { timeZone: "Asia/Jakarta", dateStyle: "medium", timeStyle: "medium" }) + " WIB" : "Belum ada"; }
  onMount(() => { void load(); timer = setInterval(() => void load(), 60_000); });
  onDestroy(() => { generation++; if (timer) clearInterval(timer); request?.abort(); });
</script>

<svelte:head><title>Status operasional | Tiket Online</title><meta name="robots" content="noindex" /></svelte:head>

{#snippet timestamp(value: string | null)}
  {#if value}<time datetime={value}><span class="admin-card-date">{adminDate.format(new Date(value))}</span><span class="admin-card-date">{workerTime.format(new Date(value))} WIB</span></time>{:else}Belum ada{/if}
{/snippet}

  <AdminLayout page="operations" contentId="operations-content" skipLabel="Lewati ke status operasional" kicker="OPERASIONAL EVENT • ADMIN" title="Status operasional." description="Antrean dan worker diperbarui setiap menit." onLogout={onLogout} loggingOut={loggingOut} primaryAction={{ label: "Muat ulang", onclick: load, disabled: loading || updating }}>
    {#if error}<p class="staff-form-message staff-form-error" role="alert">{error} <button class="staff-text-button" type="button" onclick={load}>Coba lagi</button></p>{/if}
    {#if loading && !data}<p class="staff-muted" role="status" aria-live="polite">Memuat status…</p>{/if}
    {#if data}
      <p class="sales-period">Data Operasional per: {#if mobile.current}{@render timestamp(data.collectedAt)}{:else}{time(data.collectedAt)}{/if}</p>
      {#if updating}<p class="report-refresh-note" role="status" aria-live="polite">Memperbarui kondisi operasional…</p>{:else if error}<p class="report-refresh-note" role="status">Pembaruan gagal. Data terakhir tetap ditampilkan.</p>{/if}
      <section class="ops-alerts" aria-labelledby="ops-alert-title">
        <div class="staff-panel-heading"><h2 id="ops-alert-title">Kondisi yang perlu diperiksa</h2></div>
        {#if data.alerts.length}<ul class="order-detail-list" role="alert">{#each data.alerts as alert}<li><strong>Perlu diperiksa</strong><span>{alert}</span></li>{/each}</ul>{:else}<p class="staff-muted" role="status">Tidak ada kondisi yang melewati ambang pemantauan.</p>{/if}
      </section>
      {#if data.openPaymentCases || data.openRefunds || data.api5xxLast5m}<section class="ops-actions" aria-label="Tindak lanjut operasional">
        <div class="ops-action-list">
          {#if data.openPaymentCases}<a href="/admin/issues"><strong>Kasus pembayaran</strong><span>{adminNumber.format(data.openPaymentCases)} masih terbuka · Buka Masalah →</span></a>{/if}
          {#if data.openRefunds}<a href="/admin/orders"><strong>Refund menunggu hasil</strong><span>{adminNumber.format(data.openRefunds)} perlu ditinjau · Buka Pesanan →</span></a>{/if}
        </div>
        {#if data.api5xxLast5m}<p class="ops-escalation">Error API 5xx tercatat {adminNumber.format(data.api5xxLast5m)} kali dalam lima menit. Hubungi pengelola layanan untuk menindaklanjuti gangguan sistem.</p>{/if}
      </section>{/if}
      <AdminReportDetails title="Ringkasan metrik">
        <dl class="ops-metrics"><dt>Error API 5xx · 5 menit</dt><dd>{adminNumber.format(data.api5xxLast5m)}</dd><dt>Email gagal dalam cakupan pemantauan</dt><dd>{adminNumber.format(data.failedEmailJobs)}</dd><dt>Antrean email tiket tertua</dt><dd>{adminNumber.format(data.oldestPendingEmailSeconds)} detik</dd><dt>Kasus pembayaran terbuka</dt><dd>{adminNumber.format(data.openPaymentCases)}</dd><dt>Refund menunggu hasil</dt><dd>{adminNumber.format(data.openRefunds)}</dd><dt>Tiket tertahan reservasi aktif</dt><dd>{adminNumber.format(data.heldTickets)}</dd><dt>Order menunggu pembayaran</dt><dd>{adminNumber.format(data.pendingPayments)}</dd></dl>
      </AdminReportDetails>
      <AdminReportDetails title="Worker" desktopOpen={false}>
        <p class="staff-muted">Status dan waktu keberhasilan worker menjadi detail teknis untuk pemeriksaan lanjutan.</p>
        {#if mobile.current}
          {#if data.workers.length}<ul class="admin-mobile-list admin-mobile-list-vertical" aria-label="Status worker">
            {#each data.workers as worker (worker.name)}<li class="admin-mobile-card">
              <div class="admin-mobile-card-header"><strong>{worker.name}</strong><span class="admin-order-status" class:is-paid={worker.running}>{worker.running ? "Memproses" : "Menunggu"}</span></div>
              <dl class="admin-card-fields">
                <div><dt>Batch terakhir</dt><dd>{@render timestamp(worker.lastFinishedAt)}</dd></div>
                <div><dt>Berhasil terakhir</dt><dd>{@render timestamp(worker.lastSuccessAt)}</dd></div>
                <div><dt>Gagal beruntun</dt><dd>{adminNumber.format(worker.consecutiveFailures)}</dd></div>
              </dl>
            </li>{/each}
          </ul>{:else}<p class="staff-muted" role="status">Belum ada data worker.</p>{/if}
        {:else}
          <!-- svelte-ignore a11y_no_noninteractive_tabindex -->
          <div class="history-table-wrap" tabindex="0" role="region" aria-label="Status worker"><table class="history-table worker-mobile-table"><thead><tr><th>Worker</th><th>Status</th><th>Batch terakhir</th><th>Berhasil terakhir</th><th>Gagal beruntun</th></tr></thead><tbody>{#each data.workers as worker (worker.name)}<tr><td data-label="Worker">{worker.name}</td><td data-label="Status">{worker.running ? "Memproses" : "Menunggu"}</td><td data-label="Batch terakhir">{time(worker.lastFinishedAt)}</td><td data-label="Berhasil terakhir">{time(worker.lastSuccessAt)}</td><td data-label="Gagal beruntun">{adminNumber.format(worker.consecutiveFailures)}</td></tr>{:else}<tr><td colspan="5">Belum ada data worker.</td></tr>{/each}</tbody></table></div>
        {/if}
      </AdminReportDetails>
    {/if}
</AdminLayout>

<style>
  .history-table-wrap { position: relative; }
</style>
