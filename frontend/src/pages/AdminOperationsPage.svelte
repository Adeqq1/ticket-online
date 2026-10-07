<script lang="ts">
  import { onDestroy, onMount } from "svelte";
  import { ApiError, getAdminOperations, type AdminOperations } from "../lib/api.ts";

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
<div class="scan-shell staff-admin-shell">
  <a class="skip-link" href="#operations-content">Lewati ke status operasional</a>
  <header class="scan-topbar"><a class="scan-brand" href="/" aria-label="Kembali ke Tiket Online"><span class="scan-brand-mark" aria-hidden="true">TO</span><span>Tiket Online <b>/ Gate Control</b></span></a><span class="staff-login-label">ADMINISTRATOR</span><button class="staff-text-button" type="button" disabled={loggingOut} onclick={onLogout}>Keluar</button></header>
  <main id="operations-content" class="staff-admin-content">
    <div class="staff-admin-heading"><div><p class="scan-kicker">OPERASIONAL EVENT <span>•</span> ADMIN</p><h1>Status operasional.</h1><p>Antrean dan worker diperbarui setiap menit.</p></div><button class="staff-secondary-button" type="button" onclick={load} disabled={loading}>Muat ulang</button></div>
    <nav class="admin-tool-nav" aria-label="Administrasi event"><a href="/admin/events">Konser</a><a href="/admin/orders">Pesanan</a><a href="/admin/issues">Masalah</a><a aria-current="page" href="/admin/operations">Operasional</a><a href="/admin/staff">Kelola petugas</a><a href="/admin/check-ins">Riwayat check-in</a></nav>
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
  </main>
</div>
