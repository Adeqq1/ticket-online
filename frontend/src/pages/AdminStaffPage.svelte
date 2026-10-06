<script lang="ts">
  import { onMount } from "svelte";
  import StaffAssignmentsEditor from "../components/StaffAssignmentsEditor.svelte";
  import { ApiError, createStaff, getAdminEvents, listStaff, resetStaffPassword, updateStaff, replaceStaffAssignments, type Staff, type StaffAssignment } from "../lib/api.ts";
  import type { AdminApiEvent } from "../lib/api.ts";

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
  let selectedId = $state("");
  let editName = $state("");
  let editActive = $state(false);
  let editAssignments = $state<StaffAssignment[]>([]);
  let resetPassword = $state("");
  let selected = $derived(staff.find((item) => item.id === selectedId && item.role === "STAFF"));

  async function loadStaff(): Promise<boolean> {
    loadingStaff = true; staffError = "";
    try { staff = await listStaff(accessToken); return true; }
    catch (cause) { if (cause instanceof ApiError && cause.status === 401) onUnauthorized(); else staffError = "Daftar petugas tidak dapat dimuat."; return false; }
    finally { loadingStaff = false; }
  }

  async function loadEvents() {
    loadingEvents = true; eventsError = "";
    try { events = await getAdminEvents(accessToken); }
    catch { eventsError = "Katalog event belum dapat dimuat."; }
    finally { loadingEvents = false; }
  }

  onMount(() => { void loadStaff(); void loadEvents(); });

  function selectStaff(person: Staff) {
    if (saving) return;
    selectedId = person.id; editName = person.name; editActive = person.active;
    editAssignments = person.assignments.map((assignment) => ({ ...assignment }));
    resetPassword = ""; error = ""; notice = "";
  }

  function showError(cause: unknown) {
    if (cause instanceof ApiError && cause.status === 401) { onUnauthorized(); return; }
    error = cause instanceof ApiError && cause.code === "UNKNOWN_OUTCOME"
      ? "Hasil belum diketahui karena respons terputus. Muat ulang data sebelum mencoba kembali."
      : cause instanceof ApiError ? cause.message : "Tidak dapat menghubungi server.";
  }

  async function create(event: SubmitEvent) {
    event.preventDefault(); if (saving) return;
    saving = true; error = ""; notice = "";
    try {
      await createStaff(accessToken, { name: newName, email: newEmail.trim(), password: newPassword, assignments: newAssignments });
      newName = ""; newEmail = ""; newPassword = ""; newAssignments = [];
      notice = "Akun petugas berhasil dibuat."; await loadStaff();
    } catch (cause) { showError(cause); }
    finally { saving = false; }
  }

  async function saveDetails(event: SubmitEvent) {
    event.preventDefault(); if (saving || !selected) return;
    if (selected.active && !editActive && !window.confirm(`Nonaktifkan ${selected.name}? Semua sesi petugas ini akan dicabut.`)) return;
    saving = true; error = ""; notice = "";
    try { await updateStaff(accessToken, selected.id, { name: editName, active: editActive }); notice = "Profil petugas berhasil diperbarui."; await loadStaff(); }
    catch (cause) { showError(cause); }
    finally { saving = false; }
  }

  async function saveAssignments(event: SubmitEvent) {
    event.preventDefault(); if (saving || !selected) return;
    saving = true; error = ""; notice = "";
    try {
      const staffId = selected.id;
      await replaceStaffAssignments(accessToken, staffId, editAssignments);
      const loaded = await loadStaff();
      const refreshed = loaded && staff.find((item) => item.id === staffId);
      if (refreshed && selectedId === staffId) editAssignments = refreshed.assignments.map((item) => ({ ...item }));
      notice = loaded ? "Penugasan berhasil diperbarui." : "Penugasan tersimpan, tetapi daftar belum dapat dimuat ulang.";
    }
    catch (cause) { showError(cause); }
    finally { saving = false; }
  }

  async function savePassword(event: SubmitEvent) {
    event.preventDefault(); if (saving || !selected) return;
    saving = true; error = ""; notice = "";
    try { await resetStaffPassword(accessToken, selected.id, resetPassword); resetPassword = ""; notice = "Password direset dan sesi lama petugas dicabut."; }
    catch (cause) { showError(cause); }
    finally { saving = false; }
  }
</script>

<svelte:head>
  <title>Kelola Petugas | Gate Control</title>
  <meta name="description" content="Kelola akun dan penugasan petugas event." />
  <meta name="robots" content="noindex" />
</svelte:head>

