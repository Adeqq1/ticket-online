<script lang="ts">
  import { onDestroy, onMount, tick, type Snippet } from "svelte";
  import AdminHelp from "../components/admin/AdminHelp.svelte";

  type Action = { label: string; onclick?: () => void; form?: string; disabled?: boolean; id?: string; controls?: string; expanded?: boolean };
  type Page = "pricing" | "events" | "staff" | "orders" | "issues" | "operations" | "reports" | "attendance" | "conversion" | "history";
  let { page, contentId, skipLabel, kicker, title, description, mobileDescriptionHelp = false, onLogout, onBeforeNavigate, loggingOut, logoutDisabled, primaryAction, secondaryAction, children }: {
    page: Page; contentId: string; skipLabel: string; kicker: string; title: string; description: string; mobileDescriptionHelp?: boolean;
    onLogout: () => void; onBeforeNavigate?: () => Promise<boolean>; loggingOut: boolean; logoutDisabled?: boolean; primaryAction?: Action; secondaryAction?: Action; children: Snippet;
  } = $props();

  let dialog: HTMLDialogElement;
  let confirmDialog: HTMLDialogElement;
  let cancelButton = $state<HTMLButtonElement>();
  let confirmation = $state<{ title: string; message: string; confirmLabel: string } | null>(null);
  let resolveConfirmation: ((accepted: boolean) => void) | undefined;
  let confirmTrigger: HTMLElement | null = null;
  let destroyed = false;
  let navigating = false;
  let approvedNavigation = false;

  export function consumeNavigationApproval() {
    const approved = approvedNavigation;
    approvedNavigation = false;
    return approved;
  }

  async function protectNavigation(event: MouseEvent) {
    if (!onBeforeNavigate || event.defaultPrevented || event.button !== 0 || event.ctrlKey || event.metaKey || event.shiftKey || event.altKey || !(event.target instanceof Element)) return;
    const link = event.target.closest<HTMLAnchorElement>("a[href]");
    if (!link || link.hasAttribute("download") || (link.target && link.target !== "_self")) return;
    const destination = new URL(link.href);
    if (destination.origin !== location.origin || (destination.pathname === location.pathname && destination.search === location.search && destination.hash)) return;
    event.preventDefault();
    if (navigating) return;
    navigating = true;
    try {
      if (await onBeforeNavigate() && !destroyed) {
        approvedNavigation = true;
        window.location.assign(destination.href);
      }
    } finally { navigating = false; }
  }

  onMount(() => {
    document.addEventListener("click", protectNavigation, true);
    return () => document.removeEventListener("click", protectNavigation, true);
  });

  export function confirm(options: { title: string; message: string; confirmLabel: string }): Promise<boolean> {
    if (destroyed || resolveConfirmation) return Promise.resolve(false);
    confirmTrigger = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    confirmation = options;
    return new Promise<boolean>((resolve) => {
      resolveConfirmation = resolve;
      void tick().then(() => {
        if (resolveConfirmation !== resolve) return;
        confirmDialog.showModal();
        cancelButton?.focus();
      });
    });
  }

  function finishConfirmation(accepted: boolean) {
    const resolve = resolveConfirmation;
    if (!resolve) return;
    resolveConfirmation = undefined;
    if (confirmDialog?.open) confirmDialog.close();
    confirmation = null;
    if (!destroyed && confirmTrigger?.isConnected) confirmTrigger.focus({ preventScroll: true });
    confirmTrigger = null;
    resolve(accepted);
  }

  function handleConfirmKeydown(event: KeyboardEvent) {
    if (event.key !== "Tab") return;
    const items = [...confirmDialog.querySelectorAll<HTMLElement>('button, [tabindex="0"]')];
    if (event.shiftKey && document.activeElement === items[0]) { event.preventDefault(); items.at(-1)?.focus(); }
    else if (!event.shiftKey && document.activeElement === items.at(-1)) { event.preventDefault(); items[0]?.focus(); }
  }

  function closeConfirmOnBackdrop(event: MouseEvent) {
    if (event.target !== confirmDialog || event.detail > 1) return;
    const bounds = confirmDialog.getBoundingClientRect();
    if (event.clientX < bounds.left || event.clientX > bounds.right || event.clientY < bounds.top || event.clientY > bounds.bottom) finishConfirmation(false);
  }

  onDestroy(() => { destroyed = true; finishConfirmation(false); });

  let menuButton: HTMLButtonElement;
  let menuOpen = $state(false);
  const groups = [
    { label: "Pengelolaan", links: [{ page: "pricing", label: "Biaya dan voucher", href: "/admin/pricing" }, { page: "events", label: "Konser", href: "/admin/events" }, { page: "staff", label: "Kelola petugas", href: "/admin/staff" }] },
    { label: "Operasional", links: [{ page: "orders", label: "Pesanan", href: "/admin/orders" }, { page: "issues", label: "Masalah", href: "/admin/issues" }, { page: "operations", label: "Operasional", href: "/admin/operations" }, { page: "history", label: "Riwayat check-in", href: "/admin/check-ins" }] },
    { label: "Laporan", links: [{ page: "reports", label: "Penjualan dan refund", href: "/admin/reports" }, { page: "attendance", label: "Kehadiran", href: "/admin/reports/attendance" }, { page: "conversion", label: "Konversi", href: "/admin/reports/conversion" }] },
  ] as const;

  async function openMenu() {
    dialog.showModal();
    menuOpen = true;
    await tick();
    dialog.querySelector<HTMLButtonElement>("[data-menu-close]")?.focus();
  }

  function closeOnDesktop() {
    if (matchMedia("(min-width: 1024px)").matches && dialog.open) dialog.close();
  }

  function restoreMenuFocus() {
    menuOpen = false;
    if (matchMedia("(max-width: 1023px)").matches) menuButton?.focus();
    else document.querySelector<HTMLElement>(".admin-topbar .scan-brand")?.focus();
  }

  function handleMenuKeydown(event: KeyboardEvent) {
    if (event.key === "Escape") { dialog.close(); return; }
    if (event.key !== "Tab") return;
    const items = [...dialog.querySelectorAll<HTMLElement>('button:not(:disabled), a[href], input:not(:disabled), select:not(:disabled), textarea:not(:disabled), [tabindex]:not([tabindex="-1"])')];
    const first = items[0];
    const last = items.at(-1);
    if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last?.focus(); }
    else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first?.focus(); }
  }
