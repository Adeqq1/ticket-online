<script lang="ts">
  import { onDestroy, onMount } from "svelte";
  import { ApiError, getAdminConversionReport, type ConversionBreakdown, type ConversionReport } from "../../lib/api.ts";

  let { accessToken, onUnauthorized, onLogout, loggingOut }: { accessToken: string; onUnauthorized: () => void; onLogout: () => void; loggingOut: boolean } = $props();
  function todayWIB() { const parts = new Intl.DateTimeFormat("en-CA", { timeZone: "Asia/Jakarta", year: "numeric", month: "2-digit", day: "2-digit" }).formatToParts(new Date()); return `${parts.find((p) => p.type === "year")?.value}-${parts.find((p) => p.type === "month")?.value}-${parts.find((p) => p.type === "day")?.value}`; }
  function shiftDate(value: string, days: number) { const date = new Date(`${value}T12:00:00+07:00`); date.setUTCDate(date.getUTCDate() + days); return date.toISOString().slice(0, 10); }
  const today = todayWIB();
  let eventId = $state(""); let device = $state(""); let dateFrom = $state(shiftDate(today, -29)); let dateTo = $state(today);
  let report = $state<ConversionReport | null>(null); let eventOptions = $state<Array<{ eventId?: string; eventName?: string }>>([]); let loading = $state(true); let error = $state(""); let request: AbortController | undefined; let generation = 0;
  async function load(filters = { eventId, device, dateFrom, dateTo }) {
    request?.abort(); const controller = new AbortController(); request = controller; const current = ++generation; loading = true; error = ""; report = null;
    try { const value = await getAdminConversionReport(accessToken, filters, controller.signal); if (current === generation) { report = value; eventOptions = [...new Map([...eventOptions, ...value.byEvent].map((item) => [item.eventId, item])).values()]; } }
    catch (cause) { if (current !== generation || cause instanceof DOMException && cause.name === "AbortError") return; if (cause instanceof ApiError && cause.status === 401) onUnauthorized(); else error = cause instanceof ApiError ? cause.message : "Laporan belum dapat dimuat. Periksa koneksi lalu coba lagi."; }
    finally { if (current === generation) { loading = false; request = undefined; } }
  }
  function search(event: SubmitEvent) { event.preventDefault(); void load({ eventId, device, dateFrom, dateTo }); }
  function count(value: number) { return value.toLocaleString("id-ID"); }
  function rate(value: number, previous: number) { return previous ? `${(value / previous * 100).toFixed(1)}%` : "—"; }
  function losses(row: ConversionBreakdown) { return [row.lost.detailToReservation, row.lost.reservationToOrder, row.lost.orderToPayment, row.lost.paymentToSuccess]; }
  const stages = ["Detail konser", "Reservasi", "Order", "Pembayaran dimulai", "Pembayaran berhasil"] as const;
  onMount(() => { void load({ eventId: "", device: "", dateFrom: shiftDate(today, -29), dateTo: today }); });
  onDestroy(() => { generation++; request?.abort(); });
</script>

