<script lang="ts">
  import AdminLayout from "../../layouts/AdminLayout.svelte";
  import { onDestroy, onMount } from "svelte";
  import { ApiError, getAdminConversionReport, type ConversionBreakdown, type ConversionReport } from "../../lib/api.ts";
  import AdminReportDetails from "../../components/admin/AdminReportDetails.svelte";

  let { accessToken, onUnauthorized, onLogout, loggingOut }: { accessToken: string; onUnauthorized: () => void; onLogout: () => void; loggingOut: boolean } = $props();
  function todayWIB() { const parts = new Intl.DateTimeFormat("en-CA", { timeZone: "Asia/Jakarta", year: "numeric", month: "2-digit", day: "2-digit" }).formatToParts(new Date()); return `${parts.find((p) => p.type === "year")?.value}-${parts.find((p) => p.type === "month")?.value}-${parts.find((p) => p.type === "day")?.value}`; }
  function shiftDate(value: string, days: number) { const date = new Date(`${value}T12:00:00+07:00`); date.setUTCDate(date.getUTCDate() + days); return date.toISOString().slice(0, 10); }
  const today = todayWIB();
  let eventId = $state(""); let device = $state(""); let dateFrom = $state(shiftDate(today, -29)); let dateTo = $state(today);
  let applied = $state({ eventId: "", device: "", dateFrom: shiftDate(today, -29), dateTo: today }); let lastRequested = $state<typeof applied | null>(null);
  let report = $state<ConversionReport | null>(null); let eventOptions = $state<Array<{ eventId?: string; eventName?: string }>>([]); let loading = $state(true); let error = $state(""); let request: AbortController | undefined; let generation = 0;
  async function load(filters = applied) {
    const requested = { ...filters }; lastRequested = requested;
    request?.abort(); const controller = new AbortController(); request = controller; const current = ++generation; loading = true; error = "";
    try { const value = await getAdminConversionReport(accessToken, requested, controller.signal); if (current === generation) { report = value; applied = requested; eventOptions = [...new Map([...eventOptions, ...value.byEvent].map((item) => [item.eventId, item])).values()]; } }
    catch (cause) { if (current !== generation || cause instanceof DOMException && cause.name === "AbortError") return; if (cause instanceof ApiError && cause.status === 401) onUnauthorized(); else error = cause instanceof ApiError ? cause.message : "Laporan belum dapat dimuat. Periksa koneksi lalu coba lagi."; }
    finally { if (current === generation) { loading = false; request = undefined; } }
  }
  function search(event: SubmitEvent) { event.preventDefault(); void load({ eventId, device, dateFrom, dateTo }); }
  function retry() { if (lastRequested) void load(lastRequested); }
  function count(value: number) { return value.toLocaleString("id-ID"); }
  function rate(value: number, previous: number) { return previous ? `${new Intl.NumberFormat("id-ID", { maximumFractionDigits: 1 }).format(value / previous * 100)}%` : "Belum dapat dihitung"; }
  function losses(row: ConversionBreakdown) { return [row.lost.detailToReservation, row.lost.reservationToOrder, row.lost.orderToPayment, row.lost.paymentToSuccess]; }
  function blockerLabel(kind: string, reason: string) {
    const labels: Record<string, string> = {
      "STOCK_UNAVAILABLE:RESERVATION": "Stok tidak tersedia saat reservasi.",
      "RESERVATION_EXPIRED:RESERVATION": "Reservasi kedaluwarsa sebelum checkout selesai.",
      "VALIDATION_FAILED:BUYER_DATA": "Data pembeli tidak lolos validasi.",
      "VALIDATION_FAILED:ATTENDEE_DATA": "Data peserta tidak lolos validasi.",
      "PAYMENT_FAILURE:PAYMENT_PROVIDER": "Penyedia pembayaran gagal mengonfirmasi pembayaran.",
      "SERVICE_FAILURE:SERVICE": "Layanan mengalami gangguan.",
    };
    return labels[`${kind}:${reason}`] ?? "Hambatan tercatat; alasan belum dikenali.";
  }
  const matured = $derived(report?.summary.matured);
  const zeroJourneys = $derived(Boolean(report && !report.summary.total.detail && !report.summary.pendingObservation));
  const draftChanged = $derived(Boolean(report && (eventId !== applied.eventId || device !== applied.device || dateFrom !== applied.dateFrom || dateTo !== applied.dateTo)));
  const stages = ["Detail konser", "Reservasi", "Order", "Pembayaran dimulai", "Pembayaran berhasil"] as const;
  onMount(() => { void load(applied); });
  onDestroy(() => { generation++; request?.abort(); });
