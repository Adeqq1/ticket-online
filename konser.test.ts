import { expect, test } from "bun:test";
import { filterConcerts } from "./konser.ts";

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
