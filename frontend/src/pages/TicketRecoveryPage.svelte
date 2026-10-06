<script lang="ts">
  import { onMount, tick } from "svelte";
  import { ApiError, getOrder, listOrderTickets, requestTicketRecovery, verifyTicketRecovery, type ApiTicket, type OrderDetail } from "../lib/api.ts";
  import { saveOrderAccess } from "../lib/order-access.ts";
  import { recoveredOrderAccess, recoveryToken } from "../lib/ticket-recovery.ts";

  let token = $state<string | null>(null);
  let email = $state("");
  let reference = $state("");
  let errors = $state({ email: "", reference: "" });
  let busy = $state(false);
  let requested = $state("");
  let formError = $state("");
  let verifyError = $state("");
  let linkSpent = $state(false);
  let unsaved = $state<{ detail: OrderDetail | null; tickets: ApiTicket[]; reference: string } | null>(null);

  onMount(() => {
    // Keep the token in memory only; the address bar and history never hold it after this point.
    token = recoveryToken(window.location.hash);
    if (window.location.hash) history.replaceState(history.state, "", window.location.pathname + window.location.search);
  });

  function validate() {
    errors = {
      email: /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email.trim()) ? "" : "Masukkan email pembeli yang valid.",
      reference: /^TO-[0-9a-f]{20}$/i.test(reference.trim()) ? "" : "Masukkan reference pesanan, misalnya TO- diikuti 20 karakter.",
    };
    return !errors.email && !errors.reference;
  }

  async function submitRequest(event: SubmitEvent) {
    event.preventDefault();
    if (busy) return;
    formError = "";
    if (!validate()) {
      await tick();
      document.querySelector<HTMLInputElement>(errors.email ? "#recovery-email" : "#recovery-reference")?.focus();
      return;
    }
    busy = true;
    try {
      requested = (await requestTicketRecovery(email.trim(), reference.trim())).message;
      await tick();
      document.querySelector<HTMLElement>("#recovery-requested")?.focus();
    } catch (value) {
      formError = value instanceof ApiError && value.status === 429
        ? `Terlalu banyak permintaan. Coba lagi dalam ${Math.ceil((value.retryAfter ?? 60) / 60)} menit.`
        : value instanceof ApiError ? value.message : "Permintaan belum dapat dikirim. Coba lagi.";
    } finally { busy = false; }
  }

  async function verify() {
    if (!token || busy) return;
    busy = true;
    verifyError = "";
    try {
      const result = await verifyTicketRecovery(token);
      // The token is now spent whatever happens next, so it is never sent again.
      token = null;
      linkSpent = true;
      if (saveOrderAccess(recoveredOrderAccess(result))) {
        window.location.assign(`/pesanan/${encodeURIComponent(result.orderId)}`);
        return;
      }
      unsaved = { detail: null, tickets: [], reference: result.reference };
      try {
        const [detail, tickets] = await Promise.all([getOrder(result.orderId, result.accessToken), listOrderTickets(result.orderId, result.accessToken)]);
        unsaved = { detail, tickets, reference: result.reference };
      } catch { /* the reference is still shown below */ }
      await tick();
      document.querySelector<HTMLElement>("#recovery-unsaved")?.focus();
    } catch (value) {
      // An unknown network outcome may already have consumed the token; never retry automatically.
      token = null;
      linkSpent = true;
      verifyError = value instanceof ApiError && value.status === 410 ? value.message
        : value instanceof ApiError && value.status === 429 ? "Terlalu banyak percobaan pemulihan. Coba lagi nanti dengan tautan baru."
        : "Hasil pemulihan belum dapat dipastikan. Jika tiket belum muncul di Tiket Saya, minta tautan pemulihan baru.";
    } finally { busy = false; }
  }
</script>

<svelte:head>
  <title>Pulihkan tiket | Tiket Online</title>
  <meta name="description" content="Pulihkan akses e-ticket di browser baru menggunakan email dan reference pesanan." />
  <meta name="robots" content="noindex" />
</svelte:head>

