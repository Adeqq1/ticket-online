<script lang="ts">
  import { onDestroy, onMount } from "svelte";
  import AdminLoginPage from "./pages/AdminLoginPage.svelte";
  import AdminScanPage from "./pages/AdminScanPage.svelte";
  import AdminStaffPage from "./pages/AdminStaffPage.svelte";
  import { ApiError, getStaffProfile, type Staff } from "./lib/api.ts";
  import { clearStaffSession, logoutStaffSession, readStaffSession, roleMatchesPage, staffHome } from "./lib/staff-session.ts";

  let { page }: { page: "login" | "staff" | "scan" } = $props();
  let staff = $state<Staff | null>(null);
  let checking = $state(true);
  let error = $state("");
  let logoutPending = $state(false);
  let logoutOutcome = $state<{ localCleared: boolean; remote: "revoked" | "invalid" | "unconfirmed" } | null>(null);
  let alive = true;
  let sessionGeneration = 0;
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

  async function logout() {
    if (logoutPending) return;
    const session = readStaffSession();
    logoutPending = true;
    logoutOutcome = null;
    clearTimeout(expiryTimer);
    sessionGeneration += 1;
    request?.abort();
    request = undefined;
    staff = null;
    checking = false;
    error = "";
    if (!session) { clearStaffSession(); location.replace("/admin/login"); logoutPending = false; return; }
    try {
      const outcome = await logoutStaffSession(session.accessToken);
      if (outcome.localCleared && outcome.remote !== "unconfirmed") location.replace("/admin/login");
      else logoutOutcome = outcome;
    } finally { logoutPending = false; }
  }

  async function validate() {
    if (request) return;
    const session = readStaffSession();
    if (!session) {
      clearStaffSession(); staff = null; checking = false;
      if (page !== "login") location.replace("/admin/login");
      return;
    }
    const generation = sessionGeneration;
    const controller = new AbortController();
    request = controller; checking = true; error = "";
    try {
      const current = await getStaffProfile(session.accessToken, controller.signal);
      if (!alive || generation !== sessionGeneration) return;
      if (current.role !== "ADMIN" && current.role !== "STAFF") { clearStaffSession(); location.replace("/admin/login"); return; }
      staff = current;
      if (page === "login" || !roleMatchesPage(page, current)) { location.replace(staffHome(current.role)); return; }
      expiryTimer = setTimeout(login, Math.max(0, Date.parse(session.expiresAt) - Date.now()));
    } catch (cause) {
      if (!alive || generation !== sessionGeneration) return;
      if (cause instanceof ApiError && cause.status === 401) { clearStaffSession(); staff = null; location.replace("/admin/login"); }
      else error = "Sesi belum dapat divalidasi. Periksa koneksi lalu coba lagi.";
    } finally { if (request === controller) request = undefined; if (alive && generation === sessionGeneration) checking = false; }
  }

  function onFocus() {
    if (document.visibilityState === "visible" && staff) void validate();
  }

  onMount(() => { void validate(); });
  onDestroy(() => { alive = false; request?.abort(); clearTimeout(expiryTimer); });
</script>

{#if logoutOutcome}
  <div class="scan-shell staff-gate-state">
    <a class="skip-link" href="#logout-status">Lewati ke status logout</a>
    <header class="scan-topbar"><a class="scan-brand" href="/" aria-label="Kembali ke Tiket Online"><span class="scan-brand-mark" aria-hidden="true">TO</span><span>Tiket Online <b>/ Gate Control</b></span></a></header>
    <main id="logout-status" class="staff-gate-message" role="alert">
      <p class="scan-kicker">LOGOUT</p><h1>{logoutOutcome.localCleared ? "Sesi di tab ini telah dihapus." : "Token lokal belum dapat dihapus."}</h1>
      <p class="staff-login-copy">{#if !logoutOutcome.localCleared}Tutup tab ini. {/if}{#if logoutOutcome.remote === "revoked"}Sesi server sudah dicabut.{:else if logoutOutcome.remote === "invalid"}Sesi server sudah tidak valid.{:else}Pencabutan sesi di server belum dapat dipastikan.{/if} Gunakan login untuk masuk kembali setelah koneksi pulih.</p>
      <a class="staff-secondary-button" href="/admin/login">Kembali ke login</a>
    </main>
  </div>
{:else if logoutPending}
  <div class="scan-shell staff-gate-state" aria-live="polite"><main class="staff-gate-message" aria-busy="true"><p class="scan-kicker">LOGOUT</p><h1>Mengakhiri sesi petugas…</h1></main></div>
{:else if checking || error}
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
    {#if page === "staff"}<AdminStaffPage accessToken={readStaffSession()?.accessToken ?? ""} onUnauthorized={login} onLogout={logout} loggingOut={logoutPending} />
    {:else if page === "scan"}<AdminScanPage accessToken={readStaffSession()?.accessToken ?? ""} {staff} sessionReady={Boolean(staff) && !checking && !error && !logoutPending} onUnauthorized={login} onProfile={profileUpdated} onLogout={logout} loggingOut={logoutPending} />{/if}
  </div>
{/if}
{#if !staff && !checking && !error && page === "login"}<AdminLoginPage />{/if}
<svelte:window onfocus={onFocus} />
