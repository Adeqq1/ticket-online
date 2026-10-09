<script lang="ts">
  import { currentStart, eventStatus } from "../../lib/event-changes.ts";
  import { onMount, tick } from "svelte";
  import { eventDate } from "../../lib/concerts.ts";
  import { buyerOrderStatusLabel, refundStatusLabel } from "../../lib/buyer-refund.ts";
  import { ApiError, getOrder, listOrderTickets, type ApiTicket } from "../../lib/api.ts";
  import { buyerPaymentStatusLabel } from "../../lib/buyer-payment.ts";
  import { loadBuyerOrderTickets, uniqueBuyerTickets, type BuyerOrderTickets } from "../../lib/buyer-tickets.ts";
  import { hasPersistentTicketAccess, listOrderAccess, saveOrderTicketAccess } from "../../lib/order-access.ts";
  import { listTicketSnapshots } from "../../lib/tickets.ts";

  let orders = $state<BuyerOrderTickets[]>([]);
  let loading = $state(true);
  let loadingOrders = $state<string[]>([]);
  let checkingPayments = $state<string[]>([]);
  let paymentCheckErrors = $state<Record<string, string>>({});
  let paymentMessage = $state("");
  let paymentResult = $state<HTMLParagraphElement>();
  let listError = $state("");
  let demoDataPresent = $state(false);
  const tickets = $derived(uniqueBuyerTickets(orders));
  const ordersNeedingRetry = $derived(orders.filter((order) => order.error || order.detail?.status !== "PAID" || !order.tickets.length));
  const upcoming = (ticket: ApiTicket) => Date.parse(currentStart(ticket) ?? "") > Date.now();
  function persistTickets(access: BuyerOrderTickets["access"], ticketList: ApiTicket[]) {
    if (!ticketList.length) return;
    const saved = saveOrderTicketAccess(access, access.accessExpiresAt, ticketList.map((ticket) => ticket.id));
    if (!saved) listError = "Penyimpanan browser gagal. Kode tiket tetap terlihat di halaman ini, tetapi tautan e-ticket tidak akan tersedia setelah halaman ditutup.";
  }

  async function refreshOrder(orderId: string) {
    if (loadingOrders.includes(orderId)) return;
    loadingOrders = [...loadingOrders, orderId];
    const access = listOrderAccess().find((record) => record.orderId === orderId);
    if (!access) {
      listError = "Akses order tidak tersedia di browser ini.";
      loadingOrders = loadingOrders.filter((id) => id !== orderId);
      return;
    }
    const result = await loadBuyerOrderTickets(access, pageController?.signal);
    if (result.tickets.length) {
      persistTickets(access, result.tickets);
    }
    orders = [...orders.filter((order) => order.access.orderId !== orderId), result];
    loadingOrders = loadingOrders.filter((id) => id !== orderId);
  }

  async function checkPaymentStatus(orderId: string) {
    if (checkingPayments.includes(orderId)) return;
    const order = orders.find((item) => item.access.orderId === orderId);
    if (!order) return;
    checkingPayments = [...checkingPayments, orderId];
    paymentCheckErrors[orderId] = "";
    paymentMessage = "";
    try {
      const detail = await getOrder(orderId, order.access.accessToken, pageController?.signal);
      orders = orders.map((item) => item.access.orderId === orderId ? { ...item, detail } : item);
      paymentMessage = `Status pembayaran diperbarui: ${buyerPaymentStatusLabel(detail.status, detail.payment?.status)}.`;
    } catch (cause) {
      paymentCheckErrors[orderId] = cause instanceof Error ? cause.message : "Terjadi gangguan jaringan.";
      paymentMessage = "Status pembayaran terbaru belum dapat diperiksa.";
    } finally { checkingPayments = checkingPayments.filter((id) => id !== orderId); await tick(); paymentResult?.focus(); }
  }

  async function reloadTickets(orderId: string) {
    if (loadingOrders.includes(orderId)) return;
    const order = orders.find((item) => item.access.orderId === orderId);
    if (!order?.detail || !["PAID", "REFUND_PENDING", "REFUNDED"].includes(order.detail.status)) return;
    loadingOrders = [...loadingOrders, orderId];
    try {
      const tickets = await listOrderTickets(orderId, order.access.accessToken, pageController?.signal);
      const expected = order.detail.items.reduce((count, item) => count + item.quantity, 0);
      const error = tickets.length === expected ? null : new Error("Sebagian e-ticket belum dapat dimuat.");
      persistTickets(order.access, tickets);
      orders = orders.map((item) => item.access.orderId === orderId ? { ...item, tickets, error } : item);
    } catch (cause) {
      orders = orders.map((item) => item.access.orderId === orderId ? { ...item, error: cause instanceof Error ? cause : new Error("Daftar tiket belum dapat dimuat.") } : item);
    } finally { loadingOrders = loadingOrders.filter((id) => id !== orderId); }
  }

  onMount(() => {
    let active = true;
    pageController = new AbortController();
    demoDataPresent = listTicketSnapshots().length > 0;
    const savedOrders = listOrderAccess();
    Promise.all(savedOrders.map((order) => loadBuyerOrderTickets(order, pageController?.signal))).then((results) => {
      if (!active) return;
      orders = results;
      for (const result of results) persistTickets(result.access, result.tickets);
    }).catch(() => { if (active) listError = "Daftar order belum dapat dimuat."; }).finally(() => { if (active) loading = false; });
    return () => { active = false; pageController?.abort(); };
  });
  let pageController: AbortController | undefined;
