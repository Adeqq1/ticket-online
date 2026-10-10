<script lang="ts">
  import AdminLayout from "../../layouts/AdminLayout.svelte";
  import { onDestroy, onMount } from "svelte";
  import { ApiError, exportAdminAttendanceReportCSV, getAdminAttendanceReport, getAdminEvents, saveAdminReportCSV, type AdminAttendanceReport, type AdminApiEvent } from "../../lib/api.ts";
  import AdminReportDetails from "../../components/admin/AdminReportDetails.svelte";

  let { accessToken, onUnauthorized, onLogout, loggingOut }: { accessToken: string; onUnauthorized: () => void; onLogout: () => void; loggingOut: boolean } = $props();
  let events = $state<AdminApiEvent[]>([]);
  let eventId = $state("");
  let gate = $state("");
  let gateOptions = $state<string[]>([]);
  let applied = $state<{ eventId: string; gate?: string } | null>(null);
  let data = $state<AdminAttendanceReport | null>(null);
  let loading = $state(true);
  let exporting = $state(false);
  let error = $state("");
  let exportError = $state("");
  let request: AbortController | undefined;
  let generation = 0;

  async function load(filters = applied) {
    if (!filters) return;
    request?.abort();
    const controller = new AbortController();
    request = controller;
    const current = ++generation;
    loading = true; error = "";
    try {
      const report = await getAdminAttendanceReport(accessToken, filters, controller.signal);
      if (current !== generation) return;
      data = report;
      applied = filters;
      gateOptions = report.filterOptions.gates;
    } catch (cause) {
      if (current !== generation || (cause instanceof DOMException && cause.name === "AbortError")) return;
      if (cause instanceof ApiError && cause.status === 401) onUnauthorized();
      else if (cause instanceof ApiError && cause.status === 403) error = "Laporan kehadiran hanya tersedia untuk admin.";
      else error = cause instanceof ApiError ? cause.message : "Laporan belum dapat dimuat. Periksa koneksi lalu coba lagi.";
    } finally {
      if (current === generation) { loading = false; if (request === controller) request = undefined; }
    }
  }

  async function initialize() {
    try { events = await getAdminEvents(accessToken); }
    catch (cause) {
      if (cause instanceof ApiError && cause.status === 401) onUnauthorized();
      else error = cause instanceof ApiError ? cause.message : "Daftar event belum dapat dimuat.";
    } finally { loading = false; }
  }

  function search(event: SubmitEvent) {
    event.preventDefault();
    if (!eventId) { error = "Pilih event untuk membuka laporan kehadiran."; return; }
    void load({ eventId, ...(gate ? { gate } : {}) });
  }
  function selectEvent(value: string) { generation++; request?.abort(); request = undefined; loading = false; eventId = value; gate = ""; gateOptions = []; error = ""; }
  function reset() { generation++; request?.abort(); request = undefined; loading = false; eventId = ""; gate = ""; gateOptions = []; error = ""; }
  function retry() { if (applied) void load(applied); else void initialize(); }
  async function exportCSV() {
    if (!data || exporting) return;
    exporting = true; exportError = "";
    try {
      const file = await exportAdminAttendanceReportCSV(accessToken, { eventId: data.event.id, gate: data.gate ?? undefined });
      saveAdminReportCSV(file.blob, file.filename);
    } catch (cause) {
      if (cause instanceof ApiError && cause.status === 401) onUnauthorized();
      else exportError = cause instanceof ApiError ? cause.message : "CSV belum dapat diunduh. Periksa koneksi lalu coba lagi.";
    } finally { exporting = false; }
  }
  function number(value: number) { return value.toLocaleString("id-ID"); }
  function rate(value: number | null) { return value === null ? "Belum tersedia" : `${new Intl.NumberFormat("id-ID", { maximumFractionDigits: 1 }).format(value)}%`; }
  function localTime(value: string) { return new Date(value).toLocaleString("id-ID", { timeZone: "Asia/Jakarta", dateStyle: "medium", timeStyle: "short" }) + " WIB"; }
  function hour(value: string) { return new Date(value).toLocaleString("id-ID", { timeZone: "Asia/Jakarta", dateStyle: "medium", timeStyle: "short" }) + " WIB"; }
  function gateName(value: string | null) { return value || "Gate tidak diketahui"; }
  const draftChanged = $derived(Boolean(data && (eventId !== applied?.eventId || gate !== (applied?.gate ?? ""))));

  onMount(() => { void initialize(); });
  onDestroy(() => { generation++; request?.abort(); });
