export type Genre = "Rock" | "Pop" | "Indie";
export type TicketStatus = "Early Bird" | "Presale" | "Sold Out";
export type StageZone = { id: string; name: string; description: string };
export type TicketTier = { id: string; name: string; zoneId: string; price: number; stock: number; maxPerOrder: number; benefit: string };
export type Concert = { id: string; artist: string; city: string; venue: string; address: string; date: string; genre: Genre; price: number; status: TicketStatus; image: string; description: string; lineup: string[]; zones: StageZone[]; ticketTiers: TicketTier[] };

const zones: StageZone[] = [
  { id: "vip-a", name: "VIP A", description: "Area depan panggung" },
  { id: "vip-b", name: "VIP B", description: "Area samping panggung" },
  { id: "festival", name: "Festival", description: "Area berdiri umum" },
  { id: "tribune", name: "Tribune", description: "Area duduk bertingkat" },
];

function tiers(status: TicketStatus): TicketTier[] {
  const soldOut = status === "Sold Out";
  return [
    { id: "vip-a", name: "VIP A", zoneId: "vip-a", price: 650000, stock: soldOut ? 0 : 5, maxPerOrder: 4, benefit: "Akses area depan panggung" },
    { id: "vip-b", name: "VIP B", zoneId: "vip-b", price: 475000, stock: soldOut ? 0 : 18, maxPerOrder: 4, benefit: "Akses area samping panggung" },
    { id: "festival", name: "Festival", zoneId: "festival", price: 275000, stock: soldOut ? 0 : 42, maxPerOrder: 6, benefit: "Area berdiri umum" },
    { id: "tribune", name: "Tribune", zoneId: "tribune", price: 350000, stock: soldOut ? 0 : 24, maxPerOrder: 4, benefit: "Area duduk bertingkat" },
  ];
}

function concert(id: string, artist: string, city: string, venue: string, date: string, genre: Genre, price: number, status: TicketStatus): Concert {
  return { id, artist, city, venue, address: `Jl. Musik Raya, ${city}`, date, genre, price, status, image: `https://picsum.photos/seed/${id}/900/1100`, description: `${artist} hadir dalam konser contoh dengan pilihan tiket untuk area panggung dan penonton.`, lineup: [artist, "Pembuka Sore", "Tamu Spesial"], zones, ticketTiers: tiers(status).map((tier) => tier.name === "Festival" ? { ...tier, price } : tier) };
}

export const concerts: Concert[] = [
  concert("nusa-malam", "Nusa Malam", "Jakarta", "Ruang Selatan", "Sabtu, 24 Agustus", "Indie", 225000, "Early Bird"),
  concert("ruang-senja", "Ruang Senja", "Bandung", "Gudang Bunyi", "Jumat, 30 Agustus", "Rock", 175000, "Presale"),
  concert("dini-hari", "Dini Hari", "Yogyakarta", "Balai Irama", "Minggu, 8 September", "Pop", 350000, "Presale"),
  concert("laut-kaca", "Laut Kaca", "Surabaya", "Panggung Timur", "Sabtu, 14 September", "Indie", 275000, "Sold Out"),
  concert("ritme-kota", "Ritme Kota", "Jakarta", "Aula Tengah", "Sabtu, 21 September", "Pop", 450000, "Early Bird"),
  concert("bara-utara", "Bara Utara", "Bandung", "Pabrik Nada", "Jumat, 27 September", "Rock", 300000, "Presale"),
  concert("kamar-biru", "Kamar Biru", "Yogyakarta", "Teras Suara", "Sabtu, 5 Oktober", "Indie", 150000, "Early Bird"),
  concert("gelombang-pagi", "Gelombang Pagi", "Surabaya", "Titik Temu", "Minggu, 13 Oktober", "Pop", 500000, "Presale"),
];

export const featuredConcerts = concerts.slice(0, 4);
export const formatRupiah = new Intl.NumberFormat("id-ID", { style: "currency", currency: "IDR", maximumFractionDigits: 0 });
export function getConcertById(id: string | undefined) { return concerts.find((concert) => concert.id === id); }
