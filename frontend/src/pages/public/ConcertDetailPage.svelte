<script lang="ts">
  import EventNotice from "../../components/EventNotice.svelte";
  import { canBuy, eventStatus } from "../../lib/event-changes.ts";
  import { onMount } from "svelte";
  import { getEvent, ApiError } from "../../lib/api.ts";
  import { eventDate, formatRupiah, parseQuantities, type Concert } from "../../lib/concerts.ts";
  import NotFoundPanel from "../../components/NotFoundPanel.svelte";
  import ConcertInfoTabs from "../../components/detail/ConcertInfoTabs.svelte";
  import SeatingMap from "../../components/detail/SeatingMap.svelte";
  import { cartTotal, changeQuantity, quantityParams, ticketCount, type DetailState } from "../../lib/cart.ts";
  import { recordConversion, trackConversionActivity } from "../../lib/conversion.ts";
  let { id }: { id: string } = $props();
  let isDesktop = $state(false);
  let zoneMapDetails: HTMLDetailsElement | undefined = $state();
  let mobileCart: HTMLDivElement | undefined = $state();
  let cartObserver: ResizeObserver | undefined = $state();
  let cartRootStyle: CSSStyleDeclaration | undefined;
  let concertData: Concert | undefined = $state();
  let loading: boolean = $state(true);
  let error: string = $state("");
  function initialDetailState(): DetailState {
    const quantities = concertData ? parseQuantities(concertData, new URLSearchParams(location.search)) : {};
    const selectedTier = concertData?.ticketTiers.find((tier) => quantities[tier.id]);
    return { selectedZoneId: selectedTier?.zoneId ?? null, quantities };
  }
  let detailState: DetailState = $state({ selectedZoneId: null, quantities: {} });
  const count = $derived(ticketCount(detailState.quantities));
  const total = $derived(concertData ? cartTotal(concertData, detailState.quantities) : 0);
  const selected = $derived(concertData?.ticketTiers.filter((tier) => detailState.quantities[tier.id]) ?? []);
  const saleOpen = $derived(Boolean(concertData && concertData.status !== "Sold Out" && canBuy(concertData.currentEvent)));
  $effect(() => {
    const details = zoneMapDetails;
    if (details) details.open = isDesktop;
  });
  $effect(() => {
    const cart = mobileCart;
    const observer = cartObserver;
    const rootStyle = cartRootStyle;
    if (!cart || !observer || !rootStyle) return;
    observer.observe(cart);
    const height = Math.ceil(cart.getBoundingClientRect().height);
    rootStyle.setProperty("--detail-mobile-cart-height", `${height}px`);
    rootStyle.scrollPaddingBottom = "calc(var(--detail-mobile-cart-height, 0px) + 16px)";
    return () => observer.unobserve(cart);
  });
  function adjust(tierId: string, delta: -1 | 1) { if (concertData) detailState = changeQuantity(concertData, detailState, tierId, delta); }
  function selectZone(zoneId: string, event: MouseEvent | KeyboardEvent) {
    detailState.selectedZoneId = zoneId;
    const zoneTiers = concertData?.ticketTiers.filter((item) => item.zoneId === zoneId) ?? [];
    const tier = zoneTiers.find((item) => item.stock > 0) ?? zoneTiers[0];
    requestAnimationFrame(() => { const element = document.getElementById(`tier-${tier?.id}`); element?.scrollIntoView({ behavior: matchMedia("(prefers-reduced-motion: reduce)").matches ? "auto" : "smooth", block: "nearest" }); if (event instanceof KeyboardEvent) element?.querySelector<HTMLButtonElement>("button:not(:disabled)")?.focus(); });
  }
  function checkout() { if (concertData && saleOpen && count) location.assign(`/checkout/${concertData.id}?${quantityParams(detailState.quantities)}`); }
  function keepFocusedControlVisible(event: FocusEvent) {
    const target = event.target;
    if (!(target instanceof HTMLElement) || !target.matches(":focus-visible") || target.closest(".mobile-cart")) return;
    requestAnimationFrame(() => {
      if (!mobileCart?.getClientRects().length) return;
      const overlap = target.getBoundingClientRect().bottom - mobileCart.getBoundingClientRect().top;
      if (overlap > 0) window.scrollBy({ top: overlap + 12, behavior: "instant" });
    });
  }
  async function load(signal?: AbortSignal) { loading = true; error = ""; try { concertData = await getEvent(id, signal); detailState = initialDetailState(); if (!signal?.aborted) recordConversion(concertData.id, "DETAIL_VIEWED"); } catch (value) { if (!(value instanceof DOMException && value.name === "AbortError")) error = value instanceof ApiError && value.code === "EVENT_NOT_FOUND" ? "not-found" : value instanceof ApiError ? value.message : "Detail konser belum dapat dimuat."; } finally { if (!signal?.aborted) loading = false; } }
  onMount(() => {
    const controller = new AbortController();
    const stopTracking = trackConversionActivity(id);
    const desktop = window.matchMedia("(min-width: 768px)");
    const syncViewport = () => { isDesktop = desktop.matches; };
    const rootStyle = document.documentElement.style;
    const previousScrollPadding = rootStyle.scrollPaddingBottom;
    const previousCartHeight = rootStyle.getPropertyValue("--detail-mobile-cart-height");
    cartRootStyle = rootStyle;
    const resizeObserver = new ResizeObserver((entries) => {
      for (const entry of entries) rootStyle.setProperty("--detail-mobile-cart-height", `${Math.ceil(entry.target.getBoundingClientRect().height)}px`);
    });
    cartObserver = resizeObserver;
    syncViewport();
    desktop.addEventListener("change", syncViewport);
    void load(controller.signal);
    return () => {
      controller.abort(); stopTracking(); desktop.removeEventListener("change", syncViewport); cartObserver = undefined; resizeObserver.disconnect(); cartRootStyle = undefined;
      rootStyle.scrollPaddingBottom = previousScrollPadding;
      if (previousCartHeight) rootStyle.setProperty("--detail-mobile-cart-height", previousCartHeight); else rootStyle.removeProperty("--detail-mobile-cart-height");
    };
  });
