<script lang="ts">
  import { onDestroy, onMount } from "svelte";
  import { ApiError, getAdminSalesReport, type AdminSalesAmounts, type AdminSalesReport } from "../lib/api.ts";
  import { salesChartPaths } from "../lib/admin-sales-chart.ts";

  let { accessToken, onUnauthorized, onLogout, loggingOut }: { accessToken: string; onUnauthorized: () => void; onLogout: () => void; loggingOut: boolean } = $props();
  function jakartaToday() {
    const values = new Intl.DateTimeFormat("en-CA", { timeZone: "Asia/Jakarta", year: "numeric", month: "2-digit", day: "2-digit" }).formatToParts(new Date());
    return `${values.find((part) => part.type === "year")?.value}-${values.find((part) => part.type === "month")?.value}-${values.find((part) => part.type === "day")?.value}`;
  }
  function shiftDate(value: string, days: number) { const date = new Date(`${value}T12:00:00+07:00`); date.setUTCDate(date.getUTCDate() + days); return date.toISOString().slice(0, 10); }
  const today = jakartaToday();
  let eventId = $state("");
  let dateFrom = $state(shiftDate(today, -29));
  let dateTo = $state(today);
  let applied = $state<{ eventId?: string; dateFrom?: string; dateTo?: string }>({});
  let data = $state<AdminSalesReport | null>(null);
  let loading = $state(true);
  let error = $state("");
  let request: AbortController | undefined;
  let generation = 0;

  async function load(filters = applied) {
    request?.abort();
    const controller = new AbortController();
    request = controller;
    const current = ++generation;
    data = null;
    loading = true;
    error = "";
    try {
      const report = await getAdminSalesReport(accessToken, filters, controller.signal);
      if (current === generation) data = report;
    } catch (cause) {
      if (current !== generation || (cause instanceof DOMException && cause.name === "AbortError")) return;
      if (cause instanceof ApiError && cause.status === 401) onUnauthorized();
      else if (cause instanceof ApiError && cause.status === 403) error = "Laporan penjualan hanya tersedia untuk admin.";
      else error = cause instanceof ApiError ? cause.message : "Laporan belum dapat dimuat. Periksa koneksi lalu coba lagi.";
    } finally {
      if (current === generation) { loading = false; if (request === controller) request = undefined; }
    }
  }

  function search(event: SubmitEvent) {
    event.preventDefault();
    applied = { ...(eventId ? { eventId } : {}), dateFrom, dateTo };
    void load(applied);
  }

  function reset() {
    eventId = ""; dateFrom = shiftDate(today, -29); dateTo = today; applied = {};
    void load(applied);
  }

  function money(value: number) { return new Intl.NumberFormat("id-ID", { style: "currency", currency: "IDR", maximumFractionDigits: 0 }).format(value); }
  function time(value: string) { return new Date(value).toLocaleString("id-ID", { timeZone: "Asia/Jakarta", dateStyle: "medium", timeStyle: "short" }) + " WIB"; }
  function period(value: string) { return new Date(`${value}T12:00:00+07:00`).toLocaleDateString("id-ID", { timeZone: "Asia/Jakarta", day: "numeric", month: "short", year: "numeric" }); }
  function chart() { return salesChartPaths(data?.daily ?? []); }
  const metrics: Array<{ key: keyof AdminSalesAmounts; title: string }> = [
    { key: "successfulTransactions", title: "Transaksi berhasil" }, { key: "paymentAmount", title: "Pembayaran berhasil" },
    { key: "refundAmount", title: "Refund berhasil" }, { key: "netAmount", title: "Penerimaan setelah refund" },
    { key: "unfinishedRefunds", title: "Refund belum selesai" }, { key: "openReconciliationCases", title: "Rekonsiliasi terbuka" },
  ];

  onMount(() => { void load(); });
  onDestroy(() => { generation++; request?.abort(); });
</script>

