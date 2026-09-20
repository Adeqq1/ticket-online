<script lang="ts">
  import SiteFooter from "./components/SiteFooter.svelte";
  import SiteHeader from "./components/SiteHeader.svelte";
  import ConcertCatalogPage from "./pages/ConcertCatalogPage.svelte";
  import ConcertDetailPage from "./pages/ConcertDetailPage.svelte";
  import CheckoutPage from "./pages/CheckoutPage.svelte";
  import HomePage from "./pages/HomePage.svelte";
  import TicketPage from "./pages/TicketPage.svelte";
  import MyTicketsPage from "./pages/MyTicketsPage.svelte";
  import GuidePage from "./pages/GuidePage.svelte";
  import UnknownRoutePage from "./pages/UnknownRoutePage.svelte";
  import AdminScanPage from "./pages/AdminScanPage.svelte";
  import { matchRoute } from "./lib/route.ts";
  const route = matchRoute(location.pathname);
</script>

{#if route.name !== "admin-scan"}<SiteHeader page={route.name === "home" ? "home" : route.name === "concerts" ? "concerts" : route.name === "my-tickets" ? "my-tickets" : route.name === "guide" ? "guide" : "other"} />{/if}
<main id="konten" tabindex="-1" class:admin-main={route.name === "admin-scan"}>
  {#if route.name === "home"}<HomePage />
  {:else if route.name === "concerts"}<ConcertCatalogPage />
  {:else if route.name === "concert-detail"}<ConcertDetailPage id={route.id} />
  {:else if route.name === "checkout"}<CheckoutPage id={route.id} />
  {:else if route.name === "ticket"}<TicketPage id={route.id} />
  {:else if route.name === "my-tickets"}<MyTicketsPage />
  {:else if route.name === "guide"}<GuidePage />
  {:else if route.name === "admin-scan"}<AdminScanPage />
  {:else}<UnknownRoutePage />{/if}
</main>
{#if route.name !== "admin-scan"}<SiteFooter />{/if}
