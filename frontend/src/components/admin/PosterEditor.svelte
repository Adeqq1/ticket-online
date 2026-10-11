<script lang="ts">
  import { onDestroy } from "svelte";
  import { ApiError, uploadAdminPoster } from "../../lib/api.ts";

  let { image = $bindable(""), uploading = $bindable(false), pending = $bindable(false), artist, required, disabled, accessToken, onUnauthorized }: {
    image: string; uploading: boolean; pending: boolean; artist: string; required: boolean; disabled: boolean; accessToken: string; onUnauthorized: () => void;
  } = $props();
  let source = $state<"file" | "url">("file");
  let file = $state<File>();
  let localURL = $state("");
  let error = $state("");
  let notice = $state("");
  let failed = $state(false);
  let request: AbortController | undefined;
  let generation = 0;
  const preview = $derived(localURL || safeURL(image));

  function safeURL(value: string) {
    try { const url = new URL(value); return ["https:", "http:"].includes(url.protocol) && !url.username && !url.password ? url.href : ""; }
    catch { return ""; }
  }
  function cancel() {
    generation++; request?.abort(); request = undefined; uploading = false;
  }
  function clearFile() {
    cancel();
    if (localURL) URL.revokeObjectURL(localURL);
    localURL = ""; file = undefined; pending = false; failed = false; error = ""; notice = "";
  }
  function chooseSource(value: "file" | "url") { clearFile(); source = value; }
  function chooseFile(event: Event) {
    clearFile();
    file = (event.currentTarget as HTMLInputElement).files?.[0];
    if (!file) return;
    pending = true;
    if (!file.size || file.size > 5 * 1024 * 1024) { error = "Pilih file berisi gambar dengan ukuran maksimal 5 MiB."; return; }
    if (!["", "application/octet-stream", "image/jpeg", "image/png"].includes(file.type)) { error = "Pilih JPEG atau PNG. Ekspor HEIC ke JPEG atau PNG terlebih dahulu."; return; }
    localURL = URL.createObjectURL(file);
  }
  async function upload() {
    if (!file || !localURL || uploading || !pending || disabled) return;
    const controller = new AbortController(); request = controller;
    const current = ++generation;
    uploading = true; error = ""; notice = "";
    try {
      const result = await uploadAdminPoster(accessToken, file, controller.signal);
      if (current !== generation) return;
      image = result.url; pending = false;
      notice = "Poster terunggah. Simpan perubahan konser untuk menerapkannya.";
    } catch (cause) {
      if (current !== generation || (cause instanceof DOMException && cause.name === "AbortError")) return;
      if (cause instanceof ApiError && cause.status === 401) onUnauthorized();
      else error = cause instanceof ApiError ? cause.message : "Poster belum dapat diunggah. Coba lagi.";
    } finally { if (current === generation) { uploading = false; request = undefined; } }
  }

  onDestroy(clearFile);
</script>

<fieldset class="poster-editor" disabled={disabled} aria-describedby="poster-help">
  <legend>Poster konser</legend>
  <div class="poster-sources">
    <label><input type="radio" name="poster-source" value="file" checked={source === "file"} onchange={() => chooseSource("file")} />Unggah dari perangkat</label>
    <label><input type="radio" name="poster-source" value="url" checked={source === "url"} onchange={() => chooseSource("url")} />Gunakan URL gambar</label>
  </div>
  <p id="poster-help" class="staff-muted">JPEG atau PNG, maksimal 5 MiB. Dimensi maksimal 6.000 piksel per sisi dan total 20 megapiksel. Rasio 4:5 disarankan.</p>
  {#if source === "file"}
    <label class="poster-file">Pilih file poster<input type="file" accept="image/jpeg,image/png,.jpg,.jpeg,.png" onchange={chooseFile} aria-describedby="poster-help poster-public-note" /></label>
    {#if file}<p class="poster-file-details">{file.name} · {(file.size / 1024 / 1024).toLocaleString("id-ID", { maximumFractionDigits: 2 })} MiB</p>{/if}
    <button class="staff-secondary-button" type="button" onclick={upload} disabled={disabled || uploading || !pending || !localURL} aria-busy={uploading}>{uploading ? "Mengunggah poster…" : error && localURL ? "Coba lagi" : "Unggah poster"}</button>
    <p id="poster-public-note" class="staff-muted">File yang terunggah dapat diakses publik, termasuk sebelum konser diterbitkan. Upload belum menyimpan konser.</p>
    {#if pending}<p class="staff-muted">Unggah file pilihan sebelum menyimpan konser, atau pilih Gunakan URL gambar untuk membatalkannya.</p>{/if}
  {:else}
    <label>URL poster<input bind:value={image} oninput={() => { failed = false; notice = ""; }} type="url" maxlength="500" placeholder="https://…" {required} /></label>
  {/if}
  <div aria-live="polite" aria-atomic="true">
    {#if uploading}<p class="staff-muted" role="status">Mengunggah poster. Tunggu hingga selesai.</p>{/if}
    {#if notice}<p class="staff-form-message" role="status">{notice}</p>{/if}
    {#if error}<p class="staff-form-message staff-form-error" role="alert">{error}</p>{/if}
  </div>
  {#if preview}
    <div class="poster-preview"><h3>{pending && localURL ? "Pratinjau file pilihan" : "Pratinjau poster"}</h3>
      {#key preview}{#if failed}<p role="status">Poster tidak dapat dimuat. Periksa file atau alamat gambar.</p>{:else}<img src={preview} alt={`Pratinjau poster ${artist || "konser"}`} onerror={() => failed = true} />{/if}{/key}
    </div>
  {/if}
</fieldset>

<style>
  .poster-editor { min-width: 0; margin: 0; padding: 0; border: 0; display: grid; gap: 12px; }
  legend { padding: 0 0 12px; color: var(--scan-text); font-weight: 700; }
  .poster-sources { display: flex; flex-wrap: wrap; gap: 8px 20px; }
  .poster-sources label { display: flex; align-items: center; gap: 10px; min-height: 44px; cursor: pointer; }
  .poster-sources input { width: 20px; height: 20px; min-height: 20px; margin: 0; accent-color: var(--scan-lime); }
  .poster-file { min-width: 0; }
  .poster-file input { width: 100%; min-height: 44px; }
  .poster-file input::file-selector-button { min-height: 44px; margin-right: 10px; cursor: pointer; }
  .poster-file-details { overflow-wrap: anywhere; }
  p { margin: 0; }
  button { min-height: 44px; }
  .poster-preview { display: grid; gap: 10px; }
  .poster-preview h3 { margin: 0; color: var(--scan-muted); font-size: 14px; font-weight: 400; }
  .poster-preview img { display: block; width: min(100%, 280px); max-height: 380px; object-fit: contain; object-position: left center; background: var(--scan-bg); }
  @media (max-width: 600px) { .poster-sources { flex-direction: column; gap: 0; } button { width: 100%; } }
</style>