</script>

{#snippet orderCard(order: BuyerOrderTickets)}
  <li>
    <article class="my-order-card">
      <div>
        <b>Kode pesanan (reference): {order.access.reference}</b>
        {#if order.detail}
          <p>Status pembayaran: {buyerPaymentStatusLabel(order.detail.status, order.detail.payment?.status)}. Status order: {buyerOrderStatusLabel(order.detail.status)}{#if order.detail.refund} · {refundStatusLabel(order.detail.refund.status)}{/if}</p>
          {#if buyerPaymentStatusLabel(order.detail.status, order.detail.payment?.status) === "Menunggu konfirmasi pembayaran"}<p>Pembayaran belum dikonfirmasi. Tiket tersedia setelah pembayaran dikonfirmasi.</p>{/if}
          {#if paymentCheckErrors[order.access.orderId]}<p role="alert">Status pembayaran terbaru belum dapat diperiksa. {paymentCheckErrors[order.access.orderId]} Status terakhir yang berhasil dibaca: {buyerPaymentStatusLabel(order.detail.status, order.detail.payment?.status)}.</p>{/if}
          {#if order.error}<p>Daftar tiket: {order.error.message}</p>{/if}
        {:else if order.error instanceof ApiError && order.error.code === "ACCESS_TOKEN_EXPIRED"}
          <p>Akses order di browser ini sudah kedaluwarsa.</p>
        {:else if order.error instanceof ApiError && order.error.status === 404}
          <p>Order tidak ditemukan dengan akses yang tersimpan.</p>
        {:else}
          <p>{order.error?.message ?? "Tiket belum tersedia."}</p>
        {/if}
      </div>
      <div class="my-order-actions">
        <a class="button button-secondary" href={`/pesanan/${encodeURIComponent(order.access.orderId)}`}>Detail pesanan</a>
        {#if order.detail?.status === "PENDING"}<button class="button button-secondary" type="button" disabled={checkingPayments.includes(order.access.orderId)} onclick={() => checkPaymentStatus(order.access.orderId)}>{checkingPayments.includes(order.access.orderId) ? "Memeriksa…" : "Periksa status pembayaran"}</button>{/if}
        {#if order.detail && ["PAID", "REFUND_PENDING", "REFUNDED"].includes(order.detail.status) && (!order.tickets.length || order.error)}<button class="button button-secondary" type="button" disabled={loadingOrders.includes(order.access.orderId)} onclick={() => reloadTickets(order.access.orderId)}>{loadingOrders.includes(order.access.orderId) ? "Memuat…" : "Muat ulang tiket"}</button>{/if}
        {#if !order.detail}<button class="button button-secondary" type="button" disabled={loadingOrders.includes(order.access.orderId)} onclick={() => refreshOrder(order.access.orderId)}>{loadingOrders.includes(order.access.orderId) ? "Memuat..." : "Coba muat ulang"}</button>{/if}
      </div>
    </article>
  </li>
{/snippet}

<svelte:head>
  <title>Tiket saya | Tiket Online</title>
  <meta name="description" content="Lihat e-ticket dari order yang tersimpan pada browser ini." />
  <meta name="robots" content="noindex" />
</svelte:head>

<section class="my-tickets shell">
  <header class="my-tickets-heading">
    <p class="ticket-kicker">Dompet tiket</p>
    <h1>Tiket saya.</h1>
    <p>Order dan tiket yang tersimpan dapat dibuka di sini. Tautan pada email konfirmasi juga dapat membuka e-ticket di perangkat lain.</p>
    <p><a class="text-button" href="/pulihkan-tiket">Sudah membeli, tetapi tiket tidak muncul?</a></p>
  </header>

  {#if demoDataPresent}<p class="legacy-ticket-note" role="status">Snapshot demo lama tersimpan di browser, tetapi bukan tiket backend dan tidak ditampilkan sebagai tiket masuk.</p>{/if}
  {#if listError}<p class="legacy-ticket-note" role="alert">{listError}</p>{/if}
  <p class="sr-only" role="status" aria-live="polite" tabindex="-1" bind:this={paymentResult}>{paymentMessage}</p>
  {#if loading}
    <p role="status">Memuat order dan e-ticket...</p>
  {:else if tickets.length}
    <div class="my-tickets-summary" aria-live="polite"><strong>{tickets.length} tiket</strong><span>Urut berdasarkan jadwal acara</span></div>
    <ul class="my-tickets-list" aria-label="Daftar e-ticket">
      {#each tickets as ticket (ticket.id)}
        {@const access = orders.find((order) => order.tickets.some((item) => item.id === ticket.id))?.access}
        {@const persistent = access ? hasPersistentTicketAccess(access.orderId, ticket.id) : false}
        <li>
          <article class="my-ticket-card">
            <div class="my-ticket-card-link">
              <div class="my-ticket-poster" aria-hidden="true">{ticket.code}</div>
              <div class="my-ticket-card-body">
                <div class="my-ticket-card-topline">
                  <span class:past={!upcoming(ticket)} class="my-ticket-status">{eventStatus(ticket.currentEvent) || (ticket.usable === false ? "Tidak aktif" : upcoming(ticket) ? "Mendatang" : "Selesai")}</span>
                  {#if access}<a class="my-ticket-reference" href={`/pesanan/${encodeURIComponent(access.orderId)}`}>Kode pesanan (reference): {ticket.orderReference}</a>{:else}<span class="my-ticket-reference">Kode pesanan (reference): {ticket.orderReference}</span>{/if}
                </div>
                <h2>{ticket.eventArtist}</h2>
                <p class="my-ticket-date">{eventDate(currentStart(ticket))}</p>
                <p class="my-ticket-venue">{ticket.eventVenue}, {ticket.eventCity}</p>
                <p class="my-ticket-venue">{ticket.attendeeName} · {ticket.tierName} · {ticket.gate}</p>
                {#if persistent}
                  <a class="my-ticket-action" href={`/tiket/${encodeURIComponent(ticket.id)}`}>Lihat e-ticket <span aria-hidden="true">→</span></a>
                {:else}
                  <p class="my-ticket-action">Kode tiket tersedia selama halaman ini terbuka.</p>
                {/if}
              </div>
            </div>
          </article>
        </li>
      {/each}
    </ul>
    {#if ordersNeedingRetry.length}
      <ul class="my-order-list" aria-label="Order yang perlu diperiksa">
        {#each ordersNeedingRetry as order (order.access.orderId)}
          {@render orderCard(order)}
        {/each}
      </ul>
    {/if}
  {:else if orders.length}
    <ul class="my-order-list" aria-label="Status order tersimpan">
      {#each orders as order (order.access.orderId)}
          {@render orderCard(order)}
      {/each}
    </ul>
  {:else}
    <section class="my-tickets-empty" aria-labelledby="empty-title"><div class="ticket-illustration" aria-hidden="true"><span></span><span></span><i></i><b></b></div><div><p class="ticket-kicker">Belum ada tiket</p><h2 id="empty-title">Dompetmu masih kosong.</h2><p>Akses pesanan mungkin belum tersimpan di browser ini. Pulihkan tiket memakai email pembeli dan kode pesanan dari email konfirmasi.</p><a class="button" href="/pulihkan-tiket">Pulihkan tiket</a><p><a class="text-button" href="/konser">Cari konser</a></p></div></section>
  {/if}
</section>
