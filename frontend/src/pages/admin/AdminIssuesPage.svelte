<script lang="ts">
  import AdminLayout from "../../layouts/AdminLayout.svelte";
  import { onMount } from "svelte";
  import { ApiError, addAdminPaymentCaseNote, getAdminEmailJob, getAdminEmailJobs, getAdminPaymentCase, getAdminPaymentCases, recheckAdminPaymentCase, resolveAdminPaymentCase, retryAdminEmailJob, type AdminEmailDetail, type AdminEmailJob, type AdminPaymentCase, type AdminPaymentCaseDetail } from "../../lib/api.ts";
  import { createRequestVersion } from "../../lib/request-version.ts";
  import { auditSummary, caseStatus, emailKind, emailStatus, orderStatusLabel, paymentStatusLabel, providerStatusLabel } from "../../lib/admin-operations.ts";

  let { accessToken, onUnauthorized, onLogout, loggingOut }: { accessToken: string; onUnauthorized: () => void; onLogout: () => void; loggingOut: boolean } = $props();
  let cases = $state<AdminPaymentCase[]>([]); let emails = $state<AdminEmailJob[]>([]);
  let caseCursor = $state<string | null>(null); let emailCursor = $state<string | null>(null);
  let selectedCase = $state<AdminPaymentCaseDetail | null>(null); let selectedEmail = $state<AdminEmailDetail | null>(null);
  let note = $state(""); let busy = $state(false); let loading = $state(true); let caseLoading = $state(true); let emailLoading = $state(true); let detailLoading = $state(false); const detailRequests = createRequestVersion(); let error = $state(""); let caseError = $state(""); let emailError = $state(""); let message = $state(""); let uncertainAction = $state("");
  let uncertainBaseline: { historyCount: number; lastCheckedAt: string | null; retryJobId: string | null } | null = null;
  async function load() {
    loading = true; error = ""; caseLoading = true; emailLoading = true; caseError = ""; emailError = "";
    await Promise.all([loadCases(), loadEmails()]);
    loading = false;
  }
  async function loadCases() { caseLoading = true; caseError = ""; try { const result = await getAdminPaymentCases(accessToken); cases = result.items; caseCursor = result.nextCursor; } catch (cause) { failQueue(cause, "cases"); } finally { caseLoading = false; } }
  async function loadEmails() { emailLoading = true; emailError = ""; try { const result = await getAdminEmailJobs(accessToken); emails = result.items; emailCursor = result.nextCursor; } catch (cause) { failQueue(cause, "emails"); } finally { emailLoading = false; } }
  function failQueue(cause: unknown, kind: "cases" | "emails") { if (cause instanceof ApiError && cause.status === 401) onUnauthorized(); else if (kind === "cases") caseError = cause instanceof ApiError ? cause.message : "Kasus pembayaran belum dapat dimuat."; else emailError = cause instanceof ApiError ? cause.message : "Email gagal belum dapat dimuat."; }
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
    const baseline = { historyCount: selectedCase?.history.length ?? selectedEmail?.history.length ?? 0, lastCheckedAt: selectedCase?.lastCheckedAt ?? null, retryJobId: selectedEmail?.retryJobId ?? null };
    if ((action === "retry" && !emailId) || (action !== "retry" && !caseId)) return;
    if (action === "resolve" && !noteText.trim()) return;
    if (action === "resolve" && !window.confirm(`Tutup kasus pembayaran ${selectedCase?.reference} dengan catatan: “${noteText.trim()}”? Tindakan ini hanya menutup kasus dan tidak melakukan refund.`)) return;
    if (action === "retry" && !window.confirm(`Jadwalkan ulang email untuk ${selectedEmail?.reference} ke ${selectedEmail?.recipient || "penerima terverifikasi"}?`)) return;
    if (uncertainAction === action) return;
    busy = true; error = ""; message = "";
    try {
      if (action === "note" && caseId) await addAdminPaymentCaseNote(accessToken, caseId, noteText);
      if (action === "resolve" && caseId) await resolveAdminPaymentCase(accessToken, caseId, noteText);
      if (action === "recheck" && caseId) await recheckAdminPaymentCase(accessToken, caseId);
      const retryResult = action === "retry" && emailId ? await retryAdminEmailJob(accessToken, emailId) : null;
      message = action === "note" ? "Catatan disimpan." : action === "resolve" ? "Kasus ditutup." : action === "recheck" ? "Status pembayaran diperbarui." : "Job email dijadwalkan kembali.";
      uncertainAction = "";
      uncertainBaseline = null;
      if (retryResult?.retryJobId && selectedEmail) selectedEmail = { ...selectedEmail, retryJobId: retryResult.retryJobId };
      await load();
      if (detailRequests.isCurrent(generation) && caseId) await loadCaseDetail(caseId, generation);
      if (detailRequests.isCurrent(generation) && emailId) await loadEmailDetail(emailId, generation);
    } catch (cause) {
      fail(cause);
      if (cause instanceof ApiError && (cause.code === "UNKNOWN_OUTCOME" || cause.code === "NETWORK_ERROR" || cause.status >= 500)) {
        uncertainAction = action;
        uncertainBaseline = baseline;
        message = "Hasil tindakan belum pasti. Periksa detail terbaru sebelum mencoba lagi.";
        if (detailRequests.isCurrent(generation) && caseId) await loadCaseDetail(caseId, generation);
        if (detailRequests.isCurrent(generation) && emailId) await loadEmailDetail(emailId, generation);
      }
    } finally { busy = false; }
  }
  async function verifyUncertain() {
    const action = uncertainAction;
    const caseId = selectedCase?.id; const emailId = selectedEmail?.id;
    if (!action || busy) return;
    busy = true;
    try {
      const generation = detailRequests.next(); detailLoading = true;
      if (caseId) await loadCaseDetail(caseId, generation);
      if (emailId) await loadEmailDetail(emailId, generation);
      const historyAdvanced = (selectedCase?.history.length ?? selectedEmail?.history.length ?? 0) > (uncertainBaseline?.historyCount ?? 0);
      if ((action === "resolve" && selectedCase?.status === "RESOLVED") || (action === "note" && historyAdvanced && selectedCase?.history.some((entry) => entry.action === "NOTE" && entry.data.note === note)) || (action === "recheck" && historyAdvanced && selectedCase?.lastCheckedAt !== uncertainBaseline?.lastCheckedAt) || (action === "retry" && selectedEmail?.retryJobId && selectedEmail.retryJobId !== uncertainBaseline?.retryJobId)) {
        uncertainAction = ""; uncertainBaseline = null; message = "Hasil tindakan terlihat pada detail terbaru.";
      } else message = "Detail terbaru belum membuktikan hasil tindakan. Pengulangan tetap ditahan.";
    } finally { busy = false; }
  }
  function time(value: string | null) { return value ? new Date(value).toLocaleString("id-ID", { timeZone: "Asia/Jakarta", dateStyle: "medium", timeStyle: "short" }) + " WIB" : "Belum pernah"; }
  function money(value: number) { return new Intl.NumberFormat("id-ID", { style: "currency", currency: "IDR", maximumFractionDigits: 0 }).format(value); }
  onMount(() => { void load(); });
