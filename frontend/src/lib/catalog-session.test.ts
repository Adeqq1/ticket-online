import { expect, test } from "bun:test";
import { catalogSessionKey, readCatalogSession, saveCatalogSession, type CatalogSession } from "./catalog-session.ts";

const session: CatalogSession = { query: "senja", genres: ["Jazz"], city: "Bandung", dateFrom: "2027-08-01", dateTo: "2027-08-31", maxPrice: null, sort: "price", filtersOpen: true, scrollY: 840 };

function storage(initial: string | null = null) {
  let value = initial;
  return { getItem: (key: string) => key === catalogSessionKey ? value : null, setItem: (key: string, next: string) => { if (key === catalogSessionKey) value = next; }, value: () => value };
}

test("round trips a catalog session under its own key", () => {
  const target = storage();
  expect(saveCatalogSession(session, target)).toBe(true);
  expect(readCatalogSession(target)).toEqual(session);
});

test("ignores malformed or invalid sessions and unavailable storage", () => {
  expect(readCatalogSession(storage("{"))).toBeNull();
  expect(readCatalogSession(storage(JSON.stringify({ ...session, scrollY: -1 })))).toBeNull();
  expect(readCatalogSession(null)).toBeNull();
  expect(saveCatalogSession(session, { setItem() { throw new Error("blocked"); } })).toBe(false);
});


test("restores legacy sessions with empty dates and preserves scroll and filters", () => {
  const { dateFrom, dateTo, ...legacy } = session;
  expect(readCatalogSession(storage(JSON.stringify(legacy)))).toEqual({ ...legacy, dateFrom: "", dateTo: "" });
});

test("rejects corrupt dates while preserving an editable reversed range", () => {
  for (const dateFrom of [42, "2027-02-29", "not-a-date", "2027-13-01"]) {
    expect(readCatalogSession(storage(JSON.stringify({ ...session, dateFrom })))).toBeNull();
  }
  const reversed = { ...session, dateFrom: "2027-09-01" };
  expect(readCatalogSession(storage(JSON.stringify(reversed)))).toEqual(reversed);
});