<svelte:head><title>Laporan penjualan | Tiket Online</title><meta name="robots" content="noindex" /></svelte:head>
<div class="scan-shell staff-admin-shell">
  <a class="skip-link" href="#reports-content">Lewati ke laporan</a>
  <header class="scan-topbar"><a class="scan-brand" href="/" aria-label="Kembali ke Tiket Online"><span class="scan-brand-mark" aria-hidden="true">TO</span><span>Tiket Online <b>/ Gate Control</b></span></a><span class="staff-login-label">ADMINISTRATOR</span><button class="staff-text-button" type="button" disabled={loggingOut} onclick={onLogout}>Keluar</button></header>
  <main id="reports-content" class="staff-admin-content">
    <div class="staff-admin-heading"><div><p class="scan-kicker">LAPORAN KEUANGAN <span>•</span> ADMIN</p><h1>Penjualan dan refund.</h1><p>Pembayaran dihitung dari waktu lunas, refund dari waktu penyelesaian. Penerimaan setelah refund belum memperhitungkan biaya provider.</p></div><button class="staff-secondary-button" type="button" onclick={() => void load()} disabled={loading}>Muat ulang</button></div>
    <nav class="admin-tool-nav" aria-label="Administrasi event"><a href="/admin/events">Konser</a><a href="/admin/orders">Pesanan</a><a href="/admin/issues">Masalah</a><a href="/admin/operations">Operasional</a><a aria-current="page" href="/admin/reports">Penjualan dan refund</a><a href="/admin/reports/attendance">Kehadiran</a><a href="/admin/staff">Kelola petugas</a><a href="/admin/check-ins">Riwayat check-in</a></nav>
    <form class="history-filter-form sales-report-filters" onsubmit={search}>
      <label>Event<select bind:value={eventId}><option value="">Semua event</option>{#each data?.filterOptions.events ?? [] as event (event.id)}<option value={event.id}>{event.name}</option>{/each}</select></label>
      <label>Dari tanggal WIB<input type="date" bind:value={dateFrom} required /></label>
      <label>Sampai tanggal WIB<input type="date" bind:value={dateTo} required /></label>
      <div class="history-filter-actions"><button class="scan-submit staff-submit" type="submit" disabled={loading}>Terapkan</button><button class="staff-secondary-button" type="button" onclick={reset} disabled={loading}>Reset</button></div>
    </form>
    {#if error}<p class="staff-form-message staff-form-error" role="alert">{error} <button class="staff-text-button" type="button" onclick={() => void load()}>Coba lagi</button></p>{/if}
    {#if loading}<p class="staff-muted" role="status" aria-live="polite">Memuat laporan…</p>{:else if data}
      <p class="sales-period">Periode {period(data.period.dateFrom)}–{period(data.period.dateTo)} · WIB <span>Data diperbarui {time(data.dataUpdatedAt)}</span></p>
      <section class="sales-metrics" aria-label="Ringkasan laporan" aria-busy={loading}>
        {#each metrics as metric (metric.key)}<article class="sales-metric"><h2>{metric.title}</h2><p>{metric.key === "successfulTransactions" || metric.key === "unfinishedRefunds" || metric.key === "openReconciliationCases" ? data.summary[metric.key].toLocaleString("id-ID") : money(data.summary[metric.key])}</p>{#if metric.key === "unfinishedRefunds" || metric.key === "openReconciliationCases"}<small>Kondisi saat laporan dimuat</small>{/if}</article>{/each}
      </section>
      <section class="staff-admin-panel checkin-history-panel"><div class="staff-panel-heading"><div><span class="panel-index">01</span><h2>Tren harian</h2></div></div>
        <div class="sales-chart" role="img" aria-label="Grafik garis pembayaran, refund, dan selisih untuk periode laporan"><svg viewBox="0 0 720 180" preserveAspectRatio="none"><line x1="40" x2="708" y1={chart().zeroY} y2={chart().zeroY} class="sales-zero" /><path d={chart().payments} class="sales-line sales-payment" /><path d={chart().refunds} class="sales-line sales-refund" /><path d={chart().net} class="sales-line sales-net" /></svg></div>
        <div class="sales-chart-labels"><span>{period(data.daily[0]?.date ?? data.period.dateFrom)}</span><span>{period(data.daily.at(-1)?.date ?? data.period.dateTo)}</span></div>
        <ul class="sales-legend"><li><span class="sales-key sales-payment"></span>Pembayaran</li><li><span class="sales-key sales-refund"></span>Refund</li><li><span class="sales-key sales-net"></span>Penerimaan setelah refund</li></ul>
        <details class="sales-daily-details"><summary>Buka angka per hari</summary><div class="history-table-wrap"><table class="history-table sales-table"><thead><tr><th>Tanggal WIB</th><th>Transaksi</th><th>Pembayaran</th><th>Refund</th><th>Penerimaan setelah refund</th></tr></thead><tbody>{#each data.daily as day (day.date)}<tr><td>{period(day.date)}</td><td>{day.successfulTransactions.toLocaleString("id-ID")}</td><td>{money(day.paymentAmount)}</td><td>{money(day.refundAmount)}</td><td>{money(day.netAmount)}</td></tr>{/each}</tbody></table></div></details>
      </section>
      <section class="staff-admin-panel checkin-history-panel"><div class="staff-panel-heading"><div><span class="panel-index">02</span><h2>Rincian per event</h2></div></div>
        <div class="history-table-wrap"><table class="history-table sales-table"><thead><tr><th>Event</th><th>Transaksi</th><th>Pembayaran</th><th>Refund</th><th>Penerimaan setelah refund</th><th>Refund berjalan</th><th>Rekonsiliasi terbuka</th></tr></thead><tbody>{#each data.byEvent as event (event.id)}<tr><td>{event.name}</td><td>{event.successfulTransactions.toLocaleString("id-ID")}</td><td>{money(event.paymentAmount)}</td><td>{money(event.refundAmount)}</td><td>{money(event.netAmount)}</td><td>{event.unfinishedRefunds.toLocaleString("id-ID")}</td><td>{event.openReconciliationCases.toLocaleString("id-ID")}</td></tr>{:else}<tr><td colspan="7">Tidak ada event untuk filter ini.</td></tr>{/each}</tbody></table></div>
        {#if data.summary.successfulTransactions === 0 && data.summary.refundAmount === 0}<p class="staff-muted" role="status">Belum ada pembayaran atau refund berhasil pada periode ini.</p>{/if}
      </section>
    {/if}
  </main>
</div>
