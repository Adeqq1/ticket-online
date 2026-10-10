<script lang="ts">
  import AdminLayout from "../../layouts/AdminLayout.svelte";
  import EventChangePanel from "../../components/admin/EventChangePanel.svelte";
  import { onDestroy, onMount, tick } from "svelte";
  import { ApiError, createAdminEvent, createAdminTier, createAdminZone, getAdminEvents, updateAdminEvent, updateAdminTier, updateAdminZone, type AdminEventInput, type AdminApiEvent, type AdminTierInput, type PublicationStatus } from "../../lib/api.ts";
  import { suggestAdminId } from "../../lib/admin-id.ts";

  let { accessToken, onUnauthorized, onLogout, loggingOut }: { accessToken: string; onUnauthorized: () => void; onLogout: () => void; loggingOut: boolean } = $props();
  let events = $state<AdminApiEvent[]>([]);
  let selectedId = $state("");
  const selectedEvent = $derived(events.find((event) => event.id === selectedId));
  const scheduleLocked = $derived(Boolean(selectedEvent?.scheduleLocked || (selectedEvent?.currentEvent?.version ?? 0) > 0));
  let query = $state("");
  let publicationFilter = $state<PublicationStatus | "">("");
  let draft = $state<AdminEventInput>(emptyEvent());
  let lineupText = $state("");
  let loading = $state(true);
  let saving = $state(false);
  let error = $state("");
  let notice = $state("");
  let request: AbortController | undefined;
  let generation = 0;
  let zoneDraft = $state({ id: "", name: "", description: "" });
  let zoneSelected = $state("");
  let tierDraft = $state<AdminTierInput>({ id: "", name: "", zoneId: "", price: 0, capacity: 0, maxPerOrder: 1, benefit: "", gate: "", seating: "free-standing" });
  let tierSelected = $state("");
  let eventBaseline = $state("");
  let zoneBaseline = $state("");
  let tierBaseline = $state("");
  let eventIdManual = $state(false);
  let zoneIdManual = $state(false);
  let tierIdManual = $state(false);
  let posterFailed = $state(false);
  let posterPreview = $state("");
  let changePanelDirty = $state(false);
  const eventState = () => JSON.stringify({ draft, lineupText });
  const zoneState = () => JSON.stringify(zoneDraft);
  const tierState = () => JSON.stringify(tierDraft);
  const hasUnsaved = $derived(eventState() !== eventBaseline || changePanelDirty || (Boolean(selectedId) && (zoneState() !== zoneBaseline || tierState() !== tierBaseline)));

  function emptyEvent(): AdminEventInput { return { id: "", artist: "", city: "", venue: "", address: "", startsAt: "", genre: "Pop", status: "Presale", publicationStatus: "DRAFT", image: "", description: "", lineup: [] }; }
  function currentPosterURL() {
    try { const url = new URL(draft.image); return ["http:", "https:"].includes(url.protocol) && !url.username && !url.password ? url.href : ""; }
    catch { return ""; }
  }
  function protectUnsaved(event: BeforeUnloadEvent) {
    if (hasUnsaved) { event.preventDefault(); event.returnValue = ""; }
  }
  function allowDiscard() { return !hasUnsaved || window.confirm("Perubahan yang belum disimpan akan hilang. Lanjutkan?"); }
  function requestLogout() { if (allowDiscard()) onLogout(); }
  function setChangePanelDirty(dirty: boolean) { changePanelDirty = dirty; }
  function suggestEventId() { if (!eventIdManual) draft.id = suggestAdminId(draft.artist); }
  function suggestZoneId() { if (!zoneIdManual) zoneDraft.id = suggestAdminId(zoneDraft.name); }
  function suggestTierId() { if (!tierIdManual) tierDraft.id = suggestAdminId(tierDraft.name); }
  function localDateTime(value: string) {
    if (!value) return "";
    const parts = new Intl.DateTimeFormat("en-CA", { timeZone: "Asia/Jakarta", year: "numeric", month: "2-digit", day: "2-digit", hour: "2-digit", minute: "2-digit", hourCycle: "h23" }).formatToParts(new Date(value));
    const part = (type: string) => parts.find((item) => item.type === type)?.value ?? "";
    return `${part("year")}-${part("month")}-${part("day")}T${part("hour")}:${part("minute")}`;
  }
  function payload() {
    const lockedStartsAt = scheduleLocked ? selectedEvent?.startsAt : undefined;
    return { ...draft, startsAt: lockedStartsAt ?? (draft.startsAt ? `${draft.startsAt}:00+07:00` : ""), lineup: lineupText.split("\n").map((name) => name.trim()).filter(Boolean) };
  }
  const visible = $derived(events.filter((event) => (!publicationFilter || event.publicationStatus === publicationFilter) && `${event.artist} ${event.city} ${event.venue}`.toLowerCase().includes(query.trim().toLowerCase())));

  function select(event: AdminApiEvent, discardChecked = false) {
    if (event.id === selectedId || (!discardChecked && !allowDiscard())) return;
    selectedId = event.id;
    draft = { id: event.id, artist: event.artist, city: event.city, venue: event.venue, address: event.address, startsAt: localDateTime(event.startsAt), genre: event.genre, status: event.status, publicationStatus: event.publicationStatus, image: event.image, description: event.description, lineup: event.lineup };
    lineupText = event.lineup.join("\n");
    zoneSelected = ""; zoneDraft = { id: "", name: "", description: "" };
    tierSelected = ""; tierDraft = { id: "", name: "", zoneId: event.zones[0]?.id ?? "", price: 0, capacity: 0, maxPerOrder: 1, benefit: "", gate: "", seating: "free-standing" };
    eventIdManual = false; zoneIdManual = false; tierIdManual = false;
    eventBaseline = eventState(); zoneBaseline = zoneState(); tierBaseline = tierState();
    posterFailed = false; posterPreview = currentPosterURL();
    changePanelDirty = false;
    error = ""; notice = "";
  }
  async function startNew() {
    if (!allowDiscard()) return;
    selectedId = ""; draft = emptyEvent(); lineupText = ""; eventIdManual = false; zoneIdManual = false; tierIdManual = false;
    eventBaseline = eventState(); zoneBaseline = ""; tierBaseline = ""; posterFailed = false; posterPreview = ""; changePanelDirty = false; error = ""; notice = "";
    await tick();
    document.getElementById("event-form")?.scrollIntoView({ block: "start" });
    document.querySelector<HTMLInputElement>("#event-form input")?.focus({ preventScroll: true });
  }

  async function load() {
    request?.abort(); const controller = new AbortController(); request = controller; const current = ++generation; loading = true; error = "";
    try { events = await getAdminEvents(accessToken, controller.signal); }
    catch (cause) {
      if (current !== generation || (cause instanceof DOMException && cause.name === "AbortError")) return;
      if (cause instanceof ApiError && cause.status === 401) onUnauthorized();
      else error = cause instanceof ApiError ? cause.message : "Daftar konser belum dapat dimuat. Periksa koneksi lalu coba lagi.";
    } finally { if (current === generation) { loading = false; if (request === controller) request = undefined; } }
  }

  async function save(event: SubmitEvent) {
    event.preventDefault(); if (saving) return;
    saving = true; error = ""; notice = "";
    try {
      const value = payload();
      const result = selectedId ? await updateAdminEvent(accessToken, selectedId, value) : await createAdminEvent(accessToken, value);
      const current = events.filter((item) => item.id !== result.id);
      events = [...current, result].sort((a, b) => a.startsAt.localeCompare(b.startsAt));
      if (selectedId === result.id) {
        draft = { id: result.id, artist: result.artist, city: result.city, venue: result.venue, address: result.address, startsAt: localDateTime(result.startsAt), genre: result.genre, status: result.status, publicationStatus: result.publicationStatus, image: result.image, description: result.description, lineup: result.lineup };
        lineupText = result.lineup.join("\n"); eventBaseline = eventState();
        posterPreview = currentPosterURL(); posterFailed = false;
      } else select(result, true);
      notice = "Perubahan konser tersimpan.";
    } catch (cause) {
      if (cause instanceof ApiError && cause.status === 401) onUnauthorized();
      else if (cause instanceof ApiError && cause.code === "SCHEDULE_LOCKED") error = "Jadwal tidak dapat diubah karena konser sudah memiliki pesanan.";
      else if (cause instanceof ApiError && cause.code === "DUPLICATE_EVENT") { error = "ID konser sudah digunakan. Ubah ID lalu coba lagi."; document.querySelector<HTMLInputElement>("#event-id")?.focus(); }
      else error = cause instanceof ApiError ? cause.message : "Konser belum dapat disimpan. Periksa koneksi lalu coba lagi.";
    } finally { saving = false; }
  }

  function selectZone(id: string) {
    if (id === zoneSelected) return;
    if (zoneState() !== zoneBaseline && !window.confirm("Perubahan zona yang belum disimpan akan hilang. Lanjutkan?")) return;
    zoneSelected = id; zoneIdManual = Boolean(id);
    const zone = events.find((event) => event.id === selectedId)?.zones.find((item) => item.id === id);
    zoneDraft = zone ? { id, name: zone.name, description: zone.description } : { id: "", name: "", description: "" };
    zoneBaseline = zoneState();
  }
  function selectTier(id: string) {
    if (id === tierSelected) return;
    if (tierState() !== tierBaseline && !window.confirm("Perubahan kategori yang belum disimpan akan hilang. Lanjutkan?")) return;
    tierSelected = id; tierIdManual = Boolean(id);
    const tier = events.find((event) => event.id === selectedId)?.ticketTiers.find((item) => item.id === id);
    tierDraft = tier ? { id, name: tier.name, zoneId: tier.zoneId, price: tier.price, capacity: tier.capacity, maxPerOrder: tier.maxPerOrder, benefit: tier.benefit, gate: tier.gate, seating: tier.seating } : { id: "", name: "", zoneId: events.find((event) => event.id === selectedId)?.zones[0]?.id ?? "", price: 0, capacity: 0, maxPerOrder: 1, benefit: "", gate: "", seating: "free-standing" };
    tierBaseline = tierState();
  }
  async function saveZone(event: SubmitEvent) {
    event.preventDefault(); if (!selectedId || saving) return;
    saving = true; error = ""; notice = "";
    let committed = false;
    try {
      if (zoneSelected) await updateAdminZone(accessToken, selectedId, zoneSelected, { name: zoneDraft.name, description: zoneDraft.description });
      else await createAdminZone(accessToken, selectedId, zoneDraft);
      committed = true; zoneBaseline = zoneState(); notice = "Zona tersimpan.";
      try { events = await getAdminEvents(accessToken); }
      catch (cause) { if (cause instanceof ApiError && cause.status === 401) onUnauthorized(); notice = "Zona tersimpan; daftar belum dapat dimuat ulang."; }
    } catch (cause) {
      if (committed) { notice = "Zona tersimpan; daftar belum dapat dimuat ulang."; return; }
      if (cause instanceof ApiError && cause.status === 401) onUnauthorized();
      else if (cause instanceof ApiError && cause.code === "DUPLICATE_ZONE") { error = "ID zona sudah digunakan pada konser ini. Ubah ID lalu coba lagi."; document.querySelector<HTMLInputElement>("#zone-id")?.focus(); }
      else error = cause instanceof ApiError ? cause.message : "Zona belum dapat disimpan.";
    } finally { saving = false; }
  }
  async function saveTier(event: SubmitEvent) {
    event.preventDefault(); if (!selectedId || saving) return;
    saving = true; error = ""; notice = "";
    let committed = false;
    try {
      if (tierSelected) await updateAdminTier(accessToken, selectedId, tierSelected, tierDraft);
      else await createAdminTier(accessToken, selectedId, tierDraft);
      committed = true; tierBaseline = tierState(); notice = "Kategori tiket tersimpan.";
      try { events = await getAdminEvents(accessToken); }
      catch (cause) { if (cause instanceof ApiError && cause.status === 401) onUnauthorized(); notice = "Kategori tersimpan; daftar belum dapat dimuat ulang."; }
    } catch (cause) {
      if (committed) { notice = "Kategori tersimpan; daftar belum dapat dimuat ulang."; return; }
      if (cause instanceof ApiError && cause.status === 401) onUnauthorized();
      else if (cause instanceof ApiError && cause.code === "DUPLICATE_TIER") { error = "ID kategori sudah digunakan pada konser ini. Ubah ID lalu coba lagi."; document.querySelector<HTMLInputElement>("#tier-id")?.focus(); }
      else error = cause instanceof ApiError ? cause.message : "Kategori tiket belum dapat disimpan.";
    } finally { saving = false; }
  }

  onMount(() => { eventBaseline = eventState(); zoneBaseline = zoneState(); tierBaseline = tierState(); void load(); });
  onDestroy(() => { generation++; request?.abort(); });
