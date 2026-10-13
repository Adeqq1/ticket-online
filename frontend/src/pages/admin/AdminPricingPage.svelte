<script lang="ts">
  import AdminDateField from "../../components/admin/AdminDateField.svelte";
  import { onMount, tick } from "svelte";
  import { MediaQuery } from "svelte/reactivity";
  import AdminMobileList from "../../components/admin/AdminMobileList.svelte";
  import { mobileGroupSize, mobileListItems } from "../../lib/mobile-list.ts";
  const mobile = new MediaQuery("(max-width: 767px)");
  let voucherGroup = $state(0);
  let voucherScroll = $state(0);
  let voucherOpen = $state(false);
  let voucherStep = $state(0);
  const voucherSteps = ["Kode & diskon", "Ketentuan", "Tinjau"];
  let voucherTrigger = "voucher-create-opener";
  let voucherListScroll = 0;
  import AdminLayout from "../../layouts/AdminLayout.svelte";
  import AdminHelp from "../../components/admin/AdminHelp.svelte";
  import { ApiError, getAdminPricing, getAdminEvents, saveAdminFee, saveAdminVoucher, type PricingData, type PricingSettings, type Voucher, type AdminApiEvent } from "../../lib/api.ts";
  import { adminDate, adminTime } from "../../lib/admin-operations.ts";
  import { formatRupiah } from "../../lib/concerts.ts";
  import { pricingHistoryFields, sameVoucher, uncertainPricingSave } from "../../lib/pricing-editor.ts";

  let layout: ReturnType<typeof AdminLayout>;
  async function confirmAction(title: string, message: string, confirmLabel: string) {
    const token = accessToken;
    const before = confirmationState();
    return await layout.confirm({ title, message, confirmLabel }) && token === accessToken && before === confirmationState();
  }
  const confirmationState = () => JSON.stringify([fee, data?.settings, voucherState(), editing, pendingFee, pendingVoucher, saving, loading, loadingEvents, eventsError]);
  let { accessToken, onUnauthorized, onLogout, loggingOut }: { accessToken: string; onUnauthorized: () => void; onLogout: () => void; loggingOut: boolean } = $props();
  let data = $state<PricingData>();
  let events = $state<AdminApiEvent[]>([]);
  let loading = $state(true);
  let loadingEvents = $state(true);
  let pricingError = $state("");
  let eventsError = $state("");
  let pendingFee = $state<PricingSettings>();
  let pendingVoucher = $state<Voucher>();
  let pendingVoucherCreating = $state(false);
  let feeRecovery = $state<"unverified" | "checked" | "conflict">("unverified");
  let voucherRecovery = $state<"unverified" | "checked" | "conflict">("unverified");
  let saving = $state(false);
  let error = $state("");
  let notice = $state("");
  let fee = $state<number | undefined>();
  const feePresets = [0, 2500, 5000, 7500, 10000];
  let customFee = $state(false);
  const feeChoice = $derived(customFee || fee === undefined || !feePresets.includes(fee) ? "custom" : String(fee));
  const validFee = $derived(fee !== undefined && Number.isSafeInteger(fee) && fee >= 0 && fee <= 1_000_000_000);
  let editing = $state(false);
  const initial = emptyVoucher();
  let draft = $state<Voucher>(initial);
  let starts = $state(localTime(initial.startsAt));
  let ends = $state(localTime(initial.endsAt));
  let startsPending = $state(false), endsPending = $state(false);
  let voucherDateReset = $state(0);
  const voucherState = () => JSON.stringify({ draft, starts, ends, ...(startsPending || endsPending ? { startsPending, endsPending } : {}) });
  let voucherBaseline = $state(voucherState());
  const dirty = $derived(Boolean(data) && (fee !== data?.settings.adminFee || voucherState() !== voucherBaseline));
  const money = (v: number) => formatRupiah.format(v);
  const count = new Intl.NumberFormat("id-ID");
  function localTime(value: string) { const d = new Date(value); return new Date(d.getTime() - d.getTimezoneOffset() * 60000).toISOString().slice(0, 16); }
  function emptyVoucher(): Voucher { return { code: "", kind: "PERCENT", value: 10, startsAt: new Date().toISOString(), endsAt: new Date(Date.now() + 7 * 86400000).toISOString(), active: false, eventId: "", minimumSubtotal: 0, quota: 100, revision: 0 }; }
  async function discard() { return !dirty || (await confirmAction("Buang perubahan?", "Perubahan biaya atau voucher belum disimpan. Lanjutkan?", "Buang perubahan")); }
  function protect(event: BeforeUnloadEvent) { if (!layout?.consumeNavigationApproval() && (dirty || saving || pendingFee || pendingVoucher)) { event.preventDefault(); event.returnValue = ""; } }
  async function allowNavigation() {
    if (saving || pendingFee || pendingVoucher) { notice = "Selesaikan pemeriksaan hasil penyimpanan sebelum meninggalkan halaman."; return false; }
    return discard();
  }
  async function choose(value?: Voucher) {
    if (saving || pendingVoucher || (voucherState() !== voucherBaseline && !(await confirmAction("Buang perubahan?", "Buang perubahan voucher yang belum disimpan?", "Buang perubahan")))) return;
    voucherListScroll = window.scrollY; voucherTrigger = value ? `voucher-manage-${value.code}` : "voucher-create-opener";
    voucherOpen = true; voucherStep = 0; startsPending = endsPending = false; voucherDateReset++;
    draft = value ? { code: value.code, kind: value.kind, value: value.value, startsAt: value.startsAt, endsAt: value.endsAt, active: value.active, eventId: value.eventId, minimumSubtotal: value.minimumSubtotal, quota: value.quota, revision: value.revision } : emptyVoucher();
    editing = Boolean(value); starts = localTime(draft.startsAt); ends = localTime(draft.endsAt); voucherBaseline = voucherState(); error = ""; notice = "";
    await tick(); document.getElementById("voucher-title")?.focus();
  }
  async function closeVoucher(saved = false) {
    if (!saved && (saving || pendingVoucher)) return;
    if (!saved && voucherState() !== voucherBaseline && !(await confirmAction("Buang perubahan?", "Buang perubahan voucher yang belum disimpan?", "Buang perubahan"))) return;
    if (!saved) { const before = JSON.parse(voucherBaseline); draft = before.draft; starts = before.starts; ends = before.ends; }
    const index = data?.vouchers.findIndex(v => v.code === draft.code) ?? -1;
    if (saved && index >= 0 && voucherGroup !== Math.floor(index / mobileGroupSize)) {
      voucherGroup = Math.floor(index / mobileGroupSize); voucherScroll = 0;
      await tick();
    }
    if (saved && index < 0) notice = "Voucher tersimpan, tetapi belum tersedia di daftar. Muat ulang untuk memeriksa hasilnya.";
    voucherOpen = false; startsPending = endsPending = false; voucherDateReset++;
    await tick();
    const trigger = document.getElementById(saved ? `voucher-manage-${draft.code}` : voucherTrigger);
    trigger?.focus({ preventScroll: !saved });
    window.scrollTo({ top: voucherListScroll, behavior: "instant" });
    if (!trigger) document.getElementById("pricing-feedback")?.focus();
  }
  function report(cause: unknown) { if (cause instanceof ApiError && cause.status === 401) onUnauthorized(); else error = `Penyimpanan ditolak. ${cause instanceof Error ? cause.message : "Pengaturan belum dapat diproses."}`; }
  async function chooseFee(event: Event) {
    const choice = (event.currentTarget as HTMLSelectElement).value;
    customFee = choice === "custom";
    if (!customFee) fee = Number(choice);
    else { await tick(); document.getElementById("fee-amount")?.focus(); }
  }
  async function load(conflict?: "fee" | "voucher") {
    loading = true; pricingError = "";
    let confirmed = false;
    try {
      const current = await getAdminPricing(accessToken);
      if (!data || fee === data.settings.adminFee) fee = current.settings.adminFee;
      data = current;
      if (pendingFee) {
        if (current.settings.adminFee === pendingFee.adminFee && current.settings.revision > pendingFee.revision) {
          fee = current.settings.adminFee; pendingFee = undefined; error = ""; notice = "Biaya admin sesuai hasil penyimpanan pada data terbaru."; confirmed = true;
        } else {
          feeRecovery = conflict === "fee" || feeRecovery === "conflict" ? "conflict" : "checked";
          if (conflict === "fee") error = "Konflik biaya: data server berbeda dari draft. Periksa perbandingan sebelum memakai baseline baru.";
        }
      }
      if (pendingVoucher) {
        const found = current.vouchers.find(v => v.code === pendingVoucher?.code);
        if (found && sameVoucher(found, pendingVoucher) && (pendingVoucherCreating || found.revision > pendingVoucher.revision)) {
          draft = { ...pendingVoucher, revision: found.revision }; editing = true; starts = localTime(draft.startsAt); ends = localTime(draft.endsAt); voucherBaseline = voucherState(); pendingVoucher = undefined; error = ""; notice = "Voucher sesuai hasil penyimpanan pada data terbaru."; confirmed = true;
        } else {
          voucherRecovery = found && (conflict === "voucher" || voucherRecovery === "conflict") ? "conflict" : "checked";
          if (conflict === "voucher") error = "Konflik voucher: data server berbeda dari draft. Periksa perbandingan sebelum memakai baseline baru.";
        }
      }
      if (confirmed) { await tick(); document.getElementById("pricing-feedback")?.focus(); }
    } catch (cause) {
      if (cause instanceof ApiError && cause.status === 401) onUnauthorized();
      else pricingError = data ? "Pembaruan data biaya dan voucher gagal. Data terakhir dan draft tetap tersedia. Coba lagi." : "Pengaturan biaya dan voucher gagal dimuat. Coba lagi.";
    } finally { loading = false; }
  }
  async function loadEvents() {
    loadingEvents = true; eventsError = "";
    try { events = await getAdminEvents(accessToken); }
    catch (cause) { if (cause instanceof ApiError && cause.status === 401) onUnauthorized(); else eventsError = "Katalog konser gagal dimuat. Biaya tetap dapat disimpan; penyimpanan voucher ditahan sampai katalog berhasil dimuat."; }
    finally { loadingEvents = false; }
  }
  async function saveFee(event: SubmitEvent) {
    event.preventDefault(); if (saving || loading || pendingFee || !data || fee === undefined || !validFee) return;
    if (!(await confirmAction("Simpan biaya admin?", `Ubah biaya admin per order dari ${money(data.settings.adminFee)} menjadi ${money(fee)}? Hanya order baru yang terpengaruh.`, "Simpan biaya"))) return;
    const input = { adminFee: fee, revision: data.settings.revision };
    await sendFee(input);
  }
  async function sendFee(input: PricingSettings) {
    if (saving || loading || !data) return;
    saving = true; error = ""; notice = "";
    try { const next = await saveAdminFee(accessToken, input); pendingFee = undefined; fee = next.adminFee; data.settings = next; notice = "Biaya admin tersimpan. Order lama tetap memakai nominal sebelumnya."; await load(); }
    catch (cause) {
      if (cause instanceof ApiError && cause.status === 401) onUnauthorized();
      else if (pendingFee || uncertainPricingSave(cause)) {
        pendingFee = input; feeRecovery = "unverified"; error = "Hasil penyimpanan biaya belum pasti. Periksa data terbaru sebelum mengirim perubahan yang sama.";
        if (cause instanceof ApiError && cause.status === 409) { feeRecovery = "conflict"; await load("fee"); }
      } else report(cause);
    } finally { saving = false; }
  }
  async function retryFee() {
    if (saving || loading || pricingError || !pendingFee || feeRecovery !== "checked") return;
    await sendFee({ ...pendingFee });
  }
  async function acceptFeeBaseline() {
    if (saving || loading || pricingError || !data || !pendingFee || feeRecovery !== "conflict") return;
    fee = pendingFee.adminFee; pendingFee = undefined; error = "";
    notice = "Data server menjadi acuan biaya. Draft tetap tersedia; tinjau sebelum menyimpan lagi.";
    await tick(); (document.getElementById("fee-amount") ?? document.getElementById("fee-preset"))?.focus();
  }
  async function validateVoucher(through: number) {
    for (let index = 0; index <= through; index++) {
      const invalid = document.querySelector<HTMLInputElement | HTMLSelectElement>(`#voucher-fields-${index} :invalid`);
      if (invalid) { voucherStep = index; await tick(); invalid.reportValidity(); invalid.focus(); return false; }
    }
    const startAt = starts === localTime(draft.startsAt) ? draft.startsAt : starts;
    const endAt = ends === localTime(draft.endsAt) ? draft.endsAt : ends;
    if (through > 0 && (!Number.isFinite(Date.parse(startAt)) || !Number.isFinite(Date.parse(endAt)) || Date.parse(endAt) <= Date.parse(startAt))) {
      error = "Akhir masa berlaku harus setelah awal masa berlaku."; voucherStep = 1;
      await tick(); document.getElementById("voucher-ends")?.focus(); return false;
    }
    return true;
  }
  async function navigateVoucher(next: number) {
    if (saving || pendingVoucher) return;
    if (next > voucherStep && !await validateVoucher(next - 1)) return;
    voucherStep = next; error = "";
    await tick(); document.getElementById("voucher-section-title")?.focus();
  }
  async function saveVoucher(event: SubmitEvent) {
    event.preventDefault(); if (saving || loading || pendingVoucher || !data) return;
    if (voucherStep < 2) { await navigateVoucher(voucherStep + 1); return; }
    if (loadingEvents || eventsError || !await validateVoucher(1)) return;
    if (!Number.isFinite(Date.parse(starts)) || !Number.isFinite(Date.parse(ends))) { error = "Isi awal dan akhir masa berlaku yang valid."; return; }
    const input = { ...draft, code: draft.code.trim().toUpperCase(), startsAt: starts === localTime(draft.startsAt) ? draft.startsAt : new Date(starts).toISOString(), endsAt: ends === localTime(draft.endsAt) ? draft.endsAt : new Date(ends).toISOString() };
    if (Date.parse(input.endsAt) <= Date.parse(input.startsAt)) { error = "Akhir masa berlaku harus setelah awal masa berlaku."; return; }
    if (!(await confirmAction("Simpan voucher?", `Simpan voucher ${input.code}: ${input.kind === "PERCENT" ? input.value + "%" : money(input.value)}, kuota ${input.quota}, ${input.active ? "aktif" : "nonaktif"}? Order yang sudah dibuat tetap memakai snapshot sebelumnya.`, "Simpan voucher"))) return;
    await sendVoucher(input, !editing);
  }
  async function sendVoucher(input: Voucher, creating: boolean) {
    if (saving || loading || !data) return;
    saving = true; error = ""; notice = "";
    let completed = false;
    try { const saved = await saveAdminVoucher(accessToken, input, creating); pendingVoucher = undefined; draft = saved; editing = true; starts = localTime(saved.startsAt); ends = localTime(saved.endsAt); voucherBaseline = voucherState(); notice = "Voucher tersimpan."; await load(); completed = true; }
    catch (cause) {
      if (cause instanceof ApiError && cause.status === 401) onUnauthorized();
      else if (pendingVoucher || uncertainPricingSave(cause)) {
        pendingVoucher = input; pendingVoucherCreating = creating; voucherRecovery = "unverified"; error = "Hasil penyimpanan voucher belum pasti. Periksa data terbaru sebelum mengirim perubahan yang sama.";
        if (cause instanceof ApiError && cause.status === 409) { voucherRecovery = "conflict"; await load("voucher"); }
      } else report(cause);
    } finally { saving = false; }
    if (completed) await closeVoucher(true);
  }
  async function retryVoucher() {
    if (saving || loading || loadingEvents || eventsError || pricingError || !pendingVoucher || voucherRecovery !== "checked") return;
    await sendVoucher({ ...pendingVoucher }, pendingVoucherCreating);
  }
  async function acceptVoucherBaseline() {
    if (saving || loading || pricingError || !pendingVoucher || voucherRecovery !== "conflict") return;
    const found = data?.vouchers.find(v => v.code === pendingVoucher?.code);
    if (!found) return;
    const { usage, ...server } = found;
    draft = { ...pendingVoucher, revision: server.revision }; editing = true; starts = localTime(draft.startsAt); ends = localTime(draft.endsAt);
    voucherBaseline = JSON.stringify({ draft: server, starts: localTime(server.startsAt), ends: localTime(server.endsAt) });
    voucherStep = 2; pendingVoucher = undefined; error = ""; notice = "Data server menjadi acuan voucher. Draft tetap tersedia; tinjau sebelum menyimpan lagi.";
    await tick(); document.getElementById("voucher-title")?.focus();
  }
  onMount(() => { void load(); void loadEvents(); });
