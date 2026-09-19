export type Concert = {
  artist: string;
  city: string;
  venue: string;
  date: string;
  genre: "Rock" | "Pop" | "Indie";
  price: number;
  status: "Early Bird" | "Presale" | "Sold Out";
  image: string;
};

export type ConcertFilters = { query: string; genres: string[]; city: string; maxPrice: number };

const concerts: Concert[] = [
  { artist: "Nusa Malam", city: "Jakarta", venue: "Ruang Selatan", date: "Sabtu, 24 Agustus", genre: "Indie", price: 225000, status: "Early Bird", image: "https://picsum.photos/seed/nusa-malam-jakarta/900/1100" },
  { artist: "Ruang Senja", city: "Bandung", venue: "Gudang Bunyi", date: "Jumat, 30 Agustus", genre: "Rock", price: 175000, status: "Presale", image: "https://picsum.photos/seed/ruang-senja-bandung/900/1100" },
  { artist: "Dini Hari", city: "Yogyakarta", venue: "Balai Irama", date: "Minggu, 8 September", genre: "Pop", price: 350000, status: "Presale", image: "https://picsum.photos/seed/dini-hari-yogyakarta/900/1100" },
  { artist: "Laut Kaca", city: "Surabaya", venue: "Panggung Timur", date: "Sabtu, 14 September", genre: "Indie", price: 275000, status: "Sold Out", image: "https://picsum.photos/seed/laut-kaca-surabaya/900/1100" },
  { artist: "Ritme Kota", city: "Jakarta", venue: "Aula Tengah", date: "Sabtu, 21 September", genre: "Pop", price: 450000, status: "Early Bird", image: "https://picsum.photos/seed/ritme-kota-jakarta/900/1100" },
  { artist: "Bara Utara", city: "Bandung", venue: "Pabrik Nada", date: "Jumat, 27 September", genre: "Rock", price: 300000, status: "Presale", image: "https://picsum.photos/seed/bara-utara-bandung/900/1100" },
  { artist: "Kamar Biru", city: "Yogyakarta", venue: "Teras Suara", date: "Sabtu, 5 Oktober", genre: "Indie", price: 150000, status: "Early Bird", image: "https://picsum.photos/seed/kamar-biru-yogyakarta/900/1100" },
  { artist: "Gelombang Pagi", city: "Surabaya", venue: "Titik Temu", date: "Minggu, 13 Oktober", genre: "Pop", price: 500000, status: "Presale", image: "https://picsum.photos/seed/gelombang-pagi-surabaya/900/1100" },
];

export function filterConcerts(filters: ConcertFilters, source = concerts): Concert[] {
  const query = filters.query.trim().toLocaleLowerCase("id-ID");
  return source.filter((concert) => {
    const matchesQuery = !query || [concert.artist, concert.city, concert.venue].some((value) => value.toLocaleLowerCase("id-ID").includes(query));
    return matchesQuery && (!filters.genres.length || filters.genres.includes(concert.genre)) && (!filters.city || concert.city === filters.city) && concert.price <= filters.maxPrice;
  });
}

const rupiah = new Intl.NumberFormat("id-ID", { style: "currency", currency: "IDR", maximumFractionDigits: 0 });

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
    meta.textContent = `${concert.date} · ${concert.genre}`;
    const title = document.createElement("h2");
    title.textContent = concert.artist;
    const venue = document.createElement("p");
    venue.className = "concert-venue";
    venue.textContent = `${concert.venue}, ${concert.city}`;
    const priceText = document.createElement("p");
    priceText.className = "concert-price";
    priceText.textContent = `Mulai ${rupiah.format(concert.price)}`;
    body.append(meta, title, venue, priceText);
    article.append(image, status, body);
    return article;
  }

  function render() {
    if (!grid || !count || !empty) return;
    const matches = filterConcerts(currentFilters());
    grid.replaceChildren(...matches.map(createCard));
    count.textContent = `${matches.length} konser ditemukan`;
    empty.hidden = matches.length !== 0;
  }

  function reset() {
    form?.reset();
    if (price) price.value = price.max;
    if (output && price) output.value = `Sampai ${rupiah.format(Number(price.value))}`;
    render();
    query?.focus();
  }

  form?.addEventListener("input", () => {
    if (output && price) output.value = `Sampai ${rupiah.format(Number(price.value))}`;
    render();
  });
  form?.addEventListener("change", render);
  resetButtons.forEach((button) => button.addEventListener("click", reset));
  render();
}
