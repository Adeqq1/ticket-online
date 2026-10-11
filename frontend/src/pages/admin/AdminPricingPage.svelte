<script lang="ts">
  import { onMount, tick, untrack } from "svelte";
  import { MediaQuery } from "svelte/reactivity";
  const mobile = new MediaQuery("(max-width: 767px)");
  let voucherOpen = $state(false);
  let voucherTrigger = "voucher-create-opener";
  let voucherListScroll = 0;
  import AdminLayout from "../../layouts/AdminLayout.svelte";
  import { ApiError, getAdminPricing, getAdminEvents, saveAdminFee, saveAdminVoucher, type PricingData, type PricingSettings, type Voucher, type AdminApiEvent } from "../../lib/api.ts";
  import { formatRupiah } from "../../lib/concerts.ts";
  import { parseRupiahInput, sameVoucher, uncertainPricingSave } from "../../lib/pricing-editor.ts";

  let { accessToken, onUnauthorized, onLogout, loggingOut }: { accessToken: string; onUnauthorized: () => void; onLogout: () => void; loggingOut: boolean } = $props();
  let data = $state<PricingData>();
  let events = $state<AdminApiEvent[]>([]);
  let loading = $state(true);
  let loadingEvents = $state(true);
  let pricingError = $state("");
  let eventsError = $state("");
  let pendingFee = $state<PricingSettings>();
  let pendingVoucher = $state<Voucher>();
  let saving = $state(false);
  let error = $state("");
  let notice = $state("");
  let fee = $state<number | undefined>();
  let sample = $state("100.000");
  let sampleError = $state("");
  const sampleValue = $derived(parseRupiahInput(sample));
  let editing = $state(false);
  const initial = emptyVoucher();
  let draft = $state<Voucher>(initial);
  let baseline = $state(JSON.stringify(initial));
  let starts = $state(localTime(initial.startsAt));
  let ends = $state(localTime(initial.endsAt));
  const voucherState = () => JSON.stringify({ draft, starts, ends });
  let voucherBaseline = $state(voucherState());
  $effect(() => { if (mobile.current) untrack(() => { if (voucherState() !== voucherBaseline || document.activeElement?.closest(".voucher-editor")) voucherOpen = true; }); });
  const dirty = $derived(Boolean(data) && (fee !== data?.settings.adminFee || voucherState() !== voucherBaseline));
  const money = (v: number) => formatRupiah.format(v);
  function localTime(value: string) { const d = new Date(value); return new Date(d.getTime() - d.getTimezoneOffset() * 60000).toISOString().slice(0, 16); }
  function emptyVoucher(): Voucher { return { code: "", kind: "PERCENT", value: 10, startsAt: new Date().toISOString(), endsAt: new Date(Date.now() + 7 * 86400000).toISOString(), active: false, eventId: "", minimumSubtotal: 0, quota: 100, revision: 0 }; }
  function discard() { return !dirty || window.confirm("Perubahan biaya atau voucher belum disimpan. Lanjutkan?"); }
  function protect(event: BeforeUnloadEvent) { if (dirty || saving || pendingFee || pendingVoucher) { event.preventDefault(); event.returnValue = ""; } }
  async function choose(value?: Voucher) {
    if (saving || pendingVoucher || (voucherState() !== voucherBaseline && !window.confirm("Buang perubahan voucher yang belum disimpan?"))) return;
    if (mobile.current) { voucherListScroll = window.scrollY; voucherTrigger = value ? `voucher-manage-${value.code}` : "voucher-create-opener"; }
    voucherOpen = true;
    draft = value ? { code: value.code, kind: value.kind, value: value.value, startsAt: value.startsAt, endsAt: value.endsAt, active: value.active, eventId: value.eventId, minimumSubtotal: value.minimumSubtotal, quota: value.quota, revision: value.revision } : emptyVoucher();
    editing = Boolean(value); starts = localTime(draft.startsAt); ends = localTime(draft.endsAt); baseline = JSON.stringify(draft); voucherBaseline = voucherState(); error = ""; notice = "";
    await tick(); document.getElementById(mobile.current ? "voucher-title" : "voucher-code")?.focus();
  }
  async function closeVoucher(saved = false) {
    if (!saved && (saving || pendingVoucher)) return;
    if (!saved && voucherState() !== voucherBaseline && !window.confirm("Buang perubahan voucher yang belum disimpan?")) return;
    if (!saved) { const before = JSON.parse(voucherBaseline); draft = before.draft; starts = before.starts; ends = before.ends; }
    voucherOpen = false;
    await tick();
    (document.getElementById(saved ? `voucher-manage-${draft.code}` : voucherTrigger) ?? document.getElementById("pricing-feedback"))?.focus({ preventScroll: true });
    window.scrollTo({ top: voucherListScroll, behavior: "instant" });
  }
  function report(cause: unknown) { if (cause instanceof ApiError && cause.status === 401) onUnauthorized(); else error = `Penyimpanan ditolak. ${cause instanceof Error ? cause.message : "Pengaturan belum dapat diproses."}`; }
  function formatSample(event: FocusEvent) {
    sampleError = sampleValue === undefined ? "Isi rupiah bulat yang valid, misalnya 100.000 (maksimal 1.000.000.000.000)." : "";
    (event.currentTarget as HTMLInputElement).setCustomValidity(sampleError);
    if (sampleValue !== undefined) sample = new Intl.NumberFormat("id-ID").format(sampleValue);
  }
  async function load() {
    loading = true; pricingError = "";
    try {
      const current = await getAdminPricing(accessToken);
      if (!data || fee === data.settings.adminFee) fee = current.settings.adminFee;
      data = current;
      if (pendingFee && current.settings.adminFee === pendingFee.adminFee && current.settings.revision > pendingFee.revision) {
        fee = current.settings.adminFee; pendingFee = undefined; error = ""; notice = "Biaya admin sesuai hasil penyimpanan pada data terbaru.";
      }
      if (pendingVoucher) {
        const found = current.vouchers.find(v => v.code === pendingVoucher?.code);
        if (found && sameVoucher(found, pendingVoucher) && (!editing || found.revision > pendingVoucher.revision)) {
          draft = { ...pendingVoucher, revision: found.revision }; editing = true; starts = localTime(draft.startsAt); ends = localTime(draft.endsAt); baseline = JSON.stringify(draft); voucherBaseline = voucherState(); pendingVoucher = undefined; error = ""; notice = "Voucher sesuai hasil penyimpanan pada data terbaru.";
        }
      }
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
    event.preventDefault(); if (saving || loading || pendingFee || !data || fee === undefined || !Number.isSafeInteger(fee) || fee < 0 || fee > 1_000_000_000 || sampleValue === undefined) return;
    if (!window.confirm(`Ubah biaya admin per order dari ${money(data.settings.adminFee)} menjadi ${money(fee)}? Hanya order baru yang terpengaruh.`)) return;
    saving = true; error = ""; notice = "";
    const input = { adminFee: fee, revision: data.settings.revision };
    try { const next = await saveAdminFee(accessToken, input); data.settings = next; notice = "Biaya admin tersimpan. Order lama tetap memakai nominal sebelumnya."; await load(); }
    catch (cause) { if (uncertainPricingSave(cause)) { pendingFee = input; error = "Hasil penyimpanan biaya belum pasti. Jangan ulangi penyimpanan; periksa data terbaru."; } else report(cause); } finally { saving = false; }
  }
  async function saveVoucher(event: SubmitEvent) {
    event.preventDefault(); if (saving || loading || loadingEvents || eventsError || pendingVoucher || !data) return;
    if (!Number.isFinite(Date.parse(starts)) || !Number.isFinite(Date.parse(ends))) { error = "Isi awal dan akhir masa berlaku yang valid."; return; }
    const input = { ...draft, code: draft.code.trim().toUpperCase(), startsAt: starts === localTime(draft.startsAt) ? draft.startsAt : new Date(starts).toISOString(), endsAt: ends === localTime(draft.endsAt) ? draft.endsAt : new Date(ends).toISOString() };
    if (Date.parse(input.endsAt) <= Date.parse(input.startsAt)) { error = "Akhir masa berlaku harus setelah awal masa berlaku."; return; }
    if (!window.confirm(`Simpan voucher ${input.code}: ${input.kind === "PERCENT" ? input.value + "%" : money(input.value)}, kuota ${input.quota}, ${input.active ? "aktif" : "nonaktif"}? Order yang sudah dibuat tetap memakai snapshot sebelumnya.`)) return;
    saving = true; error = ""; notice = "";
    let completed = false;
    try { const saved = await saveAdminVoucher(accessToken, input, !editing); draft = saved; editing = true; starts = localTime(saved.startsAt); ends = localTime(saved.endsAt); baseline = JSON.stringify(saved); voucherBaseline = voucherState(); notice = "Voucher tersimpan."; await load(); completed = true; }
    catch (cause) { if (uncertainPricingSave(cause)) { pendingVoucher = input; error = "Hasil penyimpanan voucher belum pasti. Jangan ulangi penyimpanan; periksa data terbaru."; } else report(cause); } finally { saving = false; }
    if (completed && mobile.current) await closeVoucher(true);
  }
  function describe(value: unknown): string {
    if (!value || typeof value !== "object") return "Belum ada";
    if ("adminFee" in value) return `Biaya admin ${money((value as PricingSettings).adminFee)}`;
    const v = value as Voucher;
    return `${v.code}: ${v.kind === "PERCENT" ? v.value + "%" : money(v.value)}, ${v.active ? "aktif" : "nonaktif"}, kuota ${v.quota}, minimum ${money(v.minimumSubtotal)}, ${v.eventId || "semua konser"}, ${new Date(v.startsAt).toLocaleString("id-ID")} sampai ${new Date(v.endsAt).toLocaleString("id-ID")}`;
  }
  onMount(() => { void load(); void loadEvents(); });
</script>

<svelte:window onbeforeunload={protect} />
<svelte:head><title>Biaya dan voucher | Gate Control</title><meta name="robots" content="noindex" /></svelte:head>
<AdminLayout page="pricing" contentId="pricing-content" skipLabel="Lewati ke biaya dan voucher" kicker="PENGATURAN BISNIS" title="Biaya dan voucher." description="Atur biaya per order dan promosi tanpa mengubah transaksi lama." onLogout={() => { if (!saving && discard()) onLogout(); }} {loggingOut} logoutDisabled={saving || loggingOut} secondaryAction={{ label: "Muat ulang", disabled: saving || loading, onclick: () => { void load(); } }}>
  <div aria-live="polite">{#if loading}<p role="status">Memuat pengaturan…</p>{/if}{#if notice}<p id="pricing-feedback" tabindex="-1" class="staff-form-message" role="status">{notice}</p>{/if}{#if error}<p class="staff-form-message staff-form-error" role="alert">{error}</p>{/if}{#if dirty}<p>Perubahan belum disimpan.</p>{/if}</div>
  {#if pricingError}<p class="staff-form-message staff-form-error" role="alert">{pricingError} <button class="staff-secondary-button" type="button" disabled={loading || saving} onclick={() => load()}>Coba lagi</button></p>{/if}
  {#if pendingFee || pendingVoucher}<p class="staff-form-message" role="status">Data terbaru harus sesuai perubahan yang dikirim sebelum tindakan dapat diulang.</p><button class="staff-secondary-button" type="button" disabled={loading || saving} onclick={() => load()}>Periksa hasil simpan</button>{/if}
  {#if data}
  <div class="pricing-grid" class:voucher-open={voucherOpen}>
    <section class="staff-admin-panel" aria-labelledby="fee-title">
      <h2 id="fee-title">Biaya admin per order</h2>
      <form onsubmit={saveFee}><fieldset disabled={saving || Boolean(pendingFee)}>
        <label>Nominal tetap (Rp)<input type="number" min="0" max="1000000000" step="1" required bind:value={fee} /></label>
        <label>Subtotal tiket untuk pratinjau (Rp)<input type="text" inputmode="numeric" required bind:value={sample} onblur={formatSample} oninput={(event) => { sampleError = ""; event.currentTarget.setCustomValidity(""); }} aria-invalid={Boolean(sampleError)} aria-describedby={sampleError ? "preview-error" : undefined} /></label>
        {#if sampleError}<p id="preview-error" role="alert">{sampleError}</p>{/if}
        <p aria-live="polite">{#if fee !== undefined && sampleValue !== undefined}Pratinjau tanpa voucher: {money(sampleValue)} + {money(fee)} = <strong>{money(sampleValue + fee)}</strong>.{/if}</p>
        <p>Nominal ini berlaku sekali per order baru. Biaya order lama dan nilai refund mengikuti transaksi yang tersimpan.</p>
        <button class="scan-submit" type="submit" disabled={saving || loading || sampleValue === undefined || fee === data?.settings.adminFee}>{saving ? "Menyimpan…" : "Simpan biaya admin"}</button>
      </fieldset></form>
    </section>
    <section class="staff-admin-panel voucher-editor" aria-labelledby="voucher-title">
      <div class="voucher-editor-heading"><h2 id="voucher-title" tabindex="-1">{editing ? `Ubah voucher ${draft.code}` : "Buat voucher"}</h2><button class="staff-secondary-button voucher-mobile-action" type="button" disabled={saving || Boolean(pendingVoucher)} onclick={() => closeVoucher()}>Kembali ke daftar</button></div>
      {#if loadingEvents}<p role="status">Memuat katalog konser…</p>{/if}
      {#if eventsError}<p class="staff-form-message staff-form-error" role="alert">{eventsError} <button class="staff-secondary-button" type="button" onclick={loadEvents} disabled={loadingEvents || saving}>Coba lagi katalog</button></p>{/if}
      <form onsubmit={saveVoucher}><fieldset disabled={saving || Boolean(pendingVoucher)}>
        <label>Kode voucher<input id="voucher-code" bind:value={draft.code} readonly={editing} required minlength="2" maxlength="32" pattern={"[A-Za-z0-9][A-Za-z0-9_\\-]{1,31}"} aria-describedby="code-help" /></label>
        <p id="code-help">2–32 huruf/angka, tanda hubung atau garis bawah. Kode tersimpan tidak dapat diganti; nonaktifkan bila promosi selesai.</p>
        <div class="field-row"><label>Jenis diskon<select bind:value={draft.kind}><option value="PERCENT">Persentase subtotal tiket</option><option value="FIXED">Nominal rupiah</option></select></label><label>Nilai diskon ({draft.kind === "PERCENT" ? "%" : "Rp"})<input type="number" min="1" max={draft.kind === "PERCENT" ? 100 : 1000000000} step="1" required bind:value={draft.value} /></label></div>
        <div class="field-row"><label>Mulai berlaku<input type="datetime-local" required bind:value={starts} /></label><label>Akhir masa berlaku<input type="datetime-local" required bind:value={ends} /></label></div>
        <p>Waktu mengikuti zona perangkat. Awal berlaku termasuk, akhir berlaku tidak termasuk.</p>
        <label>Cakupan konser<select bind:value={draft.eventId} disabled={loadingEvents || Boolean(eventsError)}><option value="">Semua konser</option>{#each events as event}<option value={event.id}>{event.artist} · {event.city}</option>{/each}</select></label>
        <div class="field-row"><label>Minimum subtotal tiket (Rp)<input type="number" min="0" max="1000000000000" step="1" required bind:value={draft.minimumSubtotal} /></label><label>Kuota order<input type="number" min="1" max="1000000000" step="1" required bind:value={draft.quota} /></label></div>
        <label class="checkbox"><input type="checkbox" bind:checked={draft.active} />Voucher aktif</label>
        <p>Satu voucher per order. Diskon maksimal sebesar subtotal tiket; biaya admin tetap dibayar.</p>
        <button class="scan-submit" type="submit" disabled={loading || loadingEvents || Boolean(eventsError) || (editing && JSON.stringify(draft) === baseline && starts === localTime(draft.startsAt) && ends === localTime(draft.endsAt))}>{saving ? "Menyimpan…" : editing ? "Simpan perubahan voucher" : "Buat voucher"}</button>
        <button class="staff-secondary-button voucher-desktop-action" type="button" onclick={() => choose()}>Voucher baru</button><button class="staff-secondary-button voucher-mobile-action" type="button" disabled={saving || Boolean(pendingVoucher)} onclick={() => closeVoucher()}>Batal</button>
      </fieldset></form>
    </section>
  </div>
  <section class="staff-admin-panel voucher-list-panel" class:voucher-open={voucherOpen} aria-labelledby="list-title"><button id="voucher-create-opener" class="scan-submit voucher-mobile-action" type="button" disabled={saving || loading || Boolean(pendingVoucher)} onclick={() => choose()}>Buat voucher</button><h2 id="list-title">Daftar dan penggunaan voucher</h2><p>Pending menahan kuota sampai expiry/rekonsiliasi selesai. Lunas memakai kuota permanen; refund tidak mengembalikannya. Pratinjau belum menahan kuota.</p>
    {#if mobile.current && !voucherOpen}
      {#if loadingEvents}<p role="status">Memuat katalog konser…</p>{/if}
      {#if eventsError}<p class="staff-form-message staff-form-error" role="alert">{eventsError} <button class="staff-secondary-button" type="button" onclick={loadEvents} disabled={loadingEvents || saving}>Coba lagi katalog</button></p>{/if}
    {/if}
    {#if data?.vouchers.length === 0}<p>Belum ada voucher.</p>{/if}
    <ul class="voucher-list">{#each data?.vouchers ?? [] as v (v.code)}<li><div><strong>{v.code}</strong><p>{v.kind === "PERCENT" ? `${v.value}%` : money(v.value)} · {v.active ? "Aktif" : "Nonaktif"} · {events.find(e => e.id === v.eventId)?.artist ?? (v.eventId || "Semua konser")}</p><p>Minimum {money(v.minimumSubtotal)} · {new Date(v.startsAt).toLocaleString("id-ID")} sampai {new Date(v.endsAt).toLocaleString("id-ID")}</p><p>Ditahan {v.usage.held} · Dipakai {v.usage.consumed} · Dilepas {v.usage.released} · Kuota {v.quota}</p></div><button id={`voucher-manage-${v.code}`} class="staff-secondary-button" type="button" disabled={saving || loading || Boolean(pendingVoucher)} onclick={() => choose(v)}>Kelola {v.code}</button></li>{/each}</ul>
  </section>
  <details class="pricing-history" open={!mobile.current}><summary>Riwayat perubahan</summary><section class="staff-admin-panel" aria-labelledby="history-title"><h2 id="history-title">Riwayat perubahan</h2><p>100 perubahan terakhir, termasuk pelaku dan nilai sebelum/sesudah.</p><ol class="history">{#each data?.history ?? [] as item}<li><strong>{item.actor}</strong> · {new Date(item.createdAt).toLocaleString("id-ID")}<p>Sebelum: {describe(item.before)}</p><p>Sesudah: {describe(item.after)}</p></li>{/each}</ol>{#if data?.history.length === 0}<p>Belum ada perubahan.</p>{/if}</section></details>
  {/if}
</AdminLayout>

<style>
  .voucher-mobile-action, .pricing-history > summary { display: none; }
  .voucher-editor-heading { display: flex; flex-wrap: wrap; align-items: start; justify-content: space-between; gap: 12px; }
  @media (max-width: 767px) {
    .pricing-grid:not(.voucher-open) .voucher-editor, .voucher-list-panel.voucher-open, .voucher-desktop-action { display: none; }
    .voucher-mobile-action { display: inline-grid; }
    .voucher-list-panel > .voucher-mobile-action { margin-bottom: 18px; }
    .pricing-history > summary { display: flex; align-items: center; min-height: 44px; padding: 12px 16px; border: 1px solid var(--admin-border); font-size: 18px; cursor: pointer; }
    .pricing-history section { margin-top: 0; }
    .pricing-history h2 { display: none; }
  }
  .pricing-grid { display: grid; grid-template-columns: minmax(0, 1fr) minmax(0, 1.4fr); gap: 24px; }
  section { margin: 24px 0; min-width: 0; } h2 { margin: 0 0 18px; } p { line-height: 1.6; overflow-wrap: anywhere; }
  fieldset { display: grid; gap: 16px; min-width: 0; border: 0; padding: 0; margin: 0; }
  label { display: grid; gap: 8px; min-width: 0; } input,select { width: 100%; min-width: 0; min-height: 44px; box-sizing: border-box; padding: 10px; border: 1px solid var(--scan-line); border-radius: 0; background: var(--scan-panel); color: var(--scan-text); font: inherit; }
  .checkbox { display: flex; align-items: center; min-height: 44px; } .checkbox input { width: 20px; min-height: 20px; }
  .field-row { display: grid; grid-template-columns: 1fr 1fr; gap: 16px; } button { min-height: 44px; }
  .voucher-list,.history { padding: 0; list-style: none; } li { padding: 20px 0; border-top: 1px solid var(--scan-line); overflow-wrap: anywhere; } .voucher-list li { display: flex; justify-content: space-between; align-items: center; gap: 20px; }
  :is(input,select):focus-visible { outline: 2px solid var(--scan-lime); outline-offset: 3px; }
  @media(max-width: 1000px) { .pricing-grid { grid-template-columns: 1fr; gap: 0; } }
  @media(max-width: 600px) { .field-row { grid-template-columns: 1fr; } .voucher-list li { display: grid; } button { width: 100%; } }
</style>