</script>

<svelte:window onbeforeunload={protect} />
{#snippet helpHeading(id: string, title: string, description: string)}
  <div class="pricing-help-heading"><AdminHelp id={`${id}-help`} label={`Penjelasan ${title.toLowerCase()}`} text={description}><h2 id={id} tabindex="-1">{title}</h2></AdminHelp></div>
{/snippet}
{#snippet historyDetails(value: unknown)}
  {@const fields = pricingHistoryFields(value)}
  {#if fields.length}
    <dl class="history-details">{#each fields as [label, content]}<div>
      <dt>{label}</dt><dd>{#if label === "Status" && (content === "Aktif" || content === "Nonaktif")}<span class="admin-order-status" class:is-paid={content === "Aktif"}>{content}</span>{:else if label === "Konser"}{events.find(event => event.id === content)?.artist ?? content}{:else}{content}{/if}</dd>
    </div>{/each}</dl>
  {:else}<p class="history-empty staff-muted">{value == null ? "Belum ada" : "Rincian perubahan tidak tersedia."}</p>{/if}
{/snippet}
<svelte:head><title>Biaya dan voucher | Gate Control</title><meta name="robots" content="noindex" /></svelte:head>
<AdminLayout bind:this={layout} onBeforeNavigate={allowNavigation} page="pricing" contentId="pricing-content" skipLabel="Lewati ke biaya dan voucher" kicker="PENGATURAN BISNIS" title="Biaya dan voucher." description="Atur biaya per order dan promosi tanpa mengubah transaksi lama." onLogout={async () => { if (!saving && await discard()) onLogout(); }} {loggingOut} logoutDisabled={saving || loggingOut} secondaryAction={{ label: "Muat ulang", disabled: saving || loading, onclick: () => { void load(); } }}>
  <div aria-live="polite">{#if loading}<p role="status">Memuat pengaturan…</p>{/if}{#if notice}<p id="pricing-feedback" tabindex="-1" class="staff-form-message" role="status">{notice}</p>{/if}{#if error}<p class="staff-form-message staff-form-error" role="alert">{error}</p>{/if}{#if dirty}<p>Perubahan belum disimpan.</p>{/if}</div>
  {#if pricingError}<p class="staff-form-message staff-form-error" role="alert">{pricingError} <button class="staff-secondary-button" type="button" disabled={loading || saving} onclick={() => load()}>Coba lagi</button></p>{/if}
  {#if pendingFee || pendingVoucher}<p class="staff-form-message" role="status">Penyimpanan belum terkonfirmasi. Periksa hasilnya; jika belum terbukti, gunakan tindakan di bawah untuk mengirim perubahan awal yang sama.</p><button class="staff-secondary-button" type="button" disabled={loading || saving} onclick={() => load()}>Periksa hasil simpan</button>{/if}
  {#if pendingFee && data}
    <section class="staff-admin-panel pricing-recovery" aria-labelledby="fee-recovery-title">
      <h2 id="fee-recovery-title">Periksa penyimpanan biaya</h2>
      <div class="pricing-comparison"><div><h3>Draft yang dikirim</h3>{@render historyDetails(pendingFee)}<p>Revisi awal: {pendingFee.revision}</p></div><div><h3>Data server terakhir dimuat</h3>{@render historyDetails(data.settings)}<p>Revisi: {data.settings.revision}</p></div></div>
      {#if feeRecovery === "conflict"}<p>Data terbaru berbeda. Memakai baseline baru mempertahankan draft dan meminta Anda meninjau serta menyimpan lagi.</p><button class="staff-secondary-button" type="button" disabled={saving || loading || Boolean(pricingError)} onclick={acceptFeeBaseline}>Gunakan data server sebagai baseline biaya baru</button>
      {:else}<button class="staff-secondary-button" type="button" disabled={saving || loading || Boolean(pricingError) || feeRecovery !== "checked"} onclick={retryFee}>Coba kirim perubahan biaya yang sama</button>{/if}
    </section>
  {/if}
  {#if pendingVoucher && data}
    {@const server = data.vouchers.find(v => v.code === pendingVoucher?.code)}
    <section class="staff-admin-panel pricing-recovery" aria-labelledby="voucher-recovery-title">
      <h2 id="voucher-recovery-title">Periksa penyimpanan voucher {pendingVoucher.code}</h2>
      <div class="pricing-comparison"><div><h3>Draft yang dikirim</h3>{@render historyDetails(pendingVoucher)}<p>{pendingVoucherCreating ? "Pembuatan voucher" : "Perubahan voucher"} · Revisi awal: {pendingVoucher.revision}</p></div><div><h3>Data server terakhir dimuat</h3>{#if server}{@render historyDetails(server)}<p>Revisi: {server.revision}</p>{:else}<p>Kode voucher belum tersedia pada data terbaru.</p>{/if}</div></div>
      {#if voucherRecovery === "conflict"}<p>Data terbaru berbeda. Memakai baseline baru mempertahankan draft dan meminta Anda meninjau serta menyimpan lagi.</p><button class="staff-secondary-button" type="button" disabled={saving || loading || Boolean(pricingError)} onclick={acceptVoucherBaseline}>Gunakan data server sebagai baseline voucher baru</button>
      {:else}<button class="staff-secondary-button" type="button" disabled={saving || loading || loadingEvents || Boolean(eventsError) || Boolean(pricingError) || voucherRecovery !== "checked"} onclick={retryVoucher}>Coba kirim perubahan voucher yang sama</button>{/if}
    </section>
  {/if}
  {#if data}
  <div class="pricing-grid">
    <section class="staff-admin-panel fee-panel" hidden={voucherOpen} aria-labelledby="fee-title">
      {@render helpHeading("fee-title", "Biaya admin per order", "Biaya berlaku sekali untuk setiap pesanan baru. Pesanan lama dan refund mengikuti nilai yang tersimpan.")}
      <form onsubmit={saveFee}><fieldset disabled={saving || Boolean(pendingFee)}>
        <label><span id="fee-preset-label">Nominal tetap (Rp)</span><select id="fee-preset" aria-labelledby="fee-preset-label" value={feeChoice} onchange={chooseFee}>{#each feePresets as amount}<option value={String(amount)}>{money(amount)}</option>{/each}<option value="custom">Isi sendiri</option></select></label>
        {#if feeChoice === "custom"}<label>Nominal kustom (Rp)<input id="fee-amount" type="number" inputmode="numeric" min="0" max="1000000000" step="1" required bind:value={fee} oninput={() => customFee = true} /></label>{/if}
        <label>Subtotal tiket untuk pratinjau (Rp)<input type="text" value="100.000" readonly /></label>
        <p aria-live="polite">{#if validFee && fee !== undefined}Pratinjau tanpa voucher: {money(100000)} + {money(fee)} = <strong>{money(100000 + fee)}</strong>.{/if}</p>
        <p class="pricing-explanation">Nominal ini berlaku sekali per order baru. Biaya order lama dan nilai refund mengikuti transaksi yang tersimpan.</p>
        <button class="scan-submit" type="submit" disabled={saving || loading || !validFee || fee === data?.settings.adminFee}>{saving ? "Menyimpan…" : "Simpan biaya admin"}</button>
      </fieldset></form>
    </section>
    <section class="staff-admin-panel voucher-editor" hidden={!voucherOpen} aria-labelledby="voucher-title">
      <div class="voucher-editor-heading"><h2 id="voucher-title" tabindex="-1">{editing ? `Ubah voucher ${draft.code}` : "Buat voucher"}</h2><button class="staff-secondary-button" type="button" disabled={saving || Boolean(pendingVoucher)} onclick={() => closeVoucher()}>Kembali ke daftar</button></div>
      {#if loadingEvents}<p role="status">Memuat katalog konser…</p>{/if}
      {#if eventsError}<p class="staff-form-message staff-form-error" role="alert">{eventsError} <button class="staff-secondary-button" type="button" onclick={loadEvents} disabled={loadingEvents || saving}>Coba lagi katalog</button></p>{/if}
      <div class="admin-wizard">
      <nav class="voucher-section-nav admin-wizard-nav" aria-label={editing ? "Bagian edit voucher" : "Langkah tambah voucher"}>{#each voucherSteps as label, index}<button class="staff-secondary-button" type="button" title={`${index + 1}. ${label}`} aria-current={voucherStep === index ? "step" : undefined} disabled={saving || Boolean(pendingVoucher)} onclick={() => navigateVoucher(index)}><svg class="admin-wizard-mobile-icon" width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d={["M4 5h16v5a2 2 0 0 0 0 4v5H4v-5a2 2 0 0 0 0-4Z", "M4 4h16v16H4ZM8 8h8M8 12h8M8 16h5", "m5 12 4 4L19 6"][index]} /></svg><span>{index + 1}. {label}</span></button>{/each}</nav>
      <div class="admin-wizard-content admin-wizard-body voucher-content">
      <h3 id="voucher-section-title" tabindex="-1">{voucherSteps[voucherStep]}</h3>
      <p aria-live="polite">Langkah {voucherStep + 1} dari 3</p>
      <form id="voucher-form" novalidate onsubmit={saveVoucher}><fieldset disabled={saving || Boolean(pendingVoucher)}>
        <div id="voucher-fields-0" class="voucher-fields" hidden={voucherStep !== 0}>
        <label class="voucher-full">Kode voucher<input id="voucher-code" bind:value={draft.code} readonly={editing} required minlength="2" maxlength="32" pattern={"[A-Za-z0-9][A-Za-z0-9_\\-]{1,31}"} aria-describedby="code-help" /></label>
        <p id="code-help" class="voucher-full">2–32 huruf/angka, tanda hubung atau garis bawah. Kode tersimpan tidak dapat diganti; nonaktifkan bila promosi selesai.</p>
        <label>Jenis diskon<select bind:value={draft.kind}><option value="PERCENT">Persentase subtotal tiket</option><option value="FIXED">Nominal rupiah</option></select></label><label>Nilai diskon ({draft.kind === "PERCENT" ? "%" : "Rp"})<input type="number" inputmode="numeric" min="1" max={draft.kind === "PERCENT" ? 100 : 1000000000} step="1" required bind:value={draft.value} /></label>
        </div>
        <div id="voucher-fields-1" class="voucher-fields" hidden={voucherStep !== 1}>
        <AdminDateField id="voucher-starts" label="Mulai berlaku" mode="datetime" required bind:value={starts} bind:pending={startsPending} resetKey={voucherDateReset} active={voucherStep === 1} disabled={saving || Boolean(pendingVoucher)} /><AdminDateField id="voucher-ends" label="Akhir masa berlaku" mode="datetime" required bind:value={ends} bind:pending={endsPending} resetKey={voucherDateReset} active={voucherStep === 1} disabled={saving || Boolean(pendingVoucher)} />
        <p class="voucher-full">Waktu mengikuti zona perangkat. Awal berlaku termasuk, akhir berlaku tidak termasuk.</p>
        <label class="voucher-full">Cakupan konser<select bind:value={draft.eventId} disabled={loadingEvents || Boolean(eventsError)}><option value="">Semua konser</option>{#each events as event}<option value={event.id}>{event.artist} · {event.city}</option>{/each}</select></label>
        <label>Minimum subtotal tiket (Rp)<input type="number" inputmode="numeric" min="0" max="1000000000000" step="1" required bind:value={draft.minimumSubtotal} /></label><label>Kuota order<input type="number" inputmode="numeric" min="1" max="1000000000" step="1" required bind:value={draft.quota} /></label>
        <label class="checkbox voucher-full"><input type="checkbox" bind:checked={draft.active} />Voucher aktif</label>
        <p class="voucher-full">Satu voucher per order. Diskon maksimal sebesar subtotal tiket; biaya admin tetap dibayar.</p>
        </div>
        <div hidden={voucherStep !== 2}>
          <dl class="voucher-review">
            <div><dt>Kode voucher</dt><dd>{draft.code.trim().toUpperCase()}</dd></div>
            <div><dt>Diskon</dt><dd>{draft.kind === "PERCENT" ? `${draft.value}%` : money(draft.value)}</dd></div>
            <div><dt>Mulai berlaku</dt><dd>{starts.replace("T", " ")} (zona perangkat)</dd></div>
            <div><dt>Akhir masa berlaku</dt><dd>{ends.replace("T", " ")} (zona perangkat)</dd></div>
            <div><dt>Cakupan konser</dt><dd>{events.find(event => event.id === draft.eventId)?.artist ?? (draft.eventId || "Semua konser")}</dd></div>
            <div><dt>Minimum subtotal</dt><dd>{money(draft.minimumSubtotal)}</dd></div>
            <div><dt>Kuota order</dt><dd>{count.format(draft.quota)}</dd></div>
            <div><dt>Status</dt><dd>{draft.active ? "Aktif" : "Nonaktif"}</dd></div>
          </dl>
          <p>Order yang sudah dibuat tetap memakai snapshot sebelumnya. Satu voucher per order; diskon maksimal sebesar subtotal tiket dan biaya admin tetap dibayar.</p>
        </div>
        <div class="voucher-actions">
          {#if voucherStep > 0}<button class="staff-secondary-button" type="button" onclick={() => navigateVoucher(voucherStep - 1)}>Sebelumnya</button>{/if}
          <button class="scan-submit" type="submit" disabled={loading || (voucherStep > 0 && (loadingEvents || Boolean(eventsError))) || (voucherStep === 2 && editing && voucherState() === voucherBaseline)}>{saving ? "Menyimpan…" : voucherStep < 2 ? "Lanjutkan" : editing ? "Simpan perubahan voucher" : "Buat voucher"}</button>
          <button class="staff-secondary-button" type="button" disabled={saving || Boolean(pendingVoucher)} onclick={() => closeVoucher()}>Batal</button>
        </div>
      </fieldset></form>
      </div></div>
    </section>
  </div>
  <section class="staff-admin-panel voucher-list-panel" hidden={voucherOpen} aria-labelledby="list-title"><button id="voucher-create-opener" class="scan-submit" type="button" disabled={saving || loading || Boolean(pendingVoucher)} onclick={() => choose()}><svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M4 5h16v5a2 2 0 0 0 0 4v5H4v-5a2 2 0 0 0 0-4Z" /><path d="M12 9v6m-3-3h6" /></svg><span>Buat voucher</span></button>{@render helpHeading("list-title", "Daftar dan penggunaan voucher", "Pesanan menunggu pembayaran menahan kuota hingga kedaluwarsa atau selesai diperiksa. Pesanan lunas memakai kuota; refund tidak mengembalikannya. Pratinjau tidak menahan kuota.")}<p class="pricing-explanation">Pending menahan kuota sampai expiry/rekonsiliasi selesai. Lunas memakai kuota permanen; refund tidak mengembalikannya. Pratinjau belum menahan kuota.</p>
    {#if mobile.current && !voucherOpen}
      {#if loadingEvents}<p role="status">Memuat katalog konser…</p>{/if}
      {#if eventsError}<p class="staff-form-message staff-form-error" role="alert">{eventsError} <button class="staff-secondary-button" type="button" onclick={loadEvents} disabled={loadingEvents || saving}>Coba lagi katalog</button></p>{/if}
    {/if}
    {#if data?.vouchers.length === 0}<p>Belum ada voucher.</p>{/if}
    {#if mobile.current && data?.vouchers.length}<AdminMobileList count={data.vouchers.length} bind:group={voucherGroup} bind:scrollLeft={voucherScroll} headingId="list-title" disabled={saving || loading || Boolean(pendingVoucher)}><ul class="admin-mobile-list voucher-list">
      {#each mobileListItems(data.vouchers, voucherGroup, true) as v (v.code)}
        <li class="admin-mobile-card">
            <div class="admin-mobile-card-header"><strong>{v.code}</strong><span class="admin-order-status" class:is-paid={v.active}>{v.active ? "Aktif" : "Nonaktif"}</span></div>
            <dl class="admin-card-fields voucher-card-details">
              <div><dt>Diskon</dt><dd>{v.kind === "PERCENT" ? `${count.format(v.value)}%` : money(v.value)}</dd></div>
              <div><dt>Konser</dt><dd>{events.find(e => e.id === v.eventId)?.artist ?? (v.eventId || "Semua konser")}</dd></div>
              <div><dt>Minimum subtotal</dt><dd>{money(v.minimumSubtotal)}</dd></div>
              <div><dt>Mulai berlaku</dt><dd><time datetime={v.startsAt}><span class="admin-card-date">{adminDate.format(new Date(v.startsAt))}</span><span class="admin-card-date">{adminTime.format(new Date(v.startsAt))} WIB</span></time></dd></div>
              <div><dt>Berakhir</dt><dd><time datetime={v.endsAt}><span class="admin-card-date">{adminDate.format(new Date(v.endsAt))}</span><span class="admin-card-date">{adminTime.format(new Date(v.endsAt))} WIB</span></time></dd></div>
              <div><dt>Kuota order</dt><dd>{count.format(v.quota)}</dd></div>
            </dl>
            <dl class="admin-card-fields" aria-label="Penggunaan voucher">
              <div><dt>Ditahan</dt><dd>{count.format(v.usage.held)}</dd></div>
              <div><dt>Dipakai</dt><dd>{count.format(v.usage.consumed)}</dd></div>
              <div><dt>Dilepas</dt><dd>{count.format(v.usage.released)}</dd></div>
            </dl>
          <button id={`voucher-manage-${v.code}`} class="staff-secondary-button" type="button" aria-label={`Kelola voucher ${v.code}`} disabled={saving || loading || Boolean(pendingVoucher)} onclick={() => choose(v)}>Kelola voucher</button>
        </li>
      {/each}
    </ul></AdminMobileList>
    {:else if data?.vouchers.length}
      <!-- svelte-ignore a11y_no_noninteractive_tabindex (The table needs keyboard focus for native horizontal scrolling.) -->
      <div class="history-table-wrap" tabindex="0" role="region" aria-labelledby="list-title">
        <table class="history-table admin-list-table voucher-table">
          <caption class="visually-hidden">Daftar voucher dan penggunaannya</caption>
          <thead><tr><th scope="col">Kode voucher</th><th scope="col">Diskon</th><th scope="col">Konser</th><th scope="col">Masa berlaku (WIB)</th><th scope="col">Kuota order</th><th scope="col">Penggunaan</th><th scope="col">Status</th><th scope="col">Aksi</th></tr></thead>
          <tbody>{#each data.vouchers as v (v.code)}<tr>
            <td><code>{v.code}</code></td>
            <td><strong class="admin-table-number">{v.kind === "PERCENT" ? `${v.value}%` : money(v.value)}</strong><small>Minimum subtotal<br /><span class="admin-table-number">{money(v.minimumSubtotal)}</span></small></td>
            <td>{events.find(e => e.id === v.eventId)?.artist ?? (v.eventId || "Semua konser")}</td>
            <td><div class="admin-table-stack"><div><small>Mulai</small><time datetime={v.startsAt}>{adminDate.format(new Date(v.startsAt))}<br />{adminTime.format(new Date(v.startsAt))} WIB</time></div><div><small>Berakhir</small><time datetime={v.endsAt}>{adminDate.format(new Date(v.endsAt))}<br />{adminTime.format(new Date(v.endsAt))} WIB</time></div></div></td>
            <td class="admin-table-number">{count.format(v.quota)}</td>
            <td><dl class="admin-table-pairs"><div><dt>Ditahan</dt><dd>{count.format(v.usage.held)}</dd></div><div><dt>Dipakai</dt><dd>{count.format(v.usage.consumed)}</dd></div><div><dt>Dilepas</dt><dd>{count.format(v.usage.released)}</dd></div></dl></td>
            <td><span class="admin-order-status" class:is-paid={v.active}>{v.active ? "Aktif" : "Nonaktif"}</span></td>
            <td><button id={`voucher-manage-${v.code}`} class="staff-secondary-button admin-icon-button" type="button" aria-label={`Kelola voucher ${v.code}`} title={`Kelola voucher ${v.code}`} disabled={saving || loading || Boolean(pendingVoucher)} onclick={() => choose(v)}><svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="m16 3 5 5-12 12-6 1 1-6Z" /><path d="m14 5 5 5" /></svg></button></td>
          </tr>{/each}</tbody>
        </table>
      </div>
    {/if}
  </section>
  <details class="pricing-history" hidden={voucherOpen} open={!mobile.current}>
    <summary>Riwayat perubahan</summary>
    <section class="staff-admin-panel" aria-labelledby="history-title">
      <h2 id="history-title">Riwayat perubahan</h2>
      <p>100 perubahan terakhir, termasuk pelaku dan nilai sebelum/sesudah.</p>
      {#if data.history.length === 0}<p>Belum ada perubahan.</p>
      {:else if mobile.current}
        <ol class="history">{#each data.history as item}<li><strong>{item.actor}</strong> · {new Date(item.createdAt).toLocaleString("id-ID")}<h3>Sebelum</h3>{@render historyDetails(item.before)}<h3>Sesudah</h3>{@render historyDetails(item.after)}</li>{/each}</ol>
      {:else}
        <!-- svelte-ignore a11y_no_noninteractive_tabindex (The table needs keyboard focus for native horizontal scrolling.) -->
        <div class="history-table-wrap" tabindex="0" role="region" aria-labelledby="history-title">
          <table class="history-table admin-list-table pricing-history-table">
            <caption class="visually-hidden">Riwayat perubahan biaya dan voucher</caption>
            <thead><tr><th scope="col">Waktu (WIB)</th><th scope="col">Admin</th><th scope="col">Sebelum</th><th scope="col">Sesudah</th></tr></thead>
            <tbody>{#each data.history as item}<tr>
              <td><time datetime={item.createdAt}>{adminDate.format(new Date(item.createdAt))}<br />{new Date(item.createdAt).toLocaleTimeString("id-ID", { timeZone: "Asia/Jakarta" })} WIB</time></td>
              <td>{item.actor}</td><td>{@render historyDetails(item.before)}</td><td>{@render historyDetails(item.after)}</td>
            </tr>{/each}</tbody>
          </table>
        </div>
      {/if}
    </section>
  </details>
  {/if}
</AdminLayout>

<style>
  .pricing-comparison { display: grid; grid-template-columns: repeat(auto-fit, minmax(min(100%, 18rem), 1fr)); gap: 24px; }
  .pricing-history-table { table-layout: fixed; }
  .pricing-history-table th:nth-child(-n+2) { width: 16%; }
  .history-details { display: grid; gap: 12px; margin: 0; line-height: 1.6; }
  .history-details > div { display: grid; grid-template-columns: repeat(auto-fit, minmax(min(100%, 7em), 1fr)); gap: 8px 12px; }
  .history-details dt { color: var(--admin-secondary); font-size: 14px; }
  .history-details dd { min-width: 0; margin: 0; color: var(--admin-text); font-size: 16px; overflow-wrap: anywhere; }
  .history-empty { margin: 0; }
  .pricing-help-heading { display: contents; }
  .pricing-history > summary { display: none; }
  .voucher-editor-heading { display: flex; flex-wrap: wrap; align-items: start; justify-content: space-between; gap: 12px; }
  @media (max-width: 767px) {
    .pricing-explanation { display: none; }
    .pricing-help-heading { display: block; margin-bottom: 18px; }
    #voucher-create-opener { display: flex; align-items: center; justify-content: center; gap: 10px; text-align: center; }
    #voucher-create-opener svg { flex-shrink: 0; }

    .pricing-history > summary { display: flex; align-items: center; min-height: 44px; padding: 12px 16px; border: 1px solid var(--admin-border); font-size: 18px; cursor: pointer; }
    .pricing-history section { margin-top: 0; }
    .pricing-history h2 { display: none; }
  }
  .pricing-grid { display: grid; grid-template-columns: minmax(0, 1fr); gap: 0; }
  [hidden] { display: none !important; }
  #voucher-create-opener { display: inline-flex; align-items: center; gap: 10px; margin-bottom: 16px; }
  .voucher-editor { container: voucher-form / inline-size; }
  .voucher-editor h3 { margin: 0 0 16px; }
  .voucher-editor p { margin: 0; }
  .voucher-editor > p { margin-bottom: 12px; }
  @media (min-width: 768px) { .voucher-content > p { margin-bottom: 12px; } }
  .voucher-section-nav, .voucher-actions { display: flex; flex-wrap: wrap; gap: 12px; margin-block: 16px; }
  .voucher-section-nav button { white-space: normal; }
  .voucher-section-nav [aria-current] { border-color: var(--admin-accent); background: var(--admin-input); }
  .voucher-editor fieldset, .voucher-fields { display: grid; gap: 12px; }
  .voucher-fields label { gap: 6px; }
  .voucher-full { grid-column: 1 / -1; }
  .voucher-review { display: grid; gap: 12px; margin: 0 0 12px; }
  .voucher-review > div { display: grid; gap: 6px; }
  .voucher-review dt { color: var(--scan-muted); }
  .voucher-review dd { margin: 0; min-width: 0; overflow-wrap: anywhere; }
  @container voucher-form (min-width: 35em) {
    .voucher-fields { grid-template-columns: repeat(2, minmax(0, 1fr)); }
    .voucher-review > div { grid-template-columns: minmax(0, 1fr) minmax(0, 2fr); }
  }
  section { margin: 24px 0; min-width: 0; } h2 { margin: 0 0 18px; } p { line-height: 1.6; overflow-wrap: anywhere; }
  fieldset { display: grid; gap: 16px; min-width: 0; border: 0; padding: 0; margin: 0; }
  label { display: grid; gap: 8px; min-width: 0; } input,select { width: 100%; min-width: 0; min-height: 44px; box-sizing: border-box; padding: 10px; border: 1px solid var(--scan-line); border-radius: 0; background: var(--scan-panel); color: var(--scan-text); font: inherit; }
  .checkbox { display: flex; align-items: center; min-height: 44px; } .checkbox input { width: 20px; min-height: 20px; }
  button { min-height: 44px; }
  .history { padding: 0; list-style: none; } .history li { padding: 20px 0; border-top: 1px solid var(--scan-line); overflow-wrap: anywhere; }
  :is(input,select):focus-visible { outline: 2px solid var(--scan-lime); outline-offset: 3px; }
  @media (max-width: 767px) { .voucher-content { container: voucher-form / inline-size; } }
  @media(max-width: 600px) { button { width: 100%; } }
</style>
