<script lang="ts">
  import { MediaQuery } from "svelte/reactivity";
  import AdminMobileList from "../../components/admin/AdminMobileList.svelte";
  import { mobileListItems } from "../../lib/mobile-list.ts";
  const mobileList = new MediaQuery("(max-width: 767px)");

  import AdminLayout from "../../layouts/AdminLayout.svelte";
  import { onMount, tick } from "svelte";
  import { ApiError, addAdminPaymentCaseNote, getAdminEmailJob, getAdminEmailJobs, getAdminPaymentCase, getAdminPaymentCases, recheckAdminPaymentCase, resolveAdminPaymentCase, retryAdminEmailJob, type AdminIssueAudit, type AdminEmailDetail, type AdminEmailJob, type AdminPaymentCase, type AdminPaymentCaseDetail } from "../../lib/api.ts";
  import { createRequestVersion } from "../../lib/request-version.ts";
  import { auditSummary, caseStatus, emailRetryActionLabel, emailFailureReason, emailRetryConfirmed, emailKind, emailStatus, orderStatusLabel, paymentStatusLabel, providerStatusLabel } from "../../lib/admin-operations.ts";

  let { accessToken, onUnauthorized, onLogout, loggingOut }: { accessToken: string; onUnauthorized: () => void; onLogout: () => void; loggingOut: boolean } = $props();
  let caseGroup = $state(0); let emailGroup = $state(0);
  let caseScroll = $state(0); let emailScroll = $state(0);
  let cases = $state<AdminPaymentCase[]>([]); let emails = $state<AdminEmailJob[]>([]);
  let caseCursor = $state<string | null>(null); let emailCursor = $state<string | null>(null);
  let selectedCase = $state<AdminPaymentCaseDetail | null>(null); let selectedEmail = $state<AdminEmailDetail | null>(null);
  let view = $state<"list" | "case" | "email">("list"); let detailId = $state("");
  let casesLoaded = $state(false); let emailsLoaded = $state(false);
  let delivery = $state<AdminEmailDetail | null>(null); let deliveryError = $state("");
  let listScroll = 0; let triggerId = "";
  const queuedMessage = "Pengiriman ulang dijadwalkan. Email belum tentu terkirim. Muat ulang daftar dan detail untuk melihat status terbaru.";
  let note = $state(""); let busy = $state(false); let loading = $state(true); let caseLoading = $state(true); let emailLoading = $state(true); let detailLoading = $state(false); const detailRequests = createRequestVersion(); let error = $state(""); let caseError = $state(""); let emailError = $state(""); let message = $state("");
  type UncertainOperation = { kind: "case" | "email"; id: string; action: "note" | "resolve" | "recheck" | "retry"; baseline: { historyCount: number; lastCheckedAt: string | null; retryJobId: string | null; emailHistory: AdminIssueAudit[] }; noteText: string };
  let uncertainOperation = $state<UncertainOperation | null>(null);
  async function load() {
    loading = true; error = ""; caseLoading = true; emailLoading = true; caseError = ""; emailError = "";
    await Promise.all([loadCases(), loadEmails()]);
    loading = false;
  }
  async function loadCases() { caseLoading = true; caseError = ""; try { const result = await getAdminPaymentCases(accessToken); cases = result.items; caseCursor = result.nextCursor; casesLoaded = true; } catch (cause) { failQueue(cause, "cases"); } finally { caseLoading = false; } }
  async function loadEmails() { emailLoading = true; emailError = ""; try { const result = await getAdminEmailJobs(accessToken); emails = result.items; emailCursor = result.nextCursor; emailsLoaded = true; } catch (cause) { failQueue(cause, "emails"); } finally { emailLoading = false; } }
  function failQueue(cause: unknown, kind: "cases" | "emails") { if (cause instanceof ApiError && cause.status === 401) onUnauthorized(); else if (kind === "cases") caseError = cause instanceof ApiError ? cause.message : "Kasus pembayaran belum dapat dimuat."; else emailError = cause instanceof ApiError ? cause.message : "Email gagal belum dapat dimuat."; }
  async function more(kind: "cases" | "emails") {
    const cursor = kind === "cases" ? caseCursor : emailCursor; if (!cursor || busy || caseLoading || emailLoading) return; busy = true; error = "";
    try { if (kind === "cases") { const result = await getAdminPaymentCases(accessToken, undefined, cursor); cases = [...cases, ...result.items.filter((item) => !cases.some((old) => old.id === item.id))]; caseCursor = result.nextCursor; }
      else { const result = await getAdminEmailJobs(accessToken, undefined, cursor); emails = [...emails, ...result.items.filter((item) => !emails.some((old) => old.id === item.id))]; emailCursor = result.nextCursor; }
      return true; }
    catch (cause) { fail(cause); } finally { busy = false; }
  }
  function fail(cause: unknown) { if (cause instanceof ApiError && cause.status === 401) onUnauthorized(); else error = cause instanceof ApiError ? cause.message : "Data operasional belum dapat dimuat."; }
  async function loadCaseDetail(id: string, generation: number) {
    try { const result = await getAdminPaymentCase(accessToken, id); if (detailRequests.isCurrent(generation)) selectedCase = result; }
    catch (cause) { if (detailRequests.isCurrent(generation)) fail(cause); }
    finally { if (detailRequests.isCurrent(generation)) detailLoading = false; }
  }
  async function loadEmailDetail(id: string, generation: number) {
    try { const result = await getAdminEmailJob(accessToken, id); if (detailRequests.isCurrent(generation)) {
        selectedEmail = result; deliveryError = "";
        if (result.retryJobId && result.retryJobId !== result.id) {
          try { const latest = await getAdminEmailJob(accessToken, result.retryJobId); if (detailRequests.isCurrent(generation)) delivery = latest; }
          catch (cause) { if (detailRequests.isCurrent(generation)) { deliveryError = "Status pengiriman ulang gagal dimuat. Muat ulang detail untuk memeriksa pengiriman."; if (cause instanceof ApiError && cause.status === 401) onUnauthorized(); } }
        } else delivery = null;
      } }
    catch (cause) { if (detailRequests.isCurrent(generation)) fail(cause); }
    finally { if (detailRequests.isCurrent(generation)) detailLoading = false; }
  }
  async function openDetail(kind: "case" | "email", id: string) {
    if (busy || uncertainOperation) return;
    if (view === "list") { listScroll = window.scrollY; triggerId = `${kind}-${id}`; }
    view = kind; detailId = id;
    const generation = detailRequests.next();
    selectedCase = null; selectedEmail = null; delivery = null; deliveryError = ""; note = ""; message = ""; error = ""; detailLoading = true;
    await tick(); document.getElementById("issue-detail-title")?.focus();
    if (kind === "case") await loadCaseDetail(id, generation); else await loadEmailDetail(id, generation);
  }
  async function backToList() {
    if (busy || uncertainOperation) return;
    if (note.trim() && !window.confirm("Tinggalkan catatan yang belum disimpan?")) return;
    const index = (triggerId.startsWith("case-") ? cases : emails).findIndex(item => triggerId.endsWith(`-${item.id}`));
    if (mobileList.current && index >= 0) {
      if (triggerId.startsWith("case-") && caseGroup !== Math.floor(index / 5)) { caseGroup = Math.floor(index / 5); caseScroll = 0; }
      if (triggerId.startsWith("email-") && emailGroup !== Math.floor(index / 5)) { emailGroup = Math.floor(index / 5); emailScroll = 0; }
    }
    detailRequests.next(); view = "list"; selectedCase = null; selectedEmail = null; error = ""; note = "";
    await tick(); (document.getElementById(triggerId) ?? document.getElementById(triggerId.startsWith("email-") ? "email-list-title" : "case-list-title"))?.focus({ preventScroll: true }); window.scrollTo({ top: listScroll, behavior: "instant" });
  }
  async function refreshDetail() {
    if (busy || detailLoading) return;
    error = ""; detailLoading = true; const generation = detailRequests.next();
    if (view === "case") await loadCaseDetail(detailId, generation); else await loadEmailDetail(detailId, generation);
  }
  async function refresh() { await load(); if (view !== "list") await refreshDetail(); }
  async function act(action: "note" | "resolve" | "recheck" | "retry") {
    if (busy || detailLoading || uncertainOperation) return;
    const caseId = selectedCase?.id; const emailId = selectedEmail?.id; const noteText = note; const generation = detailRequests.value();
    const baseline = { historyCount: selectedCase?.history.length ?? selectedEmail?.history.length ?? 0, lastCheckedAt: selectedCase?.lastCheckedAt ?? null, retryJobId: selectedEmail?.retryJobId ?? null, emailHistory: selectedEmail?.history ?? [] };
    if ((action === "retry" && (!emailId || !selectedEmail?.canRetry)) || (action !== "retry" && !caseId)) return;
    if (action === "resolve" && !noteText.trim()) return;
    if (action === "resolve" && !window.confirm(`Tutup kasus pembayaran ${selectedCase?.reference} dengan catatan: “${noteText.trim()}”? Tindakan ini hanya menutup kasus dan tidak melakukan refund.`)) return;
    if (action === "retry" && !window.confirm(`${emailRetryActionLabel(selectedEmail!)}\nJadwalkan ulang email untuk ${selectedEmail?.reference} ke ${selectedEmail?.recipient || "alamat penerima belum tersedia"}?`)) return;
    const kind = action === "retry" ? "email" : "case";
    busy = true; error = ""; message = "";
    try {
      if (action === "note" && caseId) await addAdminPaymentCaseNote(accessToken, caseId, noteText);
      if (action === "resolve" && caseId) await resolveAdminPaymentCase(accessToken, caseId, noteText);
      if (action === "recheck" && caseId) await recheckAdminPaymentCase(accessToken, caseId);
      const retryResult = action === "retry" && emailId ? await retryAdminEmailJob(accessToken, emailId) : null;
      message = action === "note" ? "Catatan disimpan." : action === "resolve" ? "Kasus ditutup." : action === "recheck" ? "Status pembayaran diperbarui." : queuedMessage;
      if (action === "note" || action === "resolve") note = "";
      if (retryResult && selectedEmail) selectedEmail = { ...selectedEmail, canRetry: false, retryReason: "Pengiriman ulang sudah dijadwalkan. Periksa status terbaru sebelum tindakan lain." };
      await load();
      if (detailRequests.isCurrent(generation) && caseId) await loadCaseDetail(caseId, generation);
      if (detailRequests.isCurrent(generation) && emailId) await loadEmailDetail(emailId, generation);
      if (action === "retry") { await tick(); document.getElementById("issue-feedback")?.focus(); }
    } catch (cause) {
      fail(cause);
      if (cause instanceof ApiError && (cause.code === "UNKNOWN_OUTCOME" || cause.code === "NETWORK_ERROR" || cause.status >= 500)) {
        uncertainOperation = { kind, id: kind === "case" ? caseId! : emailId!, action, baseline, noteText: noteText.trim() };
        message = "Hasil tindakan belum pasti. Periksa detail terbaru sebelum mencoba lagi.";
        if (detailRequests.isCurrent(generation) && caseId) await loadCaseDetail(caseId, generation);
        if (detailRequests.isCurrent(generation) && emailId) await loadEmailDetail(emailId, generation);
      }
    } finally { busy = false; }
  }
  async function verifyUncertain() {
    const operation = uncertainOperation;
    if (!operation || busy) return;
    busy = true;
    try {
      const detail = operation.kind === "case"
        ? await getAdminPaymentCase(accessToken, operation.id)
        : await getAdminEmailJob(accessToken, operation.id);
      if (uncertainOperation !== operation) return;
      if (operation.kind === "case") selectedCase = detail as AdminPaymentCaseDetail;
      else selectedEmail = detail as AdminEmailDetail;
      const historyAdvanced = detail.history.length > operation.baseline.historyCount;
      const confirmed = operation.kind === "case"
        ? (() => {
            const caseDetail = detail as AdminPaymentCaseDetail;
            return operation.action === "resolve" && caseDetail.status === "RESOLVED"
              || operation.action === "note" && historyAdvanced && caseDetail.history.some((entry) => entry.action === "NOTE" && entry.data.note === operation.noteText)
              || operation.action === "recheck" && historyAdvanced && caseDetail.lastCheckedAt !== operation.baseline.lastCheckedAt;
          })()
        : (() => {
            const emailDetail = detail as AdminEmailDetail;
            return operation.action === "retry" && emailRetryConfirmed(emailDetail, operation.baseline.emailHistory);
          })();
      if (confirmed) {
        uncertainOperation = null; message = operation.kind === "email" ? queuedMessage : "Hasil tindakan terlihat pada detail terbaru.";
        await load(); if (operation.kind === "email") await loadEmailDetail(operation.id, detailRequests.value());
        await tick(); document.getElementById("issue-feedback")?.focus();
      } else message = "Detail terbaru belum membuktikan hasil tindakan. Pengulangan tetap ditahan.";
    } catch (cause) { fail(cause); message = "Detail terbaru belum dapat diperiksa. Operasi tetap dikunci; coba periksa lagi."; }
    finally { busy = false; }
  }
  function time(value: string | null) { return value ? new Date(value).toLocaleString("id-ID", { timeZone: "Asia/Jakarta", dateStyle: "medium", timeStyle: "medium" }) + " WIB" : "Belum pernah"; }
  function money(value: number) { return new Intl.NumberFormat("id-ID", { style: "currency", currency: "IDR", maximumFractionDigits: 0 }).format(value); }
  function protectUnload(event: BeforeUnloadEvent) { if (busy || uncertainOperation || note.trim()) { event.preventDefault(); event.returnValue = ""; } }
  onMount(() => {
    void load();
    const protectNavigation = (event: MouseEvent) => {
      if (!(event.target instanceof Element) || !event.target.closest("a[href]")) return;
      if (busy || uncertainOperation) { event.preventDefault(); message = "Selesaikan pemeriksaan hasil tindakan sebelum meninggalkan halaman."; }
      else if (note.trim() && !window.confirm("Tinggalkan catatan yang belum disimpan?")) event.preventDefault();
    };
    document.addEventListener("click", protectNavigation, true);
    return () => document.removeEventListener("click", protectNavigation, true);
  });
