<script lang="ts">
  import { tick } from "svelte";
  import { getEvents } from "../../lib/api.ts";
  import type { Staff } from "../../lib/api.ts";
  import { checkTicketStatus, continueToNextScan, submitScan, type ScannerState } from "../../lib/scanner.ts";
  import CameraScanner from "../../components/admin/CameraScanner.svelte";

  let { accessToken, staff, sessionReady, onUnauthorized, onProfile, onLogout, loggingOut }: {
    accessToken: string;
    staff: Staff;
    sessionReady: boolean;
    onUnauthorized: () => void;
    onProfile: (staff: Staff) => void;
    onLogout: () => void;
    loggingOut: boolean;
  } = $props();
  let assignmentKey = $state("");
  let eventNames = $state<Record<string, string>>({});
  let code = $state("");
  let codeInput: HTMLInputElement | undefined;
  let scan = $state<ScannerState>({ busy: false, status: "idle", resultTicket: null, expectedGate: "", checkedInAt: "", resultDetail: "", scanCount: 0, lastScan: "Belum ada scan", locked: false, lastAttempt: null, statusChecking: false, ticketStatus: null, statusCheckError: "" });
  let scanSource = $state<"camera" | "manual" | null>(null);
  let unknownAcknowledged = $state(false);
  let assignment = $derived(staff.assignments.find((item) => `${item.eventId}:${item.gate}` === assignmentKey));

  $effect(() => {
    if (assignmentKey && !staff.assignments.some((item) => `${item.eventId}:${item.gate}` === assignmentKey)) assignmentKey = "";
  });

  $effect(() => { if (sessionReady) void getEvents().then((events) => { eventNames = Object.fromEntries(events.map((event) => [event.id, event.artist])); }).catch(() => {}); });

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

  async function scanCode(value: string, source: "camera" | "manual") {
    if (scan.busy || scan.locked || !assignment) return;
    scanSource = source;
    unknownAcknowledged = false;
    code = value;
    await submitScan(scan, accessToken, assignment, value, onProfile, onUnauthorized);
    if (scan.status !== "unknown") {
      code = "";
      await tick();
      codeInput?.focus();
    }
  }

  async function handleScan(event: SubmitEvent) {
    event.preventDefault();
    await scanCode(code, "manual");
  }

  function handleNextScan() {
    if (!continueToNextScan(scan, unknownAcknowledged)) return false;
    unknownAcknowledged = false;
    scanSource = null;
    code = "";
    void tick().then(() => codeInput?.focus());
    return true;
  }

  async function handleStatusCheck() {
    unknownAcknowledged = false;
    await checkTicketStatus(scan, accessToken, onUnauthorized);
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
    <div class="scan-live"><span class="scan-live-dot" aria-hidden="true"></span><span class="scan-live-state">{sessionReady ? "Sesi aktif" : "Memeriksa sesi"}</span><span class="scan-staff-name">{staff.name}</span><span class="scan-assignment-name">{assignment ? `${eventNames[assignment.eventId] ?? assignment.eventId} · ${assignment.gate}` : "Pilih event dan gate"}</span><button class="staff-text-button" type="button" disabled={scan.busy || scan.statusChecking || loggingOut} onclick={onLogout}>Keluar</button></div>
  </header>

  <main id="scan-content" class="scan-content">
    <section class="scan-heading" aria-labelledby="scan-title">
      <div><p class="scan-kicker">OPERASIONAL EVENT <span>•</span> CHECK-IN</p><h1 id="scan-title">Buka pintu.<br /><em>Jaga momen.</em></h1><p class="scan-intro">Verifikasi tiket pengunjung dengan penugasan gate yang terdaftar.</p></div>
        <div class="scan-event-meta"><label class="meta-label" for="staff-assignment">Penugasan aktif</label>
        {#if staff.assignments.length}<select id="staff-assignment" bind:value={assignmentKey} disabled={scan.busy || scan.locked || scan.statusChecking}><option value="">Pilih event dan gate</option>{#each staff.assignments as item (`${item.eventId}:${item.gate}`)}<option value={`${item.eventId}:${item.gate}`}>{eventNames[item.eventId] ?? item.eventId} · {item.gate}</option>{/each}</select>
        {:else}<strong>Tidak ada penugasan</strong><span>Hubungi administrator untuk mengaktifkan akses gate.</span>{/if}
      </div>
    </section>

    <section class="scanner-grid" aria-label="Area scanner gate">
      <div class="scanner-column">
        <div class="scanner-panel">
          <div class="panel-topline"><span class="panel-index">01</span><h2>Masukkan kode tiket</h2><span class="keyboard-hint">Enter ↵</span></div>
          <form class="scan-form" onsubmit={handleScan} aria-busy={scan.busy}>
            <label for="ticket-code">Kode e-ticket</label>
            <div class="scan-input-wrap"><span aria-hidden="true">⌁</span><input id="ticket-code" bind:this={codeInput} bind:value={code} placeholder="Contoh: ET-…" autocomplete="off" spellcheck="false" required disabled={scan.busy || scan.locked || !assignment} aria-describedby="ticket-code-note" /><button class="scan-submit" type="submit" disabled={scan.busy || scan.locked || !assignment}>{scan.busy ? "Memeriksa…" : "Verifikasi"} <span aria-hidden="true">↗</span></button></div>
            <p id="ticket-code-note" class="input-note">Masukkan kode ET- dari e-ticket. Scanner keyboard dapat langsung mengetik kode di sini.</p>
          </form>
          <CameraScanner enabled={Boolean(assignment) && sessionReady} busy={scan.busy || scan.statusChecking || loggingOut} locked={scan.locked} resumeCamera={scanSource === "camera"} canContinue={!scan.busy && !scan.statusChecking && (scan.status !== "unknown" || scan.ticketStatus?.status === "CHECKED_IN" || unknownAcknowledged)} onNext={handleNextScan} onCode={(value) => scanCode(value, "camera")} />
          {#if scan.locked && scanSource === "manual"}<button class="scan-submit next-scan" type="button" disabled={scan.busy || scan.statusChecking || (scan.status === "unknown" && scan.ticketStatus?.status !== "CHECKED_IN" && !unknownAcknowledged)} onclick={handleNextScan}>Scan berikutnya</button>{/if}
          {#if scan.status === "unknown"}<section class="scan-status-check" aria-labelledby="status-check-title"><h3 id="status-check-title">Hasil check-in belum diketahui</h3><p>Gate tetap ditahan sampai status tiket diperiksa.</p><button class="staff-secondary-button" type="button" disabled={scan.statusChecking || scan.busy} onclick={handleStatusCheck}>{scan.statusChecking ? "Memeriksa status…" : "Periksa status"}</button>{#if scan.ticketStatus}<p role="status">{scan.ticketStatus.status === "CHECKED_IN" ? "Check-in tiket sudah tercatat." : `Belum tercatat saat diperiksa. Status order: ${scan.ticketStatus.orderStatus}. Ini belum memastikan permintaan sebelumnya gagal.`}</p>{#if scan.ticketStatus.status === "CHECKED_IN" && scan.ticketStatus.checkedInAt}<p>Waktu check-in: <time datetime={scan.ticketStatus.checkedInAt}>{new Date(scan.ticketStatus.checkedInAt).toLocaleString("id-ID")}</time></p>{/if}<p>Kode tiket: <strong>{scan.ticketStatus.ticket.code}</strong></p>{/if}{#if scan.statusCheckError}<p role="alert">{scan.statusCheckError} Gate tetap ditahan.</p>{/if}{#if scan.ticketStatus?.status !== "CHECKED_IN"}<button class="staff-text-button" type="button" disabled={scan.statusChecking || unknownAcknowledged} onclick={() => unknownAcknowledged = true}>{unknownAcknowledged ? "Penanganan dikonfirmasi" : "Konfirmasi penanganan"}</button>{/if}</section>{/if}
          {#if !staff.assignments.length}<p class="staff-muted" role="status">Check-in dinonaktifkan karena akun ini belum memiliki penugasan event dan gate.</p>{/if}
        </div>
        <div class="session-strip"><div><span class="strip-label">Aktivitas check-in sesi ini</span><strong>{String(scan.scanCount).padStart(2, "0")}</strong></div><div><span class="strip-label">Aktivitas terakhir</span><strong>{scan.lastScan}</strong></div></div>
      </div>

      <section class:result-valid={scan.status === "valid"} class:result-denied={scan.status === "used" || scan.status === "not-found" || scan.status === "wrong-gate" || scan.status === "unpaid" || scan.status === "denied"} class="result-panel" aria-live="polite" aria-atomic="true">
        <span class="visually-hidden">Percobaan check-in {scan.scanCount}</span>
        <div class="result-topline"><span class="panel-index">02</span><span class="result-label">HASIL CHECK-IN</span><span class="result-signal" aria-hidden="true"></span></div>
        <div class="result-main">
          {#if scan.status === "idle"}<div class="result-icon idle-icon" aria-hidden="true">⌁</div>{:else if scan.status === "valid"}<div class="result-icon" aria-hidden="true">✓</div>{:else}<div class="result-icon" aria-hidden="true">×</div>{/if}
          <p class="result-status">{copy[scan.status].label}</p><h2>{copy[scan.status].title}</h2><p class="result-detail">{scan.status === "unknown" ? copy.unknown.detail : scan.resultDetail || copy[scan.status].detail}</p>
          {#if scan.status === "unknown" && scan.resultDetail}<p class="result-detail">{scan.resultDetail}</p>{/if}
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
