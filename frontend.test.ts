import { expect, test } from "bun:test";
import { filterEvents } from "./src/lib/filters.ts";

test("filters sample events by artist, city, and venue", () => {
  expect(filterEvents("Bandung")).toHaveLength(1);
  expect(filterEvents("nusa")).toHaveLength(1);
  expect(filterEvents("tidak ada")).toHaveLength(0);
});
