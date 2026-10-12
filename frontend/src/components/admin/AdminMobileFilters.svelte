<script lang="ts">
  import { MediaQuery } from "svelte/reactivity";
  import type { Snippet } from "svelte";
  let { id, title = "Pencarian dan filter", open = $bindable(false), children }: { id: string; title?: string; open?: boolean; children: Snippet } = $props();
  const mobile = new MediaQuery("(max-width: 767px)");
  $effect(() => { if (mobile.current && document.activeElement?.closest(`#${id}`)) open = true; });
</script>

<div class="mobile-filters" class:is-open={open}>
  <div class="mobile-filter-heading"><h2>{title}</h2><button class="staff-secondary-button" type="button" aria-label={`${open ? "Tutup" : "Buka"} ${title.toLowerCase()}`} aria-expanded={open} aria-controls={id} onclick={() => { open = !open; }}><svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" aria-hidden="true"><circle cx="10.5" cy="10.5" r="6.5" /><path d="m16 16 4.5 4.5" /></svg></button></div>
  <div id={id} class="mobile-filter-content">{@render children()}</div>
</div>

<style>
  .mobile-filter-heading { display: none; }
  @media (max-width: 767px) {
    .mobile-filters { margin-block: 12px; }
    .mobile-filter-heading { display: flex; align-items: center; justify-content: space-between; gap: 12px; }
    h2 { margin: 0; font-size: 18px; }
    button { display: inline-flex; align-items: center; justify-content: center; flex: 0 0 44px; min-width: 44px; min-height: 44px; padding: 10px; }
    .mobile-filters:not(.is-open) > .mobile-filter-content { display: none; }
    .is-open > .mobile-filter-content { margin-top: 12px; }
  }
</style>
