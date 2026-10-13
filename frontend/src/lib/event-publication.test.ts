import { expect, test } from "bun:test";
import { publicationGaps } from "./event-publication.ts";
import type { AdminEventInput, ApiZone, AdminTicketTier } from "./api.ts";

test("publication requires complete event, zones and ticket gates; sold out capacity is allowed", () => {
  const event: AdminEventInput = { id: "konser", artist: "Artis", city: "Jakarta", venue: "Venue", address: "Alamat", startsAt: "2027-01-01T19:00", genre: "Pop", status: "Presale", publicationStatus: "DRAFT", image: "https://example.test/poster.jpg", description: "Deskripsi", lineup: ["Artis"] };
  const zones: ApiZone[] = [{ id: "a", name: "A", description: "" }];
  const tiers: AdminTicketTier[] = [{ id: "a", zoneId: "a", name: "A", gate: "Gate A", price: 100, capacity: 0, availableQuantity: 0, boundQuantity: 0, gateLocked: false, maxPerOrder: 1, benefit: "", seating: "free-standing" }];
  expect(publicationGaps(event, zones, tiers)).toEqual([]);
  expect(publicationGaps({ ...event, startsAt: "", description: "  ", lineup: [" "] }, [], []).map(gap => gap.section)).toEqual([1, 2, 3, 4]);
  expect(publicationGaps(event, [{ ...zones[0]!, name: " " }], [{ ...tiers[0]!, gate: " " }]).map(gap => gap.section)).toEqual([3, 4]);
});
