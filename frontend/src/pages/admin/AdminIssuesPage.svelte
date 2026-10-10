<script lang="ts">
  import AdminLayout from "../../layouts/AdminLayout.svelte";
  import { onMount } from "svelte";
  import { ApiError, addAdminPaymentCaseNote, getAdminEmailJob, getAdminEmailJobs, getAdminPaymentCase, getAdminPaymentCases, recheckAdminPaymentCase, resolveAdminPaymentCase, retryAdminEmailJob, type AdminEmailDetail, type AdminEmailJob, type AdminPaymentCase, type AdminPaymentCaseDetail } from "../../lib/api.ts";
  import { createRequestVersion } from "../../lib/request-version.ts";

  let { accessToken, onUnauthorized, onLogout, loggingOut }: { accessToken: string; onUnauthorized: () => void; onLogout: () => void; loggingOut: boolean } = $props();
  let cases = $state<AdminPaymentCase[]>([]); let emails = $state<AdminEmailJob[]>([]);
  let caseCursor = $state<string | null>(null); let emailCursor = $state<string | null>(null);
  let selectedCase = $state<AdminPaymentCaseDetail | null>(null); let selectedEmail = $state<AdminEmailDetail | null>(null);
  let note = $state(""); let busy = $state(false); let loading = $state(true); let detailLoading = $state(false); const detailRequests = createRequestVersion(); let error = $state(""); let message = $state("");
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
  async function loadCaseDetail(id: string, generation: number) {
    try { const result = await getAdminPaymentCase(accessToken, id); if (detailRequests.isCurrent(generation)) selectedCase = result; }
    catch (cause) { if (detailRequests.isCurrent(generation)) fail(cause); }
    finally { if (detailRequests.isCurrent(generation)) detailLoading = false; }
  }
  async function loadEmailDetail(id: string, generation: number) {
    try { const result = await getAdminEmailJob(accessToken, id); if (detailRequests.isCurrent(generation)) selectedEmail = result; }
    catch (cause) { if (detailRequests.isCurrent(generation)) fail(cause); }
    finally { if (detailRequests.isCurrent(generation)) detailLoading = false; }
  }
  async function openCase(id: string) {
    if (busy) return;
    const generation = detailRequests.next();
    selectedCase = null; selectedEmail = null; note = ""; message = ""; error = ""; detailLoading = true;
    await loadCaseDetail(id, generation);
  }
  async function openEmail(id: string) {
    if (busy) return;
    const generation = detailRequests.next();
    selectedCase = null; selectedEmail = null; note = ""; message = ""; error = ""; detailLoading = true;
    await loadEmailDetail(id, generation);
  }
  async function act(action: "note" | "resolve" | "recheck" | "retry") {
    if (busy || detailLoading) return;
    const caseId = selectedCase?.id; const emailId = selectedEmail?.id; const noteText = note; const generation = detailRequests.value();
    if ((action === "retry" && !emailId) || (action !== "retry" && !caseId)) return;
    busy = true; error = ""; message = "";
    try {
      if (action === "note" && caseId) await addAdminPaymentCaseNote(accessToken, caseId, noteText);
      if (action === "resolve" && caseId) await resolveAdminPaymentCase(accessToken, caseId, noteText);
      if (action === "recheck" && caseId) await recheckAdminPaymentCase(accessToken, caseId);
      if (action === "retry" && emailId) await retryAdminEmailJob(accessToken, emailId);
      message = action === "note" ? "Catatan disimpan." : action === "resolve" ? "Kasus ditutup." : action === "recheck" ? "Status pembayaran diperbarui." : "Job email dijadwalkan kembali.";
      await load();
      if (detailRequests.isCurrent(generation) && caseId) await loadCaseDetail(caseId, generation);
      if (detailRequests.isCurrent(generation) && emailId) await loadEmailDetail(emailId, generation);
    } catch (cause) { fail(cause); } finally { busy = false; }
  }
  function time(value: string | null) { return value ? new Date(value).toLocaleString("id-ID", { timeZone: "Asia/Jakarta", dateStyle: "medium", timeStyle: "short" }) + " WIB" : "Belum pernah"; }
  function money(value: number) { return new Intl.NumberFormat("id-ID", { style: "currency", currency: "IDR", maximumFractionDigits: 0 }).format(value); }
  onMount(() => { void load(); });
</script>

