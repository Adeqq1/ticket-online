<script lang="ts">
  import { onMount, tick, type Snippet } from "svelte";
  import { MediaQuery } from "svelte/reactivity";
  import { clampMobileGroup, mobileGroupSize } from "../../lib/mobile-list.ts";
  let { count, group = $bindable(0), scrollLeft = $bindable(0), headingId, hasMore = false, loadMore, disabled = false, children }: {
    count: number; group?: number; scrollLeft?: number; headingId: string; hasMore?: boolean;
    loadMore?: () => Promise<boolean | undefined>; disabled?: boolean; children: Snippet;
  } = $props();
  const mobile = new MediaQuery("(max-width: 767px)");
  let viewport: HTMLDivElement;
  let pending = $state(false);
  let previousGroup = group;
  $effect(() => { group = clampMobileGroup(group, count); });
  $effect(() => {
    if (group !== previousGroup) { previousGroup = group; scrollLeft = 0; if (viewport) viewport.scrollLeft = 0; }
  });
  onMount(() => { if (viewport) viewport.scrollLeft = scrollLeft; });
  async function change(direction: -1 | 1) {
    if (pending || disabled) return;
    let next = group + direction;
    if (next < 0) return;
    pending = true;
    try {
      if (direction === 1 && next * mobileGroupSize >= count && hasMore && loadMore) {
        const previousCount = count;
        if (!await loadMore()) return;
        if (previousCount % mobileGroupSize && count > previousCount) next = group;
      }
      if (next * mobileGroupSize >= count) return;
      group = next;
      await tick();
      if (viewport) { scrollLeft = 0; viewport.scrollLeft = 0; }
      document.getElementById(headingId)?.focus();
    } finally { pending = false; }
  }
</script>

{#if mobile.current}
  <p class="staff-muted" role="status" aria-live="polite">Menampilkan {group * mobileGroupSize + 1} sampai {Math.min((group + 1) * mobileGroupSize, count)} dari {count} item dimuat</p>
  {#if Math.min(mobileGroupSize, count - group * mobileGroupSize) > 1}<p class="staff-muted">Geser kartu untuk melihat item lainnya.</p>{/if}
{/if}
<!-- svelte-ignore a11y_no_noninteractive_tabindex (Native horizontal scrolling needs keyboard focus.) -->
<div class="mobile-list-viewport" bind:this={viewport} tabindex="0" role="region" aria-labelledby={headingId} onscroll={(event) => { scrollLeft = event.currentTarget.scrollLeft; }}>
  {@render children()}
</div>
{#if mobile.current}
  <nav class="order-pagination" aria-label="Kelompok daftar"><button class="staff-secondary-button" type="button" disabled={disabled || pending || group === 0} onclick={() => change(-1)}>Sebelumnya</button><button class="staff-secondary-button" type="button" disabled={disabled || pending || ((group + 1) * mobileGroupSize >= count && !hasMore)} onclick={() => change(1)}>{pending ? "Memuat…" : "Berikutnya"}</button></nav>
{:else if hasMore}
  <button class="staff-secondary-button order-load-more" type="button" disabled={pending || disabled} onclick={async () => { if (!pending && !disabled) { pending = true; try { await loadMore?.(); } finally { pending = false; } } }}>Muat berikutnya</button>
{/if}

<style>
  .mobile-list-viewport { position: relative; min-width: 0; max-width: 100%; overflow-x: auto; overscroll-behavior-inline: contain; }
  .mobile-list-viewport:focus-visible { outline: 3px solid var(--admin-accent); outline-offset: 3px; }
  @media (max-width: 767px) {
    .mobile-list-viewport { scroll-snap-type: x mandatory; }
    .mobile-list-viewport :global(.history-table-wrap) { overflow: visible; }
    .mobile-list-viewport :global(.history-table) { display: block; width: 100%; min-width: 0; white-space: normal; }
    .mobile-list-viewport :global(thead) { position: absolute; width: 1px; height: 1px; padding: 0; overflow: hidden; clip: rect(0, 0, 0, 0); clip-path: inset(50%); }
    .mobile-list-viewport :global(tbody), .mobile-list-viewport :global(ul.staff-list), .mobile-list-viewport :global(ul.event-admin-list) { display: flex; width: 100%; gap: 16px; margin: 0; padding: 0; }
    .mobile-list-viewport :global(tr), .mobile-list-viewport :global(ul.staff-list > li), .mobile-list-viewport :global(ul.event-admin-list > li) { display: grid; flex: 0 0 100%; min-width: 0; grid-template-columns: minmax(0, 1fr); gap: 12px; padding: 12px 0; scroll-snap-align: start; overflow-wrap: anywhere; }
    .mobile-list-viewport :global(td) { display: grid; grid-template-columns: minmax(0, .8fr) minmax(0, 1.2fr); gap: 12px; padding: 5px 0; border: 0; font-size: 16px; overflow-wrap: anywhere; }
    .mobile-list-viewport :global(td::before) { content: attr(data-label); color: var(--admin-secondary); font-size: 14px; }
    .mobile-list-viewport :global(td > small) { grid-column: 2; }
    .mobile-list-viewport :global(li > button), .mobile-list-viewport :global(.staff-account-meta) { grid-column: 1; grid-row: auto; }
    .mobile-list-viewport :global(.staff-list-identity) { min-width: 0; }
  }
</style>
