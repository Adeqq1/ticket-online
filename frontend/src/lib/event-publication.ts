import type { AdminEventInput, ApiZone, AdminTicketTier } from "./api.ts";

export function publicationGaps(event: AdminEventInput, zones: ApiZone[], tiers: AdminTicketTier[]) {
  const gaps: { section: number; label: string }[] = [];
  if (!event.artist.trim()) gaps.push({ section: 0, label: "Nama konser" });
  if (!event.startsAt || !event.city.trim() || !event.venue.trim() || !event.address.trim()) gaps.push({ section: 1, label: "Jadwal dan lokasi" });
  if (!event.image.trim() || !event.description.trim() || !event.lineup.some(name => name.trim())) gaps.push({ section: 2, label: "Poster, deskripsi, dan lineup" });
  if (!zones.length || zones.some(zone => !zone.name.trim())) gaps.push({ section: 3, label: "Zona" });
  if (!tiers.length || tiers.some(tier => !tier.name.trim() || !tier.gate.trim())) gaps.push({ section: 4, label: "Kategori tiket dan gate" });
  return gaps;
}
