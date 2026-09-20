<script lang="ts">
  import type { Concert } from "../../lib/concerts.ts";
  import type { Tab } from "../../lib/cart.ts";
  import { detailTerms } from "../../lib/terms.ts";
  let { concert }: { concert: Concert } = $props();
  let active = $state<Tab>("description");
  const tabs: Array<{ id: Tab; label: string }> = [{ id: "description", label: "Deskripsi" }, { id: "lineup", label: "Line-up" }, { id: "location", label: "Lokasi" }, { id: "terms", label: "Ketentuan" }];
  let buttons: HTMLButtonElement[] = [];
  function keydown(event: KeyboardEvent, index: number) {
    const next = event.key === "ArrowRight" ? (index + 1) % tabs.length : event.key === "ArrowLeft" ? (index - 1 + tabs.length) % tabs.length : event.key === "Home" ? 0 : event.key === "End" ? tabs.length - 1 : index;
    if (next === index && event.key !== "Home" && event.key !== "End") return;
    event.preventDefault(); active = tabs[next]!.id; buttons[next]?.focus();
  }
</script>

<div class="tabs" role="tablist" aria-label="Informasi konser">{#each tabs as tab, index}<button bind:this={buttons[index]} role="tab" id={`tab-${tab.id}`} aria-selected={active === tab.id} aria-controls={`panel-${tab.id}`} tabindex={active === tab.id ? 0 : -1} onclick={() => active = tab.id} onkeydown={(event) => keydown(event, index)}>{tab.label}</button>{/each}</div>
<div id="panel-description" role="tabpanel" aria-labelledby="tab-description" hidden={active !== "description"}><h2>Untuk malam yang panjang.</h2><p>{concert.description}</p></div>
<div id="panel-lineup" role="tabpanel" aria-labelledby="tab-lineup" hidden={active !== "lineup"}><h2>Line-up artis</h2><ul class="lineup-list">{#each concert.lineup as artist}<li>{artist}</li>{/each}</ul></div>
<div id="panel-location" role="tabpanel" aria-labelledby="tab-location" hidden={active !== "location"}><h2>Lokasi</h2><figure class="location-map"><div aria-hidden="true"><span></span><b>{concert.venue}</b></div><figcaption>{concert.address}</figcaption></figure></div>
<div id="panel-terms" role="tabpanel" aria-labelledby="tab-terms" hidden={active !== "terms"}><h2>Syarat & ketentuan</h2><ul class="terms-list">{#each detailTerms as term}<li>{term}</li>{/each}</ul></div>
