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
  import OrderPage from "./pages/OrderPage.svelte";
  import TicketRecoveryPage from "./pages/TicketRecoveryPage.svelte";
  import UnknownRoutePage from "./pages/UnknownRoutePage.svelte";
  import AdminArea from "./AdminArea.svelte";
  import { matchRoute } from "./lib/route.ts";
  const route = $state(matchRoute(location.pathname));
</script>

{#if !["admin-scan", "admin-staff", "admin-events", "admin-check-ins", "admin-login"].includes(route.name)}<SiteHeader page={route.name === "home" ? "home" : route.name === "concerts" ? "concerts" : route.name === "my-tickets" ? "my-tickets" : route.name === "guide" ? "guide" : "other"} />{/if}
<main id="konten" tabindex="-1" class:admin-main={["admin-scan", "admin-staff", "admin-events", "admin-check-ins", "admin-login"].includes(route.name)}>
  {#if route.name === "home"}<HomePage />
  {:else if route.name === "concerts"}<ConcertCatalogPage />
  {:else if route.name === "concert-detail"}<ConcertDetailPage id={route.id} />
  {:else if route.name === "checkout"}<CheckoutPage id={route.id} />
  {:else if route.name === "ticket"}<TicketPage id={route.id} />
  {:else if route.name === "my-tickets"}<MyTicketsPage />
  {:else if route.name === "ticket-recovery"}<TicketRecoveryPage />
  {:else if route.name === "order"}<OrderPage id={route.id} />
  {:else if route.name === "guide"}<GuidePage />
  {:else if route.name === "admin-login"}<AdminArea page="login" />
  {:else if route.name === "admin-staff"}<AdminArea page="staff" />
  {:else if route.name === "admin-events"}<AdminArea page="events" />
  {:else if route.name === "admin-check-ins"}<AdminArea page="history" />
  {:else if route.name === "admin-scan"}<AdminArea page="scan" />
  {:else}<UnknownRoutePage />{/if}
</main>
{#if !["admin-scan", "admin-staff", "admin-events", "admin-check-ins", "admin-login"].includes(route.name)}<SiteFooter />{/if}
