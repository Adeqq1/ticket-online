import { expect, test } from "bun:test";
import { filterConcerts } from "./konser.ts";

test("filters concerts with combined controls and inclusive maximum price", () => {
  expect(filterConcerts({ query: "", genres: ["Rock"], city: "Bandung", maxPrice: 300000 })).toHaveLength(2);
  expect(filterConcerts({ query: "ruang", genres: [], city: "", maxPrice: 1500000 })).toHaveLength(2);
  expect(filterConcerts({ query: "", genres: [], city: "", maxPrice: 150000 })).toHaveLength(1);
  expect(filterConcerts({ query: "tidak ada", genres: [], city: "", maxPrice: 1500000 })).toHaveLength(0);
});
