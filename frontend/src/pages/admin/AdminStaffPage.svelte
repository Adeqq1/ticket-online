<script lang="ts">
  import AdminLayout from "../../layouts/AdminLayout.svelte";
  import { onMount, tick } from "svelte";
  import StaffAssignmentsEditor from "../../components/admin/StaffAssignmentsEditor.svelte";
  import { ApiError, createStaff, getAdminEvents, listStaff, resetStaffPassword, updateStaff, replaceStaffAssignments, type Staff, type StaffAssignment } from "../../lib/api.ts";
  import type { AdminApiEvent } from "../../lib/api.ts";

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
  let creating = $state(false);
  let createUncertain = $state(false);
  let selectedId = $state("");
  let editName = $state("");
  let editActive = $state(false);
  let editAssignments = $state<StaffAssignment[]>([]);
  let resetPassword = $state("");
  let profileBaseline = $state("");
  let assignmentBaseline = $state("");
  let selected = $derived(staff.find((item) => item.id === selectedId && item.role === "STAFF"));
  const staffEditorDirty = $derived(Boolean(selectedId) && (JSON.stringify({ name: editName, active: editActive }) !== profileBaseline || JSON.stringify(editAssignments) !== assignmentBaseline || Boolean(resetPassword)));
  const createDirty = $derived(creating && Boolean(newName || newEmail || newPassword || newAssignments.length));

  function assignmentLabel(assignment: StaffAssignment) {
    const event = events.find((item) => item.id === assignment.eventId);
    return `${event ? `${event.artist} · ${event.city}` : `Event ${assignment.eventId}`} · ${assignment.gate}`;
  }
  function protectUnsaved(event: BeforeUnloadEvent) {
    if (staffEditorDirty || createDirty || saving) { event.preventDefault(); event.returnValue = ""; }
  }
  function allowStaffDiscard() { return !(staffEditorDirty || createDirty) || window.confirm("Perubahan akun petugas yang belum disimpan akan hilang. Lanjutkan?"); }
  function requestLogout() { if (!saving && allowStaffDiscard()) onLogout(); }

  async function focusElement(id: string) {
    await tick();
    document.getElementById(id)?.focus();
  }

  function clearCreate() {
    newName = ""; newEmail = ""; newPassword = ""; newAssignments = [];
    creating = false; createUncertain = false;
  }

  function openCreate() {
    if (saving || creating || !allowStaffDiscard()) return;
    selectedId = ""; resetPassword = ""; error = ""; notice = ""; creating = true;
    void focusElement("staff-create-title");
  }

  function closePanel() {
    if (saving || !allowStaffDiscard()) return;
    const previousId = selectedId;
    clearCreate(); selectedId = ""; resetPassword = ""; error = "";
    void focusElement(previousId ? `staff-manage-${previousId}` : "staff-create-opener");
  }

  async function loadStaff(): Promise<boolean> {
    loadingStaff = true; staffError = "";
    try { staff = await listStaff(accessToken); return true; }
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
    resetPassword = ""; error = ""; notice = "";
    void focusElement("staff-edit-title");
  }

  function showError(cause: unknown) {
    if (cause instanceof ApiError && cause.status === 401) { onUnauthorized(); return; }
    error = cause instanceof ApiError && cause.code === "UNKNOWN_OUTCOME"
      ? "Hasil belum diketahui karena respons terputus. Muat ulang data sebelum mencoba kembali."
      : cause instanceof ApiError ? cause.message : "Tidak dapat menghubungi server.";
  }

  async function create(event: SubmitEvent) {
    event.preventDefault(); if (saving || !creating || createUncertain || loadingEvents || eventsError) return;
    saving = true; error = ""; notice = "";
    try {
      const created = await createStaff(accessToken, { name: newName, email: newEmail.trim(), password: newPassword, assignments: newAssignments });
      clearCreate();
      notice = "Akun petugas berhasil dibuat.";
      const loaded = await loadStaff();
      if (!loaded) notice = "Akun petugas berhasil dibuat, tetapi daftar belum dapat dimuat ulang. Muat ulang daftar untuk melihat akun baru.";
      void focusElement(loaded && staff.some((person) => person.id === created.id) ? `staff-account-${created.id}` : "staff-result");
    } catch (cause) {
      createUncertain = cause instanceof ApiError && (cause.code === "UNKNOWN_OUTCOME" || cause.code === "NETWORK_ERROR" || cause.status >= 500);
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
        void focusElement(`staff-account-${found.id}`);
      } else {
        createUncertain = false;
        error = "Akun dengan email tersebut belum ditemukan. Periksa input sebelum mencoba kembali.";
      }
    } finally { saving = false; }
  }

  async function saveDetails(event: SubmitEvent) {
    event.preventDefault(); if (saving || !selected) return;
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
    catch (cause) { showError(cause); }
    finally { saving = false; }
  }

  async function saveAssignments(event: SubmitEvent) {
    event.preventDefault(); if (saving || !selected) return;
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
    catch (cause) { showError(cause); }
    finally { saving = false; }
  }

  async function savePassword(event: SubmitEvent) {
    event.preventDefault(); if (saving || !selected) return;
    if (!window.confirm(`Reset password ${selected.name}? Semua sesi aktif petugas ini akan dicabut.`)) return;
    saving = true; error = ""; notice = "";
    try { await resetStaffPassword(accessToken, selected.id, resetPassword); resetPassword = ""; notice = "Password direset dan sesi lama petugas dicabut."; }
    catch (cause) { showError(cause); }
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
        <div class="staff-panel-heading"><h2 id="staff-list-title">Akun petugas</h2><button class="staff-text-button" type="button" onclick={() => { if (!saving) void loadStaff(); }} disabled={loadingStaff || saving}>{loadingStaff ? "Memuat…" : "Muat ulang"}</button></div>
        {#if loadingStaff}<p class="staff-muted" aria-live="polite">Memuat daftar petugas…</p>
        {:else if staffError}<p class="staff-form-message staff-form-error" role="alert">{staffError}</p>
        {:else if staff.length === 0}<p class="staff-muted">Belum ada akun petugas.</p>
        {:else}<ul class="staff-list">{#each staff as person (person.id)}
          <li id={`staff-account-${person.id}`} tabindex="-1" class:staff-selected={selectedId === person.id}>
            <div class="staff-list-identity"><strong>{person.name}</strong><span>{person.email}</span><span>{person.assignments.length ? person.assignments.map(assignmentLabel).join(", ") : "Tanpa penugasan"}</span></div>
            <div class="staff-account-meta"><span class="staff-role">{person.role === "ADMIN" ? "Admin" : "Staff"}</span><span class="staff-status" class:is-active={person.active}>{person.active ? "Aktif" : "Nonaktif"}</span></div>
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
          {#if loadingEvents}<p class="staff-muted" aria-live="polite">Memuat pilihan event…</p>{:else if eventsError}<p class="staff-form-message staff-form-error" role="alert">{eventsError} <button class="staff-text-button" type="button" onclick={loadEvents}>Coba lagi</button></p>{:else}<StaffAssignmentsEditor {events} bind:value={newAssignments} disabled={saving || createUncertain} />{/if}
          {#if createUncertain}<button class="staff-secondary-button" type="button" disabled={saving} onclick={checkCreateResult}>Periksa daftar petugas</button>{/if}
          <div class="staff-create-actions"><button class="scan-submit staff-submit" type="submit" disabled={saving || createUncertain || loadingEvents || Boolean(eventsError)}>{saving ? "Menyimpan…" : "Buat akun petugas"}</button><button class="staff-secondary-button" type="button" disabled={saving} onclick={closePanel}>Batal</button></div>
        </form>
      </section>
    {:else if selected}<section id="staff-work-panel" class="staff-admin-panel staff-edit-panel" aria-labelledby="staff-edit-title">
      <div class="staff-panel-heading"><h2 id="staff-edit-title" tabindex="-1">{selected.name}</h2><button class="staff-text-button" type="button" disabled={saving} onclick={closePanel}>Tutup</button></div>
      {#if staffEditorDirty}<p class="staff-edit-unsaved" role="status">Perubahan akun belum disimpan</p>{/if}
      <div class="staff-edit-grid">
        <form class="staff-form" onsubmit={saveDetails} aria-busy={saving}>
          <h3>Profil dan akses</h3>
          <p class="staff-action-note">Simpan nama dan status akun. Menonaktifkan akun langsung mencabut semua sesi.</p>
          <label>Nama<input bind:value={editName} minlength="2" maxlength="80" required disabled={saving} /></label>
          <label class="staff-checkbox"><input type="checkbox" bind:checked={editActive} disabled={saving} /><span>Akun aktif</span></label>
          <small>Menonaktifkan akun langsung mencabut semua sesi petugas tersebut.</small>
          <button class="staff-secondary-button" type="submit" disabled={saving}>Simpan profil</button>
        </form>
        <form class="staff-form" onsubmit={saveAssignments} aria-busy={saving}>
          <h3>Penugasan</h3>
          <p class="staff-action-note">Daftar ini menentukan event dan gate yang dapat diakses petugas. Perubahan berlaku saat disimpan; sesi tetap aktif.</p>
          {#if loadingEvents}<p class="staff-muted">Memuat katalog…</p>{:else if eventsError}<p class="staff-form-message staff-form-error">{eventsError} <button class="staff-text-button" type="button" onclick={loadEvents}>Coba lagi</button></p>{:else}<StaffAssignmentsEditor {events} bind:value={editAssignments} disabled={saving} />{/if}
          <button class="staff-secondary-button" type="submit" disabled={saving || loadingEvents || Boolean(eventsError)}>Simpan penugasan</button>
        </form>
        <form class="staff-form" onsubmit={savePassword} aria-busy={saving}>
          <h3>Reset password</h3>
          <p class="staff-action-note">Reset password adalah tindakan terpisah dan mencabut semua sesi aktif petugas.</p>
          <label>Password baru<input bind:value={resetPassword} type="password" autocomplete="new-password" minlength="12" maxlength="128" required disabled={saving} /></label>
          <small>Reset password mencabut semua sesi aktif milik petugas.</small>
          <button class="staff-secondary-button" type="submit" disabled={saving}>Reset password</button>
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
  .staff-list li { grid-template-columns: minmax(0, 1fr) auto auto; gap: 16px; padding-block: 20px; }
  .staff-account-meta { display: flex; flex-wrap: wrap; align-items: center; gap: 10px; }
  .staff-role { color: var(--admin-secondary); font-size: 14px; }
  .staff-status { display: inline-block; padding: 4px 10px; border: 1px solid var(--admin-control-border); border-radius: 4px; color: var(--admin-text); background: var(--admin-neutral-bg); font-size: 14px; line-height: 1.5; }
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
  }
</style>
