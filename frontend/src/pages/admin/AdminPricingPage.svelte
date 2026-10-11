<script lang="ts">
  import { onMount } from "svelte";
  import AdminLayout from "../../layouts/AdminLayout.svelte";
  import { ApiError, getAdminPricing, getAdminEvents, saveAdminFee, saveAdminVoucher, type PricingData, type PricingSettings, type Voucher, type AdminApiEvent } from "../../lib/api.ts";
  import { formatRupiah } from "../../lib/concerts.ts";

  let { accessToken, onUnauthorized, onLogout, loggingOut }: { accessToken: string; onUnauthorized: () => void; onLogout: () => void; loggingOut: boolean } = $props();
  let data = $state<PricingData>();
  let events = $state<AdminApiEvent[]>([]);
  let loading = $state(true);
  let saving = $state(false);
  let error = $state("");
  let notice = $state("");
  let fee = $state<number | undefined>();
  let sample = $state<number | undefined>(100_000);
  let editing = $state(false);
  const initial = emptyVoucher();
  let draft = $state<Voucher>(initial);
  let baseline = $state(JSON.stringify(initial));
  let starts = $state(localTime(initial.startsAt));
  let ends = $state(localTime(initial.endsAt));
  const voucherState = () => JSON.stringify({ draft, starts, ends });
  let voucherBaseline = $state(voucherState());
  const dirty = $derived(Boolean(data) && (fee !== data?.settings.adminFee || voucherState() !== voucherBaseline));
  const money = (v: number) => formatRupiah.format(v);
  function localTime(value: string) { const d = new Date(value); return new Date(d.getTime() - d.getTimezoneOffset() * 60000).toISOString().slice(0, 16); }
  function emptyVoucher(): Voucher { return { code: "", kind: "PERCENT", value: 10, startsAt: new Date().toISOString(), endsAt: new Date(Date.now() + 7 * 86400000).toISOString(), active: false, eventId: "", minimumSubtotal: 0, quota: 100, revision: 0 }; }
  function discard() { return !dirty || window.confirm("Perubahan biaya atau voucher belum disimpan. Lanjutkan?"); }
  function protect(event: BeforeUnloadEvent) { if (dirty) { event.preventDefault(); event.returnValue = ""; } }
  function choose(value?: Voucher) {
    if (saving || (voucherState() !== voucherBaseline && !window.confirm("Buang perubahan voucher yang belum disimpan?"))) return;
    draft = value ? { code: value.code, kind: value.kind, value: value.value, startsAt: value.startsAt, endsAt: value.endsAt, active: value.active, eventId: value.eventId, minimumSubtotal: value.minimumSubtotal, quota: value.quota, revision: value.revision } : emptyVoucher();
    editing = Boolean(value); starts = localTime(draft.startsAt); ends = localTime(draft.endsAt); baseline = JSON.stringify(draft); voucherBaseline = voucherState(); error = ""; notice = "";
    document.getElementById("voucher-code")?.focus();
  }
  function report(cause: unknown) { if (cause instanceof ApiError && cause.status === 401) onUnauthorized(); else error = cause instanceof Error ? cause.message : "Pengaturan belum dapat diproses."; }
  async function load(preserve = false) {
    loading = true; error = "";
    try {
      const [current, catalog] = await Promise.all([getAdminPricing(accessToken), getAdminEvents(accessToken)]);
      data = current; events = catalog;
      if (!preserve) { fee = current.settings.adminFee; draft = emptyVoucher(); editing = false; starts = localTime(draft.startsAt); ends = localTime(draft.endsAt); baseline = JSON.stringify(draft); voucherBaseline = voucherState(); }
    } catch (cause) { report(cause); } finally { loading = false; }
  }
  async function saveFee(event: SubmitEvent) {
    event.preventDefault(); if (saving || !data || fee === undefined || !Number.isSafeInteger(fee) || fee < 0 || fee > 1_000_000_000) return;
    if (!window.confirm(`Ubah biaya admin per order dari ${money(data.settings.adminFee)} menjadi ${money(fee)}? Hanya order baru yang terpengaruh.`)) return;
    saving = true; error = ""; notice = "";
    try { const next = await saveAdminFee(accessToken, { adminFee: fee, revision: data.settings.revision }); data.settings = next; notice = "Biaya admin tersimpan. Order lama tetap memakai nominal sebelumnya."; await load(true); }
    catch (cause) { report(cause); } finally { saving = false; }
  }
  async function saveVoucher(event: SubmitEvent) {
    event.preventDefault(); if (saving || !data) return;
    if (!Number.isFinite(Date.parse(starts)) || !Number.isFinite(Date.parse(ends))) { error = "Isi awal dan akhir masa berlaku yang valid."; return; }
    const input = { ...draft, code: draft.code.trim().toUpperCase(), startsAt: starts === localTime(draft.startsAt) ? draft.startsAt : new Date(starts).toISOString(), endsAt: ends === localTime(draft.endsAt) ? draft.endsAt : new Date(ends).toISOString() };
    if (Date.parse(input.endsAt) <= Date.parse(input.startsAt)) { error = "Akhir masa berlaku harus setelah awal masa berlaku."; return; }
    if (!window.confirm(`Simpan voucher ${input.code}: ${input.kind === "PERCENT" ? input.value + "%" : money(input.value)}, kuota ${input.quota}, ${input.active ? "aktif" : "nonaktif"}? Order yang sudah dibuat tetap memakai snapshot sebelumnya.`)) return;
    saving = true; error = ""; notice = "";
    try { const saved = await saveAdminVoucher(accessToken, input, !editing); draft = saved; editing = true; starts = localTime(saved.startsAt); ends = localTime(saved.endsAt); baseline = JSON.stringify(saved); voucherBaseline = voucherState(); notice = "Voucher tersimpan."; await load(true); }
    catch (cause) { report(cause); } finally { saving = false; }
  }
  function describe(value: unknown): string {
    if (!value || typeof value !== "object") return "Belum ada";
    if ("adminFee" in value) return `Biaya admin ${money((value as PricingSettings).adminFee)}`;
    const v = value as Voucher;
    return `${v.code}: ${v.kind === "PERCENT" ? v.value + "%" : money(v.value)}, ${v.active ? "aktif" : "nonaktif"}, kuota ${v.quota}, minimum ${money(v.minimumSubtotal)}, ${v.eventId || "semua konser"}, ${new Date(v.startsAt).toLocaleString("id-ID")} sampai ${new Date(v.endsAt).toLocaleString("id-ID")}`;
  }
  onMount(() => { void load(); });
