import { concerts, featuredConcerts, type Concert } from "./concerts.ts";

export type ConcertFilters = { query: string; genres: string[]; city: string; maxPrice: number | null; dateFrom?: string; dateTo?: string };
export type ConcertSort = "date" | "price";

export function validCalendarDate(value: string): boolean {
  return /^\d{4}-\d{2}-\d{2}$/.test(value) && value >= "0001-01-01" && Number.isFinite(Date.parse(`${value}T00:00:00Z`)) && new Date(`${value}T00:00:00Z`).toISOString().slice(0, 10) === value;
}

export function dateRangeError(dateFrom = "", dateTo = ""): string {
  if (dateFrom && !validCalendarDate(dateFrom) || dateTo && !validCalendarDate(dateTo)) return "Masukkan tanggal yang valid.";
  return dateFrom && dateTo && dateTo < dateFrom ? "Tanggal akhir tidak boleh sebelum tanggal mulai." : "";
}

export function filterEvents(query: string, source: Concert[] = featuredConcerts): Concert[] {
  return filterConcerts({ query, genres: [], city: "", maxPrice: null }, source);
}

export function filterConcerts(filters: ConcertFilters, source: Concert[] = concerts): Concert[] {
  const query = filters.query.trim().toLocaleLowerCase("id-ID");
  const { dateFrom = "", dateTo = "" } = filters;
  if (dateRangeError(dateFrom, dateTo)) return [];
  return source.filter((concert) => {
    const matchesQuery = !query || [concert.artist, concert.city, concert.venue].some((value) => value.toLocaleLowerCase("id-ID").includes(query));
    const startsAt = concert.currentEvent ? concert.currentEvent.startsAt : concert.startsAt;
    const timestamp = startsAt ? Date.parse(startsAt) : NaN;
    const day = Number.isFinite(timestamp) ? new Date(timestamp + 7 * 60 * 60 * 1000).toISOString().slice(0, 10) : "";
    const matchesDate = !dateFrom && !dateTo || Boolean(day) && (!dateFrom || day >= dateFrom) && (!dateTo || day <= dateTo);
    return matchesQuery && matchesDate && (!filters.genres.length || filters.genres.includes(concert.genre)) && (!filters.city || concert.city === filters.city) && (filters.maxPrice === null || concert.price <= filters.maxPrice);
  });
}

export function sortConcerts(source: Concert[], sort: ConcertSort): Concert[] {
  return source.map((concert, index) => {
    const start = concert.currentEvent ? concert.currentEvent.startsAt : concert.startsAt;
    const value = sort === "price" ? concert.ticketTiers.length ? concert.price : null : start ? Date.parse(start) : null;
    return { concert, index, value: value !== null && !Number.isFinite(value) ? null : value };
  })
    .sort((a, b) => a.value === null ? b.value === null ? a.index - b.index : 1 : b.value === null ? -1 : a.value - b.value || a.index - b.index)
    .map(({ concert }) => concert);
}
