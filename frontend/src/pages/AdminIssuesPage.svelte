<script lang="ts">
  import { onMount } from "svelte";
  import { ApiError, addAdminPaymentCaseNote, getAdminEmailJob, getAdminEmailJobs, getAdminPaymentCase, getAdminPaymentCases, recheckAdminPaymentCase, resolveAdminPaymentCase, retryAdminEmailJob, type AdminEmailDetail, type AdminEmailJob, type AdminPaymentCase, type AdminPaymentCaseDetail } from "../lib/api.ts";

  let { accessToken, onUnauthorized, onLogout, loggingOut }: { accessToken: string; onUnauthorized: () => void; onLogout: () => void; loggingOut: boolean } = $props();
  let cases = $state<AdminPaymentCase[]>([]); let emails = $state<AdminEmailJob[]>([]);
  let caseCursor = $state<string | null>(null); let emailCursor = $state<string | null>(null);
  let selectedCase = $state<AdminPaymentCaseDetail | null>(null); let selectedEmail = $state<AdminEmailDetail | null>(null);
  let note = $state(""); let busy = $state(false); let loading = $state(true); let error = $state(""); let message = $state("");
  async function load() {
    loading = true; error = "";
    try { const [caseResult, emailResult] = await Promise.all([getAdminPaymentCases(accessToken), getAdminEmailJobs(accessToken)]); cases = caseResult.items; emails = emailResult.items; caseCursor = caseResult.nextCursor; emailCursor = emailResult.nextCursor; }
    catch (cause) { fail(cause); }
    finally { loading = false; }
  }
  async function more(kind: "cases" | "emails") {
    const cursor = kind === "cases" ? caseCursor : emailCursor; if (!cursor || busy) return; busy = true;
    try { if (kind === "cases") { const result = await getAdminPaymentCases(accessToken, undefined, cursor); cases = [...cases, ...result.items]; caseCursor = result.nextCursor; }
      else { const result = await getAdminEmailJobs(accessToken, undefined, cursor); emails = [...emails, ...result.items]; emailCursor = result.nextCursor; } }
    catch (cause) { fail(cause); } finally { busy = false; }
  }
  function fail(cause: unknown) { if (cause instanceof ApiError && cause.status === 401) onUnauthorized(); else error = cause instanceof ApiError ? cause.message : "Data operasional belum dapat dimuat."; }
  async function openCase(id: string) { selectedEmail = null; message = ""; try { selectedCase = await getAdminPaymentCase(accessToken, id); note = ""; } catch (cause) { fail(cause); } }
  async function openEmail(id: string) { selectedCase = null; message = ""; try { selectedEmail = await getAdminEmailJob(accessToken, id); } catch (cause) { fail(cause); } }
  async function act(action: "note" | "resolve" | "recheck" | "retry") {
    if (busy) return; busy = true; error = ""; message = "";
    try {
      if (action === "note" && selectedCase) await addAdminPaymentCaseNote(accessToken, selectedCase.id, note);
      if (action === "resolve" && selectedCase) await resolveAdminPaymentCase(accessToken, selectedCase.id, note);
      if (action === "recheck" && selectedCase) await recheckAdminPaymentCase(accessToken, selectedCase.id);
      if (action === "retry" && selectedEmail) await retryAdminEmailJob(accessToken, selectedEmail.id);
      message = action === "note" ? "Catatan disimpan." : action === "resolve" ? "Kasus ditutup." : action === "recheck" ? "Status pembayaran diperbarui." : "Job email dijadwalkan kembali.";
      await load(); if (selectedCase) await openCase(selectedCase.id); if (selectedEmail) await openEmail(selectedEmail.id);
    } catch (cause) { fail(cause); } finally { busy = false; }
  }
  function time(value: string | null) { return value ? new Date(value).toLocaleString("id-ID", { timeZone: "Asia/Jakarta", dateStyle: "medium", timeStyle: "short" }) + " WIB" : "Belum pernah"; }
  function money(value: number) { return new Intl.NumberFormat("id-ID", { style: "currency", currency: "IDR", maximumFractionDigits: 0 }).format(value); }
  onMount(() => { void load(); });
