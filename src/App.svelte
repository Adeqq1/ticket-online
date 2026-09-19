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
  import { matchRoute } from "./lib/route.ts";
  import { onMount, tick } from "svelte";

  let route = $state(matchRoute(location.pathname));

  async function navigate(path: string) {
    history.pushState({}, "", path);
    route = matchRoute(location.pathname);
    await tick();
    window.scrollTo({ top: 0, behavior: "smooth" });
    document.querySelector<HTMLElement>("#konten")?.focus();
  }

  onMount(() => {
    const handlePopState = () => { route = matchRoute(location.pathname); };
    addEventListener("popstate", handlePopState);
    return () => removeEventListener("popstate", handlePopState);
  });
</script>

<SiteHeader page={route.name === "home" ? "home" : route.name === "concerts" ? "concerts" : route.name === "my-tickets" ? "my-tickets" : route.name === "guide" ? "guide" : "other"} />
<main id="konten" tabindex="-1">
  {#if route.name === "home"}<HomePage />
  {:else if route.name === "concerts"}<ConcertCatalogPage />
  {:else if route.name === "concert-detail"}<ConcertDetailPage id={route.id} />
  {:else if route.name === "checkout"}<CheckoutPage id={route.id} />
  {:else if route.name === "ticket"}<TicketPage id={route.id} />
  {:else if route.name === "my-tickets"}<MyTicketsPage />
  {:else if route.name === "guide"}<GuidePage />
  {:else}<UnknownRoutePage onHome={() => navigate("/")} />{/if}
</main>
<SiteFooter />
