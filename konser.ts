import { concerts, eventDate, formatRupiah, type Concert } from "./concerts.ts";

export type ConcertFilters = { query: string; genres: string[]; city: string; maxPrice: number };

export function filterConcerts(filters: ConcertFilters, source = concerts): Concert[] {
  const query = filters.query.trim().toLocaleLowerCase("id-ID");
  return source.filter((concert) => {
    const matchesQuery = !query || [concert.artist, concert.city, concert.venue].some((value) => value.toLocaleLowerCase("id-ID").includes(query));
    return matchesQuery && (!filters.genres.length || filters.genres.includes(concert.genre)) && (!filters.city || concert.city === filters.city) && concert.price <= filters.maxPrice;
  });
}

if (typeof document !== "undefined") {
  const form = document.querySelector<HTMLFormElement>("#concert-filters");
  const query = document.querySelector<HTMLInputElement>("#concert-query");
  const city = document.querySelector<HTMLSelectElement>("#concert-city");
  const price = document.querySelector<HTMLInputElement>("#concert-price");
  const output = document.querySelector<HTMLOutputElement>("#price-output");
  const grid = document.querySelector<HTMLDivElement>("#concert-grid");
  const count = document.querySelector<HTMLParagraphElement>("#results-count");
  const empty = document.querySelector<HTMLElement>("#catalog-empty");
  const resetButtons = document.querySelectorAll<HTMLButtonElement>("#reset-filters, #empty-reset");

  function currentFilters(): ConcertFilters {
    return { query: query?.value ?? "", genres: [...document.querySelectorAll<HTMLInputElement>('input[name="genre"]:checked')].map((input) => input.value), city: city?.value ?? "", maxPrice: Number(price?.value ?? 1500000) };
  }

  function createCard(concert: Concert) {
    const article = document.createElement("article");
    article.className = `concert-card${concert.status === "Sold Out" ? " is-sold-out" : ""}`;
    const link = document.createElement("a");
    link.className = "concert-card-link";
    link.href = `/konser/${concert.id}`;
    const image = document.createElement("img");
    image.src = concert.image;
    image.alt = `Poster contoh konser ${concert.artist}`;
    image.width = 900;
    image.height = 1100;
    image.loading = "lazy";
    const status = document.createElement("span");
    status.className = "concert-status";
    status.textContent = concert.status;
    const body = document.createElement("div");
    body.className = "concert-card-body";
    const meta = document.createElement("p");
    meta.className = "concert-meta";
    meta.textContent = `${eventDate(concert.startsAt)}, ${concert.genre}`;
    const title = document.createElement("h2");
    title.textContent = concert.artist;
    const venue = document.createElement("p");
    venue.className = "concert-venue";
    venue.textContent = `${concert.venue}, ${concert.city}`;
    const priceText = document.createElement("p");
    priceText.className = "concert-price";
    priceText.textContent = `Mulai ${formatRupiah.format(concert.price)}`;
    body.append(meta, title, venue, priceText);
    link.append(image, status, body);
    article.append(link);
    return article;
  }

  function render() {
    if (!grid || !count || !empty) return;
    const matches = filterConcerts(currentFilters());
    grid.replaceChildren(...matches.map(createCard));
    count.textContent = matches.length ? `${matches.length} konser ditemukan` : "Tidak ada konser ditemukan";
    empty.hidden = matches.length !== 0;
  }

  function reset() {
    form?.reset();
    if (price) price.value = price.max;
    if (output && price) output.value = `Sampai ${formatRupiah.format(Number(price.value))}`;
    render();
    query?.focus();
  }

  form?.addEventListener("submit", (event) => {
    event.preventDefault();
    render();
  });
  form?.addEventListener("input", () => {
    if (output && price) output.value = `Sampai ${formatRupiah.format(Number(price.value))}`;
    render();
  });
  resetButtons.forEach((button) => button.addEventListener("click", reset));
  render();
}
