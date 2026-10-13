import { expect, test } from "bun:test";
import { dateRangeError, filterConcerts, filterEvents, sortConcerts } from "./filters.ts";
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


const noFilters = { query: "", city: "", genres: [], maxPrice: null };
const changed = (startsAt: string | null) => ({ ...concerts[0]!, currentEvent: { id: "changed", status: "RESCHEDULED" as const, version: 2, salesPaused: false, startsAt, announcement: "", refundDeadline: null } });

test("date boundaries include the entire WIB calendar day across UTC midnight", () => {
  const source = ["2027-08-23T16:59:59Z", "2027-08-23T17:00:00Z", "2027-08-24T16:59:59Z", "2027-08-24T17:00:00Z"].map(changed);
  expect(filterConcerts({ ...noFilters, dateFrom: "2027-08-24", dateTo: "2027-08-24" }, source)).toEqual(source.slice(1, 3));
  expect(filterConcerts({ ...noFilters, dateFrom: "2027-08-24" }, source)).toEqual(source.slice(1));
  expect(filterConcerts({ ...noFilters, dateTo: "2027-08-24" }, source)).toEqual(source.slice(0, 3));
});

test("uses the latest schedule and combines query, city, genre, price and dates with AND", () => {
  const rescheduled = changed("2027-09-05T19:00:00+07:00");
  const source = [concerts[0]!, rescheduled, { ...rescheduled, city: "Bandung" }, { ...rescheduled, price: 900000 }];
  expect(filterConcerts({ query: "Ruang", city: "Jakarta", genres: ["Indie"], maxPrice: 225000, dateFrom: "2027-09-05", dateTo: "2027-09-05" }, source)).toEqual([rescheduled]);
});

test("unknown or invalid schedules stay visible without dates and never reuse an obsolete date", () => {
  const source = [changed(null), changed("invalid"), { ...concerts[0]!, startsAt: "" }];
  expect(filterConcerts(noFilters, source)).toEqual(source);
  expect(filterConcerts({ ...noFilters, dateFrom: "2027-08-01" }, source)).toEqual([]);
  expect(sortConcerts([source[0]!, concerts[0]!], "date")).toEqual([concerts[0]!, source[0]!]);
});

test("rejects reversed ranges and impossible dates; supports leap days", () => {
  expect(dateRangeError("2027-08-25", "2027-08-24")).toBe("Tanggal akhir tidak boleh sebelum tanggal mulai.");
  expect(dateRangeError("2027-02-29", "")).toBe("Masukkan tanggal yang valid.");
  expect(dateRangeError("2028-02-29", "2028-02-29")).toBe("");
  expect(filterConcerts({ ...noFilters, dateFrom: "2027-08-25", dateTo: "2027-08-24" })).toEqual([]);
});
