<script lang="ts">
  import { tick, type Snippet } from "svelte";

  type Action = { label: string; onclick?: () => void; form?: string; disabled?: boolean };
  type Page = "events" | "staff" | "orders" | "issues" | "operations" | "reports" | "attendance" | "conversion" | "history";
  let { page, contentId, skipLabel, kicker, title, description, onLogout, loggingOut, logoutDisabled, primaryAction, secondaryAction, children }: {
    page: Page; contentId: string; skipLabel: string; kicker: string; title: string; description: string;
    onLogout: () => void; loggingOut: boolean; logoutDisabled?: boolean; primaryAction: Action; secondaryAction?: Action; children: Snippet;
  } = $props();

  let dialog: HTMLDialogElement;
  let menuButton: HTMLButtonElement;
  const groups = [
    { label: "Pengelolaan", links: [{ page: "events", label: "Konser", href: "/admin/events" }, { page: "staff", label: "Kelola petugas", href: "/admin/staff" }] },
    { label: "Operasional", links: [{ page: "orders", label: "Pesanan", href: "/admin/orders" }, { page: "issues", label: "Masalah", href: "/admin/issues" }, { page: "operations", label: "Operasional", href: "/admin/operations" }, { page: "history", label: "Riwayat check-in", href: "/admin/check-ins" }] },
    { label: "Laporan", links: [{ page: "reports", label: "Penjualan dan refund", href: "/admin/reports" }, { page: "attendance", label: "Kehadiran", href: "/admin/reports/attendance" }, { page: "conversion", label: "Konversi", href: "/admin/reports/conversion" }] },
  ] as const;

  async function openMenu() {
    dialog.showModal();
    await tick();
    dialog.querySelector<HTMLButtonElement>("[data-menu-close]")?.focus();
  }

  function closeOnDesktop() {
    if (matchMedia("(min-width: 1024px)").matches && dialog.open) dialog.close();
  }

  function restoreMenuFocus() {
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

<div class="scan-shell staff-admin-shell admin-layout">
  <a class="skip-link" href={`#${contentId}`}>{skipLabel}</a>
  <header class="scan-topbar admin-topbar">
    <a class="scan-brand" href="/" aria-label="Kembali ke Tiket Online"><span class="scan-brand-mark" aria-hidden="true">TO</span><span>Tiket Online <b>/ Gate Control</b></span></a>
    <span class="staff-login-label">ADMINISTRATOR</span>
    <button class="staff-text-button admin-menu-trigger" bind:this={menuButton} type="button" aria-haspopup="dialog" aria-controls="admin-mobile-menu" onclick={openMenu}>Menu</button>
    <button class="staff-text-button admin-logout" type="button" disabled={logoutDisabled ?? loggingOut} onclick={onLogout}>Keluar</button>
  </header>

  <dialog id="admin-mobile-menu" class="admin-mobile-menu" bind:this={dialog} onclose={restoreMenuFocus} onkeydown={handleMenuKeydown}>
    <div class="admin-mobile-menu-heading"><strong>Navigasi admin</strong><button class="staff-text-button" data-menu-close type="button" onclick={() => dialog.close()}>Tutup</button></div>
    {#each groups as group (group.label)}
      <nav aria-label={group.label} class="admin-nav-group"><h2>{group.label}</h2>{#each group.links as link (link.href)}<a href={link.href} aria-current={page === link.page ? "page" : undefined}>{link.label}</a>{/each}</nav>
    {/each}
  </dialog>

  <div class="admin-layout-grid">
    <aside class="admin-sidebar" aria-label="Navigasi admin">
      {#each groups as group (group.label)}
        <nav aria-label={group.label} class="admin-nav-group"><h2>{group.label}</h2>{#each group.links as link (link.href)}<a href={link.href} aria-current={page === link.page ? "page" : undefined}>{link.label}</a>{/each}</nav>
      {/each}
    </aside>

    <main id={contentId} class="staff-admin-content" tabindex="-1">
      <div class="staff-admin-heading admin-page-heading"><div><p class="scan-kicker">{kicker}</p><h1>{title}</h1><p>{description}</p></div><div class="admin-heading-actions"><button class="scan-submit staff-submit" type={primaryAction.form ? "submit" : "button"} form={primaryAction.form} onclick={primaryAction.onclick} disabled={primaryAction.disabled}>{primaryAction.label}</button>{#if secondaryAction}<button class="staff-secondary-button" type="button" onclick={secondaryAction.onclick} disabled={secondaryAction.disabled}>{secondaryAction.label}</button>{/if}</div></div>
      {@render children()}
    </main>
  </div>
</div>

<svelte:window onresize={closeOnDesktop} />

<style>
  .admin-topbar { width: min(100% - 56px, 1500px); }
  .admin-topbar .scan-brand { min-height: 44px; }
  .admin-menu-trigger { display: none; }
  .admin-logout, .admin-mobile-menu-heading button { min-width: 44px; min-height: 44px; }
  .admin-layout-grid { display: grid; grid-template-columns: 248px minmax(0, 1fr); gap: clamp(28px, 4vw, 64px); width: min(100% - 56px, 1500px); margin: 0 auto; }
  .admin-sidebar { position: sticky; top: 20px; align-self: start; display: grid; gap: 24px; padding: 32px 0; }
  .admin-nav-group { display: grid; align-content: start; gap: 4px; }
  .admin-nav-group h2 { margin: 0 0 6px; color: var(--scan-muted); font: 14px "Courier New", monospace; letter-spacing: .06em; text-transform: uppercase; }
  .admin-nav-group a { display: flex; align-items: center; min-height: 44px; padding: 8px 12px; color: var(--scan-muted); font-size: 16px; line-height: 1.3; text-decoration: none; border-left: 2px solid transparent; }
  .admin-nav-group a:hover, .admin-nav-group a[aria-current="page"] { color: var(--scan-text); border-left-color: var(--scan-lime); background: color-mix(in srgb, var(--scan-lime) 7%, transparent); }
  .admin-layout .staff-admin-content { width: auto; min-width: 0; margin: 0; padding: 42px 0 40px; }
  .admin-page-heading { align-items: center; }
  .admin-page-heading > div:first-child { min-width: 0; }
  .admin-page-heading .scan-kicker { font-size: 14px; }
  .admin-page-heading > div > p:last-child { font-size: 16px; }
  .admin-heading-actions { display: flex; flex-wrap: wrap; align-items: center; justify-content: flex-end; gap: 10px; }
  .admin-heading-actions > * { min-height: 44px; }
  .admin-mobile-menu { width: min(100% - 28px, 440px); max-height: min(90dvh, 760px); margin: 12px 14px auto auto; padding: 20px; border: 1px solid var(--scan-line); background: var(--scan-panel); color: var(--scan-text); }
  .admin-mobile-menu::backdrop { background: rgb(0 0 0 / .65); }
  .admin-mobile-menu-heading { display: flex; align-items: center; justify-content: space-between; gap: 16px; margin-bottom: 24px; font-size: 18px; }
  .admin-mobile-menu .admin-nav-group { margin-top: 20px; }
  .admin-layout :is(a, button):focus-visible { outline: 2px solid var(--scan-lime); outline-offset: 3px; }
  @media (max-width: 1023px) {
    .admin-topbar { width: min(100% - 32px, 1500px); min-height: 64px; }
    .admin-menu-trigger { display: inline-flex; min-width: 44px; min-height: 44px; justify-content: center; }
    .admin-layout .staff-login-label { display: none; }
    .admin-sidebar { display: none; }
    .admin-layout-grid { display: block; width: min(100% - 32px, 900px); }
    .admin-layout .staff-admin-content { padding: 36px 0 40px; }
  }
  @media (max-width: 600px) {
    .admin-page-heading { display: grid; gap: 20px; }
    .admin-heading-actions { justify-content: stretch; }
    .admin-heading-actions > * { flex: 1 1 auto; }
    .admin-mobile-menu { margin: 10px 10px auto auto; }
  }
  @media (prefers-reduced-motion: reduce) { .admin-layout * { scroll-behavior: auto; } }
</style>