</script>

<svelte:head><title>Konversi | Tiket Online</title><meta name="robots" content="noindex" /></svelte:head>
  <AdminLayout page="conversion" contentId="conversion-content" skipLabel="Lewati ke laporan" kicker="LAPORAN PERJALANAN • ADMIN" title="Konversi pembeli." description="Tahapan mengikuti perjalanan anonim per tab dan event. Periode memakai tanggal WIB; hasil dianggap matang setelah 24 jam." onLogout={onLogout} loggingOut={loggingOut} primaryAction={{ label: "Muat ulang", onclick: () => void load(), disabled: loading }}>
    <form class="history-filter-form sales-report-filters report-filter-form" onsubmit={search}>
      <label>Event<select bind:value={eventId}><option value="">Semua event</option>{#each eventOptions as event (event.eventId)}<option value={event.eventId}>{event.eventName}</option>{/each}</select></label>
      <label>Perangkat<select bind:value={device}><option value="">Semua perangkat</option><option value="mobile">Mobile</option><option value="desktop">Desktop</option><option value="unknown">Tidak diketahui</option></select></label>
      <label>Dari tanggal WIB<input type="date" bind:value={dateFrom} required /></label><label>Sampai tanggal WIB<input type="date" bind:value={dateTo} required /></label>
      <div class="history-filter-actions"><button class="scan-submit staff-submit" type="submit" disabled={loading}>Terapkan</button></div>
    </form>
    {#if error}<p class="staff-form-message staff-form-error" role="alert">{error} <button class="staff-text-button" type="button" onclick={retry}>Coba lagi</button></p>{/if}
    {#if loading && !report}<p class="staff-muted" role="status" aria-live="polite">Memuat laporan…</p>{/if}
    {#if report}
      <p class="sales-period">Cohort {report.period.dateFrom} sampai {report.period.dateTo} WIB · Jendela {report.period.observationHours} jam <span>Data diperbarui {new Date(report.dataUpdatedAt).toLocaleString("id-ID", { timeZone: "Asia/Jakarta", dateStyle: "medium", timeStyle: "short" })} WIB</span></p>
      {#if draftChanged}<p class="report-refresh-note" role="status">Filter berubah. Tekan Terapkan untuk memperbarui laporan; ringkasan masih memakai cakupan di atas.</p>{/if}
      {#if loading}<p class="report-refresh-note" role="status" aria-live="polite">Memperbarui laporan. Angka pada layar masih memakai filter dan snapshot sebelumnya.</p>{:else if error}<p class="report-refresh-note" role="status">Pembaruan gagal. Snapshot terakhir tetap ditampilkan.</p>{/if}
      {#if zeroJourneys}<p class="report-empty-state" role="status">Belum ada perjalanan pada cakupan ini.</p>{/if}
      <section class="sales-metrics conversion-primary-metrics" aria-label="Ringkasan konversi">
        <article class="sales-metric"><h2>Pembayaran berhasil</h2><p>{count(matured?.paymentSucceeded ?? 0)}</p><small>Perjalanan matang</small></article>
        <article class="sales-metric"><h2>Menunggu observasi</h2><p>{count(report.summary.pendingObservation)}</p><small>Belum masuk hitungan matang</small></article>
        <article class="sales-metric"><h2>Perjalanan matang</h2><p>{count(matured?.detail ?? 0)}</p><small>Detail konser dalam jendela observasi</small></article>
      </section>
      <AdminReportDetails title="Tahap perjalanan">
        <p class="report-definition">Rasio memakai tahap sebelumnya sebagai penyebut. Hanya perjalanan matang yang dihitung.</p>
        <!-- svelte-ignore a11y_no_noninteractive_tabindex -->
        <div class="history-table-wrap" tabindex="0" role="region" aria-label="Tahap perjalanan pembeli"><table class="history-table sales-table"><thead><tr><th>Tahap</th><th>Matang</th><th>Konversi dari tahap sebelumnya</th></tr></thead><tbody>{#each stages as stage, index}<tr><td>{stage}</td><td>{count([report.summary.matured.detail, report.summary.matured.reservation, report.summary.matured.order, report.summary.matured.paymentStarted, report.summary.matured.paymentSucceeded][index] ?? 0)}</td><td>{index === 0 ? (report.summary.matured.detail ? "100%" : "Belum dapat dihitung") : rate([report.summary.matured.detail, report.summary.matured.reservation, report.summary.matured.order, report.summary.matured.paymentStarted, report.summary.matured.paymentSucceeded][index] ?? 0, [report.summary.matured.detail, report.summary.matured.reservation, report.summary.matured.order, report.summary.matured.paymentStarted, report.summary.matured.paymentSucceeded][index - 1] ?? 0)}</td></tr>{/each}</tbody></table></div>
        {#if report.period.device}<p class="staff-muted">Jumlah tanpa atribusi tidak dapat dibagi menurut perangkat; pilih “Semua perangkat” untuk melihat cakupan keseluruhan.</p>{:else}<p class="staff-muted">Jumlah transaksi tanpa atribusi dan yang tidak tersedia: {report.unattributedReservations === null ? "reservasi belum tersedia" : `${count(report.unattributedReservations)} reservasi`}, {report.unattributedPayments === null ? "pembayaran belum tersedia" : `${count(report.unattributedPayments)} pembayaran`}. Nilai ini tidak digabungkan ke rasio perjalanan.</p>{/if}
      </AdminReportDetails>
      <AdminReportDetails title="Di mana perjalanan berhenti?">
        <!-- svelte-ignore a11y_no_noninteractive_tabindex -->
        <div class="history-table-wrap" tabindex="0" role="region" aria-label="Rincian perjalanan yang tidak lanjut"><table class="history-table sales-table"><thead><tr><th>Antartahap</th><th>Perjalanan matang yang tidak lanjut</th></tr></thead><tbody>{#each ["Detail → reservasi", "Reservasi → order", "Order → pembayaran dimulai", "Pembayaran dimulai → berhasil"] as label, index}<tr><td>{label}</td><td>{count(losses(report.summary)[index] ?? 0)}</td></tr>{/each}</tbody></table></div>
        <p class="staff-muted">Kehilangan adalah perilaku teramati, bukan alasan yang dipastikan. Hambatan tercatat terpisah dan dapat tumpang tindih.</p>
      </AdminReportDetails>
      <AdminReportDetails title="Hambatan tercatat">
        <!-- svelte-ignore a11y_no_noninteractive_tabindex -->
        <div class="history-table-wrap" tabindex="0" role="region" aria-label="Hambatan perjalanan yang tercatat"><table class="history-table sales-table"><thead><tr><th>Kategori</th><th>Jumlah perjalanan</th></tr></thead><tbody>{#each report.blockers as blocker (`${blocker.kind}-${blocker.reason}`)}<tr><td><span>{blockerLabel(blocker.kind, blocker.reason)}</span><details class="report-technical-detail"><summary>Kode teknis</summary><code>{blocker.kind} · {blocker.reason}</code></details></td><td>{count(blocker.count)}</td></tr>{:else}<tr><td colspan="2">Belum ada hambatan tercatat.</td></tr>{/each}</tbody></table></div>
      </AdminReportDetails>
      <AdminReportDetails title="Per event">
        <!-- svelte-ignore a11y_no_noninteractive_tabindex -->
        <div class="history-table-wrap" tabindex="0" role="region" aria-label="Perjalanan matang per event"><table class="history-table sales-table"><thead><tr><th>Event</th><th>Detail</th><th>Reservasi</th><th>Order</th><th>Bayar mulai</th><th>Berhasil</th></tr></thead><tbody>{#each report.byEvent as row (row.eventId)}<tr><td>{row.eventName}</td><td>{count(row.matured.detail)}</td><td>{count(row.matured.reservation)}</td><td>{count(row.matured.order)}</td><td>{count(row.matured.paymentStarted)}</td><td>{count(row.matured.paymentSucceeded)}</td></tr>{:else}<tr><td colspan="6">Belum ada perjalanan detail yang matang.</td></tr>{/each}</tbody></table></div>
      </AdminReportDetails>
      <AdminReportDetails title="Per perangkat">
        <!-- svelte-ignore a11y_no_noninteractive_tabindex -->
        <div class="history-table-wrap" tabindex="0" role="region" aria-label="Perjalanan matang per perangkat"><table class="history-table sales-table"><thead><tr><th>Perangkat</th><th>Detail</th><th>Reservasi</th><th>Order</th><th>Bayar mulai</th><th>Berhasil</th></tr></thead><tbody>{#each report.byDevice as row (row.device)}<tr><td>{row.device === "mobile" ? "Mobile" : row.device === "desktop" ? "Desktop" : "Tidak diketahui"}</td><td>{count(row.matured.detail)}</td><td>{count(row.matured.reservation)}</td><td>{count(row.matured.order)}</td><td>{count(row.matured.paymentStarted)}</td><td>{count(row.matured.paymentSucceeded)}</td></tr>{:else}<tr><td colspan="6">Belum ada data perangkat.</td></tr>{/each}</tbody></table></div>
      </AdminReportDetails>
    {/if}
</AdminLayout>
