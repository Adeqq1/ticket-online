<script lang="ts">
  import { onMount, type Component } from "svelte";
  import SiteFooter from "./layouts/SiteFooter.svelte";
  import SiteHeader from "./layouts/SiteHeader.svelte";
  import { matchRoute } from "./lib/route.ts";

  const route = $state(matchRoute(location.pathname));
  const isAdmin = route.name.startsWith("admin-");
  let Page = $state<Component<any>>();
  let pageProps = $state<Record<string, string>>({});
  let pageError = $state(false);

  async function loadPage() {
    pageError = false;
    Page = undefined;
    pageProps = {};
    try {
      switch (route.name) {
        case "home": Page = (await import("./pages/public/HomePage.svelte")).default; break;
        case "concerts": Page = (await import("./pages/public/ConcertCatalogPage.svelte")).default; break;
        case "concert-detail": Page = (await import("./pages/public/ConcertDetailPage.svelte")).default; pageProps = { id: route.id }; break;
        case "checkout": Page = (await import("./pages/public/CheckoutPage.svelte")).default; pageProps = { id: route.id }; break;
        case "ticket": Page = (await import("./pages/public/TicketPage.svelte")).default; pageProps = { id: route.id }; break;
        case "my-tickets": Page = (await import("./pages/public/MyTicketsPage.svelte")).default; break;
        case "ticket-recovery": Page = (await import("./pages/public/TicketRecoveryPage.svelte")).default; break;
        case "order": Page = (await import("./pages/public/OrderPage.svelte")).default; pageProps = { id: route.id }; break;
        case "guide": Page = (await import("./pages/public/GuidePage.svelte")).default; break;
        case "admin-login": Page = (await import("./layouts/AdminArea.svelte")).default; pageProps = { page: "login" }; break;
        case "admin-staff": Page = (await import("./layouts/AdminArea.svelte")).default; pageProps = { page: "staff" }; break;
        case "admin-events": Page = (await import("./layouts/AdminArea.svelte")).default; pageProps = { page: "events" }; break;
        case "admin-orders": Page = (await import("./layouts/AdminArea.svelte")).default; pageProps = { page: "orders" }; break;
        case "admin-issues": Page = (await import("./layouts/AdminArea.svelte")).default; pageProps = { page: "issues" }; break;
        case "admin-operations": Page = (await import("./layouts/AdminArea.svelte")).default; pageProps = { page: "operations" }; break;
        case "admin-reports": Page = (await import("./layouts/AdminArea.svelte")).default; pageProps = { page: "reports" }; break;
        case "admin-attendance-report": Page = (await import("./layouts/AdminArea.svelte")).default; pageProps = { page: "attendance" }; break;
        case "admin-conversion-report": Page = (await import("./layouts/AdminArea.svelte")).default; pageProps = { page: "conversion" }; break;
        case "admin-check-ins": Page = (await import("./layouts/AdminArea.svelte")).default; pageProps = { page: "history" }; break;
        case "admin-scan": Page = (await import("./layouts/AdminArea.svelte")).default; pageProps = { page: "scan" }; break;
        default: Page = (await import("./pages/public/UnknownRoutePage.svelte")).default;
      }
    } catch {
      pageError = true;
    }
  }

  onMount(() => { void loadPage(); });
</script>

<div class:public-site={!isAdmin}>
{#if !isAdmin}<SiteHeader page={route.name === "home" ? "home" : route.name === "concerts" ? "concerts" : route.name === "my-tickets" ? "my-tickets" : route.name === "guide" ? "guide" : "other"} />{/if}
<main id="konten" tabindex="-1" class:admin-main={isAdmin} class:route-loading={!Page && !pageError}>
  {#if Page}
    <Page {...pageProps} />
  {:else if pageError}
    <section class="shell empty-state" role="alert"><p>Halaman belum dapat dimuat. Periksa koneksi internet lalu coba lagi.</p><button class="text-button" type="button" onclick={() => location.reload()}>Coba lagi</button></section>
  {:else}
    <p class="shell" role="status" aria-live="polite">Memuat halaman...</p>
  {/if}
</main>
{#if !isAdmin}<SiteFooter />{/if}
</div>
