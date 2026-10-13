<script lang="ts">
  import { MediaQuery } from "svelte/reactivity";
  import AdminMobileList from "../../components/admin/AdminMobileList.svelte";
  import { mobileListItems } from "../../lib/mobile-list.ts";
  const mobileList = new MediaQuery("(max-width: 767px)");

  import AdminMobileFilters from "../../components/admin/AdminMobileFilters.svelte";
  import AdminHelp from "../../components/admin/AdminHelp.svelte";
  import AdminLayout from "../../layouts/AdminLayout.svelte";
  import PosterEditor from "../../components/admin/PosterEditor.svelte";
  import EventChangePanel from "../../components/admin/EventChangePanel.svelte";
  import { onDestroy, onMount, tick } from "svelte";
  import { ApiError, createAdminEvent, createAdminTier, createAdminZone, getAdminEvents, updateAdminEvent, updateAdminTier, updateAdminZone, type AdminEventInput, type AdminApiEvent, type AdminTierInput, type PublicationStatus } from "../../lib/api.ts";
  import { publicationGaps } from "../../lib/event-publication.ts";
  import { suggestAdminId } from "../../lib/admin-id.ts";
  import { adminDate, adminTime, adminNumber } from "../../lib/admin-operations.ts";

  let searchOpen = $state(false);
  let mobileGroup = $state(0);
  let mobileScroll = $state(0);
  let { accessToken, onUnauthorized, onLogout, loggingOut }: { accessToken: string; onUnauthorized: () => void; onLogout: () => void; loggingOut: boolean } = $props();
  let events = $state<AdminApiEvent[]>([]);
  let selectedId = $state("");
  let mode = $state<"list" | "create" | "edit">("list");
  // ponytail: progress stays in memory; persist it only when refresh recovery is required.
  let section = $state(0);
  let visited = $state(0);
  let pendingSection = $state<number | null>(null);
  const sections = ["Informasi", "Jadwal & lokasi", "Materi", "Zona", "Kategori tiket", "Publikasi", "Perubahan acara"];
  const icons = ["M12 11v6m0-10v1M4 4h16v16H4z", "M5 5h14v15H5zM8 3v4m8-4v4M5 10h14", "M3 4h18v16H3zM3 16l5-5 4 4 3-3 6 6M16 8h1", "M3 5l6-2 6 2 6-2v16l-6 2-6-2-6 2zM9 3v16m6-14v16", "M3 6h18v4a2 2 0 0 0 0 4v4H3v-4a2 2 0 0 0 0-4zM15 6v12", "M12 16V3m-4 4 4-4 4 4M4 13v7h16v-7", "M4 8a8 8 0 1 1 0 8M4 3v5h5m3 1v4l3 2"];
  const sectionLabel = $derived(mode === "create" && section === 5 ? "Tinjau" : sections[section]);
  let listScroll = 0;
  let listTrigger = "event-create-opener";
  let listError = $state("");
  const publicationLabel = { DRAFT: "Draf", PUBLISHED: "Terbit", ARCHIVED: "Diarsipkan" };
  const selectedEvent = $derived(events.find((event) => event.id === selectedId));
  const scheduleLocked = $derived(Boolean(selectedEvent?.scheduleLocked || (selectedEvent?.currentEvent?.version ?? 0) > 0));
  const locationLocked = $derived(Boolean(selectedId && selectedEvent?.locationLocked));
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
  let posterGeneration = $state(0);
  let posterUploading = $state(false);
  let posterPending = $state(false);
  let changePanelDirty = $state(false);
  const eventState = () => JSON.stringify({ draft, lineupText });
  const zoneState = () => JSON.stringify(zoneDraft);
  const tierState = () => JSON.stringify(tierDraft);
  const hasUnsaved = $derived(mode !== "list" && (eventState() !== eventBaseline || posterPending || posterUploading || changePanelDirty || (Boolean(selectedId) && (zoneState() !== zoneBaseline || tierState() !== tierBaseline))));
  const gaps = $derived(publicationGaps({ ...draft, lineup: lineupText.split("\n") }, selectedEvent?.zones ?? [], selectedEvent?.ticketTiers ?? []));
  const inventoryDirty = $derived(zoneState() !== zoneBaseline || tierState() !== tierBaseline);

  function emptyEvent(): AdminEventInput { return { id: "", artist: "", city: "", venue: "", address: "", startsAt: "", genre: "Pop", status: "Presale", publicationStatus: "DRAFT", image: "", description: "", lineup: [] }; }
  function protectUnsaved(event: BeforeUnloadEvent) {
    if (hasUnsaved || saving) { event.preventDefault(); event.returnValue = ""; }
  }
  function allowDiscard() { return !hasUnsaved || window.confirm("Perubahan yang belum disimpan akan hilang. Lanjutkan?"); }
  function requestLogout() { if (!saving && allowDiscard()) onLogout(); }
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

  $effect(() => { query; publicationFilter; mobileGroup = 0; mobileScroll = 0; });

  function select(event: AdminApiEvent, discardChecked = false) {
    if ((saving && !discardChecked) || event.id === selectedId || (!discardChecked && !allowDiscard())) return;
    if (mode === "list") { listScroll = window.scrollY; listTrigger = `event-manage-${event.id}`; }
    mode = "edit"; section = 0; visited = 0; pendingSection = null;
    selectedId = event.id;
    draft = { id: event.id, artist: event.artist, city: event.city, venue: event.venue, address: event.address, startsAt: localDateTime(event.startsAt), genre: event.genre, status: event.status, publicationStatus: event.publicationStatus, image: event.image, description: event.description, lineup: event.lineup };
    lineupText = event.lineup.join("\n");
    zoneSelected = ""; zoneDraft = { id: "", name: "", description: "" };
    tierSelected = ""; tierDraft = { id: "", name: "", zoneId: event.zones[0]?.id ?? "", price: 0, capacity: 0, maxPerOrder: 1, benefit: "", gate: "", seating: "free-standing" };
    eventIdManual = false; zoneIdManual = false; tierIdManual = false;
    eventBaseline = eventState(); zoneBaseline = zoneState(); tierBaseline = tierState();
    posterGeneration++; posterPending = false; posterUploading = false;
    changePanelDirty = false;
    error = ""; notice = "";
    void tick().then(() => document.getElementById("event-editor-title")?.focus());
  }
  async function startNew() {
    if (saving || !allowDiscard()) return;
    if (mode === "list") listScroll = window.scrollY;
    listTrigger = "event-create-opener"; mode = "create"; section = 0; visited = 0; pendingSection = null;
    selectedId = ""; draft = emptyEvent(); lineupText = ""; eventIdManual = false; zoneIdManual = false; tierIdManual = false;
    zoneSelected = ""; zoneDraft = { id: "", name: "", description: "" }; tierSelected = ""; tierDraft = { id: "", name: "", zoneId: "", price: 0, capacity: 0, maxPerOrder: 1, benefit: "", gate: "", seating: "free-standing" };
    eventBaseline = eventState(); zoneBaseline = zoneState(); tierBaseline = tierState(); posterGeneration++; posterPending = false; posterUploading = false; changePanelDirty = false; error = ""; notice = "";
    await tick();
    document.getElementById("event-editor-title")?.focus();
  }
  async function backToList(resultNotice = "") {
    if (saving || !allowDiscard()) return;
    const index = visible.findIndex(item => `event-manage-${item.id}` === listTrigger);
    if (mobileList.current && index >= 0 && Math.floor(index / 5) !== mobileGroup) { mobileGroup = Math.floor(index / 5); mobileScroll = 0; }
    mode = "list"; selectedId = ""; posterGeneration++; posterPending = false; posterUploading = false; changePanelDirty = false;
    error = ""; notice = resultNotice;
    await tick();
    (document.getElementById(listTrigger) ?? document.getElementById("event-list-title"))?.focus({ preventScroll: true });
    window.scrollTo({ top: listScroll, behavior: "instant" });
  }

  async function load() {
    request?.abort(); const controller = new AbortController(); request = controller; const current = ++generation; loading = true; listError = "";
    try { const result = await getAdminEvents(accessToken, controller.signal); if (current === generation) events = result; }
    catch (cause) {
      if (current !== generation || (cause instanceof DOMException && cause.name === "AbortError")) return;
      if (cause instanceof ApiError && cause.status === 401) onUnauthorized();
      else listError = "Daftar konser gagal dimuat. Data terakhir tetap tersedia bila sudah pernah dimuat.";
    } finally { if (current === generation) { loading = false; if (request === controller) request = undefined; } }
  }

  async function showSection(next: number) {
    section = next; visited = Math.max(visited, next); pendingSection = null;
    await tick(); document.getElementById("event-section-title")?.focus();
  }
  async function validateFields(scope = "#event-form") {
    for (const control of document.querySelectorAll<HTMLInputElement | HTMLSelectElement | HTMLTextAreaElement>(`${scope} input, ${scope} select, ${scope} textarea`)) {
      if (!control.disabled && !control.checkValidity()) {
        const target = control.closest<HTMLElement>("[data-section]");
        if (target) await showSection(Number(target.dataset.section));
        control.reportValidity(); return false;
      }
    }
    return true;
  }
  async function navigate(next: number) {
    if (saving || next === section) return;
    if (mode === "create") {
      if (next > visited + 1) return;
      if ((section === 3 && zoneState() !== zoneBaseline) || (section === 4 && tierState() !== tierBaseline)) { pendingSection = next; return; }
      if (next > section && section < 3 && !await validateFields(`[data-section="${section}"]`)) return;
      if (next >= 3 && !selectedId && !await saveEvent("DRAFT")) return;
    }
    await showSection(next);
  }
  function discardInventory() {
    const next = pendingSection;
    if (section === 3) { zoneDraft = JSON.parse(zoneBaseline); zoneIdManual = Boolean(zoneSelected); }
    else { tierDraft = JSON.parse(tierBaseline); tierIdManual = Boolean(tierSelected); }
    if (next !== null) void showSection(next);
  }
  async function save(event: SubmitEvent) {
    event.preventDefault();
    if (mode === "create") { if (section < 5) await navigate(section + 1); }
    else await saveEvent();
  }
  async function saveEvent(publication = draft.publicationStatus) {
    if (saving || posterUploading || posterPending || !await validateFields()) return false;
    if (publication === "PUBLISHED" && (gaps.length || inventoryDirty)) {
      error = inventoryDirty ? "Simpan perubahan zona dan kategori sebelum menerbitkan konser." : "Lengkapi bagian yang ditandai sebelum menerbitkan konser.";
      await showSection(5); return false;
    }
    if (saving) return false;
    saving = true; error = ""; notice = "";
    const creating = !selectedId;
    try {
      const value = { ...payload(), publicationStatus: publication };
      const result = selectedId ? await updateAdminEvent(accessToken, selectedId, value) : await createAdminEvent(accessToken, value);
      events = [...events.filter(item => item.id !== result.id), result].sort((a, b) => a.startsAt.localeCompare(b.startsAt));
      selectedId = result.id;
      draft = { id: result.id, artist: result.artist, city: result.city, venue: result.venue, address: result.address, startsAt: localDateTime(result.startsAt), genre: result.genre, status: result.status, publicationStatus: result.publicationStatus, image: result.image, description: result.description, lineup: result.lineup };
      lineupText = result.lineup.join("\n"); eventBaseline = eventState();
      posterGeneration++; posterPending = false; posterUploading = false;
      notice = creating ? "Draf konser dibuat. Atur zona dan kategori tiket sebelum menerbitkan konser." : "Perubahan konser tersimpan.";
      return true;
    } catch (cause) {
      if (cause instanceof ApiError && cause.status === 401) onUnauthorized();
      else if (cause instanceof ApiError && cause.code === "SCHEDULE_LOCKED") error = "Jadwal tidak dapat diubah karena konser sudah memiliki pesanan.";
      else if (cause instanceof ApiError && cause.code === "DUPLICATE_EVENT") { error = "ID konser sudah digunakan. Ubah ID lalu coba lagi."; saving = false; await showSection(0); document.querySelector<HTMLInputElement>("#event-id")?.focus(); }
      else error = cause instanceof ApiError ? cause.message : "Konser belum dapat disimpan. Periksa koneksi lalu coba lagi.";
      return false;
    } finally { saving = false; }
  }
  async function finish(publication: PublicationStatus) {
    if (inventoryDirty) { error = "Simpan atau buang perubahan zona dan kategori sebelum menyelesaikan konser."; return; }
    if (!await saveEvent(publication)) return;
    listTrigger = `event-manage-${selectedId}`;
    await backToList(publication === "PUBLISHED" ? "Konser berhasil diterbitkan." : "Draf konser tersimpan. Lanjutkan melalui Kelola konser.");
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
    event.preventDefault(); if (!selectedId || saving || !await validateFields("#zone-form")) return;
    if (saving) return;
    saving = true; error = ""; notice = "";
    let committed = false;
    try {
      const result = zoneSelected ? await updateAdminZone(accessToken, selectedId, zoneSelected, { name: zoneDraft.name, description: zoneDraft.description }) : await createAdminZone(accessToken, selectedId, zoneDraft);
      zoneSelected = result.id; zoneIdManual = true;
      events = events.map(item => item.id === selectedId ? { ...item, zones: [...item.zones.filter(zone => zone.id !== result.id), result] } : item);
      committed = true; zoneBaseline = zoneState(); notice = "Zona tersimpan.";
      try { events = await getAdminEvents(accessToken); }
      catch (cause) { if (cause instanceof ApiError && cause.status === 401) onUnauthorized(); notice = "Zona tersimpan; daftar belum dapat dimuat ulang."; }
    } catch (cause) {
      if (committed) { notice = "Zona tersimpan; daftar belum dapat dimuat ulang."; return; }
      if (cause instanceof ApiError && cause.status === 401) onUnauthorized();
      else if (cause instanceof ApiError && cause.code === "DUPLICATE_ZONE") { error = "ID zona sudah digunakan pada konser ini. Ubah ID lalu coba lagi."; saving = false; await tick(); document.querySelector<HTMLInputElement>("#zone-id")?.focus(); }
      else error = cause instanceof ApiError ? cause.message : "Zona belum dapat disimpan.";
    } finally { saving = false; if (committed && pendingSection !== null) await showSection(pendingSection); }
  }
  async function saveTier(event: SubmitEvent) {
    event.preventDefault(); if (!selectedId || saving || !await validateFields("#tier-form")) return;
    if (saving) return;
    saving = true; error = ""; notice = "";
    let committed = false;
    try {
      const result = tierSelected ? await updateAdminTier(accessToken, selectedId, tierSelected, tierDraft) : await createAdminTier(accessToken, selectedId, tierDraft);
      tierSelected = result.id; tierIdManual = true;
      events = events.map(item => item.id === selectedId ? { ...item, ticketTiers: [...item.ticketTiers.filter(tier => tier.id !== result.id), result] } : item);
      committed = true; tierBaseline = tierState(); notice = "Kategori tiket tersimpan.";
      try { events = await getAdminEvents(accessToken); }
      catch (cause) { if (cause instanceof ApiError && cause.status === 401) onUnauthorized(); notice = "Kategori tersimpan; daftar belum dapat dimuat ulang."; }
    } catch (cause) {
      if (committed) { notice = "Kategori tersimpan; daftar belum dapat dimuat ulang."; return; }
      if (cause instanceof ApiError && cause.status === 401) onUnauthorized();
      else if (cause instanceof ApiError && cause.code === "DUPLICATE_TIER") { error = "ID kategori sudah digunakan pada konser ini. Ubah ID lalu coba lagi."; saving = false; await tick(); document.querySelector<HTMLInputElement>("#tier-id")?.focus(); }
      else error = cause instanceof ApiError ? cause.message : "Kategori tiket belum dapat disimpan.";
    } finally { saving = false; if (committed && pendingSection !== null) await showSection(pendingSection); }
  }

  onMount(() => { eventBaseline = eventState(); zoneBaseline = zoneState(); tierBaseline = tierState(); void load(); });
  onDestroy(() => { generation++; request?.abort(); });
