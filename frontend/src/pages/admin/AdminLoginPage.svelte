<script lang="ts">
  import { getStaffProfile, loginStaff, logoutStaff, ApiError } from "../../lib/api.ts";
  import { clearStaffSession, readStaffSession, saveStaffSession, staffHome } from "../../lib/staff-session.ts";

  let email = $state("");
  let password = $state("");
  let busy = $state(false);
  let error = $state("");

  async function validateAndEnter(accessToken: string) {
    try {
      const staff = await getStaffProfile(accessToken);
      password = "";
      location.replace(staffHome(staff.role));
    } catch (cause) {
      if (cause instanceof ApiError && cause.status === 401) clearStaffSession();
      error = cause instanceof ApiError && cause.status === 401
        ? "Sesi petugas tidak valid. Silakan login kembali."
        : "Sesi belum dapat divalidasi. Periksa koneksi lalu coba lagi.";
    }
  }

  async function submit(event: SubmitEvent) {
    event.preventDefault();
    if (busy) return;
    busy = true;
    error = "";
    try {
      const saved = readStaffSession();
      if (saved) {
        await validateAndEnter(saved.accessToken);
        return;
      }
      const session = await loginStaff({ email: email.trim(), password });
      if (!saveStaffSession({ accessToken: session.accessToken, expiresAt: session.expiresAt })) {
        await logoutStaff(session.accessToken).catch(() => undefined);
        error = "Sesi tidak dapat disimpan pada tab ini. Aktifkan penyimpanan sesi browser lalu coba kembali.";
        return;
      }
      await validateAndEnter(session.accessToken);
    } catch (cause) {
      error = cause instanceof ApiError ? cause.message : "Tidak dapat menghubungi server. Periksa koneksi lalu coba lagi.";
    } finally { busy = false; }
  }
</script>

<svelte:head>
  <title>Login Petugas | Gate Control</title>
  <meta name="description" content="Login petugas untuk mengakses operasional event." />
  <meta name="robots" content="noindex" />
</svelte:head>

<div class="scan-shell">
  <a class="skip-link" href="#staff-login">Lewati ke form login</a>
  <header class="scan-topbar"><a class="scan-brand" href="/" aria-label="Kembali ke Tiket Online"><span class="scan-brand-mark" aria-hidden="true">TO</span><span>Tiket Online <b>/ Gate Control</b></span></a><span class="staff-login-label">AKSES PETUGAS</span></header>
  <main id="staff-login" class="staff-login-wrap">
    <section class="staff-login-card" aria-labelledby="staff-login-title">
      <p class="scan-kicker">OPERASIONAL EVENT <span>•</span> AREA PETUGAS</p>
      <h1 id="staff-login-title">Masuk ke Gate Control.</h1>
      <p class="staff-login-copy">Gunakan akun yang diberikan administrator untuk melanjutkan.</p>
      <form class="staff-form" onsubmit={submit} aria-busy={busy}>
        <label>Email<input bind:value={email} type="email" name="email" autocomplete="username" maxlength="254" required disabled={busy} /></label>
        <label>Password<input bind:value={password} type="password" name="password" autocomplete="current-password" maxlength="128" required disabled={busy} /></label>
        {#if error}<p class="staff-form-message staff-form-error" role="alert">{error}</p>{/if}
        <button class="scan-submit staff-submit" type="submit" disabled={busy}>{busy ? "Memeriksa sesi…" : "Masuk"}<span aria-hidden="true"> ↗</span></button>
      </form>
      <p class="staff-session-note">Sesi petugas tersimpan hanya pada tab ini dan berakhir setelah delapan jam.</p>
    </section>
  </main>
</div>
