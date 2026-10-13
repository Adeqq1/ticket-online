<script lang="ts">
  import { MediaQuery } from "svelte/reactivity";
  import AdminMobileList from "../../components/admin/AdminMobileList.svelte";
  import { mobileListItems } from "../../lib/mobile-list.ts";
  const mobileList = new MediaQuery("(max-width: 767px)");

  import AdminLayout from "../../layouts/AdminLayout.svelte";
  import AdminHelp from "../../components/admin/AdminHelp.svelte";
  import { onMount, tick } from "svelte";
  import StaffAssignmentsEditor from "../../components/admin/StaffAssignmentsEditor.svelte";
  import { ApiError, createStaff, getAdminEvents, listStaff, resetStaffPassword, updateStaff, replaceStaffAssignments, type Staff, type StaffAssignment } from "../../lib/api.ts";
  import type { AdminApiEvent } from "../../lib/api.ts";
  import { filterStaff, sameAssignments, type StaffFilters } from "../../lib/staff-management.ts";

  let mobileGroup = $state(0);
  let mobileScroll = $state(0);
  let layout: ReturnType<typeof AdminLayout>;
  async function confirmAction(title: string, message: string, confirmLabel: string) {
    const token = accessToken;
    const before = confirmationState();
    return await layout.confirm({ title, message, confirmLabel }) && token === accessToken && before === confirmationState();
  }
  const confirmationState = () => JSON.stringify([selectedId, selected, creating, editName, editActive, editAssignments, resetPassword, newName, newEmail, newPassword, newAssignments, uncertain, createUncertain, saving]);
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
  let createStep = $state(0);
  let editSection = $state(0);
  const createSteps = ["Akun", "Penugasan", "Tinjau"];
  const editSections = ["Profil dan akses", "Penugasan", "Reset password"];
  const sectionIcons = ["M12 12a4 4 0 1 0 0-8 4 4 0 0 0 0 8ZM4 21v-2a8 8 0 0 1 16 0v2", "M4 4h16v16H4ZM8 8h8M8 12h8M8 16h5", "M7 11V7a5 5 0 0 1 10 0v4M5 11h14v10H5Z"];
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
  $effect(() => { filters.query; filters.status; filters.eventId; filters.unassigned; mobileGroup = 0; mobileScroll = 0; });
  let filtersActive = $derived(Boolean(filters.query.trim() || filters.status !== "all" || filters.eventId || filters.unassigned));
  let activeFilterCount = $derived(Number(filters.status !== "all") + Number(Boolean(filters.eventId)) + Number(filters.unassigned));
  let filterEventIds = $derived([...new Set([...events.map((item) => item.id), ...staff.flatMap((person) => person.assignments.map((item) => item.eventId))])]);
  let uncertain = $state<Record<string, { profile?: { name: string; active: boolean }; assignments?: StaffAssignment[]; password?: boolean }>>({});
  let selectedUncertain = $derived(uncertain[selectedId]);
  let hasUncertain = $derived(Object.values(uncertain).some((item) => item.profile || item.assignments || item.password));
  let selected = $derived(staff.find((item) => item.id === selectedId && item.role === "STAFF"));
  const mode = $derived(creating ? "create" : selectedId ? "edit" : "list");
  let listScroll = 0;
  const staffEditorDirty = $derived(Boolean(selectedId) && (JSON.stringify({ name: editName, active: editActive }) !== profileBaseline || JSON.stringify(editAssignments) !== assignmentBaseline || Boolean(resetPassword) || editAssignmentPending));
  const createDirty = $derived(creating && Boolean(newName || newEmail || newPassword || newAssignments.length || newAssignmentPending));

  function assignmentLabel(assignment: StaffAssignment) {
    const event = events.find((item) => item.id === assignment.eventId);
    return `${event ? `${event.artist} · ${event.city}` : `Event ${assignment.eventId}`} · ${assignment.gate}`;
  }
  function protectUnsaved(event: BeforeUnloadEvent) {
    if (!layout?.consumeNavigationApproval() && (staffEditorDirty || createDirty || saving || hasUncertain)) { event.preventDefault(); event.returnValue = ""; }
  }
  async function allowStaffDiscard() {
    if (createUncertain || selectedUncertain?.profile || selectedUncertain?.assignments || selectedUncertain?.password) return (await confirmAction("Keluar dari panel?", "Hasil tindakan akun belum pasti. Jangan mengulang tindakan sebelum hasilnya diperiksa. Lanjutkan keluar dari panel?", "Keluar dari panel"));
    return !(staffEditorDirty || createDirty) || (await confirmAction("Buang perubahan?", "Perubahan akun petugas yang belum disimpan akan hilang. Lanjutkan?", "Buang perubahan"));
  }
  async function allowNavigation() {
    if (saving) { notice = "Tunggu penyimpanan selesai sebelum meninggalkan halaman."; return false; }
    if (hasUncertain) return confirmAction("Keluar dari panel?", "Hasil tindakan akun belum pasti. Jangan mengulang tindakan sebelum hasilnya diperiksa. Lanjutkan keluar dari panel?", "Keluar dari panel");
    return allowStaffDiscard();
  }
  async function requestLogout() { if (!saving && await allowStaffDiscard()) onLogout(); }
  function resetFilters() { filters = { query: "", status: "all", eventId: "", unassigned: false }; void focusElement("staff-search"); }

  function unknownOutcome(cause: unknown) {
    return cause instanceof ApiError && (cause.code === "UNKNOWN_OUTCOME" || cause.code === "NETWORK_ERROR" || cause.status >= 500);
  }

  async function focusElement(id: string) {
    await tick();
    document.getElementById(id)?.focus();
  }

  function revealAccount(id: string, loaded = true) {
    const index = filteredStaff.findIndex(person => person.id === id);
    if (loaded && index >= 0 && mobileList.current) { mobileGroup = Math.floor(index / 5); mobileScroll = 0; }
    void focusElement(loaded && index >= 0 ? `staff-account-${id}` : "staff-result");
  }

  function clearCreate() {
    newName = ""; newEmail = ""; newPassword = ""; newAssignments = [];
    createStep = 0; creating = false; createUncertain = false; newAssignmentPending = false;
  }

  async function openCreate() {
    if (saving || creating || !await allowStaffDiscard()) return;
    listScroll = window.scrollY;
    selectedId = ""; resetPassword = ""; editAssignmentPending = false; error = ""; notice = ""; creating = true;
    void focusElement("staff-create-title");
  }

  async function closePanel() {
    if (saving || !await allowStaffDiscard()) return;
    const previousId = selectedId;
    const index = filteredStaff.findIndex(person => person.id === previousId);
    if (mobileList.current && index >= 0 && Math.floor(index / 5) !== mobileGroup) { mobileGroup = Math.floor(index / 5); mobileScroll = 0; }
    clearCreate(); selectedId = ""; resetPassword = ""; editAssignmentPending = false; error = "";
    await tick();
    document.getElementById(previousId ? filteredStaff.some((person) => person.id === previousId) ? `staff-manage-${previousId}` : "staff-list-title" : "staff-create-opener")?.focus({ preventScroll: true });
    window.scrollTo({ top: listScroll, behavior: "instant" });
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

  async function selectStaff(person: Staff) {
    if (saving) return;
    if (person.id === selectedId || !await allowStaffDiscard()) return;
    listScroll = window.scrollY;
    clearCreate();
    editSection = 0; selectedId = person.id; editName = person.name; editActive = person.active;
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

  async function navigateCreate(next: number) {
    if (saving || createUncertain) return false;
    if (next > createStep) {
      const invalid = document.querySelector<HTMLInputElement>("#staff-account-fields :invalid");
      if (invalid) { createStep = 0; await tick(); invalid.reportValidity(); invalid.focus(); return false; }
      if (next === 2 && (newAssignmentPending || loadingEvents || eventsError)) {
        createStep = 1; error = newAssignmentPending ? "Tambahkan atau batalkan pilihan penugasan sebelum meninjau." : "Tunggu katalog event tersedia sebelum meninjau.";
        await focusElement("staff-create-section-title"); return false;
      }
    }
    error = ""; createStep = next; await focusElement("staff-create-section-title"); return true;
  }
  async function navigateEdit(next: number) {
    if (saving) return;
    editSection = next;
    await focusElement(["staff-profile", "staff-assignments", "staff-password"][next]!);
  }

  async function create(event: SubmitEvent) {
    event.preventDefault(); if (saving || !creating || createUncertain) return;
    if (createStep < 2) { await navigateCreate(createStep + 1); return; }
    const invalid = document.querySelector<HTMLInputElement>("#staff-account-fields :invalid");
    if (invalid) { createStep = 0; await tick(); invalid.reportValidity(); invalid.focus(); return; }
    if (newAssignmentPending || loadingEvents || eventsError) { createStep = 1; await focusElement("staff-create-section-title"); return; }
    saving = true; error = ""; notice = "";
    try {
      const created = await createStaff(accessToken, { name: newName, email: newEmail.trim(), password: newPassword, assignments: newAssignments });
      clearCreate();
      notice = "Akun petugas berhasil dibuat.";
      const loaded = await loadStaff();
      if (!loaded) notice = "Akun petugas berhasil dibuat, tetapi daftar belum dapat dimuat ulang. Muat ulang daftar untuk melihat akun baru.";
      revealAccount(created.id, loaded);
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
        revealAccount(found.id);
      } else {
        createUncertain = false;
        error = "Akun dengan email tersebut belum ditemukan. Periksa input sebelum mencoba kembali.";
      }
    } finally { saving = false; }
  }

  async function saveDetails(event: SubmitEvent) {
    event.preventDefault(); if (saving || !selected || selectedUncertain?.profile) return;
    if (selected.active && !editActive && !(await confirmAction("Nonaktifkan akun?", `Nonaktifkan ${selected.name}? Semua sesi petugas ini akan dicabut.`, "Nonaktifkan akun"))) return;
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
    const staffId = selected.id;
    const before = selected.assignments;
    const removed = before.filter((item) => !editAssignments.some((next) => next.eventId === item.eventId && next.gate === item.gate));
    const added = editAssignments.filter((item) => !before.some((old) => old.eventId === item.eventId && old.gate === item.gate));
    if ((added.length || removed.length) && !(await confirmAction("Simpan penugasan?", `Simpan perubahan penugasan untuk ${selected.name}? Event dan gate yang diizinkan akan mengikuti daftar baru. Sesi tetap aktif.\n${added.length ? `Ditambahkan: ${added.map(assignmentLabel).join(", ")}. ` : ""}${removed.length ? `Dihapus: ${removed.map(assignmentLabel).join(", ")}.` : ""}`, "Simpan penugasan"))) return;
    saving = true; error = ""; notice = "";
    try {
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
    if (!(await confirmAction("Reset password?", `Reset password ${selected.name}? Semua sesi aktif petugas ini akan dicabut.`, "Reset password"))) return;
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

  <AdminLayout bind:this={layout} onBeforeNavigate={allowNavigation} page="staff" mobileDescriptionHelp={mode === "list"} contentId="staff-admin-content" skipLabel="Lewati ke pengelolaan petugas" kicker="OPERASIONAL EVENT • ADMINISTRASI" title={mode === "list" ? "Daftar petugas." : mode === "create" ? "Tambah petugas." : "Edit petugas."} description={mode === "list" ? "Temukan akun dan periksa akses tim pada event dan gate." : mode === "create" ? "Buat akun petugas beserta penugasan awalnya." : "Ubah profil, penugasan, atau password akun yang dipilih."} onLogout={requestLogout} loggingOut={loggingOut} logoutDisabled={loggingOut || saving} primaryAction={mode === "list" ? { label: "Tambah petugas", onclick: openCreate, disabled: saving, id: "staff-create-opener" } : undefined} secondaryAction={mode === "edit" ? { label: "Muat ulang", onclick: () => { void loadStaff(); }, disabled: loadingStaff || saving } : undefined}>
    <div id="staff-result" role="status" aria-live="polite" tabindex="-1">
      {#if notice}<p class="staff-form-message staff-form-success">{notice}</p>{/if}
    </div>
    {#if error}<p class="staff-form-message staff-form-error" role="alert">{error}</p>{/if}

    {#if mode !== "list" && staffError}<p class="staff-form-message staff-form-error" role="alert">{staffError}</p>{/if}
    <div class="staff-workspace">
      {#if mode === "list"}
      <section class="staff-admin-panel" aria-labelledby="staff-list-title">
        <div class="staff-panel-heading"><AdminHelp id="staff-list-help" label="Penjelasan pengelolaan petugas" text="Pilih Kelola untuk mengubah profil, penugasan event/gate, atau status akun petugas."><h2 id="staff-list-title" tabindex="-1">Akun petugas</h2><button class="staff-text-button" type="button" onclick={() => { if (!saving) void loadStaff(); }} disabled={loadingStaff || saving}>{loadingStaff ? "Memuat…" : "Muat ulang"}</button></AdminHelp></div>
        <p class="staff-list-help staff-list-explanation">Pilih Kelola untuk mengubah profil, penugasan event/gate, atau status akun petugas.</p>
        <div class="staff-form staff-list-filters" role="search" aria-label="Cari dan filter akun">
          <label>Cari nama atau email<input id="staff-search" type="search" bind:value={filters.query} autocomplete="off" /></label>
          <button class="staff-secondary-button staff-filter-toggle" type="button" aria-expanded={showFilters} aria-controls="staff-extra-filters" onclick={() => { showFilters = !showFilters; }}>Filter lainnya{activeFilterCount ? ` (${activeFilterCount} aktif)` : ""}</button>
          <div id="staff-extra-filters" class="staff-extra-filters" class:is-open={showFilters}>
            <label>Status akun<select bind:value={filters.status}><option value="all">Semua</option><option value="active">Aktif</option><option value="inactive">Nonaktif</option></select></label>
            <label>Event penugasan<select bind:value={filters.eventId}><option value="">Semua event</option>{#each filterEventIds as id}<option value={id}>{events.find((item) => item.id === id)?.artist ?? `Event ${id}`}</option>{/each}</select></label>
            <label class="staff-checkbox"><input type="checkbox" bind:checked={filters.unassigned} /><span>Hanya akun tanpa penugasan</span></label>
          </div>
        </div>
        <div class="staff-filter-results"><p class="staff-list-help" role="status" aria-live="polite" aria-atomic="true">{filteredStaff.length} dari {staff.length} akun</p>{#if filtersActive}<button class="staff-text-button" type="button" onclick={resetFilters}>Reset filter</button>{/if}</div>
        {#if loadingStaff}<p class="staff-muted" aria-live="polite">Memuat daftar petugas…</p>
        {:else if staffError}<p class="staff-form-message staff-form-error" role="alert">{staffError}</p>
        {:else if staff.length === 0}<p class="staff-muted">Belum ada akun petugas.</p>
        {:else if filteredStaff.length === 0}<p class="staff-muted">Tidak ada akun yang cocok. Ubah pencarian atau reset filter.</p>
        {:else}<AdminMobileList count={filteredStaff.length} bind:group={mobileGroup} bind:scrollLeft={mobileScroll} headingId="staff-list-title">{#if mobileList.current}<ul class="staff-list">{#each mobileListItems(filteredStaff, mobileGroup, mobileList.current) as person (person.id)}
          <li id={`staff-account-${person.id}`} class:admin-mobile-card={mobileList.current} tabindex="-1">
                <div class="admin-mobile-card-header"><strong>{person.name}</strong><span class="staff-status" class:is-active={person.active}>Status: {person.active ? "Aktif" : "Nonaktif"}</span></div>
                <dl class="admin-card-fields">
                  <div><dt>Email</dt><dd>{person.email}</dd></div>
                  <div><dt>Peran</dt><dd><span class="staff-role">{person.role === "ADMIN" ? "Admin" : "Staff"}</span></dd></div>
                  <div><dt>Penugasan</dt><dd class="staff-list-identity">
                    {#each person.assignments.slice(0, 2) as assignment}<span class="admin-card-assignment">{assignmentLabel(assignment)}</span>{/each}
                    {#if person.assignments.length > 2}<details><summary>Lihat semua penugasan ({person.assignments.length.toLocaleString("id-ID")})</summary>{#each person.assignments.slice(2) as assignment}<span>{assignmentLabel(assignment)}</span>{/each}</details>{/if}
                    {#if !person.assignments.length}{person.role === "STAFF" ? "Belum ditugaskan. Atur event dan gate melalui Kelola." : "Tanpa penugasan"}{/if}
                  </dd></div>
                </dl>
                {#if person.role === "ADMIN"}<span>Akun administrator tidak dapat diubah melalui halaman ini.</span>{/if}
            {#if person.role === "STAFF"}<button id={`staff-manage-${person.id}`} class="staff-secondary-button" type="button" disabled={saving} onclick={() => void selectStaff(person)}>Kelola<span class="visually-hidden"> {person.name}</span></button>{/if}
          </li>
        {/each}</ul>
        {:else}
            <table class="history-table admin-list-table staff-list-table">
              <caption class="visually-hidden">Daftar akun petugas dan penugasannya</caption>
              <thead><tr><th scope="col">Nama</th><th scope="col">Email</th><th scope="col">Peran</th><th scope="col">Status</th><th scope="col">Penugasan</th><th scope="col">Aksi</th></tr></thead>
              <tbody>{#each filteredStaff as person (person.id)}<tr id={`staff-account-${person.id}`} tabindex="-1">
                <td><strong>{person.name}</strong></td>
                <td>{person.email}</td>
                <td><span class="staff-role">{person.role === "ADMIN" ? "Admin" : "Staff"}</span></td>
                <td><span class="staff-status" class:is-active={person.active}>Status: {person.active ? "Aktif" : "Nonaktif"}</span></td>
                <td><div class="staff-list-identity">
                  {#each person.assignments.slice(0, 2) as assignment}<span>{assignmentLabel(assignment)}</span>{/each}
                  {#if person.assignments.length > 2}<details><summary>Lihat semua penugasan ({person.assignments.length.toLocaleString("id-ID")})</summary>{#each person.assignments.slice(2) as assignment}<span>{assignmentLabel(assignment)}</span>{/each}</details>{/if}
                  {#if !person.assignments.length}<span>{person.role === "STAFF" ? "Belum ditugaskan. Atur event dan gate melalui Kelola." : "Tanpa penugasan"}</span>{/if}
                </div></td>
                <td>{#if person.role === "STAFF"}<button id={`staff-manage-${person.id}`} class="staff-secondary-button admin-icon-button" type="button" aria-label={`Kelola ${person.name}`} title={`Kelola ${person.name}`} disabled={saving} onclick={() => void selectStaff(person)}><svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="m16 3 5 5-12 12-6 1 1-6Z" /><path d="m14 5 5 5" /></svg></button>{:else}<small>Akun administrator tidak dapat diubah melalui halaman ini.</small>{/if}</td>
              </tr>{/each}</tbody>
            </table>
        {/if}</AdminMobileList>{/if}
      </section>
      {:else if creating}<section id="staff-work-panel" class="staff-admin-panel staff-create-panel" aria-labelledby="staff-create-title">
        <div class="staff-panel-heading"><h2 id="staff-create-title" tabindex="-1">Tambah petugas</h2><button class="staff-secondary-button" type="button" disabled={saving} onclick={closePanel}>Kembali ke daftar</button></div>
        <div class="admin-wizard">
        <nav class="staff-section-nav admin-wizard-nav" aria-label="Langkah tambah petugas">
          {#each createSteps as label, index}<button class="staff-secondary-button" type="button" title={`${index + 1}. ${label}`} aria-current={createStep === index ? "step" : undefined} disabled={saving || createUncertain} onclick={() => navigateCreate(index)}><svg class="admin-wizard-mobile-icon" width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d={index === 2 ? "m5 12 4 4L19 6" : sectionIcons[index]} /></svg><span>{index + 1}. {label}</span></button>{/each}
        </nav>
        <div class="admin-wizard-content admin-wizard-body staff-create-content">
        <h3 id="staff-create-section-title" tabindex="-1">{createSteps[createStep]}</h3>
        <p class="staff-action-note" aria-live="polite">Langkah {createStep + 1} dari 3</p>
        <form id="staff-create-form" class="staff-form" novalidate onsubmit={create} aria-busy={saving}>
          <div id="staff-account-fields" class="staff-form-fields" hidden={createStep !== 0}>
          <label>Nama<input bind:value={newName} autocomplete="name" minlength="2" maxlength="80" required disabled={saving} readonly={createUncertain} /></label>
          <label>Email<input bind:value={newEmail} type="email" autocomplete="email" maxlength="254" required disabled={saving} readonly={createUncertain} /></label>
          <label>Password awal<input bind:value={newPassword} type="password" autocomplete="new-password" minlength="12" maxlength="128" required disabled={saving} readonly={createUncertain} /><small>Minimal 12 karakter; sesi lama akan dicabut saat password diganti.</small></label>
          </div>
          <div hidden={createStep !== 1}>
          {#if loadingEvents}<p class="staff-muted" aria-live="polite">Memuat pilihan event…</p>{:else if eventsError}<p class="staff-form-message staff-form-error" role="alert">{eventsError} Penugasan belum tersedia; input tetap tersimpan. <button class="staff-text-button" type="button" onclick={loadEvents}>Coba lagi</button></p>{:else}<StaffAssignmentsEditor {events} bind:value={newAssignments} bind:pending={newAssignmentPending} mode="create" disabled={saving || createUncertain} />{/if}
          </div>
          <div hidden={createStep !== 2}>
            <dl class="staff-review">
              <div><dt>Nama</dt><dd>{newName}</dd></div><div><dt>Email</dt><dd>{newEmail}</dd></div>
              <div><dt>Password awal</dt><dd>{newPassword ? "Sudah diisi" : "Belum diisi"}</dd></div>
              <div><dt>Penugasan</dt><dd>{#if newAssignments.length}<ul>{#each newAssignments as assignment}<li>{assignmentLabel(assignment)}</li>{/each}</ul>{:else}Tanpa penugasan; akses event/gate belum diberikan.{/if}</dd></div>
            </dl>
          </div>
          {#if createUncertain}<button class="staff-secondary-button" type="button" disabled={saving} onclick={checkCreateResult}>Periksa daftar petugas</button>{/if}
          <div class="staff-create-actions">
            {#if createStep > 0}<button class="staff-secondary-button" type="button" disabled={saving || createUncertain} onclick={() => navigateCreate(createStep - 1)}>Sebelumnya</button>{/if}
            <button class="scan-submit staff-submit" type="submit" disabled={saving || createUncertain || (createStep > 0 && (newAssignmentPending || loadingEvents || Boolean(eventsError)))}>{saving ? "Menyimpan…" : createStep === 2 ? "Buat akun petugas" : "Lanjutkan"}</button>
            <button class="staff-secondary-button" type="button" disabled={saving} onclick={closePanel}>Batal</button>
          </div>
        </form>
        </div></div>
      </section>
    {:else if selected}<section id="staff-work-panel" class="staff-admin-panel staff-edit-panel" aria-labelledby="staff-edit-title">
      <div class="staff-panel-heading"><h2 id="staff-edit-title" tabindex="-1">Edit petugas: {selected.name}</h2><button class="staff-secondary-button" type="button" disabled={saving} onclick={closePanel}>Kembali ke daftar</button></div>
      <p class="staff-list-help">{selected.email}</p>
      {#if !filteredStaff.some((person) => person.id === selectedId)}<p class="staff-list-help" role="status">{selected.name} tidak termasuk hasil saat ini. Editor dan draft tetap terbuka.</p>{/if}
      {#if selectedUncertain?.profile || selectedUncertain?.assignments}<p class="staff-list-help" role="alert">Hasil perubahan {selectedUncertain.profile ? "profil" : "penugasan"} belum pasti. Tindakan tersebut ditahan. Muat ulang daftar dan periksa hasilnya sebelum melanjutkan.</p>{/if}
      {#if selectedUncertain?.password}<p class="staff-list-help" role="alert">Hasil reset password belum pasti. Jangan mengulang reset. Minta petugas memeriksa apakah password baru dapat digunakan; daftar akun tidak dapat membuktikan hasil reset.</p>{/if}
      {#if staffEditorDirty}<p class="staff-edit-unsaved" role="status">Perubahan akun belum disimpan</p>{/if}
      <div class="admin-wizard">
      <nav class="staff-section-nav admin-wizard-nav" aria-label="Bagian pengelolaan akun">{#each editSections as label, index}<button class="staff-secondary-button" type="button" aria-label={`Bagian ${label}`} title={`Bagian ${label}`} aria-current={editSection === index ? "page" : undefined} disabled={saving} onclick={() => navigateEdit(index)}><svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d={sectionIcons[index]} /></svg><span>{label}</span></button>{/each}</nav>
      <div class="staff-edit-grid admin-wizard-content">
        <form class="staff-form" hidden={editSection !== 0} onsubmit={saveDetails} aria-busy={saving}>
          <h3 id="staff-profile" tabindex="-1">Profil dan akses</h3>
          <p class="staff-action-note">Simpan nama dan status akun. Menonaktifkan akun langsung mencabut semua sesi.</p>
          <label>Nama<input bind:value={editName} minlength="2" maxlength="80" required disabled={saving} readonly={Boolean(selectedUncertain?.profile)} /></label>
          <label class="staff-checkbox"><input type="checkbox" bind:checked={editActive} disabled={saving || Boolean(selectedUncertain?.profile)} /><span>Akun aktif</span></label>
          <small>Menonaktifkan akun langsung mencabut semua sesi petugas tersebut.</small>
          <button class="staff-secondary-button" type="submit" disabled={saving || Boolean(selectedUncertain?.profile)}>Simpan profil</button>
        </form>
        <form class="staff-form" hidden={editSection !== 1} onsubmit={saveAssignments} aria-busy={saving}>
          <h3 id="staff-assignments" tabindex="-1">Penugasan</h3>
          <p class="staff-action-note">Daftar ini menentukan event dan gate yang dapat diakses petugas. Perubahan berlaku saat disimpan; sesi tetap aktif.</p>
          {#if loadingEvents}<p class="staff-muted">Memuat katalog…</p>{:else if eventsError}<p class="staff-form-message staff-form-error" role="alert">{eventsError} Draft penugasan tetap tersedia setelah katalog dimuat ulang. <button class="staff-text-button" type="button" onclick={loadEvents}>Coba lagi</button></p>{:else}{#key selectedId}<StaffAssignmentsEditor {events} bind:value={editAssignments} bind:pending={editAssignmentPending} disabled={saving || Boolean(selectedUncertain?.assignments)} />{/key}{/if}
          <button class="staff-secondary-button" type="submit" disabled={saving || Boolean(selectedUncertain?.assignments) || editAssignmentPending || loadingEvents || Boolean(eventsError)}>Simpan penugasan</button>
        </form>
        <form class="staff-form" hidden={editSection !== 2} onsubmit={savePassword} aria-busy={saving}>
          <h3 id="staff-password" tabindex="-1">Reset password</h3>
          <p class="staff-action-note">Reset password adalah tindakan terpisah dan mencabut semua sesi aktif petugas.</p>
          <label>Password baru<input bind:value={resetPassword} type="password" autocomplete="new-password" minlength="12" maxlength="128" required disabled={saving} readonly={Boolean(selectedUncertain?.password)} /></label>
          <small>Reset password mencabut semua sesi aktif milik petugas.</small>
          <button class="staff-secondary-button" type="submit" disabled={saving || Boolean(selectedUncertain?.password)}>Reset password</button>
        </form>
      </div></div>
    </section>{/if}
    </div>
    <footer class="scan-footer"><span>Ticket Online · Admin tools</span><span>Pengaturan akun dan akses event</span></footer>
</AdminLayout>

<style>
  .staff-workspace { display: grid; grid-template-columns: minmax(0, 1fr); align-items: start; gap: 24px; }
  .staff-workspace :global(.staff-form) { font-size: 16px; }
  .staff-workspace :global(.staff-form > label) { font-size: 16px; line-height: 1.5; }
  .staff-workspace :global(.staff-form small), .staff-list-identity span { font-size: 14px; line-height: 1.6; }
  .staff-list li { grid-template-columns: minmax(0, 1fr) auto auto; gap: 12px 16px; padding-block: 12px; }
  .staff-list li > button { justify-self: end; }
  .staff-list-help { margin: 0 0 16px; color: var(--scan-muted); font-size: 14px; line-height: 1.6; overflow-wrap: anywhere; }
  @media (max-width: 767px) { .staff-list-explanation { display: none; } }
  .staff-section-nav { display: flex; flex-wrap: wrap; gap: 12px; margin-bottom: 16px; }
  .staff-section-nav button { display: inline-flex; align-items: center; gap: 8px; white-space: normal; }
  .staff-section-nav svg { flex-shrink: 0; }
  .staff-section-nav [aria-current] { border-color: var(--admin-accent); background: var(--admin-input); }
  .staff-list-identity details span { display: block; margin-top: 4px; }
  .staff-list-identity summary { min-height: 44px; padding-block: 12px; color: var(--scan-text); font-size: 14px; line-height: 1.5; cursor: pointer; }
  .staff-list-identity summary:focus-visible { outline: 3px solid var(--scan-lime); outline-offset: 3px; }
  .staff-list-filters { grid-template-columns: minmax(180px, 1.3fr) minmax(130px, .8fr) minmax(160px, 1fr); align-items: end; gap: 12px; margin: 0 0 8px; }
  .staff-extra-filters { display: contents; }
  .staff-extra-filters label { display: grid; gap: 8px; min-width: 0; color: var(--scan-muted); }
  .staff-extra-filters select { width: 100%; min-height: 46px; padding: 10px 12px; border: 1px solid var(--admin-control-border); border-radius: 0; }
  .staff-extra-filters .staff-checkbox { grid-column: 1 / -1; }
  .staff-filter-toggle { display: none; }
  .staff-filter-results { display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: 8px; min-height: 44px; margin-bottom: 8px; }
  .staff-filter-results p { margin: 0; }
  .staff-role { color: var(--admin-secondary); font-size: 14px; }
  .staff-status { display: inline-block; padding: 4px 8px; border-radius: 2px; color: var(--admin-text); background: var(--admin-neutral-bg); font-size: 14px; line-height: 1.5; cursor: default; }
  .staff-status.is-active { background: var(--admin-active-bg); color: var(--admin-active-text); }
  .staff-edit-panel { margin-top: 0; }
  .staff-edit-grid { grid-template-columns: minmax(0, 1fr); gap: 12px; }
  .staff-create-actions { display: flex; flex-wrap: wrap; gap: 12px; }
  .staff-create-actions > button { flex: 1 1 auto; margin: 0; }
  .staff-create-panel .staff-submit { width: 100%; }
  .staff-panel-heading { flex-wrap: wrap; }
  .staff-panel-heading h2 { overflow-wrap: anywhere; }
  #staff-result:not(:empty) { margin-bottom: 16px; }
  .staff-edit-grid h3 { font-size: 18px; }
  .staff-action-note { margin: 0; color: var(--scan-muted); font-size: 14px; line-height: 1.5; }
  .staff-edit-unsaved { margin: 0 0 16px; padding: 10px 12px; border-left: 3px solid var(--scan-lime); font-size: 14px; }
  .staff-workspace :global(button), .staff-list li { min-height: 44px; }
  .staff-workspace :global(.staff-assignment-list li) { min-height: 52px; overflow-wrap: anywhere; }
  .staff-workspace :global(.staff-assignment-controls) { grid-template-columns: minmax(0, 1fr); }
  .staff-create-panel, .staff-edit-grid { container: staff-form / inline-size; }
  .staff-create-panel .staff-form, .staff-edit-grid .staff-form { margin: 0; gap: 12px; }
  .staff-form-fields { display: grid; gap: 12px; }
  .staff-form-fields label, .staff-edit-grid label { display: grid; gap: 6px; min-width: 0; }
  .staff-create-panel h3 { margin: 0 0 16px; }
  .staff-edit-grid h3 { margin: 0 0 4px; }
  .staff-create-content > .staff-action-note { margin-bottom: 12px; }
  .staff-review { display: grid; gap: 12px; margin: 0; }
  .staff-review > div { display: grid; gap: 6px; }
  .staff-review dt { color: var(--scan-muted); }
  .staff-review dd, .staff-review ul { margin: 0; overflow-wrap: anywhere; }
  .staff-review ul { padding-left: 20px; }
  .staff-workspace [hidden] { display: none; }
  @container staff-form (min-width: 35em) {
    .staff-form-fields { grid-template-columns: repeat(2, minmax(0, 1fr)); }
    .staff-form-fields > label:last-child { grid-column: 1 / -1; }
    .staff-review > div { grid-template-columns: minmax(0, 1fr) minmax(0, 2fr); }
    .staff-workspace :global(.staff-assignment-controls) { grid-template-columns: minmax(0, 1fr) minmax(0, 1fr) auto; }
  }
  @media (max-width: 767px) {
    .staff-create-content { container: staff-form / inline-size; }
    .staff-list li { grid-template-columns: minmax(0, 1fr) auto; gap: 12px; }
    .staff-list li > button { justify-self: end; }
    .staff-filter-toggle { display: inline-flex; justify-content: center; }
    .staff-list-filters { grid-template-columns: minmax(0, 1fr); }
    .staff-extra-filters { display: grid; grid-template-columns: minmax(0, 1fr); gap: 12px; }
    .staff-extra-filters:not(.is-open) { display: none; }
  }
</style>