</script>

<svelte:head><title>Penanganan masalah | Tiket Online</title><meta name="robots" content="noindex" /></svelte:head>
<div class="scan-shell staff-admin-shell">
  <a class="skip-link" href="#issues-content">Lewati ke penanganan masalah</a>
  <header class="scan-topbar"><a class="scan-brand" href="/" aria-label="Kembali ke Tiket Online"><span class="scan-brand-mark" aria-hidden="true">TO</span><span>Tiket Online <b>/ Gate Control</b></span></a><span class="staff-login-label">ADMINISTRATOR</span><button class="staff-text-button" type="button" disabled={loggingOut} onclick={onLogout}>Keluar</button></header>
  <main id="issues-content" class="staff-admin-content">
    <div class="staff-admin-heading"><div><p class="scan-kicker">OPERASIONAL EVENT <span>•</span> ADMIN</p><h1>Penanganan masalah.</h1><p>Periksa pembayaran terlambat dan email yang gagal dikirim.</p></div><button class="staff-secondary-button" type="button" onclick={load} disabled={loading}>Muat ulang</button></div>
    <nav class="admin-tool-nav" aria-label="Administrasi event"><a href="/admin/events">Konser</a><a href="/admin/orders">Pesanan</a><a aria-current="page" href="/admin/issues">Masalah</a><a href="/admin/operations">Operasional</a><a href="/admin/staff">Kelola petugas</a><a href="/admin/check-ins">Riwayat check-in</a></nav>
    {#if error}<p class="staff-form-message staff-form-error" role="alert">{error}</p>{/if}{#if message}<p class="staff-muted" role="status">{message}</p>{/if}
    {#if loading}<p class="staff-muted" role="status">Memuat antrean…</p>{:else}<div class="order-detail-grid">
      <section class="staff-admin-panel checkin-history-panel"><div class="staff-panel-heading"><div><span class="panel-index">01</span><h2>Rekonsiliasi pembayaran</h2></div></div>
        {#if cases.length}<div class="history-table-wrap"><table class="history-table"><thead><tr><th>Reference</th><th>Event</th><th>Status</th><th>Provider</th><th>Terakhir diperiksa</th><th></th></tr></thead><tbody>{#each cases as item (item.id)}<tr><td><code>{item.reference}</code></td><td>{item.eventName}</td><td>{item.orderStatus} / {item.paymentStatus}</td><td>{item.providerStatus || "—"}</td><td>{time(item.lastCheckedAt)}</td><td><button class="staff-text-button" onclick={() => openCase(item.id)}>Detail</button></td></tr>{/each}</tbody></table></div>{#if caseCursor}<button class="staff-secondary-button" onclick={() => more("cases")} disabled={busy}>Muat kasus berikutnya</button>{/if}{:else}<p class="staff-muted">Tidak ada kasus pembayaran terbuka.</p>{/if}
      </section>
      <section class="staff-admin-panel checkin-history-panel"><div class="staff-panel-heading"><div><span class="panel-index">02</span><h2>Email gagal</h2></div></div>
        {#if emails.length}<div class="history-table-wrap"><table class="history-table"><thead><tr><th>Reference</th><th>Jenis</th><th>Penerima</th><th>Percobaan</th><th>Terakhir diperbarui</th><th></th></tr></thead><tbody>{#each emails as item (item.id)}<tr><td><code>{item.reference}</code></td><td>{item.kind}</td><td>{item.recipient || "Menunggu verifikasi"}</td><td>{item.attempts}</td><td>{time(item.updatedAt)}</td><td><button class="staff-text-button" onclick={() => openEmail(item.id)}>Detail</button></td></tr>{/each}</tbody></table></div>{#if emailCursor}<button class="staff-secondary-button" onclick={() => more("emails")} disabled={busy}>Muat email berikutnya</button>{/if}{:else}<p class="staff-muted">Tidak ada email gagal.</p>{/if}
      </section>
    </div>{/if}
    {#if selectedCase}<section class="staff-admin-panel checkin-history-panel" aria-labelledby="case-detail-title"><div class="staff-panel-heading"><div><span class="panel-index">03</span><h2 id="case-detail-title">Kasus {selectedCase.reference}</h2></div><a class="staff-text-button" href={`/admin/orders?orderId=${encodeURIComponent(selectedCase.orderId)}`}>Lihat pesanan</a></div>
      <dl><dt>Event</dt><dd>{selectedCase.eventName}</dd><dt>Status order / pembayaran</dt><dd>{selectedCase.orderStatus} / {selectedCase.paymentStatus}</dd><dt>Status provider</dt><dd>{selectedCase.providerStatus || "Belum diketahui"}</dd><dt>Alasan</dt><dd>{selectedCase.reason}</dd><dt>Nominal</dt><dd>{money(selectedCase.amount)}</dd><dt>Pemeriksaan terakhir</dt><dd>{time(selectedCase.lastCheckedAt)}{selectedCase.lastCheckError ? ` · ${selectedCase.lastCheckError}` : ""}</dd></dl>
      {#if selectedCase.canRecheck}<button class="scan-submit" type="button" disabled={busy} onclick={() => act("recheck")}>Periksa ulang pembayaran</button>{/if}
      {#if selectedCase.status === "OPEN"}<form onsubmit={(event) => { event.preventDefault(); void act("note"); }}><label>Catatan admin<textarea bind:value={note} maxlength="2000" required></textarea></label><button class="staff-secondary-button" type="submit" disabled={busy}>Simpan catatan</button>{#if selectedCase.canResolve}<button class="scan-submit" type="button" disabled={busy || !note.trim()} onclick={() => act("resolve")}>Tutup kasus dengan catatan ini</button>{/if}</form>{/if}
      <h3>Riwayat</h3><ul class="order-detail-list">{#each selectedCase.history as entry, i (`${entry.action}-${entry.createdAt}-${i}`)}<li><strong>{entry.action} · {entry.actorName}</strong><span>{time(entry.createdAt)} · {JSON.stringify(entry.data)}</span></li>{/each}</ul>
    </section>{/if}
    {#if selectedEmail}<section class="staff-admin-panel checkin-history-panel" aria-labelledby="email-detail-title"><div class="staff-panel-heading"><div><span class="panel-index">04</span><h2 id="email-detail-title">Email {selectedEmail.reference}</h2></div></div>
      <dl><dt>Jenis</dt><dd>{selectedEmail.kind}</dd><dt>Penerima</dt><dd>{selectedEmail.recipient || "Menunggu verifikasi"}</dd><dt>Status order</dt><dd>{selectedEmail.orderStatus || "—"}</dd><dt>Percobaan</dt><dd>{selectedEmail.attempts}</dd><dt>Kesalahan terakhir</dt><dd>{selectedEmail.lastError || "—"}</dd><dt>Status</dt><dd>{selectedEmail.status}</dd></dl>
      {#if selectedEmail.canRetry}<button class="scan-submit" type="button" disabled={busy} onclick={() => act("retry")}>Kirim ulang email</button>{:else}<p class="staff-muted">{selectedEmail.retryReason || "Job tidak dapat dikirim ulang."}</p>{/if}
      <ul class="order-detail-list">{#each selectedEmail.history as entry, i (`${entry.action}-${entry.createdAt}-${i}`)}<li><strong>{entry.action} · {entry.actorName}</strong><span>{time(entry.createdAt)} · {JSON.stringify(entry.data)}</span></li>{/each}</ul>
    </section>{/if}
  </main>
</div>
