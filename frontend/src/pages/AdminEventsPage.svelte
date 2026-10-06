<script lang="ts">
  import { onDestroy, onMount } from "svelte";
  import { ApiError, createAdminEvent, createAdminTier, createAdminZone, getAdminEvents, updateAdminEvent, updateAdminTier, updateAdminZone, type AdminEventInput, type AdminApiEvent, type AdminTierInput, type PublicationStatus } from "../lib/api.ts";

  let { accessToken, onUnauthorized, onLogout, loggingOut }: { accessToken: string; onUnauthorized: () => void; onLogout: () => void; loggingOut: boolean } = $props();
  let events = $state<AdminApiEvent[]>([]);
  let selectedId = $state("");
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

  function emptyEvent(): AdminEventInput { return { id: "", artist: "", city: "", venue: "", address: "", startsAt: "", genre: "Pop", status: "Presale", publicationStatus: "DRAFT", image: "", description: "", lineup: [] }; }
  function localDateTime(value: string) {
    if (!value) return "";
    const parts = new Intl.DateTimeFormat("en-CA", { timeZone: "Asia/Jakarta", year: "numeric", month: "2-digit", day: "2-digit", hour: "2-digit", minute: "2-digit", hourCycle: "h23" }).formatToParts(new Date(value));
    const part = (type: string) => parts.find((item) => item.type === type)?.value ?? "";
    return `${part("year")}-${part("month")}-${part("day")}T${part("hour")}:${part("minute")}`;
  }
  function payload() {
    return { ...draft, startsAt: draft.startsAt ? `${draft.startsAt}:00+07:00` : "", lineup: lineupText.split("\n").map((name) => name.trim()).filter(Boolean) };
  }
  const visible = $derived(events.filter((event) => (!publicationFilter || event.publicationStatus === publicationFilter) && `${event.artist} ${event.city} ${event.venue}`.toLowerCase().includes(query.trim().toLowerCase())));

  function select(event: AdminApiEvent) {
    selectedId = event.id;
    draft = { id: event.id, artist: event.artist, city: event.city, venue: event.venue, address: event.address, startsAt: localDateTime(event.startsAt), genre: event.genre, status: event.status, publicationStatus: event.publicationStatus, image: event.image, description: event.description, lineup: event.lineup };
    lineupText = event.lineup.join("\n");
    zoneSelected = ""; zoneDraft = { id: "", name: "", description: "" };
    tierSelected = ""; tierDraft = { id: "", name: "", zoneId: event.zones[0]?.id ?? "", price: 0, capacity: 0, maxPerOrder: 1, benefit: "", gate: "", seating: "free-standing" };
    error = ""; notice = "";
  }
  function startNew() { selectedId = ""; draft = emptyEvent(); lineupText = ""; error = ""; notice = ""; }

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
      select(result); notice = "Perubahan konser tersimpan.";
    } catch (cause) {
      if (cause instanceof ApiError && cause.status === 401) onUnauthorized();
      else if (cause instanceof ApiError && cause.code === "SCHEDULE_LOCKED") error = "Jadwal tidak dapat diubah karena konser sudah memiliki pesanan.";
      else error = cause instanceof ApiError ? cause.message : "Konser belum dapat disimpan. Periksa koneksi lalu coba lagi.";
    } finally { saving = false; }
  }

  function selectZone(id: string) { zoneSelected = id; const zone = events.find((event) => event.id === selectedId)?.zones.find((item) => item.id === id); zoneDraft = zone ? { id, name: zone.name, description: zone.description } : { id: "", name: "", description: "" }; }
  function selectTier(id: string) { tierSelected = id; const tier = events.find((event) => event.id === selectedId)?.ticketTiers.find((item) => item.id === id); tierDraft = tier ? { id, name: tier.name, zoneId: tier.zoneId, price: tier.price, capacity: tier.capacity, maxPerOrder: tier.maxPerOrder, benefit: tier.benefit, gate: tier.gate, seating: tier.seating } : { id: "", name: "", zoneId: events.find((event) => event.id === selectedId)?.zones[0]?.id ?? "", price: 0, capacity: 0, maxPerOrder: 1, benefit: "", gate: "", seating: "free-standing" }; }
  async function saveZone(event: SubmitEvent) { event.preventDefault(); if (!selectedId || saving) return; saving = true; error = ""; try { if (zoneSelected) await updateAdminZone(accessToken, selectedId, zoneSelected, { name: zoneDraft.name, description: zoneDraft.description }); else await createAdminZone(accessToken, selectedId, zoneDraft); const refreshed = await getAdminEvents(accessToken); events = refreshed; select(refreshed.find((item) => item.id === selectedId)!); notice = "Zona tersimpan."; } catch (cause) { if (cause instanceof ApiError && cause.status === 401) onUnauthorized(); else error = cause instanceof ApiError ? cause.message : "Zona belum dapat disimpan."; } finally { saving = false; } }
  async function saveTier(event: SubmitEvent) { event.preventDefault(); if (!selectedId || saving) return; saving = true; error = ""; try { if (tierSelected) await updateAdminTier(accessToken, selectedId, tierSelected, tierDraft); else await createAdminTier(accessToken, selectedId, tierDraft); const refreshed = await getAdminEvents(accessToken); events = refreshed; select(refreshed.find((item) => item.id === selectedId)!); notice = "Kategori tiket tersimpan."; } catch (cause) { if (cause instanceof ApiError && cause.status === 401) onUnauthorized(); else error = cause instanceof ApiError ? cause.message : "Kategori tiket belum dapat disimpan."; } finally { saving = false; } }

  onMount(() => { void load(); });
  onDestroy(() => { generation++; request?.abort(); });