<svelte:head><title>Penanganan masalah | Tiket Online</title><meta name="robots" content="noindex" /></svelte:head>
  <AdminLayout page="issues" contentId="issues-content" skipLabel="Lewati ke penanganan masalah" kicker="OPERASIONAL EVENT • ADMIN" title="Penanganan masalah." description="Periksa pembayaran terlambat dan email yang gagal dikirim." onLogout={onLogout} loggingOut={loggingOut} primaryAction={{ label: "Muat ulang", onclick: load, disabled: loading || busy }}>
    {#if error}<p class="staff-form-message staff-form-error" role="alert">{error}</p>{/if}{#if message}<p class="staff-muted" role="status">{message}</p>{/if}
    {#if loading}<p class="staff-muted" role="status">Memuat antrean…</p>{:else}<div class="order-detail-grid">
      <section class="staff-admin-panel checkin-history-panel"><div class="staff-panel-heading"><div><span class="panel-index">01</span><h2>Rekonsiliasi pembayaran</h2></div></div>
        {#if cases.length}<div class="history-table-wrap"><table class="history-table"><thead><tr><th>Reference</th><th>Event</th><th>Status</th><th>Provider</th><th>Terakhir diperiksa</th><th></th></tr></thead><tbody>{#each cases as item (item.id)}<tr><td><code>{item.reference}</code></td><td>{item.eventName}</td><td>{item.orderStatus} / {item.paymentStatus}</td><td>{item.providerStatus || "—"}</td><td>{time(item.lastCheckedAt)}</td><td><button class="staff-text-button" disabled={busy} onclick={() => openCase(item.id)}>Detail</button></td></tr>{/each}</tbody></table></div>{#if caseCursor}<button class="staff-secondary-button" onclick={() => more("cases")} disabled={busy}>Muat kasus berikutnya</button>{/if}{:else}<p class="staff-muted">Tidak ada kasus pembayaran terbuka.</p>{/if}
      </section>
      <section class="staff-admin-panel checkin-history-panel"><div class="staff-panel-heading"><div><span class="panel-index">02</span><h2>Email gagal</h2></div></div>
        {#if emails.length}<div class="history-table-wrap"><table class="history-table"><thead><tr><th>Reference</th><th>Jenis</th><th>Penerima</th><th>Percobaan</th><th>Terakhir diperbarui</th><th></th></tr></thead><tbody>{#each emails as item (item.id)}<tr><td><code>{item.reference}</code></td><td>{item.kind}</td><td>{item.recipient || "Menunggu verifikasi"}</td><td>{item.attempts}</td><td>{time(item.updatedAt)}</td><td><button class="staff-text-button" disabled={busy} onclick={() => openEmail(item.id)}>Detail</button></td></tr>{/each}</tbody></table></div>{#if emailCursor}<button class="staff-secondary-button" onclick={() => more("emails")} disabled={busy}>Muat email berikutnya</button>{/if}{:else}<p class="staff-muted">Tidak ada email gagal.</p>{/if}
      </section>
    </div>{/if}
    {#if detailLoading}<p class="staff-muted" role="status">Memuat detail yang dipilih…</p>{/if}
    {#if selectedCase}<section class="staff-admin-panel checkin-history-panel" aria-labelledby="case-detail-title"><div class="staff-panel-heading"><div><span class="panel-index">03</span><h2 id="case-detail-title">Kasus {selectedCase.reference}</h2></div><a class="staff-text-button" href={`/admin/orders?orderId=${encodeURIComponent(selectedCase.orderId)}`}>Lihat pesanan</a></div>
      <dl><dt>Event</dt><dd>{selectedCase.eventName}</dd><dt>Status order / pembayaran</dt><dd>{selectedCase.orderStatus} / {selectedCase.paymentStatus}</dd><dt>Status provider</dt><dd>{selectedCase.providerStatus || "Belum diketahui"}</dd><dt>Alasan</dt><dd>{selectedCase.reason}</dd><dt>Nominal</dt><dd>{money(selectedCase.amount)}</dd><dt>Pemeriksaan terakhir</dt><dd>{time(selectedCase.lastCheckedAt)}{selectedCase.lastCheckError ? ` · ${selectedCase.lastCheckError}` : ""}</dd></dl>
      {#if selectedCase.canRecheck}<button class="scan-submit" type="button" disabled={busy || detailLoading} onclick={() => act("recheck")}>Periksa ulang pembayaran</button>{/if}
      {#if selectedCase.status === "OPEN"}<form onsubmit={(event) => { event.preventDefault(); void act("note"); }}><label>Catatan admin<textarea bind:value={note} maxlength="2000" required disabled={busy || detailLoading}></textarea></label><button class="staff-secondary-button" type="submit" disabled={busy || detailLoading}>Simpan catatan</button>{#if selectedCase.canResolve}<button class="scan-submit" type="button" disabled={busy || detailLoading || !note.trim()} onclick={() => act("resolve")}>Tutup kasus dengan catatan ini</button>{/if}</form>{/if}
      <h3>Riwayat</h3><ul class="order-detail-list">{#each selectedCase.history as entry, i (`${entry.action}-${entry.createdAt}-${i}`)}<li><strong>{entry.action} · {entry.actorName}</strong><span>{time(entry.createdAt)} · {JSON.stringify(entry.data)}</span></li>{/each}</ul>
    </section>{/if}
    {#if selectedEmail}<section class="staff-admin-panel checkin-history-panel" aria-labelledby="email-detail-title"><div class="staff-panel-heading"><div><span class="panel-index">04</span><h2 id="email-detail-title">Email {selectedEmail.reference}</h2></div></div>
      <dl><dt>Jenis</dt><dd>{selectedEmail.kind}</dd><dt>Penerima</dt><dd>{selectedEmail.recipient || "Menunggu verifikasi"}</dd><dt>Status order</dt><dd>{selectedEmail.orderStatus || "—"}</dd><dt>Percobaan</dt><dd>{selectedEmail.attempts}</dd><dt>Kesalahan terakhir</dt><dd>{selectedEmail.lastError || "—"}</dd><dt>Status</dt><dd>{selectedEmail.status}</dd></dl>
      {#if selectedEmail.canRetry}<button class="scan-submit" type="button" disabled={busy || detailLoading} onclick={() => act("retry")}>Kirim ulang email</button>{:else}<p class="staff-muted">{selectedEmail.retryReason || "Job tidak dapat dikirim ulang."}</p>{/if}
      <ul class="order-detail-list">{#each selectedEmail.history as entry, i (`${entry.action}-${entry.createdAt}-${i}`)}<li><strong>{entry.action} · {entry.actorName}</strong><span>{time(entry.createdAt)} · {JSON.stringify(entry.data)}</span></li>{/each}</ul>
    </section>{/if}
</AdminLayout>
