import { expect, test } from "bun:test";
import { ApiError, getEvent, mapApiEvent, type ApiEvent } from "./src/lib/api.ts";

const apiEvent: ApiEvent = {
  id: "nusa-malam", artist: "Nusa Malam", city: "Jakarta", venue: "Ruang Selatan", address: "Jl. Musik Raya, Jakarta",
  startsAt: "2027-08-24T12:30:00Z", genre: "Indie", status: "Early Bird", image: "https://example.com/nusa.jpg",
  description: "Deskripsi", lineup: ["Nusa Malam"], price: 225000,
  zones: [{ id: "festival", name: "Festival", description: "Area umum" }],
  ticketTiers: [{ id: "festival", name: "Festival", zoneId: "festival", price: 225000, availableQuantity: 42, maxPerOrder: 6, benefit: "Area berdiri", gate: "Gate B", seating: "free-standing" }],
};

test("maps API catalog DTO to the frontend concert model", () => {
  const concert = mapApiEvent(apiEvent);
  expect(concert.date).toContain("Selasa, 24 Agustus 2027");
  expect(concert.ticketTiers[0]).toMatchObject({ stock: 42, maxPerOrder: 6, price: 225000 });
  expect(concert.ticketTiers[0]?.seating).toBe("free-standing");
});

test("parses backend error responses without treating them as success", async () => {
  const originalFetch = globalThis.fetch;
  globalThis.fetch = (async () => new Response(JSON.stringify({ error: { code: "EVENT_NOT_FOUND", message: "Event tidak ditemukan" } }), { status: 404 })) as unknown as typeof fetch;
  try {
    await expect(getEvent("unknown")).rejects.toEqual(expect.objectContaining({ code: "EVENT_NOT_FOUND", status: 404, message: "Event tidak ditemukan" } satisfies Partial<ApiError>));
  } finally {
    globalThis.fetch = originalFetch;
  }
});
