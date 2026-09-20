<script lang="ts">
  import { formatRupiah, getConcertById, parseQuantities } from "../lib/concerts.ts";
  import NotFoundPanel from "../components/NotFoundPanel.svelte";
  import ConcertInfoTabs from "../components/detail/ConcertInfoTabs.svelte";
  import SeatingMap from "../components/detail/SeatingMap.svelte";
  import { cartTotal, changeQuantity, quantityParams, ticketCount, type DetailState } from "../lib/cart.ts";
  let { id }: { id: string } = $props();
  const concert = $derived(getConcertById(id));
  function initialDetailState(): DetailState {
    const quantities = concert ? parseQuantities(concert, new URLSearchParams(location.search)) : {};
    const selectedTier = concert?.ticketTiers.find((tier) => quantities[tier.id]);
    return { selectedZoneId: selectedTier?.zoneId ?? null, quantities };
  }
  let state = $state<DetailState>(initialDetailState());
  const count = $derived(ticketCount(state.quantities));
  const total = $derived(concert ? cartTotal(concert, state.quantities) : 0);
  const selected = $derived(concert?.ticketTiers.filter((tier) => state.quantities[tier.id]) ?? []);
  function adjust(tierId: string, delta: -1 | 1) { if (concert) state = changeQuantity(concert, state, tierId, delta); }
  function selectZone(zoneId: string, event: MouseEvent | KeyboardEvent) {
    state.selectedZoneId = zoneId;
    const tier = concert?.ticketTiers.find((item) => item.zoneId === zoneId);
    requestAnimationFrame(() => { const element = document.getElementById(`tier-${tier?.id}`); element?.scrollIntoView({ behavior: matchMedia("(prefers-reduced-motion: reduce)").matches ? "auto" : "smooth", block: "nearest" }); if (event instanceof KeyboardEvent) element?.querySelector<HTMLButtonElement>("button:not(:disabled)")?.focus(); });
  }
  function checkout() { if (concert && count) location.assign(`/checkout/${concert.id}?${quantityParams(state.quantities)}`); }
</script>

<svelte:head><title>{concert ? `${concert.artist} | Tiket Online` : "Konser tidak ditemukan | Tiket Online"}</title><meta name="description" content={concert ? `Detail konser dan pilihan tiket ${concert.artist} di Tiket Online.` : "Konser tidak ditemukan di Tiket Online."} />{#if !concert}<meta name="robots" content="noindex" />{/if}</svelte:head>
{#if !concert}
  <NotFoundPanel title="Konser tidak ditemukan." message="Konser ini mungkin sudah tidak tersedia atau URL-nya tidak tepat." />
{:else}
  <section class="detail-hero"><img class="detail-backdrop" src={concert.image} alt="" aria-hidden="true" /><div class="detail-hero-inner shell"><img class="detail-poster" src={concert.image} alt={`Poster contoh konser ${concert.artist}`} width="900" height="1100" fetchpriority="high" /><div><p class="detail-status">{concert.status}</p><h1>{concert.artist}</h1><p class="detail-summary">{concert.date}, {concert.venue}, {concert.city}</p><p class="detail-summary">{concert.genre} · Mulai {formatRupiah.format(concert.price)}</p></div></div></section>
  <section class="detail-layout shell"><div class="detail-main"><ConcertInfoTabs {concert} /><section class="ticket-selection" aria-labelledby="ticket-selection-title"><h2 id="ticket-selection-title">Pilih tiketmu.</h2><p>Pilih area panggung, lalu tentukan jumlah tiket yang kamu butuhkan.</p><SeatingMap zones={concert.zones} tiers={concert.ticketTiers} selectedZoneId={state.selectedZoneId} onselect={selectZone} /><div class="tier-list">{#each concert.ticketTiers as tier}{@const quantity = state.quantities[tier.id] ?? 0}{@const limit = Math.min(tier.stock, tier.maxPerOrder)}<article id={`tier-${tier.id}`} class:is-selected={tier.zoneId === state.selectedZoneId} class="ticket-tier"><div><h3>{tier.name}</h3><p>{tier.benefit}</p><strong>{formatRupiah.format(tier.price)}</strong></div><div class="tier-controls"><span class:low-stock={tier.stock > 0 && tier.stock <= 5} class:sold-out={tier.stock === 0} class="stock">{tier.stock === 0 ? "Habis" : tier.stock <= 5 ? `Sisa ${tier.stock} tiket` : "Tersedia"}</span><div><button type="button" aria-label={`Kurangi tiket ${tier.name}`} disabled={quantity === 0} onclick={() => adjust(tier.id, -1)}>-</button><output aria-label={`Jumlah tiket ${tier.name}`}>{quantity}</output><button type="button" aria-label={`Tambah tiket ${tier.name}`} disabled={quantity >= limit} onclick={() => adjust(tier.id, 1)}>+</button></div></div></article>{/each}</div></section></div>
    <p class="sr-only" aria-live="polite" aria-atomic="true">{count ? `${count} tiket, ${formatRupiah.format(total)}` : "Keranjang tiket kosong"}</p>{#if count}<aside class="cart"><h2>Ringkasan</h2>{#each selected as tier}<p>{state.quantities[tier.id]}x {tier.name}: {formatRupiah.format(tier.price * (state.quantities[tier.id] ?? 0))}</p>{/each}<p class="cart-total">{count} tiket, {formatRupiah.format(total)}</p><button class="button cart-checkout" type="button" onclick={checkout}>Lanjut checkout</button></aside>{/if}
  </section>
  {#if count}<div class="mobile-cart"><span>{count} tiket, {formatRupiah.format(total)}</span><button class="button" type="button" onclick={checkout}>Checkout</button></div>{/if}
{/if}
