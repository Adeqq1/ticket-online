<script lang="ts">
  import { onMount, tick } from "svelte";
  import { eventDate } from "../../lib/concerts.ts";
  import { buyerOrderStatusLabel, refundStatusLabel } from "../../lib/buyer-refund.ts";
  import { buyerPaymentStatusLabel } from "../../lib/buyer-payment.ts";
  import { buyerOrderCategory, buyerOrderIssue, buyerOrderNeedsRecovery, canResumeOrderPayment, groupBuyerOrders, loadBuyerOrderTickets, orderedTicketCount, usableBuyerTickets, type BuyerOrderTickets } from "../../lib/buyer-tickets.ts";
  import { listOrderAccess, orderAccessStorageUnreadable } from "../../lib/order-access.ts";

  let orders = $state<BuyerOrderTickets[]>([]);
  let loading = $state(true);
  let loadingOrders = $state<string[]>([]);
  let selected = $state<"active" | "pending" | "history">("active");
  let now = $state(Date.now());
  let storageUnreadable = $state(false);
  let message = $state("");
  let resultMessage = $state<HTMLParagraphElement>();
  let controller: AbortController;
  const groups = $derived(groupBuyerOrders(orders));
  const followups = $derived(orders.filter(({ detail }) => detail?.currentEvent?.status === "POSTPONED" || detail?.status === "REFUND_PENDING" || detail?.refundRight?.requested && detail.refund?.status !== "SUCCEEDED" || detail?.refund && detail.refund.status !== "SUCCEEDED"));
  const storageFailed = $derived(orders.some((order) => order.accessSaved === false));
  const categories = [{ id: "active", label: "Aktif", region: "Tiket aktif" }, { id: "pending", label: "Belum dibayar", region: "Pesanan belum dibayar" }, { id: "history", label: "Riwayat", region: "Riwayat pesanan" }] as const;

  async function refreshOrder(order: BuyerOrderTickets) {
    const id = order.access.orderId;
    if (loadingOrders.includes(id)) return null;
    loadingOrders = [...loadingOrders, id];
    try {
      const result = await loadBuyerOrderTickets(order.access, controller.signal);
      if (controller.signal.aborted) return null;
      orders = orders.map((item) => item.access.orderId === id ? result : item);
      now = Date.now();
      return result;
    } finally { loadingOrders = loadingOrders.filter((value) => value !== id); }
  }

  async function paymentAction(order: BuyerOrderTickets, resume: boolean) {
    const result = await refreshOrder(order);
    if (!result) return;
    if (resume && result.detail && canResumeOrderPayment(result.detail) && result.accessSaved !== false) {
      window.location.assign(`/pesanan/${encodeURIComponent(order.access.orderId)}#pembayaran`);
      return;
    }
    message = result.error ? buyerOrderIssue(result) : `Status pembayaran diperbarui: ${buyerPaymentStatusLabel(result.detail!.status, result.detail!.payment?.status)}.`;
    await tick(); resultMessage?.focus();
  }

  onMount(() => {
    controller = new AbortController();
    storageUnreadable = orderAccessStorageUnreadable();
    Promise.all(listOrderAccess().map((access) => loadBuyerOrderTickets(access, controller.signal))).then((results) => {
      if (!controller.signal.aborted) { orders = results; loading = false; }
    });
    const refresh = () => { if (!loading && document.visibilityState === "visible") for (const order of orders) void refreshOrder(order); };
    window.addEventListener("focus", refresh);
    document.addEventListener("visibilitychange", refresh);
    const timer = window.setInterval(() => { now = Date.now(); }, 1000);
    return () => { controller.abort(); window.clearInterval(timer); window.removeEventListener("focus", refresh); document.removeEventListener("visibilitychange", refresh); };
  });
</script>