</script>

<div class="scan-shell staff-admin-shell admin-layout" class:compact={["pricing", "events", "orders", "staff"].includes(page)}>
  <a class="skip-link" href={`#${contentId}`}>{skipLabel}</a>
  <header class="scan-topbar admin-topbar">
    <a class="scan-brand" href="/" aria-label="Kembali ke Tiket Online"><span class="scan-brand-mark" aria-hidden="true">TO</span><span>Tiket Online <b>/ Gate Control</b></span></a>
    <span class="staff-login-label">ADMINISTRATOR</span>
    <button class="staff-text-button admin-menu-trigger" bind:this={menuButton} type="button" aria-label="Menu" title="Buka menu navigasi" aria-haspopup="dialog" aria-expanded={menuOpen} aria-controls="admin-mobile-menu" onclick={openMenu}><svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" aria-hidden="true"><path d="M4 6h16M4 12h16M4 18h16" /></svg><span class="admin-menu-label">Menu</span></button>
    <button class="staff-text-button admin-logout" type="button" aria-label="Keluar" title="Keluar dari akun admin" disabled={logoutDisabled ?? loggingOut} onclick={onLogout}><svg class="admin-logout-icon" width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M9 5H4v14h5M9 12h11m-4-4 4 4-4 4" /></svg><span class="admin-logout-label">Keluar</span></button>
  </header>

  <dialog id="admin-mobile-menu" class="admin-mobile-menu" aria-label="Navigasi admin" bind:this={dialog} onclose={restoreMenuFocus} onkeydown={handleMenuKeydown}>
    <div class="admin-mobile-menu-heading"><strong>Navigasi admin</strong><button class="staff-text-button" data-menu-close type="button" onclick={() => dialog.close()}>Tutup</button></div>
    {#each groups as group (group.label)}
      <nav aria-label={group.label} class="admin-nav-group"><h2>{group.label}</h2>{#each group.links as link (link.href)}<a href={link.href} aria-current={page === link.page ? "page" : undefined}>{link.label}</a>{/each}</nav>
    {/each}
  </dialog>

  <dialog class="admin-confirm" bind:this={confirmDialog} aria-labelledby="admin-confirm-title" aria-describedby="admin-confirm-message" oncancel={(event) => { event.preventDefault(); finishConfirmation(false); }} onclose={() => { if (!confirmDialog?.open) finishConfirmation(false); }} onkeydown={handleConfirmKeydown} onclick={closeConfirmOnBackdrop}>
    {#if confirmation}
      <h2 id="admin-confirm-title">{confirmation.title}</h2>
      <!-- svelte-ignore a11y_no_noninteractive_tabindex (Scrollable confirmation text must be reachable by keyboard.) -->
      <div id="admin-confirm-message" role="region" aria-label="Rincian konfirmasi" tabindex="0">{confirmation.message}</div>
      <div class="admin-confirm-actions">
        <button class="staff-secondary-button" type="button" bind:this={cancelButton} onclick={() => finishConfirmation(false)}>Batal</button>
        <button class="scan-submit staff-submit" type="button" onclick={() => finishConfirmation(true)}>{confirmation.confirmLabel}</button>
      </div>
    {/if}
  </dialog>

  <div class="admin-layout-grid">
    <aside class="admin-sidebar" aria-label="Navigasi admin">
      {#each groups as group (group.label)}
        <nav aria-label={group.label} class="admin-nav-group"><h2>{group.label}</h2>{#each group.links as link (link.href)}<a href={link.href} aria-current={page === link.page ? "page" : undefined}>{link.label}</a>{/each}</nav>
      {/each}
    </aside>

    <main id={contentId} class="staff-admin-content" tabindex="-1">
      <div class="staff-admin-heading admin-page-heading"><div><p class="scan-kicker">{kicker}</p>{#if mobileDescriptionHelp}<AdminHelp id={`${contentId}-description-help`} label={`Penjelasan ${title.replace(/\.$/, "").toLowerCase()}`} text={description}><h1>{title}</h1></AdminHelp>{:else}<h1>{title}</h1>{/if}<p class:mobile-help-description={mobileDescriptionHelp}>{description}</p></div><div class="admin-heading-actions">{#if primaryAction}<button class="scan-submit staff-submit" id={primaryAction.id} aria-controls={primaryAction.controls} aria-expanded={primaryAction.expanded} type={primaryAction.form ? "submit" : "button"} form={primaryAction.form} onclick={primaryAction.onclick} disabled={primaryAction.disabled}>{primaryAction.label}</button>{/if}{#if secondaryAction}<button class="staff-secondary-button" type="button" onclick={secondaryAction.onclick} disabled={secondaryAction.disabled}>{secondaryAction.label}</button>{/if}</div></div>
      {@render children()}
    </main>
  </div>
</div>

<svelte:window onresize={closeOnDesktop} />

<style>
  .admin-confirm { box-sizing: border-box; width: min(32rem, calc(100% - 32px)); max-height: calc(100dvh - 32px); margin: auto; padding: 24px; border: 1px solid var(--admin-border); border-radius: 16px; background: var(--admin-card); color: var(--admin-text); }
  .admin-confirm[open] { display: grid; grid-template-rows: auto minmax(0, 1fr) auto; gap: 16px; }
  .admin-confirm::backdrop { background: rgb(0 0 0 / .65); }
  .admin-confirm h2 { margin: 0; font-size: 22px; line-height: 1.3; overflow-wrap: anywhere; }
  .admin-confirm #admin-confirm-message { min-height: 0; margin: 0; overflow-y: auto; white-space: pre-line; overflow-wrap: anywhere; font-size: 16px; line-height: 1.5; }
  .admin-confirm-actions { display: flex; flex-wrap: wrap; justify-content: flex-end; gap: 12px; }
  .admin-confirm-actions button { min-width: 44px; min-height: 44px; margin: 0; border-radius: 10px; font-size: 16px; overflow-wrap: anywhere; }
  @media (max-width: 600px) { .admin-confirm { padding: 20px; } .admin-confirm-actions button { flex: 1 1 100%; } }
  @media print { .admin-confirm { display: none !important; } }
  .admin-layout .skip-link:not(:focus) { top: 0; transform: translateY(-100%); }
  .admin-topbar { width: min(100% - 56px, 1500px); }
  .admin-topbar .scan-brand { min-height: 44px; }
  .admin-menu-trigger { display: none; }
  .admin-logout-icon { display: none; }
  .admin-logout, .admin-mobile-menu-heading button { min-width: 44px; min-height: 44px; }
  .admin-layout-grid { display: grid; grid-template-columns: 248px minmax(0, 1fr); gap: clamp(28px, 4vw, 64px); width: min(100% - 56px, 1500px); margin: 0 auto; }
  .admin-sidebar { position: sticky; top: 20px; align-self: start; display: grid; gap: 24px; padding: 32px 16px; border-right: 1px solid var(--admin-border); background: var(--admin-sidebar); }
  .admin-nav-group { display: grid; align-content: start; gap: 4px; }
  .admin-nav-group h2 { margin: 0 0 6px; color: var(--scan-muted); font: 14px "Courier New", monospace; letter-spacing: .06em; text-transform: uppercase; }
  .admin-nav-group a { display: flex; align-items: center; min-height: 44px; padding: 8px 12px; color: var(--scan-muted); font-size: 16px; line-height: 1.3; text-decoration: none; border-left: 2px solid transparent; }
  .admin-nav-group a:hover { color: var(--scan-text); border-left-color: var(--scan-muted); background: var(--scan-panel); }
  .admin-nav-group a[aria-current="page"] { color: var(--scan-lime); border-left-color: var(--scan-lime); background: var(--scan-panel); }
  .admin-layout .staff-admin-content { width: auto; min-width: 0; margin: 0; padding: 42px 0 40px; }
  .admin-page-heading { align-items: center; }
  .admin-layout.compact .staff-admin-content { padding-top: 24px; }
  .compact .admin-page-heading { margin-bottom: 20px; }
  .compact .admin-page-heading .scan-kicker { margin-bottom: 8px; }
  .compact .admin-page-heading > div > p:last-child { margin-top: 12px; }
  .admin-page-heading > div:first-child { min-width: 0; }
  .admin-page-heading .scan-kicker { font-size: 14px; }
  .admin-page-heading > div > p:last-child { font-size: 16px; }
  .admin-heading-actions { display: flex; flex-wrap: wrap; align-items: center; justify-content: flex-end; gap: 10px; }
  .admin-heading-actions > * { min-height: 44px; }
  .admin-mobile-menu { width: min(100% - 28px, 440px); max-height: min(90dvh, 760px); margin: 12px 14px auto auto; padding: 20px; border: 1px solid var(--scan-line); background: var(--admin-sidebar); color: var(--scan-text); }
  .admin-mobile-menu::backdrop { background: rgb(0 0 0 / .65); }
  .admin-mobile-menu-heading { display: flex; align-items: center; justify-content: space-between; gap: 16px; margin-bottom: 24px; font-size: 18px; }
  .admin-mobile-menu .admin-nav-group { margin-top: 20px; }
  .admin-layout :is(a, button):focus-visible { outline: 2px solid var(--scan-lime); outline-offset: 3px; }
  @media (max-width: 1023px) {
    .admin-topbar { display: grid; grid-template-columns: minmax(0, 1fr) auto auto; align-items: center; gap: 8px; width: min(100% - 32px, 1500px); min-height: 68px; padding-block: 12px; }
    .admin-topbar .scan-brand { min-width: 0; gap: 8px; }
    .admin-topbar .scan-brand-mark { flex: 0 0 29px; }
    .admin-topbar .scan-brand > span:last-child { min-width: 0; overflow-wrap: anywhere; }
    .admin-topbar :is(.admin-menu-trigger, .admin-logout) { display: inline-flex; align-items: center; justify-content: center; gap: 8px; min-width: 44px; min-height: 44px; padding: 10px; border: 1px solid var(--admin-control-border); border-radius: 4px; background: var(--admin-input); color: var(--admin-text); }
    .admin-logout-icon { display: block; flex: 0 0 22px; }
    .admin-logout-label { display: none; }
    .admin-layout .staff-login-label { display: none; }
    .admin-sidebar { display: none; }
    .admin-layout-grid { display: block; width: min(100% - 32px, 900px); }
    .admin-layout .staff-admin-content { padding: 36px 0 40px; }
  }
  @media (max-width: 600px) {
    .admin-menu-label { display: none; }
    .admin-page-heading { display: grid; gap: 20px; }
    .compact .admin-page-heading { gap: 12px; }
    .admin-heading-actions { justify-content: stretch; }
    .admin-heading-actions > * { flex: 1 1 auto; }
    .admin-mobile-menu { margin: 10px 10px auto auto; }
  }
  @media (max-width: 767px) { .mobile-help-description { display: none; } }
  @media (max-width: 767px) {
    .admin-page-heading { display: grid; grid-template-columns: minmax(0, 1fr); gap: 12px; }
    .admin-heading-actions { width: 100%; justify-content: stretch; }
    .admin-heading-actions > * { flex: 1; min-width: 0; width: 100%; text-align: center; }
  }
  @media (prefers-reduced-motion: reduce) { .admin-layout * { scroll-behavior: auto; } }
</style>
