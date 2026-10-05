<script lang="ts">
  import type { Staff } from "../lib/api.ts";
  import { submitScan, type ScannerState } from "../lib/scanner.ts";

  let { accessToken, staff, onUnauthorized, onProfile, onLogout, loggingOut }: {
    accessToken: string;
    staff: Staff;
    onUnauthorized: () => void;
    onProfile: (staff: Staff) => void;
    onLogout: () => void;
    loggingOut: boolean;
  } = $props();
  let assignmentKey = $state("");
  let code = $state("");
  let scan = $state<ScannerState>({ busy: false, status: "idle", resultTicket: null, expectedGate: "", checkedInAt: "", resultDetail: "", scanCount: 0, lastScan: "Belum ada scan" });
  let assignment = $derived(staff.assignments.find((item) => `${item.eventId}:${item.gate}` === assignmentKey));

  $effect(() => {
    if (assignmentKey && !staff.assignments.some((item) => `${item.eventId}:${item.gate}` === assignmentKey)) assignmentKey = "";
  });

  const copy = {
    idle: { label: "Siap menerima tiket", title: "Scan tiket untuk membuka gate", detail: "Masukkan kode ET- pada e-ticket pengunjung." },
    valid: { label: "Berhasil", title: "Gate Masuk Terbuka", detail: "Tiket terverifikasi oleh server. Persilakan pengunjung masuk." },
    used: { label: "Ditolak", title: "Tiket Sudah Digunakan", detail: "Tiket ini sudah tercatat masuk dan tidak dapat digunakan kembali." },
    "wrong-gate": { label: "Gate salah", title: "Arahkan ke gate yang benar", detail: "Tiket ini tidak berlaku di gate yang dipilih." },
    "not-found": { label: "Ditolak", title: "Tiket Tidak Ditemukan", detail: "Kode tidak cocok dengan tiket pada event yang dipilih." },
    unpaid: { label: "Ditolak", title: "Order Belum Dibayar", detail: "Tiket hanya dapat digunakan setelah pembayaran terkonfirmasi." },
    denied: { label: "Akses ditolak", title: "Check-in Tidak Diizinkan", detail: "Sesi atau penugasan petugas tidak mengizinkan operasi ini." },
    unknown: { label: "Hasil belum diketahui", title: "Periksa status sebelum melanjutkan", detail: "Respons terputus. Gate tetap ditahan; periksa tiket atau minta bantuan admin sebelum mencoba lagi." },
    unavailable: { label: "Belum diverifikasi", title: "Check-in Tidak Dikirim", detail: "Koneksi atau penugasan belum dapat diverifikasi. Tidak ada check-in yang dikirim." },
    error: { label: "Check-in gagal", title: "Server menolak check-in", detail: "Periksa pesan server sebelum melanjutkan." },
  };

  async function handleScan(event: SubmitEvent) {
    event.preventDefault();
    if (scan.busy || !assignment) return;
    await submitScan(scan, accessToken, assignment, code, onProfile, onUnauthorized);
  }
</script>

<svelte:head>
  <title>Gate Control | Scanner Tiket</title>
  <meta name="description" content="Check-in tiket berdasarkan penugasan petugas." />
  <meta name="robots" content="noindex" />
</svelte:head>