</script>

<svelte:window onbeforeunload={protectUnload} />
<svelte:head><title>Penanganan masalah | Tiket Online</title><meta name="robots" content="noindex" /></svelte:head>
  <AdminLayout page="issues" contentId="issues-content" skipLabel="Lewati ke penanganan masalah" kicker="OPERASIONAL EVENT • ADMIN" title="Penanganan masalah." description="Periksa pembayaran terlambat dan email yang gagal dikirim." onLogout={onLogout} loggingOut={loggingOut} logoutDisabled={loggingOut || busy || Boolean(uncertainOperation)} secondaryAction={view !== "email" ? { label: "Muat ulang", onclick: refresh, disabled: loading || busy || detailLoading } : undefined}>
    {#if error}<p class="staff-form-message staff-form-error" role="alert">{error}</p>{/if}
    {#if message}<div id="issue-feedback" tabindex="-1" class="issue-action-feedback" role="status">{message}</div>{/if}
    {#if view !== "list" && (caseError || emailError)}<p class="staff-form-message staff-form-error" role="alert">Pembaruan daftar gagal. Data terakhir tetap tersedia; hasil tindakan yang sudah berhasil tetap berlaku. {caseError} {emailError}</p>{/if}
    {#if uncertainOperation}<div class="issue-action-feedback" role="status">Operasi {uncertainOperation.action} untuk {uncertainOperation.kind === "case" ? "kasus" : "email"} {uncertainOperation.id} hasilnya belum dapat dipastikan. Selesaikan pemeriksaan ini sebelum berpindah detail atau menjalankan tindakan lain. <button type="button" class="staff-text-button" disabled={busy} onclick={verifyUncertain}>Periksa hasil tindakan</button></div>{/if}
    {#if view === "list"}<div class="order-detail-grid issue-workspace">
      <section class="staff-admin-panel checkin-history-panel" aria-labelledby="case-list-title"><div class="staff-panel-heading"><h2 id="case-list-title" tabindex="-1">Rekonsiliasi pembayaran</h2></div>
        {#if caseLoading}<p role="status" class="staff-muted">Memuat kasus pembayaran…</p>{/if}
        {#if caseError}<p role="alert" class="staff-form-message staff-form-error">{casesLoaded ? "Pembaruan gagal. Data terakhir tetap ditampilkan. " : ""}{caseError} <button class="staff-text-button" disabled={caseLoading} onclick={loadCases}>Coba lagi</button></p>{/if}
        {#if cases.length}<AdminMobileList count={cases.length} bind:group={caseGroup} bind:scrollLeft={caseScroll} headingId="case-list-title" hasMore={Boolean(caseCursor)} loadMore={() => more("cases")} disabled={busy || caseLoading}><div class="history-table-wrap"><table class="history-table"><thead><tr><th>Nomor pesanan</th><th>Event</th><th>Status</th><th>Provider</th><th>Terakhir diperiksa</th><th>Aksi</th></tr></thead><tbody>{#each mobileListItems(cases, caseGroup, mobileList.current) as item (item.id)}<tr><td data-label="Nomor pesanan"><code>{item.reference}</code></td><td data-label="Event">{item.eventName}</td><td data-label="Status">{orderStatusLabel(item.orderStatus)}<small>{paymentStatusLabel(item.paymentStatus)}</small></td><td data-label="Provider">{providerStatusLabel(item.providerStatus)}</td><td data-label="Terakhir diperiksa">{time(item.lastCheckedAt)}</td><td data-label="Aksi"><button id={`case-${item.id}`} class="staff-text-button" disabled={busy} onclick={() => openDetail("case", item.id)}>Detail</button></td></tr>{/each}</tbody></table></div></AdminMobileList>{:else if casesLoaded && !caseError && !caseLoading}<p class="staff-muted">Tidak ada kasus pembayaran terbuka.</p>{/if}
      </section>
      <section class="staff-admin-panel checkin-history-panel" aria-labelledby="email-list-title"><div class="staff-panel-heading"><h2 id="email-list-title" tabindex="-1">Email gagal</h2></div>
        {#if emailLoading}<p role="status" class="staff-muted">Memuat email gagal…</p>{/if}
        {#if emailError}<p role="alert" class="staff-form-message staff-form-error">{emailsLoaded ? "Pembaruan gagal. Data terakhir tetap ditampilkan; status mungkin sudah berubah. " : ""}{emailError} <button class="staff-text-button" disabled={emailLoading} onclick={loadEmails}>Coba lagi</button></p>{/if}
        {#if emails.length}<AdminMobileList count={emails.length} bind:group={emailGroup} bind:scrollLeft={emailScroll} headingId="email-list-title" hasMore={Boolean(emailCursor)} loadMore={() => more("emails")} disabled={busy || emailLoading}><div class="history-table-wrap"><table class="history-table"><thead><tr><th>Nomor pesanan</th><th>Jenis email</th><th>Status</th><th>Alasan gagal</th><th>Aksi</th></tr></thead><tbody>{#each mobileListItems(emails, emailGroup, mobileList.current) as item (item.id)}<tr><td data-label="Nomor pesanan"><div><code>{item.reference}</code><small>{item.recipient || "Alamat penerima belum tersedia"}</small></div></td><td data-label="Jenis email">{emailKind(item.kind)}</td><td data-label="Status"><span class="admin-order-status" class:is-expired={item.status === "FAILED"} class:is-paid={item.status === "SENT"}>{emailStatus(item.status)}</span></td><td data-label="Alasan gagal">{emailFailureReason(item.lastError)}</td><td data-label="Aksi"><button id={`email-${item.id}`} class="staff-text-button" aria-label={`Lihat detail email ${item.reference}`} disabled={busy} onclick={() => openDetail("email", item.id)}>Lihat detail</button></td></tr>{/each}</tbody></table></div></AdminMobileList>{:else if emailsLoaded && !emailError && !emailLoading}<p class="staff-muted">Tidak ada email gagal yang belum digantikan pada respons daftar ini.</p>{/if}
      </section>
    </div>{:else}
      {#if view === "case"}<button class="staff-secondary-button" disabled={busy || Boolean(uncertainOperation)} onclick={backToList}>Kembali ke daftar</button>{/if}
      <h2 id="issue-detail-title" tabindex="-1">{view === "email" ? `Email ${selectedEmail?.reference ?? "yang dipilih"}` : `Kasus ${selectedCase?.reference ?? "yang dipilih"}`}</h2>
      {#if detailLoading}<p class="staff-muted" role="status">Memuat detail yang dipilih…</p>{/if}
      {#if error && view === "case"}<button class="staff-secondary-button" disabled={detailLoading || busy} onclick={refreshDetail}>Muat ulang detail</button>{/if}
    {/if}
    {#if selectedCase}<section class="staff-admin-panel checkin-history-panel issue-detail-panel" aria-labelledby="case-detail-title"><div class="staff-panel-heading"><div><span class="panel-index">03</span><h3 id="case-detail-title">Rincian kasus</h3></div><a class="staff-text-button" href={`/admin/orders?orderId=${encodeURIComponent(selectedCase.orderId)}`}>Lihat pesanan</a></div>
      <dl><dt>Event</dt><dd>{selectedCase.eventName}</dd><dt>Status kasus</dt><dd>{caseStatus(selectedCase.status)}</dd><dt>Status order / pembayaran</dt><dd>{orderStatusLabel(selectedCase.orderStatus)} / {paymentStatusLabel(selectedCase.paymentStatus)}</dd><dt>Status provider</dt><dd>{providerStatusLabel(selectedCase.providerStatus)}{#if selectedCase.providerStatus && providerStatusLabel(selectedCase.providerStatus) === "Status penyedia belum dikenali"}<details><summary>Status teknis</summary><code>{selectedCase.providerStatus}</code></details>{/if}</dd><dt>Alasan</dt><dd>{selectedCase.reason}</dd><dt>Nominal</dt><dd>{money(selectedCase.amount)}</dd><dt>Pemeriksaan terakhir</dt><dd>{time(selectedCase.lastCheckedAt)}{selectedCase.lastCheckError ? ` · ${selectedCase.lastCheckError}` : ""}</dd></dl>
      {#if selectedCase.canRecheck}<section class="issue-action-group"><h3>Pengecekan pembayaran</h3><p class="staff-muted">Minta status transaksi terbaru dari penyedia pembayaran.</p><button class="scan-submit" type="button" disabled={busy || detailLoading || Boolean(uncertainOperation)} onclick={() => act("recheck")}>Periksa ulang pembayaran</button></section>{/if}
      {#if selectedCase.status === "OPEN"}<section class="issue-action-group"><h3>Catatan admin</h3><form onsubmit={(event) => { event.preventDefault(); void act("note"); }}><label>Catatan admin<textarea bind:value={note} maxlength="2000" required disabled={busy || detailLoading}></textarea></label><button class="staff-secondary-button" type="submit" disabled={busy || detailLoading || Boolean(uncertainOperation)}>Simpan catatan</button></form>
        {#if selectedCase.canResolve}<div class="issue-close-action"><h3>Penutupan kasus</h3><p class="staff-muted">Kasus akan ditutup dengan catatan di atas. Penutupan tidak mengajukan refund atau mengubah pembayaran.</p><button class="scan-submit" type="button" disabled={busy || detailLoading || Boolean(uncertainOperation) || !note.trim()} onclick={() => act("resolve")}>Tutup kasus dengan catatan ini</button></div>{/if}
      </section>{/if}
      <h3>Riwayat</h3><ul class="order-detail-list issue-history">{#each selectedCase.history as entry, i (`${entry.action}-${entry.createdAt}-${i}`)}<li><strong>{entry.action === "NOTE" ? "Catatan ditambahkan" : entry.action === "RESOLVE" ? "Kasus ditutup" : entry.action === "RECHECK" ? "Pembayaran diperiksa" : entry.action === "RETRY" ? "Email dijadwalkan ulang" : "Tindakan operasional"} · {entry.actorName}</strong><span>{auditSummary(entry)}</span><time>{time(entry.createdAt)}</time><details><summary>Detail teknis</summary><pre>{JSON.stringify(entry.data, null, 2)}</pre></details></li>{/each}</ul>
    </section>{/if}
    {#if selectedEmail}<section class="staff-admin-panel checkin-history-panel issue-detail-panel" aria-labelledby="email-detail-title">
      {#if selectedEmail.canRetry}<button class="scan-submit" type="button" disabled={busy || detailLoading || Boolean(uncertainOperation)} onclick={() => act("retry")}>{emailRetryActionLabel(selectedEmail)}</button>{/if}
      <h3 id="email-detail-title">Rincian pengiriman</h3>
      <dl class="email-facts">
        <dt>{selectedEmail.kind === "RECOVERY" && !selectedEmail.orderStatus ? "Referensi permintaan" : "Nomor pesanan"}</dt><dd>{selectedEmail.reference}{#if selectedEmail.kind === "RECOVERY" && !selectedEmail.orderStatus}<small>Kode yang dimasukkan pada permintaan pemulihan.</small>{/if}</dd>
        <dt>Jenis email</dt><dd>{emailKind(selectedEmail.kind)}</dd>
        <dt>Penerima</dt><dd>{selectedEmail.recipient || "Alamat email tidak tersedia"}</dd>
        <dt>Status pengiriman</dt><dd><span class="admin-order-status" class:is-expired={selectedEmail.status === "FAILED"} class:is-paid={selectedEmail.status === "SENT"}>{emailStatus(selectedEmail.status)}</span></dd>
        <dt>Terakhir diperbarui</dt><dd>{time(selectedEmail.updatedAt)}</dd>
        <dt>Jumlah percobaan pengiriman</dt><dd>{selectedEmail.attempts}</dd>
        {#if selectedEmail.orderStatus}<dt>Status pesanan</dt><dd>{orderStatusLabel(selectedEmail.orderStatus)}</dd>{/if}
      </dl>
      {#if selectedEmail.lastError}<details><summary>Detail teknis alasan gagal</summary><pre>{selectedEmail.lastError}</pre></details>{/if}
      {#if deliveryError}<p role="alert" class="staff-form-message staff-form-error">{deliveryError}</p>{/if}
      {#if delivery}<h3>Pengiriman ulang</h3><dl class="email-facts"><dt>Status pengiriman ulang</dt><dd><span class="admin-order-status" class:is-expired={delivery.status === "FAILED"} class:is-paid={delivery.status === "SENT"}>{emailStatus(delivery.status)}</span></dd><dt>Penerima pengiriman ulang</dt><dd>{delivery.recipient || "Alamat email tidak tersedia"}</dd><dt>Terakhir diperbarui</dt><dd>{time(delivery.updatedAt)}</dd><dt>Jumlah percobaan pengiriman</dt><dd>{delivery.attempts}</dd>{#if delivery.lastError}<dt>Alasan gagal</dt><dd>{emailFailureReason(delivery.lastError)}<details><summary>Detail teknis pengiriman ulang</summary><pre>{delivery.lastError}</pre></details></dd>{/if}</dl>{/if}
      <details><summary>Arti status pengiriman</summary><p class="staff-muted">Dijadwalkan ulang berarti masuk antrean, belum tentu terkirim. Status Terkirim berarti layanan email telah menerima pesan.</p></details>
      <h3>Riwayat</h3><ul class="order-detail-list issue-history">{#each selectedEmail.history as entry, i (`${entry.action}-${entry.createdAt}-${i}`)}<li><strong>{entry.action === "RETRY" ? "Pengiriman ulang dijadwalkan" : "Tindakan operasional"} · {entry.actorName}</strong><span>{auditSummary(entry)}</span><time>{time(entry.createdAt)}</time><details><summary>Detail teknis</summary><pre>{JSON.stringify(entry.data, null, 2)}</pre></details></li>{:else}<li>Belum ada riwayat tindakan admin.</li>{/each}</ul>
    </section>{/if}
</AdminLayout>

<style>
  #issue-detail-title { margin: 24px 0 16px; overflow-wrap: anywhere; }
  #issue-detail-title:focus { outline: 2px solid var(--scan-lime); outline-offset: 4px; }
  #issue-feedback:focus, .issue-workspace h2:focus { outline: 2px solid var(--scan-lime); outline-offset: 4px; }
  .email-facts { display: grid; grid-template-columns: minmax(0, 1fr) minmax(0, 2fr); gap: 16px 24px; margin: 20px 0; font-size: 16px; line-height: 1.5; }
  .email-facts dt { color: var(--scan-muted); }
  .email-facts dd { margin: 0; color: var(--scan-text); }
  .email-facts small { display: block; margin-top: 4px; color: var(--scan-muted); font-size: 14px; }
  @media (max-width: 540px) { .email-facts { grid-template-columns: minmax(0, 1fr) minmax(0, 1.3fr); gap: 16px 12px; } }
  .issue-detail-panel :is(dd, pre) { overflow-wrap: anywhere; }
  .issue-detail-panel pre { white-space: pre-wrap; font: 14px/1.5 monospace; }
  .issue-detail-panel summary { min-height: 44px; align-content: center; cursor: pointer; }
  .issue-detail-panel summary:focus-visible { outline: 2px solid var(--scan-lime); outline-offset: 4px; }
</style>
