import { expect, test } from "bun:test";
import { filterConcerts, filterEvents, sortConcerts } from "./filters.ts";
import { concerts } from "./concerts.ts";

test("filters sample events by artist, city, and venue", () => {
  expect(filterEvents("Bandung")).toHaveLength(1);
  expect(filterEvents("nusa")).toHaveLength(1);
  expect(filterEvents("tidak ada")).toHaveLength(0);
});

test("filters by query", () => {
  expect(filterConcerts({ query: "ruang", genres: [], city: "", maxPrice: 1_500_000 }).map(({ artist }) => artist)).toEqual(["Nusa Malam", "Ruang Senja"]);
});

test("combines genre, city, and price", () => {
  expect(filterConcerts({ query: "", genres: ["Rock"], city: "Bandung", maxPrice: 300_000 }).map(({ artist }) => artist)).toEqual(["Ruang Senja", "Bara Utara"]);
});

test("includes exact maximum price", () => {
  expect(filterConcerts({ query: "", genres: [], city: "", maxPrice: 150_000 }).map(({ artist }) => artist)).toEqual(["Kamar Biru"]);
});

test("returns empty results", () => {
  expect(filterConcerts({ query: "tidak ada", genres: [], city: "", maxPrice: 1_500_000 })).toEqual([]);
});

test("leaves price unrestricted until a maximum is selected", () => {
  const expensive = { ...concerts[0]!, id: "expensive", price: 2_500_000 };
  expect(filterConcerts({ query: "", genres: [], city: "", maxPrice: null }, [expensive])).toEqual([expensive]);
  expect(filterConcerts({ query: "", genres: [], city: "", maxPrice: 2_500_000 }, [expensive])).toEqual([expensive]);
});

test("sorts matching concerts by effective date and lowest available price", () => {
  const events = [concerts[2]!, concerts[0]!, { ...concerts[1]!, currentEvent: { id: "ruang-senja", status: "RESCHEDULED" as const, version: 1, salesPaused: false, startsAt: "2027-08-20T19:30:00Z", announcement: "", refundDeadline: null } }];
  expect(sortConcerts(events, "date").map(({ id }) => id)).toEqual(["ruang-senja", "nusa-malam", "dini-hari"]);
  expect(sortConcerts(events, "price").map(({ id }) => id)).toEqual(["ruang-senja", "nusa-malam", "dini-hari"]);
});
