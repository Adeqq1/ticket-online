import { expect, test } from "bun:test";
import { concertMapUrl, publicConcertUrl } from "./concert-links.ts";

test("builds a clean public concert URL from the site origin", () => {
  expect(publicConcertUrl("konser /?#", "https://tickets.example/checkout/event?festival=2#private")).toBe("https://tickets.example/konser/konser%20%2F%3F%23");
});

test("builds a Google Maps search from available location fields", () => {
  const url = new URL(concertMapUrl({ venue: " Panggung & Suara ", address: "Jalan Musik #2", city: "Yogyakarta" })!);
  expect(url.origin).toBe("https://www.google.com");
  expect(url.pathname).toBe("/maps/search/");
  expect(url.searchParams.get("api")).toBe("1");
  expect(url.searchParams.get("query")).toBe("Panggung & Suara, Jalan Musik #2, Yogyakarta");
  expect(new URL(concertMapUrl({ venue: "Ruang Selatan", address: "Jl. Musik Raya, Jakarta", city: "Jakarta" })!).searchParams.get("query")).toBe("Ruang Selatan, Jl. Musik Raya, Jakarta");
  expect(concertMapUrl({ venue: " ", address: "", city: "  " })).toBeNull();
});