</script>

<svelte:head><title>Laporan kehadiran | Tiket Online</title><meta name="robots" content="noindex" /></svelte:head>
  <AdminLayout page="attendance" contentId="attendance-report-content" skipLabel="Lewati ke laporan" kicker="LAPORAN EVENT • ADMIN" title="Kehadiran dan tiket." description="Kondisi inventori dan tiket saat ini; kehadiran dihitung dari tiket masuk yang berhasil di-scan." onLogout={onLogout} loggingOut={loggingOut} primaryAction={{ label: "Muat ulang", onclick: () => void load(), disabled: loading || !applied }}>
    <section class="staff-admin-panel checkin-history-panel" aria-labelledby="attendance-filter-title">
      <div class="staff-panel-heading"><div><span class="panel-index">01</span><h2 id="attendance-filter-title">Filter laporan</h2></div></div>
      <form class="history-filter-form" onsubmit={search} aria-busy={loading}>
        <label>Event wajib<select value={eventId} onchange={(event) => selectEvent(event.currentTarget.value)} required disabled={loading}><option value="">Pilih event</option>{#each events as event (event.id)}<option value={event.id}>{event.artist} · {event.city}</option>{/each}</select></label>
        <label>Gate<select bind:value={gate} disabled={loading || !eventId || !gateOptions.length}><option value="">Semua gate</option>{#each gateOptions as item (item)}<option value={item}>{item}</option>{/each}</select></label>
        <div class="history-filter-actions"><button class="scan-submit staff-submit" type="submit" disabled={loading || !eventId}>Terapkan</button><button class="staff-secondary-button" type="button" onclick={reset} disabled={loading}>Reset</button></div>
      </form>
    </section>
    {#if error}<p class="staff-form-message staff-form-error" role="alert">{error} <button class="staff-text-button" type="button" onclick={retry}>Coba lagi</button></p>{/if}
    {#if exportError}<p class="staff-form-message staff-form-error" role="alert">{exportError}</p>{/if}
    {#if loading && !data}<p class="staff-muted" role="status" aria-live="polite">Memuat laporan kehadiran…</p>{/if}
    {#if data}
      <p class="sales-period">{data.event.name}{data.gate ? ` · ${data.gate}` : " · semua gate"} <span>Data diperbarui {localTime(data.dataUpdatedAt)}</span></p>
      {#if draftChanged}<p class="report-refresh-note" role="status">Filter berubah. Terapkan untuk memperbarui laporan; ekspor masih memakai event dan gate di atas.</p>{/if}
      {#if loading}<p class="report-refresh-note" role="status" aria-live="polite">Memperbarui laporan…</p>{:else if error}<p class="report-refresh-note" role="status">Pembaruan gagal. Snapshot terakhir tetap ditampilkan.</p>{/if}
      <p class="report-definition">Inventori, tiket, dan check-in menunjukkan kondisi snapshot. Jumlah check-in menyimpan riwayat; persentase memakai check-in pada tiket yang masih berhak masuk. {#if data.summary.attendanceRate === null}Persentase belum tersedia karena belum ada tiket yang berhak masuk.{/if}</p>
      <section class="sales-metrics attendance-primary-metrics" aria-label="Ringkasan kehadiran">
        <article class="sales-metric"><h2>Sudah check-in</h2><p>{number(data.summary.checkedIn)}</p><small>Catatan check-in historis</small></article>
        <article class="sales-metric"><h2>Berhak masuk</h2><p>{number(data.summary.eligible)}</p><small>Order lunas dan snapshot valid</small></article>
        <article class="sales-metric"><h2>Kehadiran</h2><p>{rate(data.summary.attendanceRate)}</p><small>Dari tiket yang berhak masuk</small></article>
      </section>
      <AdminReportDetails title="Kapasitas dan tiket">
        <section class="sales-metrics attendance-secondary-metrics" aria-label="Kapasitas dan tiket">
          <article class="sales-metric"><h2>Kapasitas</h2><p>{number(data.summary.capacity)}</p><small>{number(data.summary.available)} stok tersedia</small></article>
          <article class="sales-metric"><h2>Tiket diterbitkan</h2><p>{number(data.summary.issued)}</p><small>Semua status order</small></article>
          <article class="sales-metric"><h2>Tertahan karena refund</h2><p>{number(data.summary.heldForRefund)}</p><small>Kondisi saat laporan dimuat</small></article>
        </section>
      </AdminReportDetails>
      <AdminReportDetails title="Rincian per kategori">
        <!-- svelte-ignore a11y_no_noninteractive_tabindex -->
        <div class="history-table-wrap" tabindex="0" role="region" aria-label="Rincian kehadiran per kategori"><table class="history-table sales-table"><thead><tr><th>Kategori</th><th>Kapasitas</th><th>Stok tersedia</th><th>Diterbitkan</th><th>Berhak masuk</th><th>Tertahan refund</th><th>Check-in</th><th>Kehadiran</th></tr></thead><tbody>{#each data.byCategory as item (item.id)}<tr><td>{item.name}</td><td>{number(item.capacity)}</td><td>{number(item.available)}</td><td>{number(item.issued)}</td><td>{number(item.eligible)}</td><td>{number(item.heldForRefund)}</td><td>{number(item.checkedIn)}</td><td>{rate(item.attendanceRate)}</td></tr>{:else}<tr><td colspan="8">Belum ada tiket pada event ini.</td></tr>{/each}</tbody></table></div>
      </AdminReportDetails>
      <AdminReportDetails title="Rincian per gate">
        <!-- svelte-ignore a11y_no_noninteractive_tabindex -->
        <div class="history-table-wrap" tabindex="0" role="region" aria-label="Rincian kehadiran per gate"><table class="history-table sales-table"><thead><tr><th>Gate</th><th>Kapasitas</th><th>Stok tersedia</th><th>Diterbitkan</th><th>Berhak masuk</th><th>Tertahan refund</th><th>Check-in</th><th>Kehadiran</th></tr></thead><tbody>{#each data.byGate as item (item.gate ?? "unknown")}<tr><td>{gateName(item.gate)}</td><td>{number(item.capacity)}</td><td>{number(item.available)}</td><td>{number(item.issued)}</td><td>{number(item.eligible)}</td><td>{number(item.heldForRefund)}</td><td>{number(item.checkedIn)}</td><td>{rate(item.attendanceRate)}</td></tr>{:else}<tr><td colspan="8">Belum ada gate pada event ini.</td></tr>{/each}</tbody></table></div>
      </AdminReportDetails>
      <AdminReportDetails title="Check-in per jam">
        <!-- svelte-ignore a11y_no_noninteractive_tabindex -->
        <div class="history-table-wrap" tabindex="0" role="region" aria-label="Check-in per jam"><table class="history-table sales-table"><thead><tr><th>Jam WIB</th><th>Check-in berhasil</th></tr></thead><tbody>{#each data.hourly as item (item.hour)}<tr><td><time datetime={item.hour}>{hour(item.hour)}</time></td><td>{number(item.checkedIn)}</td></tr>{:else}<tr><td colspan="2">Belum ada check-in berhasil untuk filter ini.</td></tr>{/each}</tbody></table></div>
      </AdminReportDetails>
      <div class="report-export"><button class="scan-submit staff-submit" type="button" onclick={exportCSV} disabled={loading || exporting}>{exporting ? "Menyiapkan CSV…" : "Ekspor CSV"}</button></div>
      <p class="staff-muted">Kehadiran menunjukkan kondisi terkini, bukan rekonstruksi historis. Scan gagal dan scan ulang tidak dihitung sebagai check-in berhasil.</p>
    {:else if !error}<p class="staff-muted" role="status">Pilih event untuk menampilkan laporan kehadiran.</p>{/if}
    <footer class="scan-footer"><span>Ticket Online · Admin tools</span><span>Laporan kondisi tiket dan kehadiran</span></footer>
</AdminLayout>
