<script lang="ts">
  import { onDestroy, onMount } from "svelte";
  import { ApiError, exportAdminAttendanceReportCSV, getAdminAttendanceReport, getAdminEvents, saveAdminReportCSV, type AdminAttendanceReport, type AdminApiEvent } from "../../lib/api.ts";

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
    loading = true; error = ""; data = null;
    try {
      const report = await getAdminAttendanceReport(accessToken, filters, controller.signal);
      if (current !== generation) return;
      data = report;
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
    applied = { eventId, ...(gate ? { gate } : {}) };
    void load(applied);
  }
  function selectEvent(value: string) { eventId = value; gate = ""; gateOptions = []; data = null; applied = null; }
  function reset() { eventId = ""; gate = ""; gateOptions = []; data = null; applied = null; error = ""; }
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
  function rate(value: number | null) { return value === null ? "—" : `${new Intl.NumberFormat("id-ID", { maximumFractionDigits: 1 }).format(value)}%`; }
  function localTime(value: string) { return new Date(value).toLocaleString("id-ID", { timeZone: "Asia/Jakarta", dateStyle: "medium", timeStyle: "short" }) + " WIB"; }
  function hour(value: string) { return new Date(value).toLocaleString("id-ID", { timeZone: "Asia/Jakarta", dateStyle: "medium", hour: "2-digit", minute: "2-digit", hour12: false }) + " WIB"; }
  function gateName(value: string | null) { return value || "Gate tidak diketahui"; }

  onMount(() => { void initialize(); });
  onDestroy(() => { generation++; request?.abort(); });
</script>

