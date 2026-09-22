<script lang="ts">
  import { onMount } from "svelte";
  import { getEvent, ApiError } from "../lib/api.ts";
  import { formatRupiah, parseQuantities, type Concert } from "../lib/concerts.ts";
  import NotFoundPanel from "../components/NotFoundPanel.svelte";
  import ConcertInfoTabs from "../components/detail/ConcertInfoTabs.svelte";
  import SeatingMap from "../components/detail/SeatingMap.svelte";
  import { cartTotal, changeQuantity, quantityParams, ticketCount, type DetailState } from "../lib/cart.ts";
  let { id }: { id: string } = $props();
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
  function adjust(tierId: string, delta: -1 | 1) { if (concertData) detailState = changeQuantity(concertData, detailState, tierId, delta); }
  function selectZone(zoneId: string, event: MouseEvent | KeyboardEvent) {
    detailState.selectedZoneId = zoneId;
    const tier = concertData?.ticketTiers.find((item) => item.zoneId === zoneId);
    requestAnimationFrame(() => { const element = document.getElementById(`tier-${tier?.id}`); element?.scrollIntoView({ behavior: matchMedia("(prefers-reduced-motion: reduce)").matches ? "auto" : "smooth", block: "nearest" }); if (event instanceof KeyboardEvent) element?.querySelector<HTMLButtonElement>("button:not(:disabled)")?.focus(); });
  }
  function checkout() { if (concertData && count) location.assign(`/checkout/${concertData.id}?${quantityParams(detailState.quantities)}`); }
  async function load(signal?: AbortSignal) { loading = true; error = ""; try { concertData = await getEvent(id, signal); detailState = initialDetailState(); } catch (value) { if (!(value instanceof DOMException && value.name === "AbortError")) error = value instanceof ApiError && value.code === "EVENT_NOT_FOUND" ? "not-found" : value instanceof ApiError ? value.message : "Detail konser belum dapat dimuat."; } finally { if (!signal?.aborted) loading = false; } }
  onMount(() => { const controller = new AbortController(); load(controller.signal); return () => controller.abort(); });
</script>

<svelte:head><title>{concertData ? `${concertData.artist} | Tiket Online` : "Konser tidak ditemukan | Tiket Online"}</title><meta name="description" content={concertData ? `Detail konser dan pilihan tiket ${concertData.artist} di Tiket Online.` : "Konser tidak ditemukan di Tiket Online."} />{#if !concertData}<meta name="robots" content="noindex" />{/if}</svelte:head>
{#if loading}
  <p class="shell" role="status" aria-live="polite">Memuat detail konser...</p>
{:else if error}
  {#if error === "not-found"}<NotFoundPanel title="Konser tidak ditemukan." message="Konser ini mungkin sudah tidak tersedia atau URL-nya tidak tepat." />{:else}<section class="shell empty-state" role="alert"><p>{error}</p><button class="text-button" type="button" onclick={() => load()}>Coba lagi</button></section>{/if}
{:else if !concertData}
  <NotFoundPanel title="Konser tidak ditemukan." message="Konser ini mungkin sudah tidak tersedia atau URL-nya tidak tepat." />
{:else}
  <section class="detail-hero"><img class="detail-backdrop" src={concertData.image} alt="" aria-hidden="true" /><div class="detail-hero-inner shell"><img class="detail-poster" src={concertData.image} alt={`Poster contoh konser ${concertData.artist}`} width="900" height="1100" fetchpriority="high" /><div><p class="detail-status">{concertData.status}</p><h1>{concertData.artist}</h1><p class="detail-summary">{concertData.date}, {concertData.venue}, {concertData.city}</p><p class="detail-summary">{concertData.genre} · Mulai {formatRupiah.format(concertData.price)}</p></div></div></section>
  <section class="detail-layout shell"><div class="detail-main"><ConcertInfoTabs concert={concertData} /><section class="ticket-selection" aria-labelledby="ticket-selection-title"><h2 id="ticket-selection-title">Pilih tiketmu.</h2><p>Pilih area panggung, lalu tentukan jumlah tiket yang kamu butuhkan.</p><SeatingMap zones={concertData.zones} tiers={concertData.ticketTiers} selectedZoneId={detailState.selectedZoneId} onselect={selectZone} /><div class="tier-list">{#each concertData.ticketTiers as tier}{@const quantity = detailState.quantities[tier.id] ?? 0}{@const limit = Math.min(tier.stock, tier.maxPerOrder)}<article id={`tier-${tier.id}`} class:is-selected={tier.zoneId === detailState.selectedZoneId} class="ticket-tier"><div><h3>{tier.name}</h3><p>{tier.benefit}</p><strong>{formatRupiah.format(tier.price)}</strong></div><div class="tier-controls"><span class:low-stock={tier.stock > 0 && tier.stock <= 5} class:sold-out={tier.stock === 0} class="stock">{tier.stock === 0 ? "Habis" : tier.stock <= 5 ? `Sisa ${tier.stock} tiket` : "Tersedia"}</span><div><button type="button" aria-label={`Kurangi tiket ${tier.name}`} disabled={quantity === 0} onclick={() => adjust(tier.id, -1)}>-</button><output aria-label={`Jumlah tiket ${tier.name}`}>{quantity}</output><button type="button" aria-label={`Tambah tiket ${tier.name}`} disabled={quantity >= limit} onclick={() => adjust(tier.id, 1)}>+</button></div></div></article>{/each}</div></section></div>
    <p class="sr-only" aria-live="polite" aria-atomic="true">{count ? `${count} tiket, ${formatRupiah.format(total)}` : "Keranjang tiket kosong"}</p>{#if count}<aside class="cart"><h2>Ringkasan</h2>{#each selected as tier}<p>{detailState.quantities[tier.id]}x {tier.name}: {formatRupiah.format(tier.price * (detailState.quantities[tier.id] ?? 0))}</p>{/each}<p class="cart-total">{count} tiket, {formatRupiah.format(total)}</p><button class="button cart-checkout" type="button" onclick={checkout}>Lanjut checkout</button></aside>{/if}
  </section>
  {#if count}<div class="mobile-cart"><span>{count} tiket, {formatRupiah.format(total)}</span><button class="button" type="button" onclick={checkout}>Checkout</button></div>{/if}
{/if}
