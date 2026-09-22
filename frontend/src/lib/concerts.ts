export type Genre = "Rock" | "Pop" | "Indie";
export type TicketStatus = "Early Bird" | "Presale" | "Sold Out";
export type StageZone = { id: string; name: string; description: string };
export type TicketTier = { id: string; name: string; zoneId: string; price: number; stock: number; maxPerOrder: number; benefit: string; gate: string; seating: "assigned" | "free-standing" };
export type Concert = { id: string; artist: string; city: string; venue: string; address: string; date: string; startsAt: string; genre: Genre; price: number; status: TicketStatus; image: string; description: string; lineup: string[]; zones: StageZone[]; ticketTiers: TicketTier[] };

const zones: StageZone[] = [
  { id: "vip-a", name: "VIP A", description: "Area depan panggung" },
  { id: "vip-b", name: "VIP B", description: "Area samping panggung" },
  { id: "festival", name: "Festival", description: "Area berdiri umum" },
  { id: "tribune", name: "Tribune", description: "Area duduk bertingkat" },
];

function tiers(status: TicketStatus): TicketTier[] {
  const soldOut = status === "Sold Out";
  return [
    { id: "vip-a", name: "VIP A", zoneId: "vip-a", price: 650000, stock: soldOut ? 0 : 5, maxPerOrder: 4, benefit: "Akses area depan panggung", gate: "Gate A", seating: "free-standing" },
    { id: "vip-b", name: "VIP B", zoneId: "vip-b", price: 475000, stock: soldOut ? 0 : 18, maxPerOrder: 4, benefit: "Akses area samping panggung", gate: "Gate A", seating: "free-standing" },
    { id: "festival", name: "Festival", zoneId: "festival", price: 275000, stock: soldOut ? 0 : 42, maxPerOrder: 6, benefit: "Area berdiri umum", gate: "Gate B", seating: "free-standing" },
    { id: "tribune", name: "Tribune", zoneId: "tribune", price: 350000, stock: soldOut ? 0 : 24, maxPerOrder: 4, benefit: "Area duduk bertingkat", gate: "Gate C", seating: "assigned" },
  ];
}

export const formatEventDate = new Intl.DateTimeFormat("id-ID", { dateStyle: "full", timeStyle: "short", timeZone: "Asia/Jakarta" });
export function eventDate(startsAt: string) { return `${formatEventDate.format(new Date(startsAt))} WIB`; }

function concert(id: string, artist: string, city: string, venue: string, startsAt: string, genre: Genre, price: number, status: TicketStatus): Concert {
  return { id, artist, city, venue, address: `Jl. Musik Raya, ${city}`, date: eventDate(startsAt), startsAt, genre, price, status, image: `https://picsum.photos/seed/${id}/900/1100`, description: `${artist} hadir dalam konser contoh dengan pilihan tiket untuk area panggung dan penonton.`, lineup: [artist, "Pembuka Sore", "Tamu Spesial"], zones, ticketTiers: tiers(status).map((tier) => tier.name === "Festival" ? { ...tier, price } : tier) };
}

export const concerts: Concert[] = [
  concert("nusa-malam", "Nusa Malam", "Jakarta", "Ruang Selatan", "2027-08-24T19:30:00+07:00", "Indie", 225000, "Early Bird"),
  concert("ruang-senja", "Ruang Senja", "Bandung", "Gudang Bunyi", "2027-08-30T19:30:00+07:00", "Rock", 175000, "Presale"),
  concert("dini-hari", "Dini Hari", "Yogyakarta", "Balai Irama", "2027-09-08T19:30:00+07:00", "Pop", 350000, "Presale"),
  concert("laut-kaca", "Laut Kaca", "Surabaya", "Panggung Timur", "2027-09-14T19:30:00+07:00", "Indie", 275000, "Sold Out"),
  concert("ritme-kota", "Ritme Kota", "Jakarta", "Aula Tengah", "2027-09-21T19:30:00+07:00", "Pop", 450000, "Early Bird"),
  concert("bara-utara", "Bara Utara", "Bandung", "Pabrik Nada", "2027-09-27T19:30:00+07:00", "Rock", 300000, "Presale"),
  concert("kamar-biru", "Kamar Biru", "Yogyakarta", "Teras Suara", "2027-10-05T19:30:00+07:00", "Indie", 150000, "Early Bird"),
  concert("gelombang-pagi", "Gelombang Pagi", "Surabaya", "Titik Temu", "2027-10-13T19:30:00+07:00", "Pop", 500000, "Presale"),
];

export const featuredConcerts = concerts.slice(0, 4);
export const formatRupiah = new Intl.NumberFormat("id-ID", { style: "currency", currency: "IDR", maximumFractionDigits: 0 });
export function getConcertById(id: string | undefined) { return concerts.find((concert) => concert.id === id); }
export function parseQuantities(concert: Concert, params: URLSearchParams) {
  return Object.fromEntries(concert.ticketTiers.flatMap((tier) => {
    const value = params.get(tier.id);
    if (!value || !/^\d+$/.test(value)) return [];
    const quantity = Number(value);
    const limit = Math.min(tier.stock, tier.maxPerOrder);
    return quantity > 0 && quantity <= limit ? [[tier.id, quantity]] : [];
  }));
}

export function parseCheckoutQuantities(concert: Concert, params: URLSearchParams) {
  return Object.fromEntries(concert.ticketTiers.flatMap((tier) => {
    const value = params.get(tier.id);
    if (!value || !/^\d+$/.test(value)) return [];
    const quantity = Number(value);
    return quantity > 0 && quantity <= tier.maxPerOrder ? [[tier.id, quantity]] : [];
  }));
}
