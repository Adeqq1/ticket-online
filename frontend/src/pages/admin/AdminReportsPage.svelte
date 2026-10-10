<script lang="ts">
  import AdminLayout from "../../layouts/AdminLayout.svelte";
  import { onDestroy, onMount } from "svelte";
  import { ApiError, exportAdminSalesReportCSV, getAdminSalesReport, saveAdminReportCSV, type AdminSalesAmounts, type AdminSalesReport } from "../../lib/api.ts";
  import { salesChartPaths } from "../../lib/admin-sales-chart.ts";
  import AdminReportDetails from "../../components/admin/AdminReportDetails.svelte";

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
  let applied = $state<{ eventId?: string; dateFrom?: string; dateTo?: string }>({ dateFrom: shiftDate(today, -29), dateTo: today });
  let lastRequested = $state<{ eventId?: string; dateFrom?: string; dateTo?: string } | null>(null);
  let data = $state<AdminSalesReport | null>(null);
  let eventOptions = $state<AdminSalesReport["filterOptions"]["events"]>([]);
  let loading = $state(true);
  let exporting = $state(false);
  let error = $state("");
  let exportError = $state("");
  let request: AbortController | undefined;
  let generation = 0;

  async function load(filters = applied) {
    const requested = { ...filters };
    lastRequested = requested;
    request?.abort();
    const controller = new AbortController();
    request = controller;
    const current = ++generation;
    loading = true;
    error = "";
    try {
      const report = await getAdminSalesReport(accessToken, requested, controller.signal);
      if (current === generation) { data = report; applied = requested; eventOptions = report.filterOptions.events; }
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
    void load({ ...(eventId ? { eventId } : {}), dateFrom, dateTo });
  }
  function retry() { if (lastRequested) void load(lastRequested); }

  async function exportCSV() {
    if (!data || exporting) return;
    exporting = true; exportError = "";
    try {
      const file = await exportAdminSalesReportCSV(accessToken, { eventId: data.period.eventId ?? undefined, dateFrom: data.period.dateFrom, dateTo: data.period.dateTo });
      saveAdminReportCSV(file.blob, file.filename);
    } catch (cause) {
      if (cause instanceof ApiError && cause.status === 401) onUnauthorized();
      else exportError = cause instanceof ApiError ? cause.message : "CSV belum dapat diunduh. Periksa koneksi lalu coba lagi.";
    } finally { exporting = false; }
  }

  function reset() {
    eventId = ""; dateFrom = shiftDate(today, -29); dateTo = today;
    void load({ dateFrom, dateTo });
  }

  function money(value: number) { return new Intl.NumberFormat("id-ID", { style: "currency", currency: "IDR", maximumFractionDigits: 0 }).format(value); }
  function time(value: string) { return new Date(value).toLocaleString("id-ID", { timeZone: "Asia/Jakarta", dateStyle: "medium", timeStyle: "short" }) + " WIB"; }
  function period(value: string) { return new Date(`${value}T12:00:00+07:00`).toLocaleDateString("id-ID", { timeZone: "Asia/Jakarta", day: "numeric", month: "short", year: "numeric" }); }
  const chart = $derived(salesChartPaths(data?.daily ?? []));
  const draftChanged = $derived(Boolean(data && (eventId !== (applied.eventId ?? "") || dateFrom !== applied.dateFrom || dateTo !== applied.dateTo)));
  const metrics: Array<{ key: keyof AdminSalesAmounts; title: string }> = [
    { key: "paymentAmount", title: "Pembayaran berhasil" }, { key: "refundAmount", title: "Refund berhasil" },
    { key: "netAmount", title: "Penerimaan setelah refund" }, { key: "successfulTransactions", title: "Transaksi berhasil" },
    { key: "unfinishedRefunds", title: "Refund belum selesai" }, { key: "openReconciliationCases", title: "Rekonsiliasi terbuka" },
  ];

  onMount(() => { void load(); });
  onDestroy(() => { generation++; request?.abort(); });
</script>

<svelte:head><title>Laporan penjualan | Tiket Online</title><meta name="robots" content="noindex" /></svelte:head>
  <AdminLayout page="reports" contentId="reports-content" skipLabel="Lewati ke laporan" kicker="LAPORAN KEUANGAN • ADMIN" title="Penjualan dan refund." description="Pembayaran dihitung dari waktu lunas, refund dari waktu penyelesaian. Penerimaan setelah refund belum memperhitungkan biaya provider." onLogout={onLogout} loggingOut={loggingOut} primaryAction={{ label: "Muat ulang", onclick: () => void load(), disabled: loading }}>
    <form class="history-filter-form sales-report-filters" onsubmit={search}>
      <label>Event<select bind:value={eventId}><option value="">Semua event</option>{#each eventOptions as event (event.id)}<option value={event.id}>{event.name}</option>{/each}</select></label>
      <label>Dari tanggal WIB<input type="date" bind:value={dateFrom} required /></label>
      <label>Sampai tanggal WIB<input type="date" bind:value={dateTo} required /></label>
      <div class="history-filter-actions"><button class="scan-submit staff-submit" type="submit" disabled={loading}>Terapkan</button><button class="staff-secondary-button" type="button" onclick={reset} disabled={loading}>Reset</button></div>
    </form>
    {#if error}<p class="staff-form-message staff-form-error" role="alert">{error} <button class="staff-text-button" type="button" onclick={retry}>Coba lagi</button></p>{/if}
    {#if exportError}<p class="staff-form-message staff-form-error" role="alert">{exportError}</p>{/if}
    {#if loading && !data}<p class="staff-muted" role="status" aria-live="polite">Memuat laporan…</p>{/if}
    {#if data}
      <p class="sales-period">Periode {period(data.period.dateFrom)} sampai {period(data.period.dateTo)} · WIB <span>Data diperbarui {time(data.dataUpdatedAt)}</span></p>
      {#if draftChanged}<p class="report-refresh-note" role="status">Filter berubah. Tekan Terapkan untuk memperbarui laporan; ekspor masih memakai periode di atas.</p>{/if}
      {#if loading}<p class="report-refresh-note" role="status" aria-live="polite">Memperbarui laporan. Angka dan ekspor tetap mengacu pada snapshot terakhir sampai data baru tersedia.</p>{:else if error}<p class="report-refresh-note" role="status">Pembaruan gagal. Snapshot terakhir tetap ditampilkan.</p>{/if}
      <p class="report-definition">Penerimaan setelah refund bukan laba atau settlement bank; biaya provider belum dikurangi. Refund berjalan dan rekonsiliasi terbuka menunjukkan kondisi pada waktu snapshot.</p>
      <section class="sales-metrics" aria-label="Ringkasan laporan" aria-busy={loading}>
        {#each metrics as metric (metric.key)}<article class="sales-metric"><h2>{metric.title}</h2><p>{metric.key === "successfulTransactions" || metric.key === "unfinishedRefunds" || metric.key === "openReconciliationCases" ? data.summary[metric.key].toLocaleString("id-ID") : money(data.summary[metric.key])}</p>{#if metric.key === "unfinishedRefunds" || metric.key === "openReconciliationCases"}<small>Kondisi saat laporan dimuat</small>{/if}</article>{/each}
      </section>
      <AdminReportDetails title="Tren harian">
        {#if data.daily.length}
          <div class="sales-chart" role="img" aria-label="Grafik garis pembayaran, refund, dan selisih untuk periode laporan"><svg viewBox="0 0 720 180" preserveAspectRatio="none"><line x1="40" x2="708" y1={chart.zeroY} y2={chart.zeroY} class="sales-zero" /><path d={chart.payments} class="sales-line sales-payment" /><path d={chart.refunds} class="sales-line sales-refund" /><path d={chart.net} class="sales-line sales-net" />{#if chart.points}<circle cx={chart.points.payments.x} cy={chart.points.payments.y} r="3.5" class="sales-point sales-payment" /><circle cx={chart.points.refunds.x} cy={chart.points.refunds.y} r="3.5" class="sales-point sales-refund" /><circle cx={chart.points.net.x} cy={chart.points.net.y} r="3.5" class="sales-point sales-net" />{/if}</svg></div>
          <div class="sales-chart-labels"><span>{period(data.daily[0]?.date ?? data.period.dateFrom)}</span><span>{period(data.daily.at(-1)?.date ?? data.period.dateTo)}</span></div>
          <ul class="sales-legend"><li><span class="sales-key sales-payment"></span>Pembayaran</li><li><span class="sales-key sales-refund"></span>Refund</li><li><span class="sales-key sales-net"></span>Penerimaan setelah refund</li></ul>
          <details class="sales-daily-details"><summary>Buka angka per hari</summary><!-- svelte-ignore a11y_no_noninteractive_tabindex -->
        <div class="history-table-wrap" tabindex="0" role="region" aria-label="Angka penjualan per hari"><table class="history-table sales-table"><thead><tr><th>Tanggal WIB</th><th>Transaksi</th><th>Pembayaran</th><th>Refund</th><th>Penerimaan setelah refund</th></tr></thead><tbody>{#each data.daily as day (day.date)}<tr><td>{period(day.date)}</td><td>{day.successfulTransactions.toLocaleString("id-ID")}</td><td>{money(day.paymentAmount)}</td><td>{money(day.refundAmount)}</td><td>{money(day.netAmount)}</td></tr>{/each}</tbody></table></div></details>
        {:else}<p class="report-empty-state" role="status">Belum ada pembayaran atau refund pada periode ini.</p>{/if}
      </AdminReportDetails>
      <AdminReportDetails title="Rincian per event">
        <!-- svelte-ignore a11y_no_noninteractive_tabindex -->
        <div class="history-table-wrap" tabindex="0" role="region" aria-label="Rincian laporan per event"><table class="history-table sales-table"><thead><tr><th>Event</th><th>Transaksi</th><th>Pembayaran</th><th>Refund</th><th>Penerimaan setelah refund</th><th>Refund berjalan</th><th>Rekonsiliasi terbuka</th></tr></thead><tbody>{#each data.byEvent as event (event.id)}<tr><td>{event.name}</td><td>{event.successfulTransactions.toLocaleString("id-ID")}</td><td>{money(event.paymentAmount)}</td><td>{money(event.refundAmount)}</td><td>{money(event.netAmount)}</td><td>{event.unfinishedRefunds.toLocaleString("id-ID")}</td><td>{event.openReconciliationCases.toLocaleString("id-ID")}</td></tr>{:else}<tr><td colspan="7">Tidak ada event untuk filter ini.</td></tr>{/each}</tbody></table></div>
        {#if data.summary.successfulTransactions === 0 && data.summary.refundAmount === 0}<p class="staff-muted" role="status">Belum ada pembayaran atau refund berhasil pada periode ini.</p>{/if}
      </AdminReportDetails>
      <div class="report-export"><p>CSV memakai periode dan event pada snapshot yang sedang ditampilkan.</p><button class="scan-submit staff-submit" type="button" onclick={exportCSV} disabled={loading || exporting}>{exporting ? "Menyiapkan CSV…" : "Ekspor CSV"}</button></div>
    {/if}
</AdminLayout>