</script>

<svelte:window onbeforeunload={protectUnsaved} />

<svelte:head><title>Kelola Konser | Tiket Online</title><meta name="robots" content="noindex" /></svelte:head>

  <AdminLayout page="events" contentId="event-admin-content" skipLabel="Lewati ke pengelolaan konser" kicker="KATALOG EVENT • ADMIN" title="Kelola konser." description="Atur informasi, jadwal, lineup, poster, dan publikasi konser." onLogout={requestLogout} loggingOut={loggingOut} logoutDisabled={loggingOut || saving} primaryAction={{ label: "Tambah konser", onclick: startNew, disabled: saving }}>
    <div class="staff-admin-grid event-admin-grid">
      <section class="staff-admin-panel" aria-labelledby="event-list-title">
        <div class="staff-panel-heading"><div><span class="panel-index">01</span><h2 id="event-list-title">Daftar konser</h2></div><span class="staff-muted">{visible.length} konser</span></div>
        <div class="event-list-filters"><label>Cari konser<input bind:value={query} type="search" placeholder="Nama, kota, atau venue" disabled={loading} /></label><label>Status publikasi<select bind:value={publicationFilter} disabled={loading}><option value="">Semua status</option><option value="DRAFT">Draft</option><option value="PUBLISHED">Published</option><option value="ARCHIVED">Archived</option></select></label></div>
        {#if loading}<p class="staff-muted" role="status">Memuat konser…</p>
        {:else if error && !events.length}<p class="staff-form-message staff-form-error" role="alert">{error} <button class="staff-text-button" type="button" onclick={load}>Coba lagi</button></p>
        {:else if !visible.length}<p class="staff-muted">Belum ada konser yang cocok.</p>
        {:else}<ul class="event-admin-list">{#each visible as item (item.id)}<li class:event-admin-selected={selectedId === item.id}><button type="button" onclick={() => select(item)}><span><strong>{item.artist || item.id}</strong><small>{item.city || "Kota belum diatur"} · {item.startsAt ? `${new Date(item.startsAt).toLocaleString("id-ID", { timeZone: "Asia/Jakarta" })} WIB` : "Jadwal belum diatur"}</small></span><span class={`event-publication event-${item.publicationStatus.toLowerCase()}`}>{item.publicationStatus}</span></button></li>{/each}</ul>{/if}
      </section>

      <section class="staff-admin-panel event-editor-panel" aria-labelledby="event-editor-title">
        <div class="staff-panel-heading"><div><span class="panel-index">02</span><h2 id="event-editor-title">{selectedId ? draft.artist : "Konser baru"}</h2></div>{#if selectedId}<span class={`event-publication event-${draft.publicationStatus.toLowerCase()}`}>{draft.publicationStatus}</span>{/if}</div>
        {#if error}<p class="staff-form-message staff-form-error" role="alert">{error}</p>{/if}
        {#if notice}<p class="staff-form-message event-save-notice" role="status">{notice}</p>{/if}
        {#if hasUnsaved}<p class="event-unsaved" role="status">Perubahan belum disimpan</p>{/if}
        <form id="event-form" class="staff-form event-admin-form" onsubmit={save} aria-busy={saving}>
          <fieldset class="event-group"><legend>Informasi dasar</legend>
          {#if !selectedId}<label>ID URL konser<input id="event-id" bind:value={draft.id} oninput={() => eventIdManual = true} pattern="[a-z0-9]+(-[a-z0-9]+)*" maxlength="64" placeholder="nama-konser" required disabled={saving} /><small>Saran ID terisi dari nama konser dan tetap dapat diedit. Gunakan huruf kecil, angka, dan tanda hubung.</small></label>{/if}
          {#if !selectedId && draft.id && events.some((item) => item.id === draft.id)}<p class="staff-form-message staff-form-error" role="alert">ID konser sudah digunakan. Ubah ID sebelum menyimpan.</p>{/if}
          <label>Nama konser / artis<input bind:value={draft.artist} oninput={suggestEventId} minlength="2" maxlength="160" required disabled={saving} /></label>
          <label>Genre<select bind:value={draft.genre} disabled={saving}><option>Rock</option><option>Pop</option><option>Indie</option></select></label></fieldset>
          <fieldset class="event-group"><legend>Publikasi</legend><label>Label penjualan<select bind:value={draft.status} disabled={saving}><option>Early Bird</option><option>Presale</option><option>Sold Out</option></select></label>
          <label>Status publikasi<select bind:value={draft.publicationStatus} disabled={saving || !selectedId}><option value="DRAFT">Draft — belum tampil di katalog</option><option value="PUBLISHED">Published — tampil di katalog</option><option value="ARCHIVED">Archived — disembunyikan dari katalog</option></select></label>
          </fieldset>
          <fieldset class="event-group"><legend>Jadwal dan lokasi</legend>
          <label>Jadwal (WIB)<input type="datetime-local" bind:value={draft.startsAt} aria-describedby={scheduleLocked ? "schedule-lock-note" : undefined} required={draft.publicationStatus === "PUBLISHED"} disabled={saving || scheduleLocked} /></label>
          {#if selectedId && scheduleLocked}<small id="schedule-lock-note" class="event-lock-note">Jadwal terkunci setelah pesanan dibuat atau perubahan acara dicatat. Gunakan panel Pembatalan dan perubahan jadwal di bawah untuk menjaga hak pembeli dan riwayat keputusan.</small>{/if}
          <div class="event-form-row"><label>Kota<input bind:value={draft.city} maxlength="100" required={draft.publicationStatus === "PUBLISHED"} disabled={saving || Boolean(selectedId && events.find((item) => item.id === selectedId)?.locationLocked)} /></label><label>Venue<input bind:value={draft.venue} maxlength="160" required={draft.publicationStatus === "PUBLISHED"} disabled={saving || Boolean(selectedId && events.find((item) => item.id === selectedId)?.locationLocked)} /></label></div>
          <label>Alamat<input bind:value={draft.address} maxlength="255" required={draft.publicationStatus === "PUBLISHED"} disabled={saving || Boolean(selectedId && events.find((item) => item.id === selectedId)?.locationLocked)} /></label>
          {#if selectedId && events.find((item) => item.id === selectedId)?.locationLocked}<small id="location-lock-note" class="event-lock-note">Kota, venue, dan alamat terkunci setelah reservasi pertama untuk menjaga informasi pada pesanan pembeli. Perubahan lokasi tidak tersedia di panel perubahan acara.</small>{/if}
          </fieldset>
          <fieldset class="event-group"><legend>Materi publikasi</legend><label>URL poster<input bind:value={draft.image} oninput={() => { posterPreview = currentPosterURL(); posterFailed = false; }} type="url" maxlength="500" placeholder="https://…" required={draft.publicationStatus === "PUBLISHED"} disabled={saving} /></label>
          {#if posterPreview}<div class="poster-preview" aria-live="polite"><h3>Pratinjau poster</h3>{#key posterPreview}{#if posterFailed}<p role="status">Poster tidak dapat dimuat. Periksa alamat atau akses gambar.</p>{:else}<img src={posterPreview} alt={`Pratinjau poster ${draft.artist || "konser"}`} onerror={() => posterFailed = true} />{/if}{/key}</div>{/if}
          <label>Deskripsi<textarea bind:value={draft.description} rows="5" maxlength="16000" required={draft.publicationStatus === "PUBLISHED"} disabled={saving}></textarea></label>
          <label>Lineup <small>Satu nama per baris</small><textarea bind:value={lineupText} rows="4" maxlength="16000" required={draft.publicationStatus === "PUBLISHED"} disabled={saving}></textarea></label>
          </fieldset>
          {#if !events.find((item) => item.id === selectedId)?.ticketTiers.length && draft.publicationStatus === "PUBLISHED"}<p class="staff-muted">Tambahkan setidaknya satu zona dan kategori tiket sebelum menerbitkan konser.</p>{/if}
          <button class="scan-submit staff-submit" type="submit" disabled={saving || loading}>{saving ? "Menyimpan…" : selectedId ? "Simpan perubahan" : "Buat konser"}</button>
        </form>
        {#if selectedId}
          {@const currentEvent = events.find((item) => item.id === selectedId)}
      {#if selectedEvent}{#key selectedId}<EventChangePanel event={selectedEvent} {accessToken} onSaved={load} {onUnauthorized} onDirtyChange={setChangePanelDirty} />{/key}{/if}
          <section class="event-inventory-editor" aria-labelledby="event-inventory-title">
            <div class="staff-panel-heading"><div><span class="panel-index">03</span><h2 id="event-inventory-title">Zona dan kategori tiket</h2></div></div>
            <div class="event-form-row"><label>Zona<select value={zoneSelected} onchange={(event) => selectZone(event.currentTarget.value)} disabled={saving}><option value="">Tambah zona</option>{#each currentEvent?.zones ?? [] as zone}<option value={zone.id}>{zone.name}</option>{/each}</select></label></div>
            <form class="staff-form event-admin-form" onsubmit={saveZone}>
              {#if !zoneSelected}<label>ID zona<input id="zone-id" bind:value={zoneDraft.id} oninput={() => zoneIdManual = true} pattern="[a-z0-9]+(-[a-z0-9]+)*" maxlength="64" required disabled={saving} /><small>Saran ID mengikuti nama zona sampai ID diedit.</small></label>{/if}
              {#if !zoneSelected && zoneDraft.id && currentEvent?.zones.some((item) => item.id === zoneDraft.id)}<p class="staff-form-message staff-form-error" role="alert">ID zona sudah digunakan pada konser ini. Ubah ID sebelum menyimpan.</p>{/if}
              <label>Nama zona<input bind:value={zoneDraft.name} oninput={suggestZoneId} maxlength="100" required disabled={saving} /></label>
              <label>Deskripsi zona<input bind:value={zoneDraft.description} maxlength="255" disabled={saving} /></label>
              <button class="staff-secondary-button" type="submit" disabled={saving || !zoneDraft.name}>{zoneSelected ? "Simpan zona" : "Tambah zona"}</button>
            </form>
            <div class="event-inventory-list"><h3>Kategori tersimpan</h3>{#each currentEvent?.ticketTiers ?? [] as tier}<button class="staff-secondary-button" type="button" onclick={() => selectTier(tier.id)}>{tier.name} · {tier.availableQuantity}/{tier.capacity} tersedia · {tier.boundQuantity} stok terikat{tier.gateLocked ? " · Gate terkunci" : ""}</button>{/each}</div>
            <label>Kategori<select value={tierSelected} onchange={(event) => selectTier(event.currentTarget.value)} disabled={saving}><option value="">Tambah kategori</option>{#each currentEvent?.ticketTiers ?? [] as tier}<option value={tier.id}>{tier.name}</option>{/each}</select></label>
            <form class="staff-form event-admin-form" onsubmit={saveTier}>
              {#if !tierSelected}<label>ID kategori<input id="tier-id" bind:value={tierDraft.id} oninput={() => tierIdManual = true} pattern="[a-z0-9]+(-[a-z0-9]+)*" maxlength="64" required disabled={saving} /><small>Saran ID mengikuti nama kategori sampai ID diedit.</small></label>{/if}
              {#if !tierSelected && tierDraft.id && currentEvent?.ticketTiers.some((item) => item.id === tierDraft.id)}<p class="staff-form-message staff-form-error" role="alert">ID kategori sudah digunakan pada konser ini. Ubah ID sebelum menyimpan.</p>{/if}
              <label>Nama kategori<input bind:value={tierDraft.name} oninput={suggestTierId} maxlength="100" required disabled={saving} /></label>
              <label>Zona<select bind:value={tierDraft.zoneId} required disabled={saving || !currentEvent?.zones.length}><option value="">Pilih zona</option>{#each currentEvent?.zones ?? [] as zone}<option value={zone.id}>{zone.name}</option>{/each}</select></label>
              <div class="event-form-row"><label>Harga (rupiah)<input type="number" min="1" step="1" bind:value={tierDraft.price} required disabled={saving} /></label><label>Kapasitas<input type="number" min="0" step="1" bind:value={tierDraft.capacity} required disabled={saving} /></label></div>
              <div class="event-form-row"><label>Batas pembelian<input type="number" min="1" step="1" bind:value={tierDraft.maxPerOrder} required disabled={saving} /></label><label>Gate<input bind:value={tierDraft.gate} maxlength="100" required disabled={saving || Boolean(currentEvent?.ticketTiers.find((item) => item.id === tierSelected)?.gateLocked)} /></label></div>
              {#if currentEvent?.ticketTiers.find((item) => item.id === tierSelected)?.gateLocked}<small class="event-lock-note">Gate dikunci setelah reservasi pertama untuk kategori ini.</small>{/if}
              <label>Benefit<input bind:value={tierDraft.benefit} maxlength="255" disabled={saving} /></label>
              <label>Seating<select bind:value={tierDraft.seating} disabled={saving}><option value="free-standing">Berdiri</option><option value="assigned">Tempat duduk</option></select></label>
              {#if tierSelected}<p class="staff-muted">Tersedia: {currentEvent?.ticketTiers.find((item) => item.id === tierSelected)?.availableQuantity ?? 0}; stok terikat: {currentEvent?.ticketTiers.find((item) => item.id === tierSelected)?.boundQuantity ?? 0}. Stok tersedia dihitung dari kapasitas.</p>{/if}
              <button class="scan-submit staff-submit" type="submit" disabled={saving || !currentEvent?.zones.length}>{tierSelected ? "Simpan kategori" : "Tambah kategori"}</button>
            </form>
          </section>
        {/if}
      </section>
    </div>
    <footer class="scan-footer"><span>Tiket Online · Admin tools</span><span>Pengelolaan informasi dan publikasi konser</span></footer>
</AdminLayout>

<style>
  .event-admin-grid :global(.staff-form) { font-size: 16px; }
  .event-admin-grid :global(.staff-form > label), .event-list-filters label { font-size: 16px; line-height: 1.45; }
  .event-admin-grid :global(.staff-form small), .event-list-filters :global(small) { font-size: 14px; }
  .event-admin-form { gap: 20px; }
  .event-group { min-width: 0; margin: 0; padding: 18px 0 0; border: 0; border-top: 1px solid var(--scan-line); }
  .event-group legend { padding: 0 12px 0 0; color: var(--scan-text); font-size: 18px; font-weight: 700; }
  .event-group > :global(label) { margin-top: 16px; }
  .event-group > :global(label:first-of-type) { margin-top: 0; }
  .event-unsaved { margin: 0 0 12px; padding: 10px 12px; border-left: 3px solid var(--scan-lime); color: var(--scan-text); font-size: 14px; }
  .poster-preview { display: grid; gap: 10px; }
  .poster-preview h3 { margin: 0; color: var(--scan-muted); font-size: 14px; font-weight: 400; }
  .poster-preview img { display: block; width: min(100%, 280px); max-height: 380px; object-fit: contain; object-position: left center; background: #111918; }
  .event-editor-panel :global(button), .event-editor-panel :global(input), .event-editor-panel :global(select), .event-editor-panel :global(textarea) { font-size: 16px; }
  .event-editor-panel :global(button) { min-height: 44px; }
  .event-admin-list button { min-height: 64px; }
  @media (max-width: 600px) { .event-group { padding-top: 14px; } .event-group legend { font-size: 17px; } }
</style>
