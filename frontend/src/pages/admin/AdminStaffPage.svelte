<script lang="ts">
  import AdminLayout from "../../layouts/AdminLayout.svelte";
  import { onMount, tick } from "svelte";
  import StaffAssignmentsEditor from "../../components/admin/StaffAssignmentsEditor.svelte";
  import { ApiError, createStaff, getAdminEvents, listStaff, resetStaffPassword, updateStaff, replaceStaffAssignments, type Staff, type StaffAssignment } from "../../lib/api.ts";
  import type { AdminApiEvent } from "../../lib/api.ts";
  import { filterStaff, sameAssignments, type StaffFilters } from "../../lib/staff-management.ts";

  let { accessToken, onUnauthorized, onLogout, loggingOut }: { accessToken: string; onUnauthorized: () => void; onLogout: () => void; loggingOut: boolean } = $props();
  let staff = $state<Staff[]>([]);
  let events = $state<AdminApiEvent[]>([]);
  let loadingStaff = $state(true);
  let loadingEvents = $state(true);
  let staffError = $state("");
  let eventsError = $state("");
  let saving = $state(false);
  let notice = $state("");
  let error = $state("");
  let newName = $state("");
  let newEmail = $state("");
  let newPassword = $state("");
  let newAssignments = $state<StaffAssignment[]>([]);
  let newAssignmentPending = $state(false);
  let creating = $state(false);
  let createUncertain = $state(false);
  let selectedId = $state("");
  let editName = $state("");
  let editActive = $state(false);
  let editAssignments = $state<StaffAssignment[]>([]);
  let editAssignmentPending = $state(false);
  let resetPassword = $state("");
  let profileBaseline = $state("");
  let assignmentBaseline = $state("");
  let filters = $state<StaffFilters>({ query: "", status: "all", eventId: "", unassigned: false });
  let showFilters = $state(false);
  let filteredStaff = $derived(filterStaff(staff, filters));
  let filtersActive = $derived(Boolean(filters.query.trim() || filters.status !== "all" || filters.eventId || filters.unassigned));
  let filterEventIds = $derived([...new Set([...events.map((item) => item.id), ...staff.flatMap((person) => person.assignments.map((item) => item.eventId))])]);
  let uncertain = $state<Record<string, { profile?: { name: string; active: boolean }; assignments?: StaffAssignment[]; password?: boolean }>>({});
  let selectedUncertain = $derived(uncertain[selectedId]);
  let hasUncertain = $derived(Object.values(uncertain).some((item) => item.profile || item.assignments || item.password));
  let selected = $derived(staff.find((item) => item.id === selectedId && item.role === "STAFF"));
  const staffEditorDirty = $derived(Boolean(selectedId) && (JSON.stringify({ name: editName, active: editActive }) !== profileBaseline || JSON.stringify(editAssignments) !== assignmentBaseline || Boolean(resetPassword) || editAssignmentPending));
  const createDirty = $derived(creating && Boolean(newName || newEmail || newPassword || newAssignments.length || newAssignmentPending));

  function assignmentLabel(assignment: StaffAssignment) {
    const event = events.find((item) => item.id === assignment.eventId);
    return `${event ? `${event.artist} · ${event.city}` : `Event ${assignment.eventId}`} · ${assignment.gate}`;
  }
  function protectUnsaved(event: BeforeUnloadEvent) {
    if (staffEditorDirty || createDirty || saving || hasUncertain) { event.preventDefault(); event.returnValue = ""; }
  }
  function allowStaffDiscard() {
    if (createUncertain || selectedUncertain?.profile || selectedUncertain?.assignments || selectedUncertain?.password) return window.confirm("Hasil tindakan akun belum pasti. Jangan mengulang tindakan sebelum hasilnya diperiksa. Lanjutkan keluar dari panel?");
    return !(staffEditorDirty || createDirty) || window.confirm("Perubahan akun petugas yang belum disimpan akan hilang. Lanjutkan?");
  }
  function requestLogout() { if (!saving && allowStaffDiscard()) onLogout(); }
  function resetFilters() { filters = { query: "", status: "all", eventId: "", unassigned: false }; void focusElement("staff-search"); }

  function unknownOutcome(cause: unknown) {
    return cause instanceof ApiError && (cause.code === "UNKNOWN_OUTCOME" || cause.code === "NETWORK_ERROR" || cause.status >= 500);
  }

  async function focusElement(id: string) {
    await tick();
    document.getElementById(id)?.focus();
  }

  function clearCreate() {
    newName = ""; newEmail = ""; newPassword = ""; newAssignments = [];
    creating = false; createUncertain = false; newAssignmentPending = false;
  }

  function openCreate() {
    if (saving || creating || !allowStaffDiscard()) return;
    selectedId = ""; resetPassword = ""; editAssignmentPending = false; error = ""; notice = ""; creating = true;
    void focusElement("staff-create-title");
  }

  function closePanel() {
    if (saving || !allowStaffDiscard()) return;
    const previousId = selectedId;
    clearCreate(); selectedId = ""; resetPassword = ""; editAssignmentPending = false; error = "";
    void focusElement(previousId ? filteredStaff.some((person) => person.id === previousId) ? `staff-manage-${previousId}` : "staff-list-title" : "staff-create-opener");
  }

  async function loadStaff(): Promise<boolean> {
    loadingStaff = true; staffError = "";
    try {
      staff = await listStaff(accessToken);
      for (const person of staff) {
        const pending = uncertain[person.id];
        if (!pending) continue;
        if (pending.profile && person.name === pending.profile.name && person.active === pending.profile.active) {
          delete pending.profile;
          if (selectedId === person.id) { editName = person.name; editActive = person.active; profileBaseline = JSON.stringify({ name: person.name, active: person.active }); error = ""; notice = "Hasil perubahan profil terlihat pada daftar terbaru."; }
        }
        if (pending.assignments && sameAssignments(person.assignments, pending.assignments)) {
          delete pending.assignments;
          if (selectedId === person.id) { editAssignments = person.assignments.map((item) => ({ ...item })); assignmentBaseline = JSON.stringify(editAssignments); error = ""; notice = "Hasil perubahan penugasan terlihat pada daftar terbaru."; }
        }
      }
      return true;
    }
    catch (cause) { if (cause instanceof ApiError && cause.status === 401) onUnauthorized(); else staffError = "Daftar petugas tidak dapat dimuat."; return false; }
    finally { loadingStaff = false; }
  }

  async function loadEvents() {
    loadingEvents = true; eventsError = "";
    try { events = await getAdminEvents(accessToken); }
    catch (cause) { if (cause instanceof ApiError && cause.status === 401) onUnauthorized(); else eventsError = "Katalog event belum dapat dimuat."; }
    finally { loadingEvents = false; }
  }

  onMount(() => { void loadStaff(); void loadEvents(); });

  function selectStaff(person: Staff) {
    if (saving) return;
    if (person.id === selectedId || !allowStaffDiscard()) return;
    clearCreate();
    selectedId = person.id; editName = person.name; editActive = person.active;
    editAssignments = person.assignments.map((assignment) => ({ ...assignment }));
    profileBaseline = JSON.stringify({ name: editName, active: editActive });
    assignmentBaseline = JSON.stringify(editAssignments);
    resetPassword = ""; editAssignmentPending = false; error = ""; notice = "";
    void focusElement("staff-edit-title");
  }

  function showError(cause: unknown) {
    if (cause instanceof ApiError && cause.status === 401) { onUnauthorized(); return; }
    error = cause instanceof ApiError && cause.code === "UNKNOWN_OUTCOME"
      ? "Hasil belum diketahui karena respons terputus. Muat ulang data sebelum mencoba kembali."
      : cause instanceof ApiError ? cause.message : "Tidak dapat menghubungi server.";
  }

  async function create(event: SubmitEvent) {
    event.preventDefault(); if (saving || !creating || createUncertain || newAssignmentPending || loadingEvents || eventsError) return;
    saving = true; error = ""; notice = "";
    try {
      const created = await createStaff(accessToken, { name: newName, email: newEmail.trim(), password: newPassword, assignments: newAssignments });
      clearCreate();
      notice = "Akun petugas berhasil dibuat.";
      const loaded = await loadStaff();
      if (!loaded) notice = "Akun petugas berhasil dibuat, tetapi daftar belum dapat dimuat ulang. Muat ulang daftar untuk melihat akun baru.";
      void focusElement(loaded && filteredStaff.some((person) => person.id === created.id) ? `staff-account-${created.id}` : "staff-result");
    } catch (cause) {
      createUncertain = unknownOutcome(cause);
      showError(cause);
      if (createUncertain) error = "Hasil pembuatan akun belum diketahui karena respons terputus. Periksa daftar petugas sebelum mencoba kembali.";
    }
    finally { saving = false; }
  }

  async function checkCreateResult() {
    if (saving) return;
    saving = true;
    try {
      if (!await loadStaff()) return;
      const found = staff.find((person) => person.email.toLowerCase() === newEmail.trim().toLowerCase());
      if (found) {
        clearCreate(); error = "";
        notice = "Akun dengan email tersebut sudah tersedia pada daftar. Periksa akun sebelum melanjutkan.";
        void focusElement(filteredStaff.some((person) => person.id === found.id) ? `staff-account-${found.id}` : "staff-result");
      } else {
        createUncertain = false;
        error = "Akun dengan email tersebut belum ditemukan. Periksa input sebelum mencoba kembali.";
      }
    } finally { saving = false; }
  }

  async function saveDetails(event: SubmitEvent) {
    event.preventDefault(); if (saving || !selected || selectedUncertain?.profile) return;
    if (selected.active && !editActive && !window.confirm(`Nonaktifkan ${selected.name}? Semua sesi petugas ini akan dicabut.`)) return;
    saving = true; error = ""; notice = "";
    try {
      const staffId = selected.id;
      await updateStaff(accessToken, staffId, { name: editName, active: editActive });
      profileBaseline = JSON.stringify({ name: editName, active: editActive });
      notice = "Profil petugas berhasil diperbarui.";
      if (await loadStaff()) {
        const refreshed = staff.find((item) => item.id === staffId);
        if (refreshed && selectedId === staffId) {
          editName = refreshed.name; editActive = refreshed.active;
          profileBaseline = JSON.stringify({ name: editName, active: editActive });
        }
      } else notice = "Profil tersimpan, tetapi daftar petugas belum dapat dimuat ulang.";
    }
    catch (cause) {
      if (unknownOutcome(cause)) uncertain[selectedId] = { ...uncertain[selectedId], profile: { name: editName.trim(), active: editActive } };
      showError(cause);
    }
    finally { saving = false; }
  }

  async function saveAssignments(event: SubmitEvent) {
    event.preventDefault(); if (saving || !selected || selectedUncertain?.assignments || editAssignmentPending || loadingEvents || eventsError) return;
    saving = true; error = ""; notice = "";
    try {
      const staffId = selected.id;
      const before = selected.assignments;
      const removed = before.filter((item) => !editAssignments.some((next) => next.eventId === item.eventId && next.gate === item.gate));
      const added = editAssignments.filter((item) => !before.some((old) => old.eventId === item.eventId && old.gate === item.gate));
      if ((added.length || removed.length) && !window.confirm(`Simpan perubahan penugasan untuk ${selected.name}? Event dan gate yang diizinkan akan mengikuti daftar baru. Sesi tetap aktif.\n${added.length ? `Ditambahkan: ${added.map(assignmentLabel).join(", ")}. ` : ""}${removed.length ? `Dihapus: ${removed.map(assignmentLabel).join(", ")}.` : ""}`)) return;
      await replaceStaffAssignments(accessToken, staffId, editAssignments);
      assignmentBaseline = JSON.stringify(editAssignments);
      const loaded = await loadStaff();
      const refreshed = loaded && staff.find((item) => item.id === staffId);
      if (refreshed && selectedId === staffId) { editAssignments = refreshed.assignments.map((item) => ({ ...item })); assignmentBaseline = JSON.stringify(editAssignments); }
      notice = loaded ? "Penugasan berhasil diperbarui." : "Penugasan tersimpan, tetapi daftar belum dapat dimuat ulang.";
    }
    catch (cause) {
      if (unknownOutcome(cause)) uncertain[selectedId] = { ...uncertain[selectedId], assignments: editAssignments.map((item) => ({ ...item })) };
      showError(cause);
    }
    finally { saving = false; }
  }

  async function savePassword(event: SubmitEvent) {
    event.preventDefault(); if (saving || !selected || selectedUncertain?.password) return;
    if (!window.confirm(`Reset password ${selected.name}? Semua sesi aktif petugas ini akan dicabut.`)) return;
    saving = true; error = ""; notice = "";
    try { await resetStaffPassword(accessToken, selected.id, resetPassword); resetPassword = ""; notice = "Password direset dan sesi lama petugas dicabut."; }
    catch (cause) {
      if (unknownOutcome(cause)) uncertain[selectedId] = { ...uncertain[selectedId], password: true };
      showError(cause);
    }
    finally { saving = false; }
  }
