<script lang="ts">
  import AdminLayout from "../../layouts/AdminLayout.svelte";
  import { wibDateTime } from "../../lib/event-changes.ts";
  import { onDestroy, onMount, tick } from "svelte";
  import { ApiError, getAdminOrder, getAdminOrders, completeManualRefund, requestAdminRefund, type AdminOrder, type AdminOrderDetail, type AdminOrderFilter, type AdminOrderPage, type OrderStatus } from "../../lib/api.ts";
  import { applyIfCurrent } from "../../lib/admin-order-requests.ts";
  import { orderStatusLabel, paymentStatusLabel, refundStatus } from "../../lib/admin-operations.ts";

  let { accessToken, onUnauthorized, onLogout, loggingOut }: { accessToken: string; onUnauthorized: () => void; onLogout: () => void; loggingOut: boolean } = $props();
  let items = $state<AdminOrder[]>([]);
  let events = $state<AdminOrderPage["filterOptions"]["events"]>([]);
  let nextCursor = $state<string | null>(null);
  let selected = $state<AdminOrderDetail | null>(null);
  let selectedId = $state("");
  let query = $state("");
  let eventId = $state("");
  let status = $state<"" | OrderStatus>("");
  let dateFrom = $state("");
  let dateTo = $state("");
  let applied: AdminOrderFilter = {};
  let loading = $state(true);
  let loadingMore = $state(false);
  let detailLoading = $state(false);
  let error = $state("");
  let detailError = $state("");
  let refundReason = $state("");
  let refundBusy = $state(false);
  let refundMessage = $state("");
  let manualReference = $state("");
  let manualPaidAt = $state("");
  let manualNote = $state("");
  let manualConfirmed = $state(false);
  let manualResult = $state<HTMLParagraphElement>();
  let uncertainRefund = $state(false);
  let uncertainManual = $state(false);
  let lastDetailTrigger: HTMLElement | null = null;
  let detailHeading: HTMLElement | undefined = $state();

  async function finishManual() {
    if (!selected || refundBusy || !manualConfirmed) return;
    const id = selected.id;
    if (!window.confirm(`Konfirmasi pencatatan refund ${money(selected.refund?.amount ?? selected.total)} untuk ${selected.reference}? Tindakan ini mencatat bukti transfer yang sudah berhasil dan tidak mengirim uang.`)) return;
    refundBusy = true; refundMessage = "";
    const input = { reference: manualReference.trim(), paidAt: wibDateTime(manualPaidAt), note: manualNote.trim() };
    try {
      await completeManualRefund(accessToken, id, input);
      if (selectedId === id) {
        uncertainManual = false;
        refundMessage = "Refund manual tercatat dan tiket tetap dinonaktifkan.";
        manualReference = ""; manualPaidAt = ""; manualNote = ""; manualConfirmed = false;
        await openDetail(id);
      }
      await load(applied);
    } catch (cause) {
      if (cause instanceof ApiError && cause.status === 401) onUnauthorized();
      else if (selectedId === id) {
        uncertainManual = cause instanceof ApiError && (cause.code === "UNKNOWN_OUTCOME" || cause.code === "NETWORK_ERROR" || cause.status >= 500);
        refundMessage = uncertainManual ? "Hasil pencatatan belum pasti. Periksa detail pesanan sebelum mencoba tindakan lagi." : cause instanceof Error ? cause.message : "Hasil belum pasti. Muat ulang detail pesanan.";
      }
    } finally { refundBusy = false; await tick(); if (selectedId === id) manualResult?.focus(); }
  }
  let request: AbortController | undefined;
  let detailRequest: AbortController | undefined;
  let generation = 0;
  let detailGeneration = 0;

  function filters(): AdminOrderFilter { return { ...(query.trim() ? { q: query.trim() } : {}), ...(eventId ? { eventId } : {}), ...(status ? { status } : {}), ...(dateFrom ? { dateFrom } : {}), ...(dateTo ? { dateTo } : {}) }; }

  async function load(value = applied, more = false) {
    request?.abort();
    const controller = new AbortController(); request = controller;
    const current = ++generation;
    if (more) loadingMore = true; else { loading = true; items = []; nextCursor = null; }
    error = "";
    try {
      const result = await getAdminOrders(accessToken, { ...value, ...(more && nextCursor ? { cursor: nextCursor } : {}) }, controller.signal);
      if (current !== generation) return;
      events = result.filterOptions.events;
      items = more ? [...items, ...result.items] : result.items;
      nextCursor = result.nextCursor;
    } catch (cause) {
      if (current !== generation || (cause instanceof DOMException && cause.name === "AbortError")) return;
      if (cause instanceof ApiError && cause.status === 401) onUnauthorized();
      else if (cause instanceof ApiError && cause.status === 403) error = "Akses pesanan hanya tersedia untuk admin.";
      else error = cause instanceof ApiError ? cause.message : "Pesanan belum dapat dimuat. Periksa koneksi lalu coba lagi.";
    } finally { if (current === generation) { loading = false; loadingMore = false; if (request === controller) request = undefined; } }
  }

  function search(event: SubmitEvent) { event.preventDefault(); applied = filters(); closeDetail(); void load(applied); }
  function reset() { query = ""; eventId = ""; status = ""; dateFrom = ""; dateTo = ""; applied = {}; closeDetail(); void load(applied); }

  async function closeDetail() { detailRequest?.abort(); detailGeneration++; selected = null; selectedId = ""; detailError = ""; refundReason = ""; refundMessage = ""; await tick(); lastDetailTrigger?.focus(); }

  async function openDetail(id: string) {
    if (selectedId !== id) lastDetailTrigger = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    detailRequest?.abort();
    const controller = new AbortController(); detailRequest = controller;
    const current = ++detailGeneration;
    if (selectedId !== id) { refundReason = ""; refundMessage = ""; manualReference = ""; manualPaidAt = ""; manualNote = ""; manualConfirmed = false; uncertainRefund = false; uncertainManual = false; }
    selectedId = id; selected = null; detailLoading = true; detailError = "";
    try {
      await applyIfCurrent(() => current === detailGeneration, () => getAdminOrder(accessToken, id, controller.signal), async (result) => { selected = result; await tick(); detailHeading?.focus(); });
    }
    catch (cause) {
      if (current !== detailGeneration || (cause instanceof DOMException && cause.name === "AbortError")) return;
      if (cause instanceof ApiError && cause.status === 401) onUnauthorized();
      else detailError = cause instanceof ApiError ? cause.message : "Detail pesanan belum dapat dimuat.";
    } finally { if (current === detailGeneration) { detailLoading = false; if (detailRequest === controller) detailRequest = undefined; } }
  }

  function money(value: number) { return new Intl.NumberFormat("id-ID", { style: "currency", currency: "IDR", maximumFractionDigits: 0 }).format(value); }
  function localTime(value: string) { return new Date(value).toLocaleString("id-ID", { timeZone: "Asia/Jakarta", dateStyle: "medium", timeStyle: "short" }) + " WIB"; }
  function statusLabel(value: OrderStatus) { return orderStatusLabel(value); }

  async function submitRefund() {
    if (!selected || refundBusy || uncertainRefund || refundReason.trim().length < 3) return;
    if (!window.confirm(`Ajukan refund penuh ${money(selected.total)} untuk ${selected.reference}? Alasan: ${refundReason.trim()}. Tiket akan ditahan sampai Midtrans mengonfirmasi hasil.`)) return;
    const orderId = selected.id;
    const reason = refundReason;
    refundBusy = true; refundMessage = "";
    try {
      await applyIfCurrent(() => selectedId === orderId, () => requestAdminRefund(accessToken, orderId, reason), async (result) => {
        refundMessage = `${refundStatus(result.status).label} · ${money(result.amount)}. ${refundStatus(result.status).next}`;
        await openDetail(orderId);
      });
      await load(applied);
    } catch (cause) {
      if (selectedId === orderId) {
        uncertainRefund = cause instanceof ApiError && (cause.code === "UNKNOWN_OUTCOME" || cause.code === "NETWORK_ERROR" || cause.status >= 500);
        refundMessage = uncertainRefund ? "Hasil pengajuan belum pasti. Periksa ulang detail pesanan; pengajuan tidak akan diulang otomatis." : cause instanceof ApiError ? cause.message : "Status refund belum dapat dipastikan. Periksa kembali detail pesanan.";
        if (uncertainRefund) await openDetail(orderId);
      }
    }
    finally { refundBusy = false; }
  }

  async function checkUncertain(kind: "refund" | "manual") {
    if (!selected || refundBusy) return;
    const id = selected.id;
    refundBusy = true;
    try {
      await openDetail(id);
      if (selectedId === id && selected) {
        if (kind === "refund" && selected.refund) { uncertainRefund = false; refundMessage = `${refundStatus(selected.refund.status).label} · ${money(selected.refund.amount)}. ${refundStatus(selected.refund.status).next}`; }
        else if (kind === "manual" && selected.refund?.manualReference) { uncertainManual = false; refundMessage = "Bukti transfer sudah tercatat."; }
        else refundMessage = "Detail terbaru belum membuktikan hasil tindakan. Pengulangan tetap ditahan.";
      }
    } finally { refundBusy = false; }
  }

  onMount(() => { const orderId = new URLSearchParams(location.search).get("orderId"); void load({}); if (orderId && /^[0-9a-f]{32}$/.test(orderId)) void openDetail(orderId); });
  onDestroy(() => { generation++; detailGeneration++; request?.abort(); detailRequest?.abort(); });
