<script lang="ts">
  import { onMount } from "svelte";
  import { eventDate } from "../lib/concerts.ts";
  import { ApiError, type ApiTicket } from "../lib/api.ts";
  import { loadBuyerOrderTickets, uniqueBuyerTickets, type BuyerOrderTickets } from "../lib/buyer-tickets.ts";
  import { hasPersistentTicketAccess, listOrderAccess, saveOrderAccess } from "../lib/order-access.ts";
  import { listTicketSnapshots } from "../lib/tickets.ts";

  let orders = $state<BuyerOrderTickets[]>([]);
  let loading = $state(true);
  let loadingOrders = $state<string[]>([]);
  let listError = $state("");
  let demoDataPresent = $state(false);
  const tickets = $derived(uniqueBuyerTickets(orders));
  const ordersNeedingRetry = $derived(orders.filter((order) => order.error || order.detail?.status !== "PAID" || !order.tickets.length));
  const upcoming = (ticket: ApiTicket) => Date.parse(ticket.eventStartsAt) > Date.now();
  function persistTickets(access: BuyerOrderTickets["access"], ticketList: ApiTicket[]) {
    if (!ticketList.length) return;
    const saved = saveOrderAccess({ ...access, ticketIds: [...new Set([...access.ticketIds, ...ticketList.map((ticket) => ticket.id)])] });
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
  </header>

  {#if demoDataPresent}<p class="legacy-ticket-note" role="status">Snapshot demo lama tersimpan di browser, tetapi bukan tiket backend dan tidak ditampilkan sebagai tiket masuk.</p>{/if}
  {#if listError}<p class="legacy-ticket-note" role="alert">{listError}</p>{/if}
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
                  <span class:past={!upcoming(ticket)} class="my-ticket-status">{upcoming(ticket) ? "Mendatang" : "Selesai"}</span>
                  <span class="my-ticket-reference">{ticket.orderReference}</span>
                </div>
                <h2>{ticket.eventArtist}</h2>
                <p class="my-ticket-date">{eventDate(ticket.eventStartsAt)}</p>
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
          <li>
            <article class="my-order-card">
              <div>
                <b>Order {order.access.reference}</b>
                {#if order.detail}
                  <p>Status: {order.detail.status === "PAID" ? "Dibayar" : order.detail.status === "PENDING" ? "Menunggu pembayaran" : order.detail.status === "EXPIRED" ? "Kedaluwarsa" : "Dibatalkan"}</p>
                  {#if order.error}<p>{order.error.message}</p>{/if}
                {:else if order.error instanceof ApiError && order.error.code === "ACCESS_TOKEN_EXPIRED"}
                  <p>Akses order di browser ini sudah kedaluwarsa.</p>
                {:else if order.error instanceof ApiError && order.error.status === 404}
                  <p>Order tidak ditemukan dengan akses yang tersimpan.</p>
                {:else}
                  <p>{order.error?.message ?? "Tiket belum tersedia."}</p>
                {/if}
              </div>
              <button class="button button-secondary" type="button" disabled={loadingOrders.includes(order.access.orderId)} onclick={() => refreshOrder(order.access.orderId)}>
                {loadingOrders.includes(order.access.orderId) ? "Memuat..." : "Coba muat ulang"}
              </button>
            </article>
          </li>
        {/each}
      </ul>
    {/if}
  {:else if orders.length}
    <ul class="my-order-list" aria-label="Status order tersimpan">
      {#each orders as order (order.access.orderId)}
        <li>
          <article class="my-order-card">
            <div>
              <b>Order {order.access.reference}</b>
              {#if order.detail}
                <p>Status: {order.detail.status === "PAID" ? "Dibayar" : order.detail.status === "PENDING" ? "Menunggu pembayaran" : order.detail.status === "EXPIRED" ? "Kedaluwarsa" : "Dibatalkan"}</p>
                {#if order.error}<p>{order.error.message}</p>{/if}
              {:else if order.error instanceof ApiError && order.error.code === "ACCESS_TOKEN_EXPIRED"}
                <p>Akses order di browser ini sudah kedaluwarsa.</p>
              {:else if order.error instanceof ApiError && order.error.status === 404}
                <p>Order tidak ditemukan dengan akses yang tersimpan.</p>
              {:else}
                <p>{order.error?.message ?? "Tiket belum tersedia."}</p>
              {/if}
            </div>
            <button class="button button-secondary" type="button" disabled={loadingOrders.includes(order.access.orderId)} onclick={() => refreshOrder(order.access.orderId)}>
              {loadingOrders.includes(order.access.orderId) ? "Memuat..." : "Coba muat ulang"}
            </button>
          </article>
        </li>
      {/each}
    </ul>
  {:else}
    <section class="my-tickets-empty" aria-labelledby="empty-title"><div class="ticket-illustration" aria-hidden="true"><span></span><span></span><i></i><b></b></div><div><p class="ticket-kicker">Belum ada tiket</p><h2 id="empty-title">Dompetmu masih kosong.</h2><p>Setelah pembayaran dikonfirmasi server, e-ticket setiap peserta muncul di sini.</p><a class="button" href="/konser">Jelajahi Konser</a></div></section>
  {/if}
</section>
