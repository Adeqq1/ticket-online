import { concerts, featuredConcerts, type Concert } from "./concerts.ts";

export type ConcertFilters = { query: string; genres: string[]; city: string; maxPrice: number | null };
export type ConcertSort = "date" | "price";

export function filterEvents(query: string, source: Concert[] = featuredConcerts): Concert[] {
  const normalized = query.trim().toLocaleLowerCase("id-ID");
  if (!normalized) return source;
  return source.filter((event) =>
    [event.artist, event.city, event.venue].some((value) => value.toLocaleLowerCase("id-ID").includes(normalized)),
  );
}

export function filterConcerts(filters: ConcertFilters, source: Concert[] = concerts): Concert[] {
  const query = filters.query.trim().toLocaleLowerCase("id-ID");
  return source.filter((concert) => {
    const matchesQuery = !query || [concert.artist, concert.city, concert.venue].some((value) => value.toLocaleLowerCase("id-ID").includes(query));
    return matchesQuery && (!filters.genres.length || filters.genres.includes(concert.genre)) && (!filters.city || concert.city === filters.city) && (filters.maxPrice === null || concert.price <= filters.maxPrice);
  });
}

export function sortConcerts(source: Concert[], sort: ConcertSort): Concert[] {
  return source.map((concert, index) => {
    const value = sort === "price" ? concert.ticketTiers.length ? concert.price : null : Date.parse(concert.currentEvent?.startsAt ?? concert.startsAt);
    return { concert, index, value: value !== null && !Number.isFinite(value) ? null : value };
  })
    .sort((a, b) => a.value === null ? b.value === null ? a.index - b.index : 1 : b.value === null ? -1 : a.value - b.value || a.index - b.index)
    .map(({ concert }) => concert);
}