<svelte:head><title>Konversi | Tiket Online</title><meta name="robots" content="noindex" /></svelte:head>
<div class="scan-shell staff-admin-shell">
  <a class="skip-link" href="#conversion-content">Lewati ke laporan</a>
  <header class="scan-topbar"><a class="scan-brand" href="/" aria-label="Kembali ke Tiket Online"><span class="scan-brand-mark" aria-hidden="true">TO</span><span>Tiket Online <b>/ Gate Control</b></span></a><span class="staff-login-label">ADMINISTRATOR</span><button class="staff-text-button" type="button" disabled={loggingOut} onclick={onLogout}>Keluar</button></header>
  <main id="conversion-content" class="staff-admin-content">
    <div class="staff-admin-heading"><div><p class="scan-kicker">LAPORAN PERJALANAN <span>•</span> ADMIN</p><h1>Konversi pembeli.</h1><p>Tahapan mengikuti perjalanan anonim per tab dan event. Periode memakai tanggal WIB; hasil dianggap matang setelah 24 jam.</p></div><button class="staff-secondary-button" type="button" onclick={() => void load()} disabled={loading}>Muat ulang</button></div>
    <nav class="admin-tool-nav" aria-label="Administrasi event"><a href="/admin/events">Konser</a><a href="/admin/orders">Pesanan</a><a href="/admin/issues">Masalah</a><a href="/admin/operations">Operasional</a><a href="/admin/reports">Penjualan dan refund</a><a href="/admin/reports/attendance">Kehadiran</a><a aria-current="page" href="/admin/reports/conversion">Konversi</a><a href="/admin/staff">Kelola petugas</a></nav>
    <form class="history-filter-form sales-report-filters" onsubmit={search}>
      <label>Event<select bind:value={eventId}><option value="">Semua event</option>{#each eventOptions as event (event.eventId)}<option value={event.eventId}>{event.eventName}</option>{/each}</select></label>
      <label>Perangkat<select bind:value={device}><option value="">Semua perangkat</option><option value="mobile">Mobile</option><option value="desktop">Desktop</option><option value="unknown">Tidak diketahui</option></select></label>
      <label>Dari tanggal WIB<input type="date" bind:value={dateFrom} required /></label><label>Sampai tanggal WIB<input type="date" bind:value={dateTo} required /></label>
      <div class="history-filter-actions"><button class="scan-submit staff-submit" type="submit" disabled={loading}>Terapkan</button></div>
    </form>
    {#if error}<p class="staff-form-message staff-form-error" role="alert">{error} <button class="staff-text-button" type="button" onclick={() => void load()}>Coba lagi</button></p>{/if}
    {#if loading}<p class="staff-muted" role="status" aria-live="polite">Memuat laporan…</p>{:else if report}
      <p class="sales-period">Cohort {report.period.dateFrom}–{report.period.dateTo} WIB · Jendela 24 jam <span>Data diperbarui {new Date(report.dataUpdatedAt).toLocaleString("id-ID", { timeZone: "Asia/Jakarta", dateStyle: "medium", timeStyle: "short" })} WIB</span></p>
      <section class="staff-admin-panel checkin-history-panel"><div class="staff-panel-heading"><div><span class="panel-index">01</span><h2>Tahap perjalanan</h2></div></div>
        <p class="staff-muted">Angka utama hanya menghitung perjalanan matang. {count(report.summary.pendingObservation)} perjalanan masih menunggu jendela observasi. Rasio memakai tahap sebelumnya sebagai penyebut.</p>
        <div class="history-table-wrap"><table class="history-table sales-table"><thead><tr><th>Tahap</th><th>Matang</th><th>Konversi dari tahap sebelumnya</th></tr></thead><tbody>{#each stages as stage, index}<tr><td>{stage}</td><td>{count([report.summary.matured.detail, report.summary.matured.reservation, report.summary.matured.order, report.summary.matured.paymentStarted, report.summary.matured.paymentSucceeded][index] ?? 0)}</td><td>{index === 0 ? "100%" : rate([report.summary.matured.detail, report.summary.matured.reservation, report.summary.matured.order, report.summary.matured.paymentStarted, report.summary.matured.paymentSucceeded][index] ?? 0, [report.summary.matured.detail, report.summary.matured.reservation, report.summary.matured.order, report.summary.matured.paymentStarted, report.summary.matured.paymentSucceeded][index - 1] ?? 0)}</td></tr>{/each}</tbody></table></div>
        {#if report.period.device}<p class="staff-muted">Jumlah tanpa atribusi tidak dapat dibagi menurut perangkat; pilih “Semua perangkat” untuk melihat cakupan keseluruhan.</p>{:else}<p class="staff-muted">Jumlah tanpa atribusi pada periode transaksi tidak digabungkan ke rasio perjalanan: {count(report.unattributedReservations ?? 0)} reservasi dan {count(report.unattributedPayments ?? 0)} pembayaran.</p>{/if}
      </section>
      <section class="staff-admin-panel checkin-history-panel"><div class="staff-panel-heading"><div><span class="panel-index">02</span><h2>Di mana perjalanan berhenti?</h2></div></div>
        <div class="history-table-wrap"><table class="history-table sales-table"><thead><tr><th>Antartahap</th><th>Perjalanan matang yang tidak lanjut</th></tr></thead><tbody>{#each ["Detail → reservasi", "Reservasi → order", "Order → pembayaran dimulai", "Pembayaran dimulai → berhasil"] as label, index}<tr><td>{label}</td><td>{count(losses(report.summary)[index] ?? 0)}</td></tr>{/each}</tbody></table></div>
        <p class="staff-muted">Kehilangan adalah perilaku teramati, bukan alasan yang dipastikan. Hambatan tercatat terpisah dan dapat tumpang tindih.</p>
      </section>
      <section class="staff-admin-panel checkin-history-panel"><div class="staff-panel-heading"><div><span class="panel-index">03</span><h2>Hambatan tercatat</h2></div></div>
        <div class="history-table-wrap"><table class="history-table sales-table"><thead><tr><th>Kategori</th><th>Jumlah perjalanan</th></tr></thead><tbody>{#each report.blockers as blocker (`${blocker.kind}-${blocker.reason}`)}<tr><td>{blocker.kind === "STOCK_UNAVAILABLE" ? "Stok habis" : blocker.kind === "RESERVATION_EXPIRED" ? "Reservasi kedaluwarsa" : blocker.kind === "VALIDATION_FAILED" ? "Validasi gagal" : blocker.kind === "PAYMENT_FAILURE" ? "Gangguan pembayaran" : "Gangguan layanan"} · {blocker.reason}</td><td>{count(blocker.count)}</td></tr>{:else}<tr><td colspan="2">Belum ada hambatan tercatat.</td></tr>{/each}</tbody></table></div>
      </section>
      <section class="staff-admin-panel checkin-history-panel"><div class="staff-panel-heading"><div><span class="panel-index">04</span><h2>Per event</h2></div></div>
        <div class="history-table-wrap"><table class="history-table sales-table"><thead><tr><th>Event</th><th>Detail</th><th>Reservasi</th><th>Order</th><th>Bayar mulai</th><th>Berhasil</th></tr></thead><tbody>{#each report.byEvent as row (row.eventId)}<tr><td>{row.eventName}</td><td>{count(row.matured.detail)}</td><td>{count(row.matured.reservation)}</td><td>{count(row.matured.order)}</td><td>{count(row.matured.paymentStarted)}</td><td>{count(row.matured.paymentSucceeded)}</td></tr>{:else}<tr><td colspan="6">Belum ada perjalanan detail yang matang.</td></tr>{/each}</tbody></table></div>
        <div class="history-table-wrap"><table class="history-table sales-table"><thead><tr><th>Perangkat</th><th>Detail</th><th>Reservasi</th><th>Order</th><th>Bayar mulai</th><th>Berhasil</th></tr></thead><tbody>{#each report.byDevice as row (row.device)}<tr><td>{row.device === "mobile" ? "Mobile" : row.device === "desktop" ? "Desktop" : "Tidak diketahui"}</td><td>{count(row.matured.detail)}</td><td>{count(row.matured.reservation)}</td><td>{count(row.matured.order)}</td><td>{count(row.matured.paymentStarted)}</td><td>{count(row.matured.paymentSucceeded)}</td></tr>{:else}<tr><td colspan="6">Belum ada data perangkat.</td></tr>{/each}</tbody></table></div>
      </section>
    {/if}
  </main>
</div>