{#snippet orderCard(order: BuyerOrderTickets)}
  {@const detail = order.detail}
  {@const category = buyerOrderCategory(order)}
  {@const usable = usableBuyerTickets(order)}
  {@const busy = loadingOrders.includes(order.access.orderId)}
  <li>
    <article class="my-ticket-card">
      <div class="my-ticket-card-link">
        <div class="my-ticket-mark" class:past={category !== "active"} aria-hidden="true">{detail?.status === "EXPIRED" ? "Pesanan kedaluwarsa" : detail?.status === "CANCELLED" ? "Pesanan dibatalkan" : detail?.status === "PENDING" ? "Menunggu pembayaran" : order.tickets.length ? "E-ticket" : "Pesanan"}</div>
        <div class="my-ticket-card-body">
          <h2>{detail?.eventArtist || "Informasi konser belum tersedia"}</h2>
          {#if detail?.eventArtist}
            <p class="my-ticket-date">{detail.currentEvent?.startsAt ? eventDate(detail.currentEvent.startsAt) : "Jadwal belum diumumkan"}</p>
            {#if detail.eventVenue || detail.eventCity}<p class="my-ticket-venue">{[detail.eventVenue, detail.eventCity].filter(Boolean).join(", ")}</p>{/if}
          {/if}
          {#if detail}<p class="my-ticket-venue">{orderedTicketCount(detail)} tiket dipesan{#if category === "active" || category === "history" && detail.status === "PAID"}{" · "}{usable.length} dapat digunakan{/if}</p>{/if}
          <p><span class:past={category !== "active"} class="my-ticket-status">{category === "review" ? "Perlu diperiksa" : detail?.status === "EXPIRED" ? "Pesanan kedaluwarsa" : detail?.status === "CANCELLED" ? "Pesanan dibatalkan" : detail?.currentEvent?.status === "POSTPONED" ? "Acara ditunda" : category === "active" ? "Tiket aktif" : category === "history" && detail?.status === "PAID" ? detail.currentEvent?.status === "CANCELLED" ? "Acara dibatalkan" : "Tiket tidak aktif" : detail ? buyerOrderStatusLabel(detail.status) : "Perlu diperiksa"}</span></p>
          {#if detail?.refund}<p class="my-ticket-event-update">{refundStatusLabel(detail.refund.status)}</p>{:else if detail?.refundRight?.requested}<p class="my-ticket-event-update">Refund sedang diperiksa</p>{/if}
          {#if detail?.status === "EXPIRED"}<p class="my-ticket-venue">Batas waktu pembayaran telah lewat.</p>{/if}
          {#if (detail?.status === "EXPIRED" || detail?.status === "CANCELLED") && detail.payment?.status === "SUCCEEDED"}<p class="my-ticket-event-update">Pembayaran diterima setelah pesanan berakhir. Periksa detail pengembalian dana.</p>{/if}
          {#if detail?.status === "PENDING"}
            <p class="my-ticket-venue">{detail.payment?.status === "SUCCEEDED" ? "Pembayaran diterima; menunggu konfirmasi pesanan." : detail.payment?.status === "PENDING" ? "Pembayaran belum selesai. Jika sudah membayar, periksa status pembayaran." : Date.parse(detail.expiresAt) <= now ? "Batas pembayaran lewat. Hasil pembayaran perlu diperiksa." : `Bayar sebelum ${eventDate(detail.expiresAt)}.`}</p>
          {/if}
          {#if category === "review" && detail?.status === "PAID"}<p class="my-ticket-venue">{order.tickets.length} e-ticket berhasil dimuat; kelayakan belum dapat dipastikan.</p>{/if}
          {#if category === "review"}<p class="my-ticket-venue">{buyerOrderIssue(order)}</p>{/if}
          <p class="my-ticket-reference">Nomor pesanan: {detail?.reference || order.access.reference}</p>
          <div class="my-order-actions">
            {#if order.accessSaved === false || buyerOrderNeedsRecovery(order)}
              <a class="button" href="/pulihkan-tiket">Pulihkan tiket</a>
            {:else if category === "active" && usable.length === 1}
              <a class="button" href={`/tiket/${encodeURIComponent(usable[0]!.id)}`}>Buka e-ticket</a>
            {:else if category === "active" && usable.length > 1}
              <details class="ticket-picker">
                <summary class="button">Buka e-ticket</summary>
                <ul aria-label="Pilih peserta dan kategori tiket">
                  {#each usable as ticket (ticket.id)}<li><a href={`/tiket/${encodeURIComponent(ticket.id)}`}>{ticket.attendeeName} · {ticket.tierName}</a></li>{/each}
                </ul>
              </details>
            {:else if category === "pending" && detail}
              <button class="button" type="button" disabled={busy} onclick={() => paymentAction(order, canResumeOrderPayment(detail, now))}>{busy ? "Memeriksa..." : canResumeOrderPayment(detail, now) ? "Lanjutkan pembayaran" : "Periksa status pembayaran"}</button>
              {#if detail.payment?.status === "PENDING" && canResumeOrderPayment(detail, now)}<button class="text-button" type="button" disabled={busy} onclick={() => paymentAction(order, false)}>Periksa status pembayaran</button>{/if}
            {:else if category === "review"}
              <button class="button" type="button" disabled={busy} onclick={() => refreshOrder(order)}>{busy ? "Memuat..." : "Coba lagi"}</button>
            {/if}
            {#if detail && order.accessSaved !== false}<a class="text-button" href={`/pesanan/${encodeURIComponent(order.access.orderId)}`}>Lihat detail</a>{/if}
          </div>
        </div>
      </div>
    </article>
  </li>
{/snippet}

<svelte:head>
  <title>Tiket saya | Tiket Online</title>
  <meta name="description" content="Lihat konser, status tiket, dan pesanan yang tersimpan di perangkat ini." />
  <meta name="robots" content="noindex" />
</svelte:head>

<section class="my-tickets shell">
  <header class="my-tickets-heading">
    <p class="ticket-kicker">Dompet tiket</p>
    <h1>Tiket saya.</h1>
    <p>Pesanan yang tersimpan di perangkat ini ditampilkan di sini. Untuk membuka tiket di perangkat lain, gunakan tautan pada email konfirmasi.</p>
    <p><a class="text-button" href="/pulihkan-tiket">Sudah membeli, tetapi tiket tidak muncul?</a></p>
  </header>
  {#if storageUnreadable}<p class="legacy-ticket-note" role="alert">Akses yang tersimpan di perangkat ini belum dapat dibaca. Sebagian pesanan mungkin belum muncul. Gunakan tautan email konfirmasi atau <a href="/pulihkan-tiket">pulihkan tiket</a>.</p>{/if}
  {#if storageFailed}<p class="legacy-ticket-note" role="alert">Akses pesanan belum tersimpan di perangkat ini. Gunakan tautan email konfirmasi atau <a href="/pulihkan-tiket">pulihkan tiket</a> untuk membuka kembali setelah halaman ditutup.</p>{/if}
  <p role="status" aria-live="polite" tabindex="-1" bind:this={resultMessage}>{message}</p>
  {#if loading}
    <p role="status">Memeriksa pesanan dan status tiket...</p>
  {:else}
    {#if followups.length}<p class="legacy-ticket-note" role="status">{followups.length} pesanan memiliki acara ditunda atau refund yang masih berjalan. Buka detail pesanan untuk tindak lanjut.</p>{/if}
    <nav class="ticket-categories" aria-label="Kategori pesanan">
      {#each categories as category}<button type="button" aria-pressed={selected === category.id} onclick={() => { selected = category.id; }}>{category.label} <span>{groups[category.id].length}</span></button>{/each}
    </nav>
    <section aria-label={categories.find((category) => category.id === selected)!.region}>
      {#if groups[selected].length}
        <div class="my-tickets-summary"><strong>{groups[selected].length} pesanan</strong><button class="text-button" type="button" disabled={loadingOrders.length > 0} onclick={() => { for (const order of orders) void refreshOrder(order); }}>Periksa lagi</button></div>
        <ul class="my-tickets-list" aria-label="Daftar pesanan">{#each groups[selected] as order (order.access.orderId)}{@render orderCard(order)}{/each}</ul>
      {:else if selected === "active"}
        <div class="my-tickets-empty"><div><h2>Belum ada tiket aktif.</h2><p>Temukan konser dan pesan tiketmu.</p>{#if groups.review.length}<p role="status">Sebagian status tiket belum dapat dipastikan. Periksa pesanan di bagian Perlu diperiksa.</p>{/if}<a class="button" href="/konser">Cari konser</a></div></div>
      {:else}<p role="status">{selected === "pending" ? "Tidak ada pesanan yang menunggu pembayaran." : "Belum ada riwayat pesanan."}</p>{/if}
    </section>
    {#if groups.review.length}
      <section class="ticket-review" aria-labelledby="review-title"><h2 id="review-title">Perlu diperiksa</h2><p>Status pesanan berikut belum dapat dipastikan.</p><ul class="my-tickets-list" aria-label="Pesanan perlu diperiksa">{#each groups.review as order (order.access.orderId)}{@render orderCard(order)}{/each}</ul></section>
    {/if}
  {/if}
</section>

<style>
  .my-ticket-mark.past { background: var(--canvas); color: var(--muted); }
  .my-ticket-card h2 { margin-top: 0; }
  .my-ticket-card-body > p { margin-block: 6px; }
  .my-ticket-reference { margin-top: 16px; font-size: 12px; }
  .my-order-actions { align-items: center; margin-top: auto; padding-top: 18px; }
  .ticket-categories { display: flex; flex-wrap: wrap; gap: 8px; margin-bottom: 24px; }
  .ticket-categories button { display: flex; align-items: center; gap: 8px; padding: 12px 16px; border: 1px solid var(--line); border-radius: var(--radius); background: var(--surface); color: var(--ink); font: inherit; font-weight: 700; cursor: pointer; }
  .ticket-categories button[aria-pressed="true"] { background: var(--accent); border-color: var(--accent); color: var(--accent-ink); }
  .ticket-categories span { font-variant-numeric: tabular-nums; }
  .ticket-picker { width: 100%; }
  .ticket-picker summary { cursor: pointer; }
  .ticket-picker ul { display: grid; gap: 12px; padding-left: 20px; }
  .ticket-picker a { overflow-wrap: anywhere; }
  .ticket-review { margin-top: 32px; }
  .my-tickets-empty { grid-template-columns: 1fr; min-height: 240px; padding-block: 28px; }
  .my-tickets-empty h2 { font-size: clamp(30px, 4vw, 44px); }
  @media (max-width: 767px) {
    .my-ticket-card-link { grid-template-columns: minmax(0, 1fr); }
    .my-ticket-mark { display: none; }
    .my-ticket-card-body { padding: 18px; }
    .ticket-categories { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 6px; }
    .ticket-categories button { justify-content: center; min-height: 48px; padding: 8px 6px; font-size: 14px; text-align: center; }
    .my-order-actions .button { width: 100%; }
  }
</style>
