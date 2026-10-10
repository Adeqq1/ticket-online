<script lang="ts">
  import AdminLayout from "../../layouts/AdminLayout.svelte";
  import { onDestroy, onMount } from "svelte";
  import { ApiError, getAdminOperations, type AdminOperations } from "../../lib/api.ts";
  import AdminReportDetails from "../../components/admin/AdminReportDetails.svelte";

  let { accessToken, onUnauthorized, onLogout, loggingOut }: { accessToken: string; onUnauthorized: () => void; onLogout: () => void; loggingOut: boolean } = $props();
  let data = $state<AdminOperations | null>(null); let error = $state(""); let loading = $state(true); let updating = $state(false); let request: AbortController | undefined; let timer: ReturnType<typeof setInterval> | undefined; let generation = 0;
  async function load() {
    request?.abort(); const controller = new AbortController(); request = controller; const current = ++generation; error = ""; updating = Boolean(data); loading = !data;
    try { const latest = await getAdminOperations(accessToken, controller.signal); if (current === generation) data = latest; }
    catch (cause) { if (current !== generation || cause instanceof DOMException && cause.name === "AbortError") return; if (cause instanceof ApiError && cause.status === 401) onUnauthorized(); else error = cause instanceof ApiError ? cause.message : "Status operasional belum dapat dimuat."; }
    finally { if (current === generation) { loading = false; updating = false; if (request === controller) request = undefined; } }
  }
  function time(value: string | null) { return value ? new Date(value).toLocaleString("id-ID", { timeZone: "Asia/Jakarta", dateStyle: "medium", timeStyle: "short" }) + " WIB" : "Belum ada"; }
  onMount(() => { void load(); timer = setInterval(() => void load(), 60_000); });
  onDestroy(() => { generation++; if (timer) clearInterval(timer); request?.abort(); });
</script>

<svelte:head><title>Status operasional | Tiket Online</title><meta name="robots" content="noindex" /></svelte:head>
  <AdminLayout page="operations" contentId="operations-content" skipLabel="Lewati ke status operasional" kicker="OPERASIONAL EVENT • ADMIN" title="Status operasional." description="Antrean dan worker diperbarui setiap menit." onLogout={onLogout} loggingOut={loggingOut} primaryAction={{ label: "Muat ulang", onclick: load, disabled: loading || updating }}>
    {#if error}<p class="staff-form-message staff-form-error" role="alert">{error} <button class="staff-text-button" type="button" onclick={load}>Coba lagi</button></p>{/if}
    {#if loading && !data}<p class="staff-muted" role="status" aria-live="polite">Memuat status…</p>{/if}
    {#if data}
      <p class="sales-period">Snapshot dikumpulkan {time(data.collectedAt)}</p>
      {#if updating}<p class="report-refresh-note" role="status" aria-live="polite">Memperbarui kondisi operasional…</p>{:else if error}<p class="report-refresh-note" role="status">Pembaruan gagal. Data di bawah adalah snapshot terakhir yang berhasil dimuat.</p>{/if}
      <section class="ops-alerts" aria-labelledby="ops-alert-title">
        <div class="staff-panel-heading"><h2 id="ops-alert-title">Kondisi yang perlu diperiksa</h2></div>
        {#if data.alerts.length}<ul class="order-detail-list" role="alert">{#each data.alerts as alert}<li><strong>Perlu diperiksa</strong><span>{alert}</span></li>{/each}</ul>{:else}<p class="staff-muted" role="status">Tidak ada kondisi yang melewati ambang pemantauan.</p>{/if}
      </section>
      <section class="ops-actions" aria-labelledby="ops-actions-title">
        <h2 id="ops-actions-title">Penanganan yang tersedia</h2>
        {#if data.failedEmailJobs || data.openPaymentCases || data.openRefunds}
          <div class="ops-action-list">
            {#if data.failedEmailJobs}<a href="/admin/issues"><strong>Email gagal</strong><span>{data.failedEmailJobs} perlu ditinjau · Buka Masalah →</span></a>{/if}
            {#if data.openPaymentCases}<a href="/admin/issues"><strong>Kasus pembayaran</strong><span>{data.openPaymentCases} masih terbuka · Buka Masalah →</span></a>{/if}
            {#if data.openRefunds}<a href="/admin/orders"><strong>Refund menunggu hasil</strong><span>{data.openRefunds} perlu ditinjau · Buka Pesanan →</span></a>{/if}
          </div>
        {:else}<p class="staff-muted">Tidak ada email gagal, kasus pembayaran terbuka, atau refund yang perlu ditinjau.</p>{/if}
        {#if data.api5xxLast5m}<p class="ops-escalation">Error API 5xx tercatat {data.api5xxLast5m} kali dalam lima menit. Hubungi pengelola layanan untuk menindaklanjuti gangguan sistem.</p>{/if}
      </section>
      <AdminReportDetails title="Ringkasan metrik">
        <dl class="ops-metrics"><dt>Error API 5xx · 5 menit</dt><dd>{data.api5xxLast5m}</dd><dt>Email gagal yang perlu ditangani</dt><dd>{data.failedEmailJobs}</dd><dt>Antrean email tiket tertua</dt><dd>{data.oldestPendingEmailSeconds} detik</dd><dt>Kasus pembayaran terbuka</dt><dd>{data.openPaymentCases}</dd><dt>Refund menunggu hasil</dt><dd>{data.openRefunds}</dd><dt>Tiket tertahan reservasi aktif</dt><dd>{data.heldTickets}</dd><dt>Order menunggu pembayaran</dt><dd>{data.pendingPayments}</dd></dl>
      </AdminReportDetails>
      <AdminReportDetails title="Worker" desktopOpen={false}>
        <p class="staff-muted">Status dan waktu keberhasilan worker menjadi detail teknis untuk pemeriksaan lanjutan.</p>
        <!-- svelte-ignore a11y_no_noninteractive_tabindex -->
        <div class="history-table-wrap" tabindex="0" role="region" aria-label="Status worker"><table class="history-table"><thead><tr><th>Worker</th><th>Status</th><th>Batch terakhir</th><th>Berhasil terakhir</th><th>Gagal beruntun</th></tr></thead><tbody>{#each data.workers as worker (worker.name)}<tr><td>{worker.name}</td><td>{worker.running ? "Memproses" : "Menunggu"}</td><td>{time(worker.lastFinishedAt)}</td><td>{time(worker.lastSuccessAt)}</td><td>{worker.consecutiveFailures}</td></tr>{:else}<tr><td colspan="5">Belum ada data worker.</td></tr>{/each}</tbody></table></div>
      </AdminReportDetails>
    {/if}
</AdminLayout>