</script>

<svelte:head><title>Penanganan masalah | Tiket Online</title><meta name="robots" content="noindex" /></svelte:head>
  <AdminLayout page="issues" contentId="issues-content" skipLabel="Lewati ke penanganan masalah" kicker="OPERASIONAL EVENT • ADMIN" title="Penanganan masalah." description="Periksa pembayaran terlambat dan email yang gagal dikirim." onLogout={onLogout} loggingOut={loggingOut} primaryAction={{ label: "Muat ulang", onclick: load, disabled: loading || busy }}>
    {#if error}<p class="staff-form-message staff-form-error" role="alert">{error}</p>{/if}
    {#if message}<div class="issue-action-feedback" role="status">{message}{#if uncertainAction}<button type="button" class="staff-text-button" disabled={busy} onclick={verifyUncertain}>Periksa hasil tindakan</button>{/if}</div>{/if}
    {#if loading}<p class="staff-muted" role="status">Memuat antrean…</p>{:else}<div class="order-detail-grid issue-workspace">
      <section class="staff-admin-panel checkin-history-panel"><div class="staff-panel-heading"><div><span class="panel-index">01</span><h2>Rekonsiliasi pembayaran</h2></div></div>
        {#if caseLoading}<p role="status" class="staff-muted">Memuat kasus pembayaran…</p>{:else if caseError}<p role="alert" class="staff-form-message staff-form-error">{caseError} <button class="staff-text-button" onclick={loadCases}>Coba lagi</button></p>{:else if cases.length}<div class="history-table-wrap"><table class="history-table"><thead><tr><th>Reference</th><th>Event</th><th>Status</th><th>Provider</th><th>Terakhir diperiksa</th><th></th></tr></thead><tbody>{#each cases as item (item.id)}<tr><td data-label="Reference"><code>{item.reference}</code></td><td data-label="Event">{item.eventName}</td><td data-label="Status">{orderStatusLabel(item.orderStatus)}<small>{paymentStatusLabel(item.paymentStatus)}</small></td><td data-label="Provider">{providerStatusLabel(item.providerStatus)}</td><td data-label="Terakhir diperiksa">{time(item.lastCheckedAt)}</td><td data-label="Detail"><button class="staff-text-button" disabled={busy} onclick={() => openCase(item.id)}>Detail</button></td></tr>{/each}</tbody></table></div>{#if caseCursor}<button class="staff-secondary-button" onclick={() => more("cases")} disabled={busy}>Muat kasus berikutnya</button>{/if}{:else}<p class="staff-muted">Tidak ada kasus pembayaran terbuka.</p>{/if}
      </section>
      <section class="staff-admin-panel checkin-history-panel"><div class="staff-panel-heading"><div><span class="panel-index">02</span><h2>Email gagal</h2></div></div>
        {#if emailLoading}<p role="status" class="staff-muted">Memuat email gagal…</p>{:else if emailError}<p role="alert" class="staff-form-message staff-form-error">{emailError} <button class="staff-text-button" onclick={loadEmails}>Coba lagi</button></p>{:else if emails.length}<div class="history-table-wrap"><table class="history-table"><thead><tr><th>Reference</th><th>Jenis</th><th>Penerima</th><th>Percobaan</th><th>Terakhir diperbarui</th><th></th></tr></thead><tbody>{#each emails as item (item.id)}<tr><td data-label="Reference"><code>{item.reference}</code></td><td data-label="Jenis">{emailKind(item.kind)}</td><td data-label="Penerima">{item.recipient || "Menunggu verifikasi"}</td><td data-label="Percobaan">{item.attempts}</td><td data-label="Terakhir diperbarui">{time(item.updatedAt)}</td><td data-label="Detail"><button class="staff-text-button" disabled={busy} onclick={() => openEmail(item.id)}>Detail</button></td></tr>{/each}</tbody></table></div>{#if emailCursor}<button class="staff-secondary-button" onclick={() => more("emails")} disabled={busy}>Muat email berikutnya</button>{/if}{:else}<p class="staff-muted">Tidak ada email gagal.</p>{/if}
      </section>
    </div>{/if}
    {#if detailLoading}<p class="staff-muted" role="status">Memuat detail yang dipilih…</p>{/if}
    {#if selectedCase}<section class="staff-admin-panel checkin-history-panel issue-detail-panel" aria-labelledby="case-detail-title"><div class="staff-panel-heading"><div><span class="panel-index">03</span><h2 id="case-detail-title">Kasus {selectedCase.reference}</h2></div><a class="staff-text-button" href={`/admin/orders?orderId=${encodeURIComponent(selectedCase.orderId)}`}>Lihat pesanan</a></div>
      <dl><dt>Event</dt><dd>{selectedCase.eventName}</dd><dt>Status kasus</dt><dd>{caseStatus(selectedCase.status)}</dd><dt>Status order / pembayaran</dt><dd>{orderStatusLabel(selectedCase.orderStatus)} / {paymentStatusLabel(selectedCase.paymentStatus)}</dd><dt>Status provider</dt><dd>{providerStatusLabel(selectedCase.providerStatus)}{#if selectedCase.providerStatus && providerStatusLabel(selectedCase.providerStatus) === "Status penyedia belum dikenali"}<details><summary>Status teknis</summary><code>{selectedCase.providerStatus}</code></details>{/if}</dd><dt>Alasan</dt><dd>{selectedCase.reason}</dd><dt>Nominal</dt><dd>{money(selectedCase.amount)}</dd><dt>Pemeriksaan terakhir</dt><dd>{time(selectedCase.lastCheckedAt)}{selectedCase.lastCheckError ? ` · ${selectedCase.lastCheckError}` : ""}</dd></dl>
      {#if selectedCase.canRecheck}<section class="issue-action-group"><h3>Pengecekan pembayaran</h3><p class="staff-muted">Minta status transaksi terbaru dari penyedia pembayaran.</p><button class="scan-submit" type="button" disabled={busy || detailLoading} onclick={() => act("recheck")}>Periksa ulang pembayaran</button></section>{/if}
      {#if selectedCase.status === "OPEN"}<section class="issue-action-group"><h3>Catatan admin</h3><form onsubmit={(event) => { event.preventDefault(); void act("note"); }}><label>Catatan admin<textarea bind:value={note} maxlength="2000" required disabled={busy || detailLoading}></textarea></label><button class="staff-secondary-button" type="submit" disabled={busy || detailLoading}>Simpan catatan</button></form>
        {#if selectedCase.canResolve}<div class="issue-close-action"><h3>Penutupan kasus</h3><p class="staff-muted">Kasus akan ditutup dengan catatan di atas. Penutupan tidak mengajukan refund atau mengubah pembayaran.</p><button class="scan-submit" type="button" disabled={busy || detailLoading || !note.trim()} onclick={() => act("resolve")}>Tutup kasus dengan catatan ini</button></div>{/if}
      </section>{/if}
      <h3>Riwayat</h3><ul class="order-detail-list issue-history">{#each selectedCase.history as entry, i (`${entry.action}-${entry.createdAt}-${i}`)}<li><strong>{entry.action === "NOTE" ? "Catatan ditambahkan" : entry.action === "RESOLVE" ? "Kasus ditutup" : entry.action === "RECHECK" ? "Pembayaran diperiksa" : entry.action === "RETRY" ? "Email dijadwalkan ulang" : "Tindakan operasional"} · {entry.actorName}</strong><span>{auditSummary(entry)}</span><time>{time(entry.createdAt)}</time><details><summary>Detail teknis</summary><pre>{JSON.stringify(entry.data, null, 2)}</pre></details></li>{/each}</ul>
    </section>{/if}
    {#if selectedEmail}<section class="staff-admin-panel checkin-history-panel issue-detail-panel" aria-labelledby="email-detail-title"><div class="staff-panel-heading"><div><span class="panel-index">04</span><h2 id="email-detail-title">Email {selectedEmail.reference}</h2></div></div>
      <dl><dt>Jenis</dt><dd>{emailKind(selectedEmail.kind)}</dd><dt>Penerima</dt><dd>{selectedEmail.recipient || "Menunggu verifikasi"}</dd><dt>Status order</dt><dd>{orderStatusLabel(selectedEmail.orderStatus || "")}</dd><dt>Percobaan</dt><dd>{selectedEmail.attempts}</dd><dt>Kesalahan terakhir</dt><dd>{selectedEmail.lastError || "Belum tersedia"}</dd><dt>Status</dt><dd>{emailStatus(selectedEmail.status)}{#if emailStatus(selectedEmail.status) === "Status email belum dikenali"}<details><summary>Status teknis</summary><code>{selectedEmail.status}</code></details>{/if}</dd></dl>
      {#if selectedEmail.canRetry}<button class="scan-submit" type="button" disabled={busy || detailLoading} onclick={() => act("retry")}>Kirim ulang email</button>{:else}<p class="staff-muted">{selectedEmail.retryReason || "Job tidak dapat dikirim ulang."}</p>{/if}
      <h3>Riwayat</h3><ul class="order-detail-list issue-history">{#each selectedEmail.history as entry, i (`${entry.action}-${entry.createdAt}-${i}`)}<li><strong>{entry.action === "RETRY" ? "Email dijadwalkan ulang" : "Tindakan operasional"} · {entry.actorName}</strong><span>{auditSummary(entry)}</span><time>{time(entry.createdAt)}</time><details><summary>Detail teknis</summary><pre>{JSON.stringify(entry.data, null, 2)}</pre></details></li>{/each}</ul>
    </section>{/if}
</AdminLayout>
