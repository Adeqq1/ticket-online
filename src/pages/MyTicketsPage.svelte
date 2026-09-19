<script lang="ts">
  import { eventDate } from "../lib/concerts.ts";
  import { listTicketSnapshots, type TicketSnapshot } from "../lib/tickets.ts";

  const tickets = listTicketSnapshots();
  const upcoming = (ticket: TicketSnapshot) => Date.parse(ticket.concert.startsAt) > Date.now();
</script>

<svelte:head>
  <title>Tiket saya | Tiket Online</title>
  <meta name="description" content="Lihat kembali e-ticket konser yang tersimpan di dompet tiketmu." />
</svelte:head>

<section class="my-tickets shell">
  <header class="my-tickets-heading">
    <p class="ticket-kicker">Dompet tiket</p>
    <h1>Tiket saya.</h1>
    <p>Semua tiket yang kamu buat selama simulasi tersimpan di satu tempat.</p>
  </header>

  {#if tickets.length}
    <div class="my-tickets-summary" aria-live="polite">
      <strong>{tickets.length} tiket</strong>
      <span>Urut berdasarkan jadwal acara</span>
    </div>
    <ul class="my-tickets-list" aria-label="Daftar tiket tersimpan">
      {#each tickets as ticket, index (ticket.id)}
        <li>
          <article class="my-ticket-card">
            <a class="my-ticket-card-link" href={`/tiket/${encodeURIComponent(ticket.id)}`} aria-label={`Lihat e-ticket ${ticket.concert.artist}`}>
              <img src={ticket.concert.image} alt={`Poster ${ticket.concert.artist}`} width="240" height="300" loading={index < 2 ? "eager" : "lazy"} />
              <div class="my-ticket-card-body">
                <div class="my-ticket-card-topline">
                  <span class:past={!upcoming(ticket)} class="my-ticket-status">{upcoming(ticket) ? "Mendatang" : "Selesai"}</span>
                  <span class="my-ticket-reference">{ticket.reference}</span>
                </div>
                <h2>{ticket.concert.artist}</h2>
                <p class="my-ticket-date">{eventDate(ticket.concert.startsAt)}</p>
                <p class="my-ticket-venue">{ticket.concert.venue}, {ticket.concert.city}</p>
                <span class="my-ticket-action">Lihat e-ticket <span aria-hidden="true">→</span></span>
              </div>
            </a>
          </article>
        </li>
      {/each}
    </ul>
  {:else}
    <section class="my-tickets-empty" aria-labelledby="empty-title">
      <div class="ticket-illustration" aria-hidden="true"><span></span><span></span><i></i><b></b></div>
      <div>
        <p class="ticket-kicker">Belum ada tiket</p>
        <h2 id="empty-title">Dompetmu masih kosong.</h2>
        <p>Setelah checkout berhasil, e-ticket konsermu akan muncul di sini dan bisa dibuka kapan saja.</p>
        <a class="button" href="/konser">Jelajahi Konser</a>
      </div>
    </section>
  {/if}
</section>
