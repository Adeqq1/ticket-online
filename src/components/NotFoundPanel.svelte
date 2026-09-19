<script lang="ts">
  let { kicker = "Tidak ditemukan", title, message, href = "/konser", link = "Kembali ke konser", className = "detail-missing" }: { kicker?: string; title: string; message: string; href?: string; link?: string; className?: string } = $props();
</script>

<section class={`${className} shell`}>
  {#if kicker}<p class="ticket-kicker">{kicker}</p>{/if}
  <h1>{title}</h1><p>{message}</p><a class="button" {href}>{link}</a>
</section>
