<script lang="ts">
  import type { Concert } from "../../lib/concerts.ts";
  import { concertTerms } from "../../lib/terms.ts";
  let { concert }: { concert: Concert } = $props();
</script>

<div class="detail-info-list" aria-label="Informasi tambahan konser">
  <details class="detail-info"><summary>Deskripsi</summary><div class="detail-info-content"><h2>Tentang konser</h2><p>{concert.description}</p></div></details>
  <details class="detail-info"><summary>Line-up</summary><div class="detail-info-content"><h2>Line-up artis</h2><ul class="lineup-list">{#each concert.lineup as artist}<li>{artist}</li>{/each}</ul></div></details>
  <details class="detail-info"><summary>Lokasi</summary><div class="detail-info-content"><h2>Lokasi</h2><figure class="location-map"><div aria-hidden="true"><span></span><b>{concert.venue}</b></div><figcaption>{concert.address}</figcaption></figure></div></details>
  <details class="detail-info"><summary>Ketentuan</summary><div class="detail-info-content"><h2>Syarat dan ketentuan</h2><ul class="terms-list">{#each concertTerms as term}<li><b>{term.title}</b><br />{term.body}</li>{/each}</ul></div></details>
</div>
