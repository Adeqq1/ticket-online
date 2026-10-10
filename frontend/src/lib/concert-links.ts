export function publicConcertUrl(id: string, origin = globalThis.location.origin): string {
  return `${new URL(origin).origin}/konser/${encodeURIComponent(id)}`;
}

export function concertMapUrl(location: { venue: string; address: string; city: string }): string | null {
  const queryParts = [location.venue, location.address].map((value) => value.trim()).filter(Boolean);
  const city = location.city.trim();
  if (city && !queryParts.some((part) => part.toLocaleLowerCase("id-ID").includes(city.toLocaleLowerCase("id-ID")))) queryParts.push(city);
  const query = queryParts.join(", ");
  if (!query) return null;
  const url = new URL("https://www.google.com/maps/search/");
  url.searchParams.set("api", "1");
  url.searchParams.set("query", query);
  return url.href;
}
