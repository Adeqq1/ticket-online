<script lang="ts">
  import AdminLayout from "../../layouts/AdminLayout.svelte";
  import { onDestroy, onMount } from "svelte";
  import { ApiError, getAdminCheckInHistory, type CheckInHistoryEvent, type CheckInHistoryFilter, type CheckInHistoryItem } from "../../lib/api.ts";

  let { accessToken, onUnauthorized, onLogout, loggingOut }: { accessToken: string; onUnauthorized: () => void; onLogout: () => void; loggingOut: boolean } = $props();
  let events = $state<CheckInHistoryEvent[]>([]);
  let items = $state<CheckInHistoryItem[]>([]);
  let eventId = $state("");
  let gate = $state("");
  let query = $state("");
  let applied = $state<CheckInHistoryFilter>({});
  let cursor = $state<string | null>(null);
  let loading = $state(true);
  let loadingMore = $state(false);
  let error = $state("");
  let request: AbortController | undefined;
  let generation = 0;
  let gates = $derived(events.find((event) => event.id === eventId)?.gates ?? []);

  async function load(filters = applied, more = false) {
    if (request) request.abort();
    const controller = new AbortController();
    request = controller;
    const current = ++generation;
    if (more) loadingMore = true; else { loading = true; items = []; cursor = null; }
    error = "";
    try {
      const page = await getAdminCheckInHistory(accessToken, { ...filters, ...(more && cursor ? { beforeId: cursor } : {}) }, controller.signal);
      if (current !== generation) return;
      events = page.filterOptions;
      items = more ? [...items, ...page.items] : page.items;
      cursor = page.nextCursor;
    } catch (cause) {
      if (current !== generation || (cause instanceof DOMException && cause.name === "AbortError")) return;
      if (cause instanceof ApiError && cause.status === 401) onUnauthorized();
      else if (cause instanceof ApiError && cause.status === 403) error = "Akses riwayat check-in hanya tersedia untuk admin.";
      else error = cause instanceof ApiError && cause.code !== "NETWORK_ERROR" ? cause.message : "Riwayat belum dapat dimuat. Periksa koneksi lalu coba lagi.";
    } finally {
      if (current === generation) { loading = false; loadingMore = false; if (request === controller) request = undefined; }
    }
  }

  function search(event: SubmitEvent) {
    event.preventDefault();
    applied = { ...(eventId ? { eventId } : {}), ...(gate ? { gate } : {}), ...(query.trim() ? { q: query.trim() } : {}) };
    void load(applied);
  }

  function reset() {
    eventId = ""; gate = ""; query = ""; applied = {};
    void load({});
  }

  function selectEvent(value: string) { eventId = value; gate = ""; }

  function outcomeLabel(outcome: CheckInHistoryItem["outcome"]) {
    return ({ CHECKED_IN: "Check-in berhasil", TICKET_ALREADY_USED: "Tiket sudah digunakan", INVALID_REQUEST: "Kode tidak valid", TICKET_NOT_FOUND: "Tiket tidak ditemukan", ORDER_NOT_PAID: "Order belum dibayar", WRONG_GATE: "Gate tidak cocok", FORBIDDEN: "Akses ditolak", EVENT_CHANGED: "Acara berubah / check-in ditutup" })[outcome];
  }

  function localTime(value: string) { return new Date(value).toLocaleString("id-ID"); }

  onMount(() => { void load({}); });
  onDestroy(() => { generation++; request?.abort(); });
</script>

<svelte:head>
  <title>Riwayat Check-in | Gate Control</title>
  <meta name="description" content="Riwayat percobaan check-in tiket yang diterima server." />
  <meta name="robots" content="noindex" />
</svelte:head>

  <AdminLayout page="history" contentId="checkin-history-content" skipLabel="Lewati ke riwayat check-in" kicker="OPERASIONAL EVENT • AUDIT" title="Riwayat check-in." description="Periksa hasil scan yang tercatat server beserta petugas dan waktu masuk." onLogout={onLogout} loggingOut={loggingOut} primaryAction={{ label: "Cari", form: "checkin-search-form", disabled: loading }}>
    <section class="staff-admin-panel checkin-history-panel" aria-labelledby="history-filters-title">
      <div class="staff-panel-heading"><div><span class="panel-index">01</span><h2 id="history-filters-title">Filter riwayat</h2></div></div>
      <form id="checkin-search-form" class="history-filter-form" onsubmit={search} aria-busy={loading}>
        <label>Event<select value={eventId} onchange={(event) => selectEvent(event.currentTarget.value)} disabled={loading}><option value="">Semua event</option>{#each events as event (event.id)}<option value={event.id}>{event.name}</option>{/each}</select></label>
        <label>Gate<select bind:value={gate} disabled={loading || !gates.length}><option value="">Semua gate</option>{#each gates as item (item)}<option value={item}>{item}</option>{/each}</select></label>
        <label>Kode tiket<input bind:value={query} maxlength="35" placeholder="Cari sebagian kode ET-…" disabled={loading} /></label>
        <div class="history-filter-actions"><button class="scan-submit" type="submit" disabled={loading}>Cari</button><button class="staff-secondary-button" type="button" onclick={reset} disabled={loading}>Reset</button></div>
      </form>
    </section>

    <section class="staff-admin-panel checkin-history-panel" aria-labelledby="history-results-title">
      <div class="staff-panel-heading"><div><span class="panel-index">02</span><h2 id="history-results-title">Hasil scan</h2></div><button class="staff-text-button" type="button" onclick={() => { void load(applied); }} disabled={loading}>{loading ? "Memuat…" : "Muat ulang"}</button></div>
      {#if loading}<p class="staff-muted" aria-live="polite">Memuat riwayat check-in…</p>
      {:else if error}<p class="staff-form-message staff-form-error" role="alert">{error} <button class="staff-text-button" type="button" onclick={() => { void load(applied); }}>Coba lagi</button></p>
      {:else if items.length === 0}<p class="staff-muted" role="status">Belum ada percobaan check-in yang cocok.</p>
      {:else}<div class="history-table-wrap"><table class="history-table"><caption class="visually-hidden">Percobaan check-in terbaru</caption><thead><tr><th scope="col">Waktu</th><th scope="col">Kode tiket</th><th scope="col">Event / gate</th><th scope="col">Petugas</th><th scope="col">Hasil</th><th scope="col">Waktu check-in</th></tr></thead><tbody>{#each items as item (item.id)}<tr><td data-label="Waktu"><time datetime={item.recordedAt}>{localTime(item.recordedAt)}</time></td><td data-label="Kode tiket"><code>{item.code ?? "Kode tidak valid"}</code></td><td data-label="Event / gate">{item.eventName ?? item.eventId ?? "—"}<small>{item.gate ?? "—"}</small></td><td data-label="Petugas">{item.staff.name}</td><td data-label="Hasil"><span class:history-success={item.outcome === "CHECKED_IN"} class="history-outcome">{outcomeLabel(item.outcome)}</span></td><td data-label="Waktu check-in">{#if item.checkedInAt}<time datetime={item.checkedInAt}>{localTime(item.checkedInAt)}</time>{:else}—{/if}</td></tr>{/each}</tbody></table></div>
        {#if cursor}<div class="history-more"><button class="staff-secondary-button" type="button" disabled={loadingMore} onclick={() => { void load(applied, true); }}>{loadingMore ? "Memuat…" : "Muat berikutnya"}</button></div>{/if}
      {/if}
    </section>
    <footer class="scan-footer"><span>Ticket Online · Admin tools</span><span>Riwayat hasil check-in server</span></footer>
</AdminLayout>