<svelte:head><title>Laporan kehadiran | Tiket Online</title><meta name="robots" content="noindex" /></svelte:head>
<div class="scan-shell staff-admin-shell">
  <a class="skip-link" href="#attendance-report-content">Lewati ke laporan</a>
  <header class="scan-topbar"><a class="scan-brand" href="/" aria-label="Kembali ke Tiket Online"><span class="scan-brand-mark" aria-hidden="true">TO</span><span>Tiket Online <b>/ Gate Control</b></span></a><span class="staff-login-label">ADMINISTRATOR</span><button class="staff-text-button" type="button" disabled={loggingOut} onclick={onLogout}>Keluar</button></header>
  <main id="attendance-report-content" class="staff-admin-content">
    <div class="staff-admin-heading"><div><p class="scan-kicker">LAPORAN EVENT <span>•</span> ADMIN</p><h1>Kehadiran dan tiket.</h1><p>Kondisi inventori dan tiket saat ini; kehadiran dihitung dari tiket masuk yang berhasil di-scan.</p></div><div class="report-actions"><button class="staff-secondary-button" type="button" onclick={() => void load()} disabled={loading || !applied}>Muat ulang</button><button class="staff-secondary-button" type="button" onclick={exportCSV} disabled={loading || exporting || !data}>{exporting ? "Menyiapkan CSV…" : "Ekspor CSV"}</button></div></div>
    <nav class="admin-tool-nav" aria-label="Administrasi event"><a href="/admin/events">Konser</a><a href="/admin/orders">Pesanan</a><a href="/admin/issues">Masalah</a><a href="/admin/operations">Operasional</a><a href="/admin/reports">Penjualan dan refund</a><a aria-current="page" href="/admin/reports/attendance">Kehadiran</a><a href="/admin/staff">Kelola petugas</a><a href="/admin/check-ins">Riwayat check-in</a></nav>
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
    {#if loading}<p class="staff-muted" role="status" aria-live="polite">Memuat laporan kehadiran…</p>
    {:else if data}
      <p class="sales-period">{data.event.name} · kondisi terkini <span>Data diperbarui {localTime(data.dataUpdatedAt)}</span></p>
      <section class="sales-metrics" aria-label="Ringkasan kehadiran">
        <article class="sales-metric"><h2>Kapasitas</h2><p>{number(data.summary.capacity)}</p><small>{number(data.summary.available)} stok tersedia</small></article>
        <article class="sales-metric"><h2>Tiket diterbitkan</h2><p>{number(data.summary.issued)}</p><small>Semua status order</small></article>
        <article class="sales-metric"><h2>Berhak masuk</h2><p>{number(data.summary.eligible)}</p><small>Order lunas dan snapshot valid</small></article>
        <article class="sales-metric"><h2>Tertahan karena refund</h2><p>{number(data.summary.heldForRefund)}</p><small>Kondisi saat laporan dimuat</small></article>
        <article class="sales-metric"><h2>Sudah check-in</h2><p>{number(data.summary.checkedIn)}</p><small>{rate(data.summary.attendanceRate)} dari yang berhak masuk</small></article>
      </section>
      <section class="staff-admin-panel checkin-history-panel"><div class="staff-panel-heading"><div><span class="panel-index">02</span><h2>Rincian per kategori</h2></div></div>
        <div class="history-table-wrap"><table class="history-table sales-table"><thead><tr><th>Kategori</th><th>Kapasitas</th><th>Stok tersedia</th><th>Diterbitkan</th><th>Berhak masuk</th><th>Tertahan refund</th><th>Check-in</th><th>Kehadiran</th></tr></thead><tbody>{#each data.byCategory as item (item.id)}<tr><td>{item.name}</td><td>{number(item.capacity)}</td><td>{number(item.available)}</td><td>{number(item.issued)}</td><td>{number(item.eligible)}</td><td>{number(item.heldForRefund)}</td><td>{number(item.checkedIn)}</td><td>{rate(item.attendanceRate)}</td></tr>{:else}<tr><td colspan="8">Belum ada tiket pada event ini.</td></tr>{/each}</tbody></table></div>
      </section>
      <section class="staff-admin-panel checkin-history-panel"><div class="staff-panel-heading"><div><span class="panel-index">03</span><h2>Rincian per gate</h2></div></div>
        <div class="history-table-wrap"><table class="history-table sales-table"><thead><tr><th>Gate</th><th>Kapasitas</th><th>Stok tersedia</th><th>Diterbitkan</th><th>Berhak masuk</th><th>Tertahan refund</th><th>Check-in</th><th>Kehadiran</th></tr></thead><tbody>{#each data.byGate as item (item.gate ?? "unknown")}<tr><td>{gateName(item.gate)}</td><td>{number(item.capacity)}</td><td>{number(item.available)}</td><td>{number(item.issued)}</td><td>{number(item.eligible)}</td><td>{number(item.heldForRefund)}</td><td>{number(item.checkedIn)}</td><td>{rate(item.attendanceRate)}</td></tr>{:else}<tr><td colspan="8">Belum ada gate pada event ini.</td></tr>{/each}</tbody></table></div>
      </section>
      <section class="staff-admin-panel checkin-history-panel"><div class="staff-panel-heading"><div><span class="panel-index">04</span><h2>Check-in per jam</h2></div></div>
        <div class="history-table-wrap"><table class="history-table sales-table"><thead><tr><th>Jam WIB</th><th>Check-in berhasil</th></tr></thead><tbody>{#each data.hourly as item (item.hour)}<tr><td><time datetime={item.hour}>{hour(item.hour)}</time></td><td>{number(item.checkedIn)}</td></tr>{:else}<tr><td colspan="2">Belum ada check-in berhasil untuk filter ini.</td></tr>{/each}</tbody></table></div>
      </section>
      <p class="staff-muted">Kehadiran menunjukkan kondisi terkini, bukan rekonstruksi historis. Scan gagal dan scan ulang tidak dihitung sebagai check-in berhasil.</p>
    {:else if !error}<p class="staff-muted" role="status">Pilih event untuk menampilkan laporan kehadiran.</p>{/if}
    <footer class="scan-footer"><span>Ticket Online · Admin tools</span><span>Laporan kondisi tiket dan kehadiran</span></footer>
  </main>
</div>
