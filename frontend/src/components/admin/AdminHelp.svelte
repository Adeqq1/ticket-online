<script lang="ts">
  import { onMount, type Snippet } from "svelte";
  import { MediaQuery } from "svelte/reactivity";

  let { id, label, text, desktop = false, children }: { id: string; label: string; text: string; desktop?: boolean; children: Snippet } = $props();
  const mobile = new MediaQuery("(max-width: 767px)");
  const active = $derived(mobile.current || desktop);
  let opened = $state(false);
  let pinned = false;
  let trigger: HTMLButtonElement;
  let popup: HTMLDivElement;
  function close() { opened = false; pinned = false; }
  function open() {
    document.dispatchEvent(new CustomEvent("admin-help-open", { detail: id }));
    opened = true;
  }
  $effect(() => { if (!active) close(); });
  onMount(() => {
    const closeOther = (event: Event) => { if ((event as CustomEvent<string>).detail !== id) close(); };
    document.addEventListener("admin-help-open", closeOther);
    return () => document.removeEventListener("admin-help-open", closeOther);
  });
</script>

<svelte:window onclick={(event) => { if (!(event.target instanceof Node) || (!trigger?.contains(event.target) && !popup?.contains(event.target))) close(); }} onkeydown={(event) => { if (event.key === "Escape") close(); }} />
<div class="admin-help" class:active role={active ? "group" : undefined} aria-label={active ? label : undefined} onpointerleave={() => { if (!pinned && !trigger?.matches(":focus-visible")) close(); }}>
  {@render children()}
  <button bind:this={trigger} class="staff-secondary-button admin-help-button" type="button" aria-label={label} aria-expanded={opened} aria-controls={id} aria-describedby={opened ? id : undefined} onpointerenter={(event) => { if (event.pointerType === "mouse" && !opened) open(); }} onfocus={(event) => { if (event.currentTarget.matches(":focus-visible")) open(); }} onblur={() => { if (!pinned) close(); }} onclick={() => { if (pinned) close(); else { pinned = true; open(); } }}><svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" aria-hidden="true"><circle cx="12" cy="12" r="9" /><path d="M9.5 9a2.5 2.5 0 0 1 5 .5c0 1.5-2.5 2-2.5 3.5" /><path d="M12 17h.01" /></svg></button>
  <div bind:this={popup} {id} class="admin-help-popup" role="tooltip" hidden={!opened}>{text}</div>
</div>

<style>
  .admin-help { display: contents; }
  .admin-help-button, .admin-help-popup { display: none; }
  .admin-help.active { position: relative; display: flex; flex: 1; flex-wrap: wrap; align-items: center; gap: 12px; min-width: 0; }
  .admin-help.active :global(h1), .admin-help.active :global(h2) { flex: 1; min-width: min(100%, 4.5em); margin: 0; overflow-wrap: anywhere; }
  .admin-help.active .admin-help-button { display: inline-flex; align-items: center; justify-content: center; flex: 0 0 44px; width: 44px; height: 44px; padding: 10px; border: 0; background: transparent; }
  .admin-help.active .admin-help-popup:not([hidden]) { position: absolute; top: 100%; inset-inline: 0; z-index: 5; display: block; max-height: calc(100dvh - 32px); overflow-y: auto; padding: 14px; border: 1px solid var(--admin-control-border); background: var(--admin-input); color: var(--admin-text); font-size: 14px; line-height: 1.6; overflow-wrap: anywhere; }
</style>