</script>

<svelte:window onbeforeunload={protect} />
<svelte:head><title>Biaya dan voucher | Gate Control</title><meta name="robots" content="noindex" /></svelte:head>
<AdminLayout page="pricing" contentId="pricing-content" skipLabel="Lewati ke biaya dan voucher" kicker="PENGATURAN BISNIS" title="Biaya dan voucher." description="Atur biaya per order dan promosi tanpa mengubah transaksi lama." onLogout={() => { if (discard()) onLogout(); }} {loggingOut} logoutDisabled={saving || loggingOut} primaryAction={{ label: "Muat ulang", disabled: saving || loading, onclick: () => { if (discard()) void load(); } }}>
  <div aria-live="polite">{#if loading}<p role="status">Memuat pengaturan…</p>{/if}{#if notice}<p class="staff-form-message" role="status">{notice}</p>{/if}{#if error}<p class="staff-form-message staff-form-error" role="alert">{error}</p>{/if}{#if dirty}<p>Perubahan belum disimpan.</p>{/if}</div>
  <div class="pricing-grid">
    <section class="staff-admin-panel" aria-labelledby="fee-title">
      <h2 id="fee-title">Biaya admin per order</h2>
      <form onsubmit={saveFee}><fieldset disabled={loading || saving || !data}>
        <label>Nominal tetap (Rp)<input type="number" min="0" max="1000000000" step="1" required bind:value={fee} /></label>
        <label>Subtotal tiket untuk pratinjau (Rp)<input type="number" min="0" max="1000000000000" step="1" required bind:value={sample} /></label>
        <p aria-live="polite">{#if fee !== undefined && sample !== undefined}Pratinjau tanpa voucher: {money(sample)} + {money(fee)} = <strong>{money(sample + fee)}</strong>.{/if}</p>
        <p>Nominal ini berlaku sekali per order baru. Biaya order lama dan nilai refund mengikuti transaksi yang tersimpan.</p>
        <button class="scan-submit" type="submit" disabled={saving || fee === data?.settings.adminFee}>{saving ? "Menyimpan…" : "Simpan biaya admin"}</button>
      </fieldset></form>
    </section>
    <section class="staff-admin-panel" aria-labelledby="voucher-title">
      <h2 id="voucher-title">{editing ? `Ubah voucher ${draft.code}` : "Buat voucher"}</h2>
      <form onsubmit={saveVoucher}><fieldset disabled={loading || saving || !data}>
        <label>Kode voucher<input id="voucher-code" bind:value={draft.code} readonly={editing} required minlength="2" maxlength="32" pattern={"[A-Za-z0-9][A-Za-z0-9_\\-]{1,31}"} aria-describedby="code-help" /></label>
        <p id="code-help">2–32 huruf/angka, tanda hubung atau garis bawah. Kode tersimpan tidak dapat diganti; nonaktifkan bila promosi selesai.</p>
        <div class="field-row"><label>Jenis diskon<select bind:value={draft.kind}><option value="PERCENT">Persentase subtotal tiket</option><option value="FIXED">Nominal rupiah</option></select></label><label>Nilai diskon<input type="number" min="1" max={draft.kind === "PERCENT" ? 100 : 1000000000} step="1" required bind:value={draft.value} /></label></div>
        <div class="field-row"><label>Mulai berlaku<input type="datetime-local" required bind:value={starts} /></label><label>Akhir masa berlaku<input type="datetime-local" required bind:value={ends} /></label></div>
        <p>Waktu mengikuti zona perangkat. Awal berlaku termasuk, akhir berlaku tidak termasuk.</p>
        <label>Cakupan konser<select bind:value={draft.eventId}><option value="">Semua konser</option>{#each events as event}<option value={event.id}>{event.artist} · {event.city}</option>{/each}</select></label>
        <div class="field-row"><label>Minimum subtotal tiket (Rp)<input type="number" min="0" max="1000000000000" step="1" required bind:value={draft.minimumSubtotal} /></label><label>Kuota order<input type="number" min="1" max="1000000000" step="1" required bind:value={draft.quota} /></label></div>
        <label class="checkbox"><input type="checkbox" bind:checked={draft.active} />Voucher aktif</label>
        <p>Satu voucher per order. Diskon maksimal sebesar subtotal tiket; biaya admin tetap dibayar.</p>
        <button class="scan-submit" type="submit" disabled={editing && JSON.stringify(draft) === baseline && starts === localTime(draft.startsAt) && ends === localTime(draft.endsAt)}>{saving ? "Menyimpan…" : "Simpan voucher"}</button>
        <button class="staff-secondary-button" type="button" onclick={() => choose()}>Voucher baru</button>
      </fieldset></form>
    </section>
  </div>
  <section class="staff-admin-panel" aria-labelledby="list-title"><h2 id="list-title">Daftar dan penggunaan voucher</h2><p>Pending menahan kuota sampai expiry/rekonsiliasi selesai. Lunas memakai kuota permanen; refund tidak mengembalikannya. Pratinjau belum menahan kuota.</p>
    {#if data?.vouchers.length === 0}<p>Belum ada voucher.</p>{/if}
    <ul class="voucher-list">{#each data?.vouchers ?? [] as v (v.code)}<li><div><strong>{v.code}</strong><p>{v.kind === "PERCENT" ? `${v.value}%` : money(v.value)} · {v.active ? "Aktif" : "Nonaktif"} · {events.find(e => e.id === v.eventId)?.artist ?? (v.eventId || "Semua konser")}</p><p>Minimum {money(v.minimumSubtotal)} · {new Date(v.startsAt).toLocaleString("id-ID")} sampai {new Date(v.endsAt).toLocaleString("id-ID")}</p><p>Ditahan {v.usage.held} · Dipakai {v.usage.consumed} · Dilepas {v.usage.released} · Kuota {v.quota}</p></div><button class="staff-secondary-button" type="button" disabled={saving || loading} onclick={() => choose(v)}>Kelola {v.code}</button></li>{/each}</ul>
  </section>
  <section class="staff-admin-panel" aria-labelledby="history-title"><h2 id="history-title">Riwayat perubahan</h2><p>100 perubahan terakhir, termasuk pelaku dan nilai sebelum/sesudah.</p><ol class="history">{#each data?.history ?? [] as item}<li><strong>{item.actor}</strong> · {new Date(item.createdAt).toLocaleString("id-ID")}<p>Sebelum: {describe(item.before)}</p><p>Sesudah: {describe(item.after)}</p></li>{/each}</ol>{#if data?.history.length === 0}<p>Belum ada perubahan.</p>{/if}</section>
</AdminLayout>

<style>
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