</script>

<svelte:window onbeforeunload={protectUnsaved} />

<svelte:head>
  <title>Kelola Petugas | Gate Control</title>
  <meta name="description" content="Kelola akun dan penugasan petugas event." />
  <meta name="robots" content="noindex" />
</svelte:head>

  <AdminLayout page="staff" contentId="staff-admin-content" skipLabel="Lewati ke pengelolaan petugas" kicker="OPERASIONAL EVENT • ADMINISTRASI" title="Kelola petugas." description="Atur akses tim pada event dan gate yang ditugaskan." onLogout={requestLogout} loggingOut={loggingOut} logoutDisabled={loggingOut || saving} primaryAction={{ label: "Buat akun petugas", onclick: openCreate, disabled: saving || creating, id: "staff-create-opener", controls: creating ? "staff-work-panel" : undefined, expanded: creating }}>
    <div id="staff-result" role="status" aria-live="polite" tabindex="-1">
      {#if notice}<p class="staff-form-message staff-form-success">{notice}</p>{/if}
    </div>
    {#if error}<p class="staff-form-message staff-form-error" role="alert">{error}</p>{/if}

    <div class="staff-workspace" class:has-panel={creating || Boolean(selected)}>
      <section class="staff-admin-panel" aria-labelledby="staff-list-title">
        <div class="staff-panel-heading"><h2 id="staff-list-title" tabindex="-1">Akun petugas</h2><button class="staff-text-button" type="button" onclick={() => { if (!saving) void loadStaff(); }} disabled={loadingStaff || saving}>{loadingStaff ? "Memuat…" : "Muat ulang"}</button></div>
        <p class="staff-list-help">Pilih Kelola untuk mengubah profil, penugasan event/gate, atau status akun petugas.</p>
        <div class="staff-form staff-list-filters" role="search" aria-label="Cari dan filter akun">
          <label>Cari nama atau email<input id="staff-search" type="search" bind:value={filters.query} autocomplete="off" /></label>
          <button class="staff-secondary-button staff-filter-toggle" type="button" aria-expanded={showFilters} aria-controls="staff-extra-filters" onclick={() => { showFilters = !showFilters; }}>Filter lainnya</button>
          <div id="staff-extra-filters" class="staff-extra-filters" class:is-open={showFilters}>
            <label>Status akun<select bind:value={filters.status}><option value="all">Semua</option><option value="active">Aktif</option><option value="inactive">Nonaktif</option></select></label>
            <label>Event penugasan<select bind:value={filters.eventId}><option value="">Semua event</option>{#each filterEventIds as id}<option value={id}>{events.find((item) => item.id === id)?.artist ?? `Event ${id}`}</option>{/each}</select></label>
            <label class="staff-checkbox"><input type="checkbox" bind:checked={filters.unassigned} /><span>Tanpa penugasan</span></label>
          </div>
        </div>
        <div class="staff-filter-results"><p class="staff-list-help" role="status" aria-live="polite" aria-atomic="true">{filteredStaff.length} dari {staff.length} akun</p>{#if filtersActive}<button class="staff-text-button" type="button" onclick={resetFilters}>Reset filter</button>{/if}</div>
        {#if selected && !filteredStaff.some((person) => person.id === selectedId)}<p class="staff-list-help" role="status">{selected.name} tidak termasuk hasil saat ini. Panel Kelola dan draft tetap terbuka.</p>{/if}
        {#if loadingStaff}<p class="staff-muted" aria-live="polite">Memuat daftar petugas…</p>
        {:else if staffError}<p class="staff-form-message staff-form-error" role="alert">{staffError}</p>
        {:else if staff.length === 0}<p class="staff-muted">Belum ada akun petugas.</p>
        {:else if filteredStaff.length === 0}<p class="staff-muted">Tidak ada akun yang cocok. Ubah pencarian atau reset filter.</p>
        {:else}<ul class="staff-list">{#each filteredStaff as person (person.id)}
          <li id={`staff-account-${person.id}`} tabindex="-1" class:staff-selected={selectedId === person.id}>
            <div class="staff-list-identity">
              <strong>{person.name}</strong><span>{person.email}</span>
              {#each person.assignments.slice(0, 2) as assignment}<span>{assignmentLabel(assignment)}</span>{/each}
              {#if person.assignments.length > 2}<details><summary>Lihat semua penugasan ({person.assignments.length})</summary>{#each person.assignments.slice(2) as assignment}<span>{assignmentLabel(assignment)}</span>{/each}</details>{/if}
              {#if !person.assignments.length}<span>{person.role === "STAFF" ? "Belum ditugaskan. Atur event dan gate melalui Kelola." : "Tanpa penugasan"}</span>{/if}
              {#if person.role === "ADMIN"}<span>Akun administrator tidak dapat diubah melalui halaman ini.</span>{/if}
            </div>
            <div class="staff-account-meta"><span class="staff-role">Peran: {person.role === "ADMIN" ? "Admin" : "Staff"}</span><span class="staff-status" class:is-active={person.active}>Status: {person.active ? "Aktif" : "Nonaktif"}</span></div>
            {#if person.role === "STAFF"}<button id={`staff-manage-${person.id}`} class="staff-secondary-button" type="button" disabled={saving} aria-expanded={selectedId === person.id} aria-controls={selectedId === person.id ? "staff-work-panel" : undefined} onclick={() => selectStaff(person)}>Kelola<span class="visually-hidden"> {person.name}</span></button>{/if}
          </li>
        {/each}</ul>{/if}
      </section>

      {#if creating}<section id="staff-work-panel" class="staff-admin-panel staff-create-panel" aria-labelledby="staff-create-title">
        <div class="staff-panel-heading"><h2 id="staff-create-title" tabindex="-1">Tambah petugas</h2></div>
        <form id="staff-create-form" class="staff-form" onsubmit={create} aria-busy={saving}>
          <label>Nama<input bind:value={newName} autocomplete="name" minlength="2" maxlength="80" required disabled={saving} readonly={createUncertain} /></label>
          <label>Email<input bind:value={newEmail} type="email" autocomplete="email" maxlength="254" required disabled={saving} readonly={createUncertain} /></label>
          <label>Password awal<input bind:value={newPassword} type="password" autocomplete="new-password" minlength="12" maxlength="128" required disabled={saving} readonly={createUncertain} /><small>Minimal 12 karakter; sesi lama akan dicabut saat password diganti.</small></label>
          {#if loadingEvents}<p class="staff-muted" aria-live="polite">Memuat pilihan event…</p>{:else if eventsError}<p class="staff-form-message staff-form-error" role="alert">{eventsError} Penugasan belum tersedia; input tetap tersimpan. <button class="staff-text-button" type="button" onclick={loadEvents}>Coba lagi</button></p>{:else}<StaffAssignmentsEditor {events} bind:value={newAssignments} bind:pending={newAssignmentPending} mode="create" disabled={saving || createUncertain} />{/if}
          {#if createUncertain}<button class="staff-secondary-button" type="button" disabled={saving} onclick={checkCreateResult}>Periksa daftar petugas</button>{/if}
          <div class="staff-create-actions"><button class="scan-submit staff-submit" type="submit" disabled={saving || createUncertain || newAssignmentPending || loadingEvents || Boolean(eventsError)}>{saving ? "Menyimpan…" : "Buat akun petugas"}</button><button class="staff-secondary-button" type="button" disabled={saving} onclick={closePanel}>Batal</button></div>
        </form>
      </section>
    {:else if selected}<section id="staff-work-panel" class="staff-admin-panel staff-edit-panel" aria-labelledby="staff-edit-title">
      <div class="staff-panel-heading"><h2 id="staff-edit-title" tabindex="-1">{selected.name}</h2><button class="staff-text-button" type="button" disabled={saving} onclick={closePanel}>Tutup</button></div>
      <p class="staff-list-help">{selected.email}</p>
      <nav class="staff-section-nav" aria-label="Bagian pengelolaan akun"><a href="#staff-profile">Profil dan akses</a><a href="#staff-assignments">Penugasan</a><a href="#staff-password">Reset password</a></nav>
      {#if selectedUncertain?.profile || selectedUncertain?.assignments}<p class="staff-list-help" role="alert">Hasil perubahan {selectedUncertain.profile ? "profil" : "penugasan"} belum pasti. Tindakan tersebut ditahan. Muat ulang daftar dan periksa hasilnya sebelum melanjutkan.</p>{/if}
      {#if selectedUncertain?.password}<p class="staff-list-help" role="alert">Hasil reset password belum pasti. Jangan mengulang reset. Minta petugas memeriksa apakah password baru dapat digunakan; daftar akun tidak dapat membuktikan hasil reset.</p>{/if}
      {#if staffEditorDirty}<p class="staff-edit-unsaved" role="status">Perubahan akun belum disimpan</p>{/if}
      <div class="staff-edit-grid">
        <form class="staff-form" onsubmit={saveDetails} aria-busy={saving}>
          <h3 id="staff-profile" tabindex="-1">Profil dan akses</h3>
          <p class="staff-action-note">Simpan nama dan status akun. Menonaktifkan akun langsung mencabut semua sesi.</p>
          <label>Nama<input bind:value={editName} minlength="2" maxlength="80" required disabled={saving} readonly={Boolean(selectedUncertain?.profile)} /></label>
          <label class="staff-checkbox"><input type="checkbox" bind:checked={editActive} disabled={saving || Boolean(selectedUncertain?.profile)} /><span>Akun aktif</span></label>
          <small>Menonaktifkan akun langsung mencabut semua sesi petugas tersebut.</small>
          <button class="staff-secondary-button" type="submit" disabled={saving || Boolean(selectedUncertain?.profile)}>Simpan profil</button>
        </form>
        <form class="staff-form" onsubmit={saveAssignments} aria-busy={saving}>
          <h3 id="staff-assignments" tabindex="-1">Penugasan</h3>
          <p class="staff-action-note">Daftar ini menentukan event dan gate yang dapat diakses petugas. Perubahan berlaku saat disimpan; sesi tetap aktif.</p>
          {#if loadingEvents}<p class="staff-muted">Memuat katalog…</p>{:else if eventsError}<p class="staff-form-message staff-form-error" role="alert">{eventsError} Draft penugasan tetap tersedia setelah katalog dimuat ulang. <button class="staff-text-button" type="button" onclick={loadEvents}>Coba lagi</button></p>{:else}{#key selectedId}<StaffAssignmentsEditor {events} bind:value={editAssignments} bind:pending={editAssignmentPending} disabled={saving || Boolean(selectedUncertain?.assignments)} />{/key}{/if}
          <button class="staff-secondary-button" type="submit" disabled={saving || Boolean(selectedUncertain?.assignments) || editAssignmentPending || loadingEvents || Boolean(eventsError)}>Simpan penugasan</button>
        </form>
        <form class="staff-form" onsubmit={savePassword} aria-busy={saving}>
          <h3 id="staff-password" tabindex="-1">Reset password</h3>
          <p class="staff-action-note">Reset password adalah tindakan terpisah dan mencabut semua sesi aktif petugas.</p>
          <label>Password baru<input bind:value={resetPassword} type="password" autocomplete="new-password" minlength="12" maxlength="128" required disabled={saving} readonly={Boolean(selectedUncertain?.password)} /></label>
          <small>Reset password mencabut semua sesi aktif milik petugas.</small>
          <button class="staff-secondary-button" type="submit" disabled={saving || Boolean(selectedUncertain?.password)}>Reset password</button>
        </form>
      </div>
    </section>{/if}
    </div>
    <footer class="scan-footer"><span>Ticket Online · Admin tools</span><span>Pengaturan akun dan akses event</span></footer>
</AdminLayout>

<style>
  .staff-workspace { display: grid; grid-template-columns: minmax(0, 1fr); align-items: start; gap: 24px; }
  .staff-workspace.has-panel { grid-template-columns: minmax(0, 1.15fr) minmax(0, 1fr); }
  .staff-workspace :global(.staff-form) { font-size: 16px; }
  .staff-workspace :global(.staff-form > label) { font-size: 16px; line-height: 1.5; }
  .staff-workspace :global(.staff-form small), .staff-list-identity span { font-size: 14px; line-height: 1.6; }
  .staff-list li { grid-template-columns: minmax(0, 1fr) auto auto; gap: 12px 16px; padding-block: 12px; }
  .has-panel .staff-list li { grid-template-columns: minmax(0, 1fr) auto; }
  .has-panel .staff-list-identity { grid-column: 1 / -1; }
  .staff-list li > button { justify-self: end; }
  .staff-list-help { margin: 0 0 16px; color: var(--scan-muted); font-size: 14px; line-height: 1.6; overflow-wrap: anywhere; }
  .staff-section-nav { display: flex; flex-wrap: wrap; gap: 8px 16px; margin-bottom: 20px; }
  .staff-section-nav a { display: inline-flex; align-items: center; min-height: 44px; color: var(--scan-text); font-size: 14px; }
  .staff-list-identity details span { display: block; margin-top: 4px; }
  .staff-list-identity summary { min-height: 44px; padding-block: 12px; color: var(--scan-text); font-size: 14px; line-height: 1.5; cursor: pointer; }
  .staff-list-identity summary:focus-visible { outline: 3px solid var(--scan-lime); outline-offset: 3px; }
  .staff-list-filters { margin: 0 0 12px; }
  .staff-extra-filters { display: grid; grid-template-columns: minmax(0, 1fr) minmax(0, 1fr); gap: 12px; }
  .staff-extra-filters label { display: grid; gap: 8px; min-width: 0; color: var(--scan-muted); }
  .staff-extra-filters select { width: 100%; min-height: 46px; padding: 10px 12px; border: 1px solid var(--admin-control-border); border-radius: 0; }
  .staff-extra-filters .staff-checkbox { grid-column: 1 / -1; }
  .staff-filter-toggle { display: none; }
  .staff-filter-results { display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: 12px; margin-bottom: 16px; }
  .staff-filter-results p { margin: 0; }
  .staff-account-meta { display: flex; flex-wrap: wrap; align-items: center; gap: 10px; }
  .staff-role { color: var(--admin-secondary); font-size: 14px; }
  .staff-status { display: inline-block; padding: 4px 8px; border-radius: 2px; color: var(--admin-text); background: var(--admin-neutral-bg); font-size: 14px; line-height: 1.5; cursor: default; }
  .staff-status.is-active { background: var(--admin-active-bg); color: var(--admin-active-text); }
  .staff-edit-panel { margin-top: 0; }
  .staff-edit-grid { grid-template-columns: minmax(0, 1fr); gap: 28px; }
  .staff-edit-grid :global(.staff-form + .staff-form) { border-top: 1px solid var(--admin-border); padding-top: 24px; }
  .staff-create-actions { display: flex; flex-wrap: wrap; gap: 12px; }
  .staff-create-actions > button { flex: 1 1 auto; margin: 0; }
  .staff-panel-heading { flex-wrap: wrap; }
  .staff-panel-heading h2 { overflow-wrap: anywhere; }
  #staff-result:not(:empty) { margin-bottom: 16px; }
  .staff-edit-grid h3 { font-size: 18px; }
  .staff-action-note { margin: 0; color: var(--scan-muted); font-size: 14px; line-height: 1.5; }
  .staff-edit-unsaved { margin: 0 0 16px; padding: 10px 12px; border-left: 3px solid var(--scan-lime); font-size: 14px; }
  .staff-workspace :global(button), .staff-list li { min-height: 44px; }
  .staff-workspace :global(.staff-assignment-list li) { min-height: 52px; overflow-wrap: anywhere; }
  .staff-workspace :global(.staff-assignment-controls) { grid-template-columns: minmax(0, 1fr); }
  @media (max-width: 1199px) {
    .staff-workspace.has-panel { grid-template-columns: minmax(0, 1fr); }
  }
  @media (max-width: 600px) {
    .staff-list li { grid-template-columns: minmax(0, 1fr) auto; gap: 12px; }
    .staff-list-identity { grid-column: 1 / -1; }
    .staff-list li > button { justify-self: end; }
    .staff-filter-toggle { display: inline-flex; justify-content: center; }
    .staff-extra-filters { grid-template-columns: minmax(0, 1fr); }
    .staff-extra-filters:not(.is-open) { display: none; }
  }
</style>
