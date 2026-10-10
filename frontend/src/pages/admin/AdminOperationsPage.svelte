<script lang="ts">
  import AdminLayout from "../../layouts/AdminLayout.svelte";
  import { onDestroy, onMount } from "svelte";
  import { ApiError, getAdminOperations, type AdminOperations } from "../../lib/api.ts";

  let { accessToken, onUnauthorized, onLogout, loggingOut }: { accessToken: string; onUnauthorized: () => void; onLogout: () => void; loggingOut: boolean } = $props();
  let data = $state<AdminOperations | null>(null); let error = $state(""); let loading = $state(true); let request: AbortController | undefined; let timer: ReturnType<typeof setInterval> | undefined;
  async function load() {
    request?.abort(); const controller = new AbortController(); request = controller; error = "";
    try { data = await getAdminOperations(accessToken, controller.signal); }
    catch (cause) { if (cause instanceof DOMException && cause.name === "AbortError") return; if (cause instanceof ApiError && cause.status === 401) onUnauthorized(); else error = cause instanceof ApiError ? cause.message : "Status operasional belum dapat dimuat."; }
    finally { loading = false; if (request === controller) request = undefined; }
  }
  function time(value: string | null) { return value ? new Date(value).toLocaleString("id-ID", { timeZone: "Asia/Jakarta", dateStyle: "medium", timeStyle: "short" }) + " WIB" : "Belum ada"; }
  onMount(() => { void load(); timer = setInterval(() => void load(), 60_000); });
  onDestroy(() => { if (timer) clearInterval(timer); request?.abort(); });
</script>

<svelte:head><title>Status operasional | Tiket Online</title><meta name="robots" content="noindex" /></svelte:head>
  <AdminLayout page="operations" contentId="operations-content" skipLabel="Lewati ke status operasional" kicker="OPERASIONAL EVENT • ADMIN" title="Status operasional." description="Antrean dan worker diperbarui setiap menit." onLogout={onLogout} loggingOut={loggingOut} primaryAction={{ label: "Muat ulang", onclick: load, disabled: loading }}>
    {#if error}<p class="staff-form-message staff-form-error" role="alert">{error} <button class="staff-text-button" type="button" onclick={load}>Coba lagi</button></p>{/if}
    {#if loading}<p class="staff-muted" role="status">Memuat status…</p>{:else if data}
      <section class="staff-admin-panel checkin-history-panel"><div class="staff-panel-heading"><div><span class="panel-index">01</span><h2>Kondisi sistem</h2></div><span class="staff-muted">Diperbarui {time(data.collectedAt)}</span></div>
        {#if data.alerts.length}<ul class="order-detail-list" role="alert">{#each data.alerts as alert}<li><strong>Perlu diperiksa</strong><span>{alert}</span></li>{/each}</ul>{:else}<p class="staff-muted" role="status">Tidak ada kondisi yang melewati ambang pemantauan.</p>{/if}
        <dl><dt>Error API 5xx · 5 menit</dt><dd>{data.api5xxLast5m}</dd><dt>Email gagal yang perlu ditangani</dt><dd><a href="/admin/issues">{data.failedEmailJobs}</a></dd><dt>Antrean email tiket tertua</dt><dd>{data.oldestPendingEmailSeconds} detik</dd><dt>Kasus pembayaran terbuka</dt><dd><a href="/admin/issues">{data.openPaymentCases}</a></dd><dt>Refund menunggu hasil</dt><dd><a href="/admin/orders">{data.openRefunds}</a></dd><dt>Tiket tertahan reservasi aktif</dt><dd>{data.heldTickets}</dd><dt>Order menunggu pembayaran</dt><dd>{data.pendingPayments}</dd></dl>
      </section>
      <section class="staff-admin-panel checkin-history-panel"><div class="staff-panel-heading"><div><span class="panel-index">02</span><h2>Worker</h2></div></div>
        <div class="history-table-wrap"><table class="history-table"><thead><tr><th>Worker</th><th>Status</th><th>Batch terakhir</th><th>Berhasil terakhir</th><th>Gagal beruntun</th></tr></thead><tbody>{#each data.workers as worker (worker.name)}<tr><td>{worker.name}</td><td>{worker.running ? "Memproses" : "Menunggu"}</td><td>{time(worker.lastFinishedAt)}</td><td>{time(worker.lastSuccessAt)}</td><td>{worker.consecutiveFailures}</td></tr>{/each}</tbody></table></div>
      </section>
    {/if}
</AdminLayout>
