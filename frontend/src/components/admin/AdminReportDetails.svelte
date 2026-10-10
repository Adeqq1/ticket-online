<script lang="ts">
  import { onMount, type Snippet } from "svelte";

  let { title, desktopOpen = true, children }: { title: string; desktopOpen?: boolean; children: Snippet } = $props();
  let open = $state(false);

  onMount(() => {
    const desktop = window.matchMedia("(min-width: 1024px)");
    const update = () => { open = desktop.matches && desktopOpen; };
    update();
    desktop.addEventListener("change", update);
    return () => desktop.removeEventListener("change", update);
  });
</script>

<details class="admin-report-details" bind:open>
  <summary><h2>{title}</h2><span aria-hidden="true"></span></summary>
  <div class="admin-report-details-content">{@render children()}</div>
</details>

<style>
  .admin-report-details { margin-top: 16px; border: 1px solid var(--scan-line); background: var(--scan-panel); }
  summary { display: flex; min-height: 64px; align-items: center; justify-content: space-between; gap: 16px; padding: 12px 20px; cursor: pointer; list-style: none; }
  summary::-webkit-details-marker { display: none; }
  summary h2 { margin: 0; font-size: clamp(19px, 2vw, 25px); letter-spacing: -.04em; }
  summary > span::after { content: "+"; color: var(--scan-lime); font-size: 24px; }
  details[open] summary > span::after { content: "−"; }
  summary:focus-visible { outline: 2px solid var(--scan-lime); outline-offset: 2px; }
  .admin-report-details-content { min-width: 0; padding: 0 20px 20px; }
  .admin-report-details-content :global(.history-table-wrap) { max-width: 100%; overscroll-behavior-inline: contain; }
  @media (max-width: 540px) {
    summary { min-height: 56px; padding: 10px 16px; }
    .admin-report-details-content { padding: 0 16px 16px; }
  }
</style>
