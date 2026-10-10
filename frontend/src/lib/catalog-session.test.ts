import { expect, test } from "bun:test";
import { catalogSessionKey, readCatalogSession, saveCatalogSession, type CatalogSession } from "./catalog-session.ts";

const session: CatalogSession = { query: "senja", genres: ["Jazz"], city: "Bandung", maxPrice: null, sort: "price", filtersOpen: true, scrollY: 840 };

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