</script>

<svelte:window onbeforeunload={protectUnsaved} />

<svelte:head><title>Kelola Konser | Tiket Online</title><meta name="robots" content="noindex" /></svelte:head>

  <AdminLayout page="events" mobileDescriptionHelp={mode === "list"} contentId="event-admin-content" skipLabel="Lewati ke pengelolaan konser" kicker="KATALOG EVENT • ADMIN" title={mode === "list" ? "Daftar konser." : mode === "create" ? "Tambah konser." : "Edit konser."} description={mode === "list" ? "Temukan konser atau buat draf baru." : mode === "create" ? "Buat draf konser, lalu lengkapi zona dan kategori tiket." : "Ubah informasi, tiket, dan publikasi konser yang dipilih."} onLogout={requestLogout} loggingOut={loggingOut} logoutDisabled={loggingOut || saving} primaryAction={mode === "list" ? { label: "Tambah konser", onclick: startNew, disabled: saving, id: "event-create-opener" } : undefined}>
    <div class="staff-admin-grid event-admin-grid">
      {#if mode === "list"}
      <section class="staff-admin-panel" aria-labelledby="event-list-title">
        <div class="staff-panel-heading"><div><span class="panel-index">01</span><h2 id="event-list-title" tabindex="-1">Daftar konser</h2></div><span class="staff-muted">{visible.length} konser</span></div>
        <AdminMobileFilters id="event-mobile-filters" bind:open={searchOpen}><div class="event-list-filters"><label>Cari konser<input bind:value={query} type="search" placeholder="Nama, kota, atau venue" disabled={loading} /></label><label>Status publikasi<select bind:value={publicationFilter} disabled={loading}><option value="">Semua status</option><option value="DRAFT">Draf</option><option value="PUBLISHED">Terbit</option><option value="ARCHIVED">Diarsipkan</option></select></label></div></AdminMobileFilters>
        {#if listError}<p class="staff-form-message staff-form-error" role="alert">{listError} <button class="staff-text-button" type="button" disabled={loading} onclick={load}>Coba lagi</button></p>{/if}
        {#if notice}<p class="staff-form-message" role="status">{notice}</p>{/if}
        {#if loading}<p class="staff-muted" role="status">Memuat konser…</p>
        {:else if !events.length && !listError}<p class="staff-muted" role="status">Belum ada konser. Pilih Tambah konser untuk membuat draf pertama.</p>
        {:else if !visible.length && events.length}<p class="staff-muted" role="status">Tidak ada konser yang cocok dengan pencarian atau filter.</p>
        {:else}<AdminMobileList count={visible.length} bind:group={mobileGroup} bind:scrollLeft={mobileScroll} headingId="event-list-title"><ul class="event-admin-list">{#each mobileListItems(visible, mobileGroup, mobileList.current) as item (item.id)}<li class:admin-mobile-card={mobileList.current}>
          {#if mobileList.current}
            <div class="admin-mobile-card-header"><strong>{item.artist || item.id}</strong><span class="admin-order-status" class:is-paid={item.publicationStatus === "PUBLISHED"}>{publicationLabel[item.publicationStatus]}</span></div>
            <dl class="admin-card-fields">
              <div><dt>Tanggal</dt><dd>{#if item.startsAt}<time datetime={item.startsAt}>{adminDate.format(new Date(item.startsAt))}</time>{:else}Jadwal belum diatur{/if}</dd></div>
              <div><dt>Waktu</dt><dd>{item.startsAt ? `${adminTime.format(new Date(item.startsAt))} WIB` : "Belum diatur"}</dd></div>
              <div><dt>Kota</dt><dd>{item.city || "Kota belum diatur"}</dd></div>
              <div><dt>Venue</dt><dd>{item.venue || "Venue belum diatur"}</dd></div>
              <div><dt>Kuota tiket</dt><dd>{adminNumber.format(item.ticketTiers.reduce((sum, tier) => sum + tier.capacity, 0))}</dd></div>
              <div><dt>Tiket tersedia</dt><dd>{adminNumber.format(item.ticketTiers.reduce((sum, tier) => sum + tier.availableQuantity, 0))}</dd></div>
            </dl>
          {:else}<div><strong>{item.artist || item.id}</strong><small>{item.startsAt ? `${new Date(item.startsAt).toLocaleString("id-ID", { timeZone: "Asia/Jakarta" })} WIB` : "Jadwal belum diatur"}</small><small>{item.city || "Kota belum diatur"} · {item.venue || "Venue belum diatur"}</small></div><span class={`event-publication event-${item.publicationStatus.toLowerCase()}`}>{publicationLabel[item.publicationStatus]}</span>{/if}
          <button id={`event-manage-${item.id}`} class="staff-secondary-button" type="button" aria-label={`Kelola konser ${item.artist || item.id}`} onclick={() => select(item)}>Kelola konser</button>
        </li>{/each}</ul></AdminMobileList>{/if}
      </section>
      {:else}
      <section class="staff-admin-panel event-editor-panel" aria-labelledby="event-editor-title">
        <div class="staff-panel-heading"><h2 id="event-editor-title" tabindex="-1">{mode === "edit" ? `Edit konser: ${draft.artist}` : "Tambah konser"}</h2><button class="staff-secondary-button" type="button" disabled={saving} onclick={() => backToList()}>Kembali ke daftar</button></div>
        {#if selectedId}<p class={`event-publication event-${draft.publicationStatus.toLowerCase()}`}>{publicationLabel[draft.publicationStatus]}</p>{/if}
        {#if error}<p class="staff-form-message staff-form-error" role="alert">{error}</p>{/if}
        {#if notice}<p class="staff-form-message event-save-notice" role="status">{notice}</p>{/if}
        {#if hasUnsaved}<p class="event-unsaved" role="status">Perubahan belum disimpan</p>{/if}
        <div class="event-workspace">
        <nav class="event-section-nav" aria-label={mode === "create" ? "Langkah tambah konser" : "Fitur edit konser"}>
          {#if mode === "create"}<p class="event-progress" aria-live="polite">Langkah {section + 1} dari 6</p><label class="event-step-picker">Pilih langkah<select value={section} disabled={saving} onchange={(event) => { const next = Number(event.currentTarget.value); event.currentTarget.value = String(section); void navigate(next); }}>{#each sections.slice(0, 6) as label, index}<option value={index} disabled={index > visited}>{index + 1}. {index === 5 ? "Tinjau" : label}</option>{/each}</select></label>{/if}
          <ol class:create-steps={mode === "create"}>
            {#each sections.slice(0, mode === "create" ? 6 : 7) as label, index}<li><button type="button" aria-current={section === index ? "step" : undefined} disabled={saving || (mode === "create" && index > visited)} onclick={() => navigate(index)}><svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d={icons[index]} /></svg><span>{mode === "create" && index === 5 ? "Tinjau" : label}</span></button></li>{/each}
          </ol>
        </nav>
        <div class="event-section-content">
        <h3 id="event-section-title" tabindex="-1">{sectionLabel}</h3>
        {#if pendingSection !== null}<div class="event-leave-prompt" role="alert"><p>Perubahan {section === 3 ? "zona" : "kategori tiket"} belum disimpan.</p><div class="event-actions"><button class="staff-secondary-button" type="submit" form={section === 3 ? "zone-form" : "tier-form"} disabled={saving}>Simpan & lanjutkan</button><button class="staff-secondary-button" type="button" disabled={saving} onclick={discardInventory}>Buang & lanjutkan</button><button class="staff-text-button" type="button" disabled={saving} onclick={() => pendingSection = null}>Tetap di sini</button></div></div>{/if}
        <form id="event-form" hidden={[3, 4, 6].includes(section)} novalidate class="staff-form event-admin-form" onsubmit={save} aria-busy={saving}>
          <fieldset class="event-group" data-section="0" hidden={section !== 0}><legend class="sr-only">Informasi dasar</legend>
          <label class:event-full={Boolean(selectedId)}>Nama konser / artis<input bind:value={draft.artist} oninput={suggestEventId} minlength="2" maxlength="160" required disabled={saving} /></label>
          {#if !selectedId}<label>ID URL konser<input id="event-id" bind:value={draft.id} oninput={() => eventIdManual = true} pattern="[a-z0-9]+(-[a-z0-9]+)*" maxlength="64" placeholder="nama-konser" required disabled={saving} /><small>Saran ID terisi dari nama konser dan tetap dapat diedit. Gunakan huruf kecil, angka, dan tanda hubung.</small></label>{/if}
          {#if !selectedId && draft.id && events.some((item) => item.id === draft.id)}<p class="staff-form-message staff-form-error" role="alert">ID konser sudah digunakan. Ubah ID sebelum menyimpan.</p>{/if}
          <label>Genre<select bind:value={draft.genre} disabled={saving}><option>Rock</option><option>Pop</option><option>Indie</option></select></label>
          <label>Label penjualan<select bind:value={draft.status} disabled={saving}><option>Early Bird</option><option>Presale</option><option>Sold Out</option></select></label></fieldset>
          <fieldset class="event-group" data-section="1" hidden={section !== 1}><legend class="sr-only">Jadwal dan lokasi</legend>
          <div class="event-field event-full">
            {#if selectedId && scheduleLocked}<AdminHelp id="schedule-lock-note" label="Penjelasan jadwal terkunci" desktop text="Jadwal terkunci setelah pesanan dibuat atau perubahan acara dicatat. Gunakan panel Pembatalan dan perubahan jadwal di bawah untuk menjaga hak pembeli dan riwayat keputusan."><label for="event-starts-at">Jadwal (WIB)</label></AdminHelp>{:else}<label for="event-starts-at">Jadwal (WIB)</label>{/if}
            <input id="event-starts-at" type="datetime-local" bind:value={draft.startsAt} aria-describedby={scheduleLocked ? "schedule-lock-note" : undefined} required={draft.publicationStatus === "PUBLISHED"} disabled={saving || scheduleLocked} />
          </div>
          <div class="event-form-row"><label>Kota<input bind:value={draft.city} maxlength="100" aria-describedby={locationLocked ? "location-lock-note" : undefined} required={draft.publicationStatus === "PUBLISHED"} disabled={saving || locationLocked} /></label><label>Venue<input bind:value={draft.venue} maxlength="160" aria-describedby={locationLocked ? "location-lock-note" : undefined} required={draft.publicationStatus === "PUBLISHED"} disabled={saving || locationLocked} /></label></div>
          <div class="event-field event-full">
            {#if locationLocked}<AdminHelp id="location-lock-note" label="Penjelasan lokasi terkunci" desktop text="Kota, venue, dan alamat terkunci setelah reservasi pertama untuk menjaga informasi pada pesanan pembeli. Perubahan lokasi tidak tersedia di panel perubahan acara."><label for="event-address">Alamat</label></AdminHelp>{:else}<label for="event-address">Alamat</label>{/if}
            <input id="event-address" bind:value={draft.address} maxlength="255" aria-describedby={locationLocked ? "location-lock-note" : undefined} required={draft.publicationStatus === "PUBLISHED"} disabled={saving || locationLocked} />
          </div>
          </fieldset>
          <fieldset class="event-group" data-section="2" hidden={section !== 2}><legend class="sr-only">Materi publikasi</legend>
          {#key posterGeneration}<PosterEditor bind:image={draft.image} bind:uploading={posterUploading} bind:pending={posterPending} artist={draft.artist} required={draft.publicationStatus === "PUBLISHED"} disabled={saving} {accessToken} {onUnauthorized} />{/key}
          <label>Deskripsi<textarea bind:value={draft.description} rows="3" maxlength="16000" required={draft.publicationStatus === "PUBLISHED"} disabled={saving}></textarea></label>
          <label>Lineup<textarea bind:value={lineupText} rows="3" maxlength="16000" required={draft.publicationStatus === "PUBLISHED"} disabled={saving}></textarea><small>Satu nama per baris</small></label>
          </fieldset>
          <fieldset class="event-group" data-section="5" hidden={section !== 5}><legend class="sr-only">{mode === "create" ? "Tinjau konser" : "Publikasi"}</legend>
          {#if mode === "edit"}<label class="event-full">Status publikasi<select bind:value={draft.publicationStatus} disabled={saving}><option value="DRAFT">Draf (belum tampil di katalog)</option><option value="PUBLISHED">Terbit (tampil di katalog)</option><option value="ARCHIVED">Diarsipkan (disembunyikan dari katalog)</option></select></label>{/if}
          <dl class="event-review">
            <div><dt>Konser</dt><dd>{draft.artist || "Belum diisi"} · {draft.genre} · {draft.status}</dd></div>
            <div><dt>Jadwal WIB</dt><dd>{draft.startsAt ? `${adminDate.format(new Date(`${draft.startsAt}:00+07:00`))} · ${adminTime.format(new Date(`${draft.startsAt}:00+07:00`))} WIB` : "Belum diatur"}</dd></div>
            <div><dt>Lokasi</dt><dd>{[draft.city, draft.venue, draft.address].filter(Boolean).join(" · ") || "Belum diisi"}</dd></div>
            <div><dt>Materi</dt><dd>{draft.description || "Deskripsi belum diisi"}<br />{lineupText || "Lineup belum diisi"}</dd></div>
            <div><dt>Poster</dt><dd>{draft.image || "Belum diisi"}</dd></div>
            <div><dt>Zona</dt><dd>{selectedEvent?.zones.map(zone => zone.name).join(", ") || "Belum tersedia"}</dd></div>
            <div><dt>Kategori tiket</dt><dd>{#each selectedEvent?.ticketTiers ?? [] as tier}<p>{tier.name} · Rp{adminNumber.format(tier.price)} · kapasitas {adminNumber.format(tier.capacity)} · {tier.gate}</p>{:else}Belum tersedia{/each}</dd></div>
          </dl>
          {#if gaps.length}<p class="staff-muted">Lengkapi sebelum menerbitkan:</p><ul class="event-gaps">{#each gaps as gap}<li><button class="staff-text-button" type="button" disabled={saving} onclick={() => navigate(gap.section)}>{gap.label}</button></li>{/each}</ul>{:else}<p class="staff-muted">Informasi konser sudah lengkap untuk diterbitkan.</p>{/if}
          </fieldset>
          {#if mode === "edit"}<button class="scan-submit staff-submit" type="submit" disabled={saving || loading || posterUploading || posterPending}>{saving ? "Menyimpan…" : "Simpan perubahan"}</button>{/if}
        </form>
        {#if selectedId}
          {@const currentEvent = events.find((item) => item.id === selectedId)}
      <div hidden={section !== 6}>{#if selectedEvent}{#key selectedId}<EventChangePanel event={selectedEvent} {accessToken} onSaved={load} {onUnauthorized} onDirtyChange={setChangePanelDirty} />{/key}{/if}</div>
          <section class="event-inventory-editor event-admin-form" data-section="3" hidden={section !== 3} aria-label="Pengaturan zona">
            <label>Zona<select value={zoneSelected} onchange={(event) => { selectZone(event.currentTarget.value); event.currentTarget.value = zoneSelected; }} disabled={saving}><option value="">Tambah zona</option>{#each currentEvent?.zones ?? [] as zone}<option value={zone.id}>{zone.name}</option>{/each}</select></label>
            <form id="zone-form" class="staff-form event-admin-form" onsubmit={saveZone}>
              <label class:event-full={Boolean(zoneSelected)}>Nama zona<input bind:value={zoneDraft.name} oninput={suggestZoneId} maxlength="100" required disabled={saving} /></label>
              {#if !zoneSelected}<label>ID zona<input id="zone-id" bind:value={zoneDraft.id} oninput={() => zoneIdManual = true} pattern="[a-z0-9]+(-[a-z0-9]+)*" maxlength="64" required disabled={saving} /><small>Saran ID mengikuti nama zona sampai ID diedit.</small></label>{/if}
              {#if !zoneSelected && zoneDraft.id && currentEvent?.zones.some((item) => item.id === zoneDraft.id)}<p class="staff-form-message staff-form-error" role="alert">ID zona sudah digunakan pada konser ini. Ubah ID sebelum menyimpan.</p>{/if}
              <label class="event-full">Deskripsi zona<input bind:value={zoneDraft.description} maxlength="255" disabled={saving} /></label>
              <button class="staff-secondary-button" type="submit" disabled={saving || !zoneDraft.name}>{zoneSelected ? "Simpan zona" : "Tambah zona"}</button>
            </form>
          </section>
          <section class="event-inventory-editor event-admin-form" data-section="4" hidden={section !== 4} aria-label="Pengaturan kategori tiket">
            {#if currentEvent?.ticketTiers.length}<div class="event-inventory-list"><h4>Kategori tersimpan</h4>{#each currentEvent?.ticketTiers ?? [] as tier}<button class="staff-secondary-button" type="button" disabled={saving} onclick={() => selectTier(tier.id)}>{tier.name} · {adminNumber.format(tier.availableQuantity)}/{adminNumber.format(tier.capacity)} tersedia · {adminNumber.format(tier.boundQuantity)} stok terikat · {tier.gateLocked ? "Gate terkunci" : "Gate dapat diedit"}</button>{/each}</div>{/if}
            {#if !currentEvent?.zones.length}<p class="staff-muted">Tambahkan zona pada langkah Zona sebelum membuat kategori tiket.</p>{/if}
            <label>Kategori<select value={tierSelected} onchange={(event) => { selectTier(event.currentTarget.value); event.currentTarget.value = tierSelected; }} disabled={saving}><option value="">Tambah kategori</option>{#each currentEvent?.ticketTiers ?? [] as tier}<option value={tier.id}>{tier.name}</option>{/each}</select></label>
            <form id="tier-form" class="staff-form event-admin-form" onsubmit={saveTier}>
              <label class:event-full={Boolean(tierSelected)}>Nama kategori<input bind:value={tierDraft.name} oninput={suggestTierId} maxlength="100" required disabled={saving} /></label>
              {#if !tierSelected}<label>ID kategori<input id="tier-id" bind:value={tierDraft.id} oninput={() => tierIdManual = true} pattern="[a-z0-9]+(-[a-z0-9]+)*" maxlength="64" required disabled={saving} /><small>Saran ID mengikuti nama kategori sampai ID diedit.</small></label>{/if}
              {#if !tierSelected && tierDraft.id && currentEvent?.ticketTiers.some((item) => item.id === tierDraft.id)}<p class="staff-form-message staff-form-error" role="alert">ID kategori sudah digunakan pada konser ini. Ubah ID sebelum menyimpan.</p>{/if}
              <label>Zona<select bind:value={tierDraft.zoneId} required disabled={saving || !currentEvent?.zones.length}><option value="">Pilih zona</option>{#each currentEvent?.zones ?? [] as zone}<option value={zone.id}>{zone.name}</option>{/each}</select></label>
              <label>Seating<select bind:value={tierDraft.seating} disabled={saving}><option value="free-standing">Berdiri</option><option value="assigned">Tempat duduk</option></select></label>
              <div class="event-form-row"><label>Harga (rupiah)<input type="number" inputmode="numeric" min="1" step="1" bind:value={tierDraft.price} required disabled={saving} /></label><label>Kapasitas<input type="number" inputmode="numeric" min="0" step="1" bind:value={tierDraft.capacity} required disabled={saving} /></label></div>
              <div class="event-form-row"><label>Batas pembelian<input type="number" inputmode="numeric" min="1" step="1" bind:value={tierDraft.maxPerOrder} required disabled={saving} /></label><div class="event-field"><label for="tier-gate">Gate</label><input id="tier-gate" aria-describedby={currentEvent?.ticketTiers.find((item) => item.id === tierSelected)?.gateLocked ? "tier-gate-note" : undefined} bind:value={tierDraft.gate} maxlength="100" required disabled={saving || Boolean(currentEvent?.ticketTiers.find((item) => item.id === tierSelected)?.gateLocked)} />{#if currentEvent?.ticketTiers.find((item) => item.id === tierSelected)?.gateLocked}<small id="tier-gate-note" class="event-lock-note">Gate dikunci setelah reservasi pertama untuk kategori ini.</small>{/if}</div></div>
              <label class="event-full">Benefit<input bind:value={tierDraft.benefit} maxlength="255" disabled={saving} /></label>
              {#if tierSelected}<p class="staff-muted">Tersedia: {adminNumber.format(currentEvent?.ticketTiers.find((item) => item.id === tierSelected)?.availableQuantity ?? 0)}; stok terikat: {adminNumber.format(currentEvent?.ticketTiers.find((item) => item.id === tierSelected)?.boundQuantity ?? 0)}. Stok tersedia dihitung dari kapasitas.</p>{/if}
              <button class="scan-submit staff-submit" type="submit" disabled={saving || !currentEvent?.zones.length}>{tierSelected ? "Simpan kategori" : "Tambah kategori"}</button>
            </form>
          </section>
        {/if}
        {#if mode === "create"}<div class="event-actions event-wizard-actions">
          {#if section > 0}<button class="staff-secondary-button" type="button" disabled={saving} onclick={() => navigate(section - 1)}>Sebelumnya</button>{/if}
          {#if section < 5}<button class="scan-submit" type={section < 3 ? "submit" : "button"} form={section < 3 ? "event-form" : undefined} disabled={saving || loading || (section === 2 && (posterPending || posterUploading))} onclick={section < 3 ? undefined : () => navigate(section + 1)}>{saving ? "Menyimpan…" : section === 2 && !selectedId ? "Simpan draf & lanjutkan" : "Berikutnya"}</button>
          {:else}<button class="staff-secondary-button" type="button" disabled={saving || posterPending || posterUploading || inventoryDirty} onclick={() => finish("DRAFT")}>Simpan sebagai draf</button><button class="scan-submit" type="button" disabled={saving || loading || gaps.length > 0 || inventoryDirty || posterPending || posterUploading} onclick={() => finish("PUBLISHED")}>{saving ? "Menyimpan…" : "Terbitkan konser"}</button>{/if}
        </div>{/if}
        </div></div>
      </section>
      {/if}
    </div>
    <footer class="scan-footer"><span>Tiket Online · Admin tools</span><span>Pengelolaan informasi dan publikasi konser</span></footer>
</AdminLayout>

<style>
  .event-editor-panel [hidden] { display: none !important; }
  .event-workspace { display: grid; grid-template-columns: 210px minmax(0, 1fr); gap: 32px; align-items: start; }
  .event-section-content { min-width: 0; container: concert-form / inline-size; }
  #event-section-title { margin: 0 0 16px; font-size: 22px; overflow-wrap: anywhere; }
  .event-section-nav ol { list-style: none; margin: 0; padding: 0; display: grid; gap: 8px; }
  .event-section-nav button { width: 100%; display: flex; align-items: center; gap: 12px; padding: 12px; border: 1px solid var(--admin-border); background: transparent; color: var(--admin-secondary); text-align: left; line-height: 1.4; overflow-wrap: anywhere; }
  .event-section-nav svg { flex: 0 0 22px; }
  .event-section-nav button[aria-current] { border-color: var(--admin-accent); color: var(--admin-text); background: var(--admin-input); }
  .event-section-nav button:disabled { opacity: .45; }
  .event-progress { margin: 0 0 16px; color: var(--admin-secondary); }
  .event-step-picker { display: none; }
  .event-step-picker select { width: 100%; min-width: 0; min-height: 46px; padding: 10px; background: var(--admin-input); color: var(--admin-text); border: 1px solid var(--admin-border); }
  .event-actions { display: flex; flex-wrap: wrap; gap: 12px; }
  .event-actions button { flex: 1 1 140px; }
  .event-wizard-actions { margin-top: 12px; padding-top: 12px; border-top: 1px solid var(--admin-border); }
  .event-leave-prompt { margin-bottom: 20px; padding: 16px; border: 1px solid var(--admin-accent); }
  .event-review { display: grid; gap: 12px; margin: 0; }
  .event-review div { display: grid; gap: 6px 12px; min-width: 0; }
  .event-review dt { color: var(--admin-secondary); }
  .event-review dd { margin: 0; overflow-wrap: anywhere; white-space: pre-line; }
  .event-review p { margin: 0; }
  .event-gaps { margin: 0; padding-left: 20px; }
  @media (max-width: 767px) {
    .event-workspace { grid-template-columns: minmax(0, 1fr); gap: 24px; }
    .event-section-nav ol { grid-template-columns: repeat(2, minmax(0, 1fr)); }
    .event-section-nav .create-steps { display: none; }
    .event-step-picker { display: grid; gap: 8px; color: var(--admin-secondary); }
    .event-section-nav button { height: 100%; gap: 8px; }
    .event-actions button { flex-basis: 100%; }
  }
  .event-admin-grid { grid-template-columns: minmax(0, 1fr); }
  .event-admin-list li { display: grid; grid-template-columns: minmax(0, 1fr) auto auto; gap: 16px; align-items: center; padding: 12px; }
  .event-admin-list small { display: block; color: var(--scan-muted); font-size: 14px; line-height: 1.6; overflow-wrap: anywhere; }
  .event-admin-list strong { overflow-wrap: anywhere; }
  .event-admin-list button { width: auto; min-height: 44px; }
  .event-editor-panel .staff-panel-heading { flex-wrap: wrap; }
  .event-admin-grid :global(.staff-form) { font-size: 16px; }
  .event-admin-grid :global(.staff-form > label), .event-list-filters label { font-size: 16px; line-height: 1.45; }
  .event-admin-grid :global(.staff-form small), .event-list-filters :global(small) { font-size: 14px; }
  .event-admin-form { gap: 12px; margin: 0; }
  .event-group, #zone-form, #tier-form { display: grid; grid-template-columns: minmax(0, 1fr); gap: 12px; align-items: start; }
  .event-group { min-width: 0; margin: 0; padding: 0; border: 0; }
  .event-admin-form label, .event-field { min-width: 0; display: grid; gap: 6px; margin: 0; align-content: start; line-height: 1.4; overflow-wrap: anywhere; }
  .event-field :global(.admin-help.active) { flex-wrap: nowrap; }
  .event-field :global(.admin-help.active label) { flex: 0 1 6.5em; }
  .event-admin-form .staff-submit { margin: 0; }
  .event-form-row { grid-column: 1 / -1; grid-template-columns: minmax(0, 1fr); gap: 12px; }
  .event-full, .event-admin-form > p, .event-admin-form > button, .event-group > p, .event-group > ul, .event-review, .event-group > :global(.poster-editor) { grid-column: 1 / -1; }
  .event-admin-form p, .event-inventory-list h4 { margin: 0; }
  .event-lock-note { margin: 0; }
  .event-inventory-list { display: grid; gap: 12px; min-width: 0; }
  .event-inventory-list h4 { font-size: 16px; line-height: 1.25; }
  .event-inventory-list button { white-space: normal; overflow-wrap: anywhere; text-align: left; }
  @container concert-form (min-width: 35em) {
    .event-group, #zone-form, #tier-form, .event-form-row { grid-template-columns: repeat(2, minmax(0, 1fr)); }
    .event-review > div { grid-template-columns: minmax(0, 10em) minmax(0, 1fr); }
  }
  .event-unsaved { margin: 0 0 12px; padding: 10px 12px; border-left: 3px solid var(--scan-lime); color: var(--scan-text); font-size: 14px; }
  .event-editor-panel :global(button), .event-editor-panel :global(input), .event-editor-panel :global(select), .event-editor-panel :global(textarea) { font-size: 16px; }
  .event-editor-panel :global(button) { min-height: 44px; }
  .event-editor-panel :global(input:not([type="radio"]):not([type="checkbox"])), .event-editor-panel :global(select) { min-height: 44px; }
  @media (max-width: 600px) { .event-admin-list li { grid-template-columns: minmax(0, 1fr) auto; } .event-admin-list li > div { grid-column: 1 / -1; } }
</style>