<div class="scan-shell">
  <a class="skip-link" href="#scan-content">Lewati ke scanner</a>
  <header class="scan-topbar">
    <a class="scan-brand" href="/" aria-label="Kembali ke Tiket Online"><span class="scan-brand-mark" aria-hidden="true">TO</span><span>Tiket Online <b>/ Gate Control</b></span></a>
    <div class="scan-live"><span class="scan-live-dot" aria-hidden="true"></span><span class="scan-live-state">Sistem aktif <i>•</i></span><strong>{assignment?.gate ?? "Pilih gate"}</strong><button class="staff-text-button" type="button" disabled={scan.busy || loggingOut} onclick={onLogout}>Keluar</button></div>
  </header>

  <main id="scan-content" class="scan-content">
    <section class="scan-heading" aria-labelledby="scan-title">
      <div><p class="scan-kicker">OPERASIONAL EVENT <span>•</span> CHECK-IN</p><h1 id="scan-title">Buka pintu.<br /><em>Jaga momen.</em></h1><p class="scan-intro">Verifikasi tiket pengunjung dengan penugasan gate yang terdaftar.</p></div>
        <div class="scan-event-meta"><label class="meta-label" for="staff-assignment">Penugasan aktif</label>
        {#if staff.assignments.length}<select id="staff-assignment" bind:value={assignmentKey} disabled={scan.busy}><option value="">Pilih event dan gate</option>{#each staff.assignments as item (`${item.eventId}:${item.gate}`)}<option value={`${item.eventId}:${item.gate}`}>{item.eventId} · {item.gate}</option>{/each}</select>
        {:else}<strong>Tidak ada penugasan</strong><span>Hubungi administrator untuk mengaktifkan akses gate.</span>{/if}
      </div>
    </section>

    <section class="scanner-grid" aria-label="Area scanner gate">
      <div class="scanner-column">
        <div class="scanner-panel">
          <div class="panel-topline"><span class="panel-index">01</span><h2>Masukkan kode tiket</h2><span class="keyboard-hint">Enter ↵</span></div>
          <form class="scan-form" onsubmit={handleScan} aria-busy={scan.busy}>
            <label for="ticket-code">Kode e-ticket</label>
            <div class="scan-input-wrap"><span aria-hidden="true">⌁</span><input id="ticket-code" bind:value={code} placeholder="Contoh: ET-…" autocomplete="off" spellcheck="false" required disabled={scan.busy || !assignment} aria-describedby="ticket-code-note" /><button class="scan-submit" type="submit" disabled={scan.busy || !assignment}>{scan.busy ? "Memeriksa…" : "Verifikasi"} <span aria-hidden="true">↗</span></button></div>
            <p id="ticket-code-note" class="input-note">Masukkan kode ET- dari e-ticket. Scanner keyboard dapat langsung mengetik kode di sini.</p>
          </form>
          {#if !staff.assignments.length}<p class="staff-muted" role="status">Check-in dinonaktifkan karena akun ini belum memiliki penugasan event dan gate.</p>{/if}
        </div>
        <div class="session-strip"><div><span class="strip-label">Aktivitas check-in sesi ini</span><strong>{String(scan.scanCount).padStart(2, "0")}</strong></div><div><span class="strip-label">Aktivitas terakhir</span><strong>{scan.lastScan}</strong></div></div>
      </div>

      <section class:result-valid={scan.status === "valid"} class:result-denied={scan.status === "used" || scan.status === "not-found" || scan.status === "wrong-gate" || scan.status === "unpaid" || scan.status === "denied"} class="result-panel" aria-live="polite" aria-atomic="true">
        <span class="visually-hidden">Percobaan check-in {scan.scanCount}</span>
        <div class="result-topline"><span class="panel-index">02</span><span class="result-label">HASIL CHECK-IN</span><span class="result-signal" aria-hidden="true"></span></div>
        <div class="result-main">
          {#if scan.status === "idle"}<div class="result-icon idle-icon" aria-hidden="true">⌁</div>{:else if scan.status === "valid"}<div class="result-icon" aria-hidden="true">✓</div>{:else}<div class="result-icon" aria-hidden="true">×</div>{/if}
          <p class="result-status">{copy[scan.status].label}</p><h2>{copy[scan.status].title}</h2><p class="result-detail">{scan.resultDetail || copy[scan.status].detail}</p>
          {#if scan.expectedGate}<p class="result-detail">Gate tiket: <strong>{scan.expectedGate}</strong></p>{/if}
          {#if scan.checkedInAt}<p class="result-detail">Waktu tercatat: <time datetime={scan.checkedInAt}>{new Date(scan.checkedInAt).toLocaleString("id-ID")}</time></p>{/if}
        </div>
        {#if scan.resultTicket}<div class="ticket-detail-card"><div><span>Pengunjung</span><strong>{scan.resultTicket.attendeeName}</strong></div><div><span>Kategori</span><strong>{scan.resultTicket.tierName}</strong></div><div><span>Gate tiket</span><strong>{scan.resultTicket.gate}</strong></div><div><span>Kode</span><strong>{scan.resultTicket.code}</strong></div></div>{:else}<div class="result-placeholder"><span aria-hidden="true">↳</span><p>Detail tiket akan muncul<br />setelah server memverifikasi kode.</p></div>{/if}
        <div class="result-footer"><span><i class="footer-dot"></i> Status server</span><span>{scan.status === "valid" ? "Akses diberikan" : scan.status === "idle" ? "Menunggu input" : "Akses ditahan"}</span></div>
      </section>
    </section>

    <footer class="scan-footer"><span>Ticket Online · Admin tools</span><span>Penghitung menampilkan aktivitas sesi ini</span></footer>
  </main>
</div>
