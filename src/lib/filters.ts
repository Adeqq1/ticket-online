import { concerts, featuredConcerts, type Concert } from "./concerts.ts";

export type ConcertFilters = { query: string; genres: string[]; city: string; maxPrice: number };

export function filterEvents(query: string, source: Concert[] = featuredConcerts): Concert[] {
  const normalized = query.trim().toLocaleLowerCase("id-ID");
  if (!normalized) return source;
  return source.filter((event) =>
    [event.artist, event.city, event.venue].some((value) => value.toLocaleLowerCase("id-ID").includes(normalized)),
  );
}

export function filterConcerts(filters: ConcertFilters, source = concerts): Concert[] {
  const query = filters.query.trim().toLocaleLowerCase("id-ID");
  return source.filter((concert) => {
    const matchesQuery = !query || [concert.artist, concert.city, concert.venue].some((value) => value.toLocaleLowerCase("id-ID").includes(query));
    return matchesQuery && (!filters.genres.length || filters.genres.includes(concert.genre)) && (!filters.city || concert.city === filters.city) && concert.price <= filters.maxPrice;
  });
}