</script>

<svelte:head><title>{concertData ? `${concertData.artist} | Tiket Online` : "Konser tidak ditemukan | Tiket Online"}</title><meta name="description" content={concertData ? `Detail konser dan pilihan tiket ${concertData.artist} di Tiket Online.` : "Konser tidak ditemukan di Tiket Online."} />{#if !concertData}<meta name="robots" content="noindex" />{/if}</svelte:head>
{#if loading}
  <p class="shell" role="status" aria-live="polite">Memuat detail konser...</p>
{:else if error}
  {#if error === "not-found"}<NotFoundPanel title="Konser tidak ditemukan." message="Konser ini mungkin sudah tidak tersedia atau URL-nya tidak tepat." />{:else}<section class="shell empty-state" role="alert"><p>{error}</p><button class="text-button" type="button" onclick={() => load()}>Coba lagi</button></section>{/if}
{:else if !concertData}
  <NotFoundPanel title="Konser tidak ditemukan." message="Konser ini mungkin sudah tidak tersedia atau URL-nya tidak tepat." />
{:else}
  {@const effectiveStart = concertData.currentEvent ? concertData.currentEvent.startsAt : concertData.startsAt}
  {@const startingPrice = concertData.ticketTiers.length ? Math.min(...concertData.ticketTiers.map((tier) => tier.price)) : undefined}
  {@const eventChange = eventStatus(concertData.currentEvent)}
  <div class="concert-detail" onfocusin={keepFocusedControlVisible}>
    <section class="detail-hero"><div class="detail-hero-inner shell"><img class="detail-poster" src={concertData.image} alt={`Poster konser ${concertData.artist}`} width="900" height="1100" fetchpriority="high" /><div class="detail-heading"><p class="detail-status">{eventChange || (concertData.status === "Sold Out" ? "Tiket habis" : startingPrice === undefined ? "Tiket belum tersedia" : concertData.status)}</p><h1>{concertData.artist}</h1><p class="detail-summary">{eventDate(effectiveStart)}</p><p class="detail-summary">{concertData.venue}, {concertData.city}</p><p class="detail-summary">{startingPrice === undefined ? "Harga tiket belum tersedia" : `Mulai ${formatRupiah.format(startingPrice)}`}</p></div></div></section>
    <div class="shell detail-notices"><EventNotice state={concertData.currentEvent} />
      {#if concertData.currentEvent?.status === "RESCHEDULED"}<p class="refund-note">Pembeli lama dapat meminta refund penuh melalui halaman pesanan{concertData.currentEvent.refundDeadline ? ` sampai ${eventDate(concertData.currentEvent.refundDeadline)}` : "; tenggat refund belum diumumkan"}.</p>
      {:else if concertData.currentEvent?.status === "POSTPONED"}<p class="refund-note">Hak refund pembeli lama tetap tersedia; tenggat belum dibatasi sebelum jadwal pengganti diumumkan.</p>
      {:else if concertData.currentEvent?.status === "CANCELLED"}<p class="refund-note">Semua order lunas berhak mendapat refund penuh.</p>{/if}
    </div>
    <section class="detail-layout shell"><div class="detail-main">
      {#if concertData.ticketTiers.length}<section class="ticket-selection" aria-labelledby="ticket-selection-title"><h2 id="ticket-selection-title">Pilih tiketmu.</h2><p>{saleOpen ? "Pilih kategori dan jumlah tiket yang kamu butuhkan." : "Kategori dan harga tetap dapat dilihat; penjualan saat ini ditutup."}</p><details bind:this={zoneMapDetails} class="zone-details"><summary>Lihat area</summary><SeatingMap zones={concertData.zones} tiers={concertData.ticketTiers} selectedZoneId={detailState.selectedZoneId} onselect={selectZone} /></details><div class="tier-list">{#each concertData.ticketTiers as tier}{@const quantity = detailState.quantities[tier.id] ?? 0}{@const limit = Math.min(tier.stock, tier.maxPerOrder)}<article id={`tier-${tier.id}`} class:is-selected={tier.zoneId === detailState.selectedZoneId} class="ticket-tier"><div class="tier-info"><h3>{tier.name}</h3><p>{tier.benefit}</p><strong>{formatRupiah.format(tier.price)} per tiket</strong><span class="tier-limit">Maksimal {tier.maxPerOrder} tiket per pesanan</span></div><div class="tier-controls"><span class:low-stock={tier.stock > 0 && tier.stock <= 5} class:sold-out={tier.stock === 0} class="stock">{tier.stock === 0 ? "Habis" : tier.stock <= 5 ? `Sisa ${tier.stock} tiket` : "Tersedia"}</span>{#if quantity >= limit && limit > 0}<span class="limit-message">{tier.stock <= tier.maxPerOrder ? "Stok tersedia sudah dipilih" : "Batas pembelian tercapai"}</span>{/if}<div><button type="button" aria-label={`Kurangi tiket ${tier.name}`} disabled={quantity === 0} onclick={() => adjust(tier.id, -1)}>-</button><output aria-label={`Jumlah tiket ${tier.name}`}>{quantity}</output><button type="button" aria-label={`Tambah tiket ${tier.name}`} disabled={quantity >= limit || !saleOpen} onclick={() => adjust(tier.id, 1)}>+</button></div></div></article>{/each}</div></section>
      {:else}<section class="ticket-selection" aria-live="polite"><h2>Penjualan belum tersedia.</h2><p>Tiket dan harga belum diumumkan. Periksa informasi acara lagi nanti.</p></section>{/if}
      <ConcertInfoTabs concert={concertData} />
    </div>
    <p class="sr-only" aria-live="polite" aria-atomic="true">{count ? `${count} tiket, subtotal ${formatRupiah.format(total)}` : "Belum ada tiket dipilih"}</p><aside class="cart"><h2>Ringkasan</h2>{#if selected.length}{#each selected as tier}<p>{detailState.quantities[tier.id]}x {tier.name}: {formatRupiah.format(tier.price * (detailState.quantities[tier.id] ?? 0))}</p>{/each}{:else}<p class="cart-empty">Belum ada tiket dipilih.</p>{/if}<p class="cart-total">{count} tiket · Subtotal {formatRupiah.format(total)}</p>{#if !saleOpen}<p class="sale-closed">Penjualan tidak tersedia.</p>{/if}<button class="button cart-checkout" type="button" disabled={!saleOpen || !count} onclick={checkout}>Lanjut checkout</button></aside>
    </section>
    <div bind:this={mobileCart} class="mobile-cart" aria-label="Ringkasan tiket"><div><strong>{count} tiket</strong><span>Subtotal {formatRupiah.format(total)}</span>{#if !saleOpen}<span class="sale-closed">{concertData.status === "Sold Out" ? "Tiket habis" : eventChange || "Penjualan tidak tersedia"}</span>{/if}</div><button class="button" type="button" disabled={!saleOpen || !count} onclick={checkout}>Lanjut checkout</button></div>
  </div>
{/if}