</script>

<svelte:head><title>Pesanan | Tiket Online</title><meta name="robots" content="noindex" /></svelte:head>

  <AdminLayout page="orders" contentId="order-admin-content" skipLabel="Lewati ke pengelolaan pesanan" kicker="OPERASIONAL EVENT • ADMIN" title="Pengelolaan pesanan." description="Telusuri pembayaran dan status tiket pesanan." onLogout={onLogout} loggingOut={loggingOut} primaryAction={{ label: "Cari", form: "order-search-form", disabled: loading }}>
    <section class="staff-admin-panel checkin-history-panel" aria-labelledby="order-filter-title">
      <div class="staff-panel-heading"><div><span class="panel-index">01</span><h2 id="order-filter-title">Cari pesanan</h2></div></div>
      <form id="order-search-form" class="order-filter-form" onsubmit={search} aria-busy={loading}>
        <label>Reference<input bind:value={query} maxlength="32" placeholder="Cari reference pesanan" disabled={loading} /></label>
        <label>Event<select bind:value={eventId} disabled={loading}><option value="">Semua event</option>{#each events as event (event.id)}<option value={event.id}>{event.name}</option>{/each}</select></label>
        <label>Status<select bind:value={status} disabled={loading}><option value="">Semua status</option><option value="PENDING">Menunggu pembayaran</option><option value="PAID">Lunas</option><option value="CANCELLED">Dibatalkan</option><option value="EXPIRED">Kedaluwarsa</option><option value="REFUND_PENDING">Refund diproses</option><option value="REFUNDED">Refund berhasil</option></select></label>
        <label>Dibuat sejak (WIB)<input type="date" bind:value={dateFrom} disabled={loading} /></label>
        <label>Dibuat sampai (WIB)<input type="date" bind:value={dateTo} disabled={loading} /></label>
        <div class="history-filter-actions"><button class="scan-submit" type="submit" disabled={loading}>Cari</button><button class="staff-secondary-button" type="button" onclick={reset} disabled={loading}>Reset</button></div>
      </form>
    </section>
    <div class:has-selected={selectedId} class="order-workspace">
    <section class="staff-admin-panel checkin-history-panel order-list-panel" aria-labelledby="order-results-title">
      <div class="staff-panel-heading"><div><span class="panel-index">02</span><h2 id="order-results-title">Daftar pesanan</h2></div><button class="staff-text-button" type="button" onclick={() => load(applied)} disabled={loading}>Muat ulang</button></div>
      {#if loading}<p class="staff-muted" role="status">Memuat pesanan…</p>
      {:else if error && !items.length}<p class="staff-form-message staff-form-error" role="alert">{error} <button class="staff-text-button" type="button" onclick={() => load(applied)}>Coba lagi</button></p>
      {:else if !items.length}<p class="staff-muted" role="status">Belum ada pesanan yang cocok.</p>
      {:else}<div class="history-table-wrap"><table class="history-table"><caption class="visually-hidden">Daftar pesanan terbaru</caption><thead><tr><th scope="col">Reference</th><th scope="col">Event</th><th scope="col">Pembeli</th><th scope="col">Dibuat (WIB)</th><th scope="col">Tiket</th><th scope="col">Total</th><th scope="col">Status</th><th scope="col">Detail</th></tr></thead><tbody>{#each items as item (item.id)}<tr><td data-label="Reference"><code>{item.reference}</code><small>{item.eventName} · {item.buyerName}</small></td><td data-label="Event">{item.eventName}</td><td data-label="Pembeli">{item.buyerName}</td><td data-label="Dibuat (WIB)"><time datetime={item.createdAt}>{localTime(item.createdAt)}</time></td><td data-label="Tiket">{item.ticketCount}</td><td data-label="Total">{money(item.total)}</td><td data-label="Status">{statusLabel(item.status)}</td><td data-label="Detail"><button class="staff-text-button" type="button" aria-label={`Buka pesanan ${item.reference}`} aria-expanded={selectedId === item.id} onclick={() => selectedId === item.id ? closeDetail() : openDetail(item.id)}>Lihat</button></td></tr>{/each}</tbody></table></div>
        {#if nextCursor}<button class="staff-secondary-button order-load-more" type="button" disabled={loadingMore} onclick={() => load(applied, true)}>{loadingMore ? "Memuat…" : "Muat berikutnya"}</button>{/if}
      {/if}
      {#if error && items.length}<p class="staff-form-message staff-form-error" role="alert">{error}</p>{/if}
    </section>
    {#if selectedId}<section class="staff-admin-panel checkin-history-panel order-selected-panel" aria-labelledby="order-detail-title" aria-busy={detailLoading}><div class="staff-panel-heading"><div><span class="panel-index">03</span><h2 id="order-detail-title" tabindex="-1" bind:this={detailHeading}>Detail pesanan</h2></div><button class="staff-text-button" type="button" onclick={closeDetail}>Kembali ke daftar</button></div>
      {#if detailLoading}<p class="staff-muted" role="status">Memuat detail pesanan…</p>
      {:else if detailError}<p class="staff-form-message staff-form-error" role="alert">{detailError} <button class="staff-text-button" type="button" onclick={() => openDetail(selectedId)}>Coba lagi</button></p>
      {:else if selected}<div class="order-detail-grid">
        <div><h3>Pesanan</h3><dl><dt>Reference</dt><dd>{selected.reference}</dd><dt>Event</dt><dd>{selected.eventName}</dd><dt>Status</dt><dd>{statusLabel(selected.status)}</dd><dt>Dibuat</dt><dd>{localTime(selected.createdAt)}</dd><dt>Batas pembayaran</dt><dd>{localTime(selected.expiresAt)}</dd></dl></div>
        <div><h3>Pembeli</h3><dl><dt>Nama</dt><dd>{selected.buyer.name}</dd><dt>Email</dt><dd><a href={`mailto:${selected.buyer.email}`}>{selected.buyer.email}</a></dd><dt>Telepon</dt><dd>{selected.buyer.phone}</dd><dt>Identitas</dt><dd>{selected.buyer.identityMasked}</dd></dl></div>
        <div><h3>Rincian biaya</h3><dl><dt>Subtotal</dt><dd>{money(selected.subtotal)}</dd><dt>Biaya admin</dt><dd>{money(selected.adminFee)}</dd><dt>Diskon</dt><dd>−{money(selected.discount)}</dd><dt>Total</dt><dd><strong>{money(selected.total)}</strong></dd></dl><h3>Pembayaran</h3>{#if selected.payment}<dl><dt>Metode</dt><dd>{selected.payment.method}</dd><dt>Jumlah</dt><dd>{money(selected.payment.amount)}</dd><dt>Status</dt><dd>{paymentStatusLabel(selected.payment.status)}</dd><dt>Dibayar</dt><dd>{selected.payment.paidAt ? localTime(selected.payment.paidAt) : "Belum dibayar"}</dd></dl>{:else}<p class="staff-muted">Belum ada pembayaran.</p>{/if}</div>
        <div><h3>Kategori tiket</h3><ul class="order-detail-list">{#each selected.items as item (`${item.tierId}-${item.name}`)}<li><strong>{item.name}</strong><span>{item.quantity} × {money(item.unitPrice)} = {money(item.lineTotal)}</span></li>{/each}</ul><h3>E-ticket dan check-in</h3>{#if selected.tickets.length}<ul class="order-detail-list">{#each selected.tickets as ticket (ticket.id)}<li><strong>{ticket.attendeeName} · {ticket.tierName}</strong><code>{ticket.code}</code><span>{ticket.gate} · {ticket.status === "CHECKED_IN" ? `Check-in ${ticket.checkedInAt ? localTime(ticket.checkedInAt) : "berhasil"}${ticket.checkedInBy ? ` · ${ticket.checkedInBy}` : ""}` : "Belum check-in"}</span></li>{/each}</ul>{:else}<p class="staff-muted">E-ticket belum diterbitkan.</p>{/if}</div>
      </div>
      {#if selected.refund}
       <section class="staff-form"><h3>{refundStatus(selected.refund.status).label}</h3>{#if refundStatus(selected.refund.status).label === "Status refund belum dikenali"}<details><summary>Status teknis</summary><code>{selected.refund.status}</code></details>{/if}<p>{money(selected.refund.amount)} · {selected.refund.reason}</p><p class="staff-muted">{refundStatus(selected.refund.status).next}</p>
        {#if selected.refund.manualReference}<p>Referensi transfer: {selected.refund.manualReference} · {selected.refund.manualPaidAt ? localTime(selected.refund.manualPaidAt) : ""}</p>{/if}
        {#if selected.refund.status === "MANUAL_REQUIRED"}
         <form class="staff-form" onsubmit={(e)=>{e.preventDefault();void finishManual()}}>
          <p>Catat hanya transfer penuh yang sudah berhasil. Form ini mencatat bukti dan tidak mengirim uang.</p>
          <label>Referensi transfer<input bind:value={manualReference} minlength="3" maxlength="160" required disabled={refundBusy || uncertainManual}/></label>
          <label>Waktu transfer (WIB)<input type="datetime-local" step="1" bind:value={manualPaidAt} required disabled={refundBusy || uncertainManual}/></label>
          <label>Catatan bukti<textarea bind:value={manualNote} minlength="3" maxlength="500" required disabled={refundBusy || uncertainManual}></textarea></label>
          <label><input type="checkbox" bind:checked={manualConfirmed} required disabled={refundBusy || uncertainManual}/> Transfer {money(selected.refund.amount)} sudah berhasil dan bukti sudah diperiksa.</label>
          <button class="staff-secondary-button" type="submit" disabled={refundBusy || uncertainManual || !manualConfirmed}>Catat refund manual selesai</button>
          {#if uncertainManual}<button class="staff-text-button" type="button" disabled={refundBusy} onclick={() => checkUncertain("manual")}>Periksa detail refund</button>{/if}
         </form>
        {/if}
        <p role="status" aria-live="polite" tabindex="-1" bind:this={manualResult}>{refundMessage}</p>
       </section>
      {/if}
      {#if selected.status === "PAID" || selected.status === "CANCELLED" || selected.status === "EXPIRED"}
        <form class="recovery-panel" onsubmit={(event) => { event.preventDefault(); void submitRefund(); }}>
          <h3>Ajukan refund penuh</h3><p>Nominal dihitung server: <strong>{money(selected.total)}</strong>. Tiket akan ditahan sampai Midtrans mengonfirmasi hasil.</p>
          <label>Alasan refund<textarea bind:value={refundReason} minlength="3" maxlength="500" required disabled={refundBusy}></textarea></label>
          <button class="staff-secondary-button" type="submit" disabled={refundBusy || uncertainRefund || refundReason.trim().length < 3}>{refundBusy ? "Mengajukan…" : "Ajukan refund"}</button>
          {#if uncertainRefund}<button class="staff-text-button" type="button" disabled={refundBusy} onclick={() => checkUncertain("refund")}>Periksa detail refund</button>{/if}
          <p role="status" aria-live="polite">{refundMessage}</p>
        </form>
      {/if}
      {/if}
    </section>{/if}
    </div>
</AdminLayout>
