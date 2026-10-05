<script lang="ts">
  import { onDestroy, onMount } from "svelte";
  import AdminLoginPage from "./pages/AdminLoginPage.svelte";
  import AdminScanPage from "./pages/AdminScanPage.svelte";
  import AdminStaffPage from "./pages/AdminStaffPage.svelte";
  import { ApiError, getStaffProfile, type Staff } from "./lib/api.ts";
  import { clearStaffSession, readStaffSession, roleMatchesPage, staffHome } from "./lib/staff-session.ts";

  let { page }: { page: "login" | "staff" | "scan" } = $props();
  let staff = $state<Staff | null>(null);
  let checking = $state(true);
  let error = $state("");
  let alive = true;
  let request: AbortController | undefined;
  let expiryTimer: ReturnType<typeof setTimeout> | undefined;

  function login() {
    clearStaffSession();
    location.replace("/admin/login");
  }

  function profileUpdated(current: Staff) {
    staff = current;
    if (page !== "login" && !roleMatchesPage(page, current)) location.replace(staffHome(current.role));
  }

  async function validate() {
    if (request) return;
    const session = readStaffSession();
    if (!session) {
      clearStaffSession(); staff = null; checking = false;
      if (page !== "login") location.replace("/admin/login");
      return;
    }
    request = new AbortController(); checking = true; error = "";
    try {
      const current = await getStaffProfile(session.accessToken, request.signal);
      if (!alive) return;
      if (current.role !== "ADMIN" && current.role !== "STAFF") { clearStaffSession(); location.replace("/admin/login"); return; }
      staff = current;
      if (page === "login" || !roleMatchesPage(page, current)) { location.replace(staffHome(current.role)); return; }
      expiryTimer = setTimeout(login, Math.max(0, Date.parse(session.expiresAt) - Date.now()));
    } catch (cause) {
      if (!alive) return;
      if (cause instanceof ApiError && cause.status === 401) { clearStaffSession(); staff = null; location.replace("/admin/login"); }
      else error = "Sesi belum dapat divalidasi. Periksa koneksi lalu coba lagi.";
    } finally { request = undefined; if (alive) checking = false; }
  }

  function onFocus() {
    if (document.visibilityState === "visible" && staff) void validate();
  }

  onMount(() => { void validate(); });
  onDestroy(() => { alive = false; request?.abort(); clearTimeout(expiryTimer); });
</script>

{#if checking || error}
  <div class="scan-shell staff-gate-state" aria-live="polite">
    <a class="skip-link" href="#staff-gate-status">Lewati ke status sesi</a>
    <header class="scan-topbar"><a class="scan-brand" href="/" aria-label="Kembali ke Tiket Online"><span class="scan-brand-mark" aria-hidden="true">TO</span><span>Tiket Online <b>/ Gate Control</b></span></a></header>
    <main id="staff-gate-status" class="staff-gate-message" aria-busy={checking}>
      {#if checking}<p class="scan-kicker">VALIDASI SESI</p><h1>Memeriksa akses petugas…</h1>
      {:else}<p class="scan-kicker">SESI BELUM TERVERIFIKASI</p><h1>{error}</h1><button class="scan-submit staff-submit" type="button" onclick={validate}>Coba lagi</button>{#if page === "login"}<button class="staff-text-button" type="button" onclick={login}>Hapus sesi lokal</button>{/if}{/if}
    </main>
  </div>
{/if}
{#if staff}
  <div inert={checking || Boolean(error)} aria-hidden={checking || Boolean(error)}>
    {#if page === "staff"}<AdminStaffPage accessToken={readStaffSession()?.accessToken ?? ""} onUnauthorized={login} />
    {:else if page === "scan"}<AdminScanPage accessToken={readStaffSession()?.accessToken ?? ""} {staff} onUnauthorized={login} onProfile={profileUpdated} />{/if}
  </div>
{/if}
{#if !staff && !checking && !error && page === "login"}<AdminLoginPage />{/if}
<svelte:window onfocus={onFocus} />