<div class="scan-shell staff-admin-shell">
  <a class="skip-link" href="#staff-admin-content">Lewati ke pengelolaan petugas</a>
  <header class="scan-topbar"><a class="scan-brand" href="/" aria-label="Kembali ke Tiket Online"><span class="scan-brand-mark" aria-hidden="true">TO</span><span>Tiket Online <b>/ Gate Control</b></span></a><span class="staff-login-label">ADMINISTRATOR</span><button class="staff-text-button" type="button" disabled={saving || loggingOut} onclick={onLogout}>Keluar</button></header>
  <main id="staff-admin-content" class="staff-admin-content">
    <div class="staff-admin-heading"><div><p class="scan-kicker">OPERASIONAL EVENT <span>•</span> ADMINISTRASI</p><h1>Kelola petugas.</h1><p>Atur akses tim pada event dan gate yang ditugaskan.</p></div></div>
    <nav class="admin-tool-nav" aria-label="Administrasi event"><a href="/admin/events">Konser</a><a aria-current="page" href="/admin/staff">Kelola petugas</a><a href="/admin/check-ins">Riwayat check-in</a></nav>

    {#if notice}<p class="staff-form-message staff-form-success" role="status">{notice}</p>{/if}
    {#if error}<p class="staff-form-message staff-form-error" role="alert">{error}</p>{/if}

    <div class="staff-admin-grid">
      <section class="staff-admin-panel" aria-labelledby="staff-list-title">
        <div class="staff-panel-heading"><div><span class="panel-index">01</span><h2 id="staff-list-title">Akun petugas</h2></div><button class="staff-text-button" type="button" onclick={() => { if (!saving) void loadStaff(); }} disabled={loadingStaff || saving}>{loadingStaff ? "Memuat…" : "Muat ulang"}</button></div>
        {#if loadingStaff}<p class="staff-muted" aria-live="polite">Memuat daftar petugas…</p>
        {:else if staffError}<p class="staff-form-message staff-form-error" role="alert">{staffError}</p>
        {:else if staff.length === 0}<p class="staff-muted">Belum ada akun petugas.</p>
        {:else}<ul class="staff-list">{#each staff as person (person.id)}
          <li class:staff-selected={selectedId === person.id}><div class="staff-list-identity"><strong>{person.name}</strong><span>{person.email}</span><span>{person.assignments.length ? person.assignments.map((assignment) => `${assignment.eventId} · ${assignment.gate}`).join(", ") : "Tanpa penugasan"}</span></div><span class:staff-state-inactive={!person.active} class="staff-state">{person.role}{person.active ? " · Aktif" : " · Nonaktif"}</span>{#if person.role === "STAFF"}<button class="staff-text-button" type="button" disabled={saving} onclick={() => selectStaff(person)}>{selectedId === person.id ? "Dipilih" : "Kelola"}</button>{/if}</li>
        {/each}</ul>{/if}
      </section>

      <section class="staff-admin-panel" aria-labelledby="staff-create-title">
        <div class="staff-panel-heading"><div><span class="panel-index">02</span><h2 id="staff-create-title">Tambah petugas</h2></div></div>
        <form class="staff-form" onsubmit={create} aria-busy={saving}>
          <label>Nama<input bind:value={newName} autocomplete="name" minlength="2" maxlength="80" required disabled={saving} /></label>
          <label>Email<input bind:value={newEmail} type="email" autocomplete="email" maxlength="254" required disabled={saving} /></label>
          <label>Password awal<input bind:value={newPassword} type="password" autocomplete="new-password" minlength="12" maxlength="128" required disabled={saving} /><small>Minimal 12 karakter; sesi lama akan dicabut saat password diganti.</small></label>
          {#if loadingEvents}<p class="staff-muted" aria-live="polite">Memuat pilihan event…</p>{:else if eventsError}<p class="staff-form-message staff-form-error" role="alert">{eventsError} <button class="staff-text-button" type="button" onclick={loadEvents}>Coba lagi</button></p>{:else}<StaffAssignmentsEditor {events} bind:value={newAssignments} disabled={saving} />{/if}
          <button class="scan-submit staff-submit" type="submit" disabled={saving || loadingEvents || Boolean(eventsError)}>{saving ? "Menyimpan…" : "Buat akun petugas"}</button>
        </form>
      </section>
    </div>

    {#if selected}<section class="staff-admin-panel staff-edit-panel" aria-labelledby="staff-edit-title">
      <div class="staff-panel-heading"><div><span class="panel-index">03</span><h2 id="staff-edit-title">{selected.name}</h2></div><button class="staff-text-button" type="button" disabled={saving} onclick={() => { if (saving) return; selectedId = ""; notice = ""; error = ""; }}>Tutup</button></div>
      <div class="staff-edit-grid">
        <form class="staff-form" onsubmit={saveDetails} aria-busy={saving}>
          <h3>Profil dan akses</h3>
          <label>Nama<input bind:value={editName} minlength="2" maxlength="80" required disabled={saving} /></label>
          <label class="staff-checkbox"><input type="checkbox" bind:checked={editActive} disabled={saving} /><span>Akun aktif</span></label>
          <small>Menonaktifkan akun langsung mencabut semua sesi petugas tersebut.</small>
          <button class="staff-secondary-button" type="submit" disabled={saving}>Simpan profil</button>
        </form>
        <form class="staff-form" onsubmit={saveAssignments} aria-busy={saving}>
          <h3>Penugasan</h3>
          {#if loadingEvents}<p class="staff-muted">Memuat katalog…</p>{:else if eventsError}<p class="staff-form-message staff-form-error">{eventsError} <button class="staff-text-button" type="button" onclick={loadEvents}>Coba lagi</button></p>{:else}<StaffAssignmentsEditor {events} bind:value={editAssignments} disabled={saving} />{/if}
          <button class="staff-secondary-button" type="submit" disabled={saving || loadingEvents || Boolean(eventsError)}>Simpan penugasan</button>
        </form>
        <form class="staff-form" onsubmit={savePassword} aria-busy={saving}>
          <h3>Reset password</h3>
          <label>Password baru<input bind:value={resetPassword} type="password" autocomplete="new-password" minlength="12" maxlength="128" required disabled={saving} /></label>
          <small>Reset password mencabut semua sesi aktif milik petugas.</small>
          <button class="staff-secondary-button" type="submit" disabled={saving}>Reset password</button>
        </form>
      </div>
    </section>{/if}
    <footer class="scan-footer"><span>Ticket Online · Admin tools</span><span>Pengaturan akun dan akses event</span></footer>
  </main>
</div>