<section class="my-tickets recovery shell">
  <header class="my-tickets-heading">
    <p class="ticket-kicker">Pemulihan tiket</p>
    <h1>Pulihkan tiket.</h1>
    <p>Buka kembali e-ticket di browser baru. Tautan pemulihan dikirim ke email pembeli yang tersimpan pada pesanan.</p>
  </header>

  {#if unsaved}
    <div class="recovery-panel" id="recovery-unsaved" tabindex="-1" role="alert">
      <h2>Akses belum tersimpan di browser ini.</h2>
      <p>Penyimpanan browser diblokir, jadi pesanan hanya terlihat selama halaman ini terbuka. Catat kode tiket di bawah atau aktifkan penyimpanan lalu minta tautan baru.</p>
      <p><b>Pesanan {unsaved.reference}</b>{#if unsaved.detail} · {unsaved.detail.items.reduce((count, item) => count + item.quantity, 0)} tiket{/if}</p>
      {#if unsaved.tickets.length}
        <ul class="recovery-codes" aria-label="Kode e-ticket">
          {#each unsaved.tickets as ticket (ticket.id)}<li><span>{ticket.attendeeName} · {ticket.tierName} · {ticket.gate}</span><code>{ticket.code}</code></li>{/each}
        </ul>
      {:else}<p>Kode tiket belum dapat dimuat.</p>{/if}
    </div>
  {:else if token}
    <div class="recovery-panel">
      <h2>Tautan pemulihan siap.</h2>
      <p>Tekan tombol di bawah untuk menyimpan akses pesanan di browser ini. Tautan hanya dapat dipakai sekali.</p>
      <button class="button" type="button" disabled={busy} aria-busy={busy} onclick={verify}>{busy ? "Memulihkan..." : "Pulihkan tiket"}</button>
    </div>
  {:else}
    {#if verifyError}<p class="legacy-ticket-note" role="alert">{verifyError}</p>{/if}
    {#if requested}
      <div class="recovery-panel" id="recovery-requested" tabindex="-1" role="status">
        <h2>Periksa email pembeli.</h2>
        <p>{requested}</p>
        <p>Tautan berlaku 15 menit. Belum menerima email? Periksa folder spam, lalu kirim permintaan lagi setelah satu menit.</p>
        <button class="button button-secondary" type="button" onclick={() => { requested = ""; }}>Kirim permintaan lain</button>
      </div>
    {:else}
      <form class="recovery-panel field-grid" novalidate onsubmit={submitRequest} aria-describedby="recovery-form-error">
        {#if linkSpent}<p class="recovery-wide">Minta tautan baru dengan email dan reference pesanan.</p>{/if}
        <label for="recovery-email">Email pembeli
          <input id="recovery-email" type="email" autocomplete="email" maxlength="254" bind:value={email} required aria-invalid={Boolean(errors.email)} aria-describedby="recovery-email-error" class:is-invalid={Boolean(errors.email)} />
          <small id="recovery-email-error" aria-live="polite">{errors.email}</small>
        </label>
        <label for="recovery-reference">Reference pesanan
          <input id="recovery-reference" type="text" autocomplete="off" autocapitalize="characters" spellcheck="false" maxlength="23" placeholder="TO-…" bind:value={reference} required aria-invalid={Boolean(errors.reference)} aria-describedby="recovery-reference-hint recovery-reference-error" class:is-invalid={Boolean(errors.reference)} />
          <span id="recovery-reference-hint" class="recovery-hint">Tertera pada email e-ticket dan halaman checkout.</span>
          <small id="recovery-reference-error" aria-live="polite">{errors.reference}</small>
        </label>
        <p id="recovery-form-error" class="form-error recovery-wide" aria-live="polite">{formError}</p>
        <div class="recovery-wide"><button class="button" type="submit" disabled={busy} aria-busy={busy}>{busy ? "Mengirim..." : "Kirim tautan pemulihan"}</button></div>
      </form>
    {/if}
  {/if}
  <p class="recovery-footer"><a class="text-button" href="/tiket-saya">Kembali ke Tiket Saya</a></p>
</section>
