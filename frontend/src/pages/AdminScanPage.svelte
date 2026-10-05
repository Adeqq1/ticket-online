<script lang="ts">
  import { ApiError, checkInTicket, getStaffProfile, type CheckInTicket, type Staff } from "../lib/api.ts";
  import { normalizeScanCode } from "../lib/scanner.ts";

  let { accessToken, staff, onUnauthorized, onProfile }: {
    accessToken: string;
    staff: Staff;
    onUnauthorized: () => void;
    onProfile: (staff: Staff) => void;
  } = $props();
  let assignmentKey = $state("");
  let code = $state("");
  let busy = $state(false);
  let status = $state<"idle" | "valid" | "used" | "wrong-gate" | "not-found" | "unpaid" | "denied" | "unknown" | "unavailable">("idle");
  let resultTicket = $state<CheckInTicket | null>(null);
  let expectedGate = $state("");
  let checkedInAt = $state("");
  let resultDetail = $state("");
  let scanCount = $state(0);
  let lastScan = $state("Belum ada scan");
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
  };

  function showFailure(cause: unknown) {
    resultTicket = cause instanceof ApiError ? cause.ticket ?? null : null;
    expectedGate = cause instanceof ApiError ? cause.expectedGate ?? "" : "";
    checkedInAt = cause instanceof ApiError ? cause.checkedInAt ?? "" : "";
    resultDetail = cause instanceof ApiError ? cause.message : "Tidak dapat menghubungi server.";
    if (cause instanceof ApiError && cause.status === 401) { onUnauthorized(); return; }
    if (cause instanceof ApiError && cause.status === 403) status = "denied";
    else if (cause instanceof ApiError && cause.code === "TICKET_ALREADY_USED") status = "used";
    else if (cause instanceof ApiError && cause.code === "WRONG_GATE") status = "wrong-gate";
    else if (cause instanceof ApiError && cause.code === "TICKET_NOT_FOUND") status = "not-found";
    else if (cause instanceof ApiError && cause.code === "ORDER_NOT_PAID") status = "unpaid";
    else if (cause instanceof ApiError && (cause.code === "NETWORK_ERROR" || cause.code === "UNKNOWN_OUTCOME")) status = "unknown";
    else status = "unavailable";
  }

  async function handleScan(event: SubmitEvent) {
    event.preventDefault();
    if (busy || !assignment) return;
    const selected = { ...assignment };
    const normalizedCode = normalizeScanCode(code);
    if (!normalizedCode) return;
    code = normalizedCode;
    busy = true;
    status = "idle";
    resultTicket = null;
    expectedGate = "";
    checkedInAt = "";
    resultDetail = "";
    scanCount += 1;
    lastScan = new Date().toLocaleTimeString("id-ID", { hour: "2-digit", minute: "2-digit" });
    let checkInSent = false;
    try {
      const current = await getStaffProfile(accessToken);
      onProfile(current);
      if (current.role !== "STAFF" || !current.assignments.some((item) => item.eventId === selected.eventId && item.gate === selected.gate)) {
        status = "unavailable";
        resultDetail = "Penugasan ini sudah berubah. Pilih penugasan terbaru sebelum mengirim check-in.";
        return;
      }
      checkInSent = true;
      const result = await checkInTicket(accessToken, { eventId: selected.eventId, gate: selected.gate, code: normalizedCode });
      if (result.status !== "CHECKED_IN") { status = "unknown"; return; }
      status = "valid";
      resultTicket = result.ticket;
      checkedInAt = result.checkedInAt;
    } catch (cause) {
      if (cause instanceof ApiError && cause.status === 401) { showFailure(cause); return; }
      if (cause instanceof ApiError && (cause.code === "NETWORK_ERROR" || cause.code === "UNKNOWN_OUTCOME")) {
        if (checkInSent) showFailure(cause);
        else { status = "unavailable"; resultDetail = "Sesi atau penugasan belum dapat diverifikasi. Check-in tidak dikirim."; }
      } else showFailure(cause);
    } finally { busy = false; }
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
    <div class="scan-live"><span class="scan-live-dot" aria-hidden="true"></span><span class="scan-live-state">Sistem aktif <i>•</i></span><strong>{assignment?.gate ?? "Pilih gate"}</strong></div>
  </header>

  <main id="scan-content" class="scan-content">
    <section class="scan-heading" aria-labelledby="scan-title">
      <div><p class="scan-kicker">OPERASIONAL EVENT <span>•</span> CHECK-IN</p><h1 id="scan-title">Buka pintu.<br /><em>Jaga momen.</em></h1><p class="scan-intro">Verifikasi tiket pengunjung dengan penugasan gate yang terdaftar.</p></div>
      <div class="scan-event-meta"><label class="meta-label" for="staff-assignment">Penugasan aktif</label>
        {#if staff.assignments.length}<select id="staff-assignment" bind:value={assignmentKey} disabled={busy}><option value="">Pilih event dan gate</option>{#each staff.assignments as item (`${item.eventId}:${item.gate}`)}<option value={`${item.eventId}:${item.gate}`}>{item.eventId} · {item.gate}</option>{/each}</select>
        {:else}<strong>Tidak ada penugasan</strong><span>Hubungi administrator untuk mengaktifkan akses gate.</span>{/if}
      </div>
    </section>

    <section class="scanner-grid" aria-label="Area scanner gate">
      <div class="scanner-column">
        <div class="scanner-panel">
          <div class="panel-topline"><span class="panel-index">01</span><h2>Masukkan kode tiket</h2><span class="keyboard-hint">Enter ↵</span></div>
          <form class="scan-form" onsubmit={handleScan} aria-busy={busy}>
            <label for="ticket-code">Kode e-ticket</label>
            <div class="scan-input-wrap"><span aria-hidden="true">⌁</span><input id="ticket-code" bind:value={code} placeholder="Contoh: ET-…" autocomplete="off" spellcheck="false" required disabled={busy || !assignment} aria-describedby="ticket-code-note" /><button class="scan-submit" type="submit" disabled={busy || !assignment}>{busy ? "Memeriksa…" : "Verifikasi"} <span aria-hidden="true">↗</span></button></div>
            <p id="ticket-code-note" class="input-note">Masukkan kode ET- dari e-ticket. Scanner keyboard dapat langsung mengetik kode di sini.</p>
          </form>
          {#if !staff.assignments.length}<p class="staff-muted" role="status">Check-in dinonaktifkan karena akun ini belum memiliki penugasan event dan gate.</p>{/if}
        </div>
        <div class="session-strip"><div><span class="strip-label">Aktivitas check-in sesi ini</span><strong>{String(scanCount).padStart(2, "0")}</strong></div><div><span class="strip-label">Aktivitas terakhir</span><strong>{lastScan}</strong></div></div>
      </div>

      <section class:result-valid={status === "valid"} class:result-denied={status === "used" || status === "not-found" || status === "wrong-gate" || status === "unpaid" || status === "denied"} class="result-panel" aria-live="polite" aria-atomic="true">
        <span class="visually-hidden">Percobaan check-in {scanCount}</span>
        <div class="result-topline"><span class="panel-index">02</span><span class="result-label">HASIL CHECK-IN</span><span class="result-signal" aria-hidden="true"></span></div>
        <div class="result-main">
          {#if status === "idle"}<div class="result-icon idle-icon" aria-hidden="true">⌁</div>{:else if status === "valid"}<div class="result-icon" aria-hidden="true">✓</div>{:else}<div class="result-icon" aria-hidden="true">×</div>{/if}
          <p class="result-status">{copy[status].label}</p><h2>{copy[status].title}</h2><p class="result-detail">{resultDetail || copy[status].detail}</p>
          {#if expectedGate}<p class="result-detail">Gate tiket: <strong>{expectedGate}</strong></p>{/if}
          {#if checkedInAt}<p class="result-detail">Waktu tercatat: <time datetime={checkedInAt}>{new Date(checkedInAt).toLocaleString("id-ID")}</time></p>{/if}
        </div>
        {#if resultTicket}<div class="ticket-detail-card"><div><span>Pengunjung</span><strong>{resultTicket.attendeeName}</strong></div><div><span>Kategori</span><strong>{resultTicket.tierName}</strong></div><div><span>Gate tiket</span><strong>{resultTicket.gate}</strong></div><div><span>Kode</span><strong>{resultTicket.code}</strong></div></div>{:else}<div class="result-placeholder"><span aria-hidden="true">↳</span><p>Detail tiket akan muncul<br />setelah server memverifikasi kode.</p></div>{/if}
        <div class="result-footer"><span><i class="footer-dot"></i> Status server</span><span>{status === "valid" ? "Akses diberikan" : status === "idle" ? "Menunggu input" : status === "unknown" ? "Akses ditahan" : "Akses ditolak"}</span></div>
      </section>
    </section>

    <footer class="scan-footer"><span>Ticket Online · Admin tools</span><span>Penghitung menampilkan aktivitas sesi ini</span></footer>
  </main>
</div>
