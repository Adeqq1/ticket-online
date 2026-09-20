<script lang="ts">
  import { demoTickets, scanTicket, type ScanStatus, type ScanTicket } from "../lib/scanner.ts";

  let tickets = $state<ScanTicket[]>(demoTickets.map((ticket) => ({ ...ticket })));
  let code = $state("");
  let status = $state<ScanStatus>("idle");
  let resultTicket = $state<ScanTicket | null>(null);
  let scanCount = $state(0);
  let lastScan = $state("Belum ada scan");

  const statusCopy = {
    idle: { label: "Siap menerima tiket", title: "Scan tiket untuk membuka gate", detail: "Masukkan kode tiket atau pilih salah satu ID demo di bawah." },
    valid: { label: "Valid", title: "Gate Masuk Terbuka", detail: "Tiket terverifikasi. Persilakan pengunjung masuk ke area event." },
    used: { label: "Ditolak", title: "Tiket Sudah Digunakan", detail: "Tiket ini sudah tercatat masuk dan tidak bisa digunakan kembali." },
    "not-found": { label: "Ditolak", title: "Kode Tidak Ditemukan", detail: "Periksa kembali kode tiket atau minta pengunjung menunjukkan e-ticket yang benar." },
  };

  function handleScan(value = code) {
    const trimmed = value.trim();
    if (!trimmed) {
      status = "not-found";
      resultTicket = null;
      lastScan = "Input kosong";
      return;
    }
    const result = scanTicket(tickets, trimmed);
    code = trimmed.toUpperCase();
    status = result.status;
    resultTicket = result.ticket;
    scanCount += 1;
    lastScan = new Date().toLocaleTimeString("id-ID", { hour: "2-digit", minute: "2-digit" });
  }

  function resetDemo() {
    tickets = demoTickets.map((ticket) => ({ ...ticket }));
    code = "";
    status = "idle";
    resultTicket = null;
    scanCount = 0;
    lastScan = "Belum ada scan";
  }
</script>

<svelte:head>
  <title>Gate Control | Scanner Tiket</title>
  <meta name="description" content="Simulasi scanner ticket check-in untuk tim gate event." />
</svelte:head>

<div class="scan-shell">
  <header class="scan-topbar">
    <a class="scan-brand" href="/" aria-label="Kembali ke Tiket Online"><span class="scan-brand-mark" aria-hidden="true">TO</span><span>Tiket Online <b>/ Gate Control</b></span></a>
    <div class="scan-live"><span aria-hidden="true"></span> Sistem aktif <i>•</i> Gate A</div>
  </header>

  <main class="scan-content">
    <section class="scan-heading" aria-labelledby="scan-title">
      <div>
        <p class="scan-kicker">OPERASIONAL EVENT <span>•</span> CHECK-IN DEMO</p>
        <h1 id="scan-title">Buka pintu.<br /><em>Jaga momen.</em></h1>
        <p class="scan-intro">Verifikasi tiket pengunjung dengan cepat sebelum mereka masuk ke venue.</p>
      </div>
      <div class="scan-event-meta"><span class="meta-label">Event yang aktif</span><strong>Nusa Malam</strong><span>24 Agustus 2027 · Jakarta</span></div>
    </section>

    <section class="scanner-grid" aria-label="Area scanner gate">
      <div class="scanner-column">
        <div class="scanner-panel">
          <div class="panel-topline"><span class="panel-index">01</span><h2>Masukkan kode tiket</h2><span class="keyboard-hint">Enter ↵</span></div>
          <form class="scan-form" onsubmit={(event) => { event.preventDefault(); handleScan(); }}>
            <label for="ticket-code">Kode reference tiket</label>
            <div class="scan-input-wrap"><span aria-hidden="true">⌁</span><input id="ticket-code" bind:value={code} placeholder="Contoh: TO-ALPHA-2027" autocomplete="off" spellcheck="false" /><button class="scan-submit" type="submit">Verifikasi <span aria-hidden="true">↗</span></button></div>
            <p class="input-note">Kode demo tidak case-sensitive. Pastikan kode sesuai dengan e-ticket pengunjung.</p>
          </form>
          <div class="preset-block"><div class="preset-heading"><span>Atau coba tiket demo</span><span>{tickets.filter((ticket) => !ticket.isUsed).length} tersedia</span></div><div class="preset-list">{#each tickets as ticket}<button class:already-used={ticket.isUsed} type="button" onclick={() => { code = ticket.id; handleScan(ticket.id); }}><span class="preset-status" aria-hidden="true"></span><span><b>{ticket.id}</b><small>{ticket.attendee} · {ticket.ticketType}</small></span><span class="preset-arrow" aria-hidden="true">↗</span></button>{/each}</div></div>
        </div>
        <div class="session-strip"><div><span class="strip-label">Scan sesi ini</span><strong>{String(scanCount).padStart(2, "0")}</strong></div><div><span class="strip-label">Scan terakhir</span><strong>{lastScan}</strong></div><button type="button" onclick={resetDemo}>Reset demo</button></div>
      </div>

      <section class:result-valid={status === "valid"} class:result-denied={status === "used" || status === "not-found"} class="result-panel" aria-live="polite" aria-atomic="true">
        <div class="result-topline"><span class="panel-index">02</span><span class="result-label">HASIL VERIFIKASI</span><span class="result-signal" aria-hidden="true"></span></div>
        <div class="result-main">
          {#if status === "idle"}<div class="result-icon idle-icon" aria-hidden="true">⌁</div>
          {:else if status === "valid"}<div class="result-icon" aria-hidden="true">✓</div>
          {:else}<div class="result-icon" aria-hidden="true">×</div>{/if}
          <p class="result-status">{statusCopy[status].label}</p>
          <h2>{statusCopy[status].title}</h2>
          <p class="result-detail">{statusCopy[status].detail}</p>
        </div>
        {#if resultTicket}<div class="ticket-detail-card"><div><span>Pengunjung</span><strong>{resultTicket.attendee}</strong></div><div><span>Kategori</span><strong>{resultTicket.ticketType}</strong></div><div><span>Gate</span><strong>{resultTicket.gate}</strong></div><div><span>Reference</span><strong>{resultTicket.id}</strong></div></div>{:else}<div class="result-placeholder"><span aria-hidden="true">↳</span><p>Detail tiket akan muncul<br />setelah kode diverifikasi.</p></div>{/if}
        <div class="result-footer"><span><i class="footer-dot"></i> Local demo state</span><span>{status === "valid" ? "Akses diberikan" : status === "idle" ? "Menunggu input" : "Akses ditahan"}</span></div>
      </section>
    </section>

    <footer class="scan-footer"><span>Ticket Online · Admin tools</span><span>Simulasi frontend · Tidak terhubung ke sistem produksi</span></footer>
  </main>
</div>
