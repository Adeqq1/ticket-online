<script lang="ts">
  import type { StageZone, TicketTier } from "../../lib/concerts.ts";
  let { zones, tiers, selectedZoneId, onselect }: { zones: StageZone[]; tiers: TicketTier[]; selectedZoneId: string | null; onselect: (zoneId: string, event: MouseEvent | KeyboardEvent) => void } = $props();
  const position = (zoneId: string) => zoneId === "festival" ? "map-festival" : zoneId === "tribune" ? "map-tribune" : "map-vip";
</script>

<div class="stage-layout" role="group" aria-label="Denah area konser">
  <div class="stage">PANGGUNG</div>
  {#each zones as zone}
    {@const zoneTiers = tiers.filter((item) => item.zoneId === zone.id)}
    {@const tier = zoneTiers.find((item) => item.stock > 0) ?? zoneTiers[0]}
    {@const stock = zoneTiers.reduce((total, item) => total + item.stock, 0)}
    <button class:map-vip={position(zone.id) === "map-vip"} class:map-festival={position(zone.id) === "map-festival"} class:map-tribune={position(zone.id) === "map-tribune"} class:is-sold-out={!tier || stock === 0} class="zone" type="button" aria-pressed={selectedZoneId === zone.id} aria-controls={tier ? `tier-${tier.id}` : undefined} disabled={!tier || stock === 0} onclick={(event) => onselect(zone.id, event)}>
      <span>{zone.name}</span>
      <small>{zone.description}</small>
      {#if tier && stock > 0}<em>{stock <= 5 ? `Sisa ${stock}` : "Tersedia"}</em>{/if}
    </button>
  {/each}
</div>