</script>

<svelte:head><title>Kelola Konser | Tiket Online</title><meta name="robots" content="noindex" /></svelte:head>

<div class="scan-shell staff-admin-shell">
  <a class="skip-link" href="#event-admin-content">Lewati ke pengelolaan konser</a>
  <header class="scan-topbar"><a class="scan-brand" href="/" aria-label="Kembali ke Tiket Online"><span class="scan-brand-mark" aria-hidden="true">TO</span><span>Tiket Online <b>/ Gate Control</b></span></a><span class="staff-login-label">ADMINISTRATOR</span><button class="staff-text-button" type="button" disabled={loggingOut || saving} onclick={onLogout}>Keluar</button></header>
  <main id="event-admin-content" class="staff-admin-content">
    <div class="staff-admin-heading"><div><p class="scan-kicker">KATALOG EVENT <span>•</span> ADMIN</p><h1>Kelola konser.</h1><p>Atur informasi, jadwal, lineup, poster, dan publikasi konser.</p></div><button class="staff-secondary-button" type="button" disabled={saving} onclick={startNew}>Tambah konser</button></div>
    <nav class="admin-tool-nav" aria-label="Administrasi event"><a aria-current="page" href="/admin/events">Konser</a><a href="/admin/staff">Kelola petugas</a><a href="/admin/check-ins">Riwayat check-in</a></nav>

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
        <form class="staff-form event-admin-form" onsubmit={save} aria-busy={saving}>
          {#if !selectedId}<label>ID URL konser<input bind:value={draft.id} pattern="[a-z0-9]+(-[a-z0-9]+)*" maxlength="64" placeholder="nama-konser" required disabled={saving} /><small>Huruf kecil, angka, dan tanda hubung.</small></label>{/if}
          <label>Nama konser / artis<input bind:value={draft.artist} minlength="2" maxlength="160" required disabled={saving} /></label>
          <div class="event-form-row"><label>Genre<select bind:value={draft.genre} disabled={saving}><option>Rock</option><option>Pop</option><option>Indie</option></select></label><label>Label penjualan<select bind:value={draft.status} disabled={saving}><option>Early Bird</option><option>Presale</option><option>Sold Out</option></select></label></div>
          <label>Status publikasi<select bind:value={draft.publicationStatus} disabled={saving || !selectedId}><option value="DRAFT">Draft — belum tampil di katalog</option><option value="PUBLISHED">Published — tampil di katalog</option><option value="ARCHIVED">Archived — disembunyikan dari katalog</option></select></label>
          <label>Jadwal (WIB)<input type="datetime-local" bind:value={draft.startsAt} required={draft.publicationStatus === "PUBLISHED"} disabled={saving || Boolean(selectedId && events.find((item) => item.id === selectedId)?.scheduleLocked)} /></label>
          {#if selectedId && events.find((item) => item.id === selectedId)?.scheduleLocked}<small class="event-lock-note">Jadwal terkunci karena konser sudah memiliki pesanan. Akses pesanan dan snapshot tiket bergantung pada tanggal ini.</small>{/if}
          <div class="event-form-row"><label>Kota<input bind:value={draft.city} maxlength="100" required={draft.publicationStatus === "PUBLISHED"} disabled={saving} /></label><label>Venue<input bind:value={draft.venue} maxlength="160" required={draft.publicationStatus === "PUBLISHED"} disabled={saving} /></label></div>
          <label>Alamat<input bind:value={draft.address} maxlength="255" required={draft.publicationStatus === "PUBLISHED"} disabled={saving} /></label>
          <label>URL poster<input bind:value={draft.image} type="url" maxlength="500" placeholder="https://…" required={draft.publicationStatus === "PUBLISHED"} disabled={saving} /></label>
          <label>Deskripsi<textarea bind:value={draft.description} rows="5" maxlength="16000" required={draft.publicationStatus === "PUBLISHED"} disabled={saving}></textarea></label>
          <label>Lineup <small>Satu nama per baris</small><textarea bind:value={lineupText} rows="4" maxlength="16000" required={draft.publicationStatus === "PUBLISHED"} disabled={saving}></textarea></label>
          {#if !events.find((item) => item.id === selectedId)?.ticketTiers.length && draft.publicationStatus === "PUBLISHED"}<p class="staff-muted">Tanpa kategori tiket, konser akan tampil dengan keterangan “Tiket belum tersedia”.</p>{/if}
          <button class="scan-submit staff-submit" type="submit" disabled={saving || loading}>{saving ? "Menyimpan…" : selectedId ? "Simpan perubahan" : "Buat konser"}</button>
        </form>
        {#if selectedId}
          {@const currentEvent = events.find((item) => item.id === selectedId)}
          <section class="event-inventory-editor" aria-labelledby="event-inventory-title">
            <div class="staff-panel-heading"><div><span class="panel-index">03</span><h2 id="event-inventory-title">Zona dan kategori tiket</h2></div></div>
            <div class="event-form-row"><label>Zona<select value={zoneSelected} onchange={(event) => selectZone(event.currentTarget.value)} disabled={saving}><option value="">Tambah zona</option>{#each currentEvent?.zones ?? [] as zone}<option value={zone.id}>{zone.name}</option>{/each}</select></label></div>
            <form class="staff-form event-admin-form" onsubmit={saveZone}>
              {#if !zoneSelected}<label>ID zona<input bind:value={zoneDraft.id} pattern="[a-z0-9]+(-[a-z0-9]+)*" maxlength="64" required disabled={saving} /></label>{/if}
              <label>Nama zona<input bind:value={zoneDraft.name} maxlength="100" required disabled={saving} /></label>
              <label>Deskripsi zona<input bind:value={zoneDraft.description} maxlength="255" disabled={saving} /></label>
              <button class="staff-secondary-button" type="submit" disabled={saving || !zoneDraft.name}>{zoneSelected ? "Simpan zona" : "Tambah zona"}</button>
            </form>
            <div class="event-inventory-list"><h3>Kategori tersimpan</h3>{#each currentEvent?.ticketTiers ?? [] as tier}<button class="staff-secondary-button" type="button" onclick={() => selectTier(tier.id)}>{tier.name} · {tier.availableQuantity}/{tier.capacity} tersedia · {tier.boundQuantity} stok terikat{tier.gateLocked ? " · Gate terkunci" : ""}</button>{/each}</div>
            <label>Kategori<select value={tierSelected} onchange={(event) => selectTier(event.currentTarget.value)} disabled={saving}><option value="">Tambah kategori</option>{#each currentEvent?.ticketTiers ?? [] as tier}<option value={tier.id}>{tier.name}</option>{/each}</select></label>
            <form class="staff-form event-admin-form" onsubmit={saveTier}>
              {#if !tierSelected}<label>ID kategori<input bind:value={tierDraft.id} pattern="[a-z0-9]+(-[a-z0-9]+)*" maxlength="64" required disabled={saving} /></label>{/if}
              <label>Nama kategori<input bind:value={tierDraft.name} maxlength="100" required disabled={saving} /></label>
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
  </main>
</div>
