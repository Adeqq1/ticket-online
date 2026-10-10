import type { ConcertSort } from "./filters.ts";

export const catalogSessionKey = "ticket-online:catalog:v1";
export type CatalogSession = { query: string; genres: string[]; city: string; maxPrice: number | null; sort: ConcertSort; filtersOpen: boolean; scrollY: number };

export function readCatalogSession(storage: Pick<Storage, "getItem"> | null = getSessionStorage()): CatalogSession | null {
  try {
    const value: unknown = JSON.parse(storage?.getItem(catalogSessionKey) ?? "null");
    if (!value || typeof value !== "object") return null;
    const item = value as Record<string, unknown>;
    if (typeof item.query !== "string" || !Array.isArray(item.genres) || !item.genres.every((genre) => typeof genre === "string") || typeof item.city !== "string" || !(item.maxPrice === null || Number.isSafeInteger(item.maxPrice) && (item.maxPrice as number) >= 0) || !(item.sort === "date" || item.sort === "price") || typeof item.filtersOpen !== "boolean" || !Number.isFinite(item.scrollY) || (item.scrollY as number) < 0) return null;
    return item as CatalogSession;
  } catch { return null; }
}

export function saveCatalogSession(value: CatalogSession, storage: Pick<Storage, "setItem"> | null = getSessionStorage()): boolean {
  try { storage?.setItem(catalogSessionKey, JSON.stringify(value)); return Boolean(storage); } catch { return false; }
}

function getSessionStorage(): Storage | null { try { return globalThis.sessionStorage; } catch { return null; } }
