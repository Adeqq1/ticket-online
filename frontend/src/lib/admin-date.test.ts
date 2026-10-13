import { expect, test } from "bun:test";
import { calendarDays, dateFieldValue, displayDate, parseDisplayDate, parseISODate, shiftCalendarDate, shiftCalendarMonth } from "./admin-date.ts";

test("strict dates, leap years and display round trips", () => {
  for (const value of ["2024-02-29", "2000-02-29", "0001-01-01", "9999-12-31"]) expect(parseDisplayDate(displayDate(value))).toBe(value);
  for (const value of ["1900-02-29", "2026-02-29", "2026-04-31", "0000-01-01", "2026-13-01", "2026-01-00", "2026-1-01"]) expect(parseISODate(value)).toBeNull();
  for (const value of ["1/01/2026", "31/04/2026", "2026-01-01", "01/01/26", "01/01/2026x"]) expect(parseDisplayDate(value)).toBeNull();
});
test("time precision, partial drafts, blank optional dates and time ranges", () => {
  expect(dateFieldValue("29/02/2024", "7", "3", "9", "datetime", "second")).toBe("2024-02-29T07:03:09");
  expect(dateFieldValue("29/02/2024", "0", "0", "", "datetime", "minute")).toBe("2024-02-29T00:00");
  expect(dateFieldValue("29/02/2024", "", "", "", "date", "minute")).toBe("2024-02-29");
  expect(dateFieldValue("", "", "", "", "datetime", "second")).toBe("");
  for (const time of [["24", "00", "00"], ["23", "60", "00"], ["23", "59", "60"], ["-1", "00", "00"], ["1.5", "00", "00"], ["", "00", "00"]]) expect(dateFieldValue("01/01/2026", ...time as [string, string, string], "datetime", "second")).toBeNull();
  expect(dateFieldValue("01/01", "00", "00", "", "datetime", "minute")).toBeNull();
});
test("Monday calendar and navigation cross month/year and DST boundaries", () => {
  const days = calendarDays(2026, 2);
  expect(days).toHaveLength(42);
  expect(days[0]).toBe("2026-01-26");
  expect(days[6]).toBe("2026-02-01");
  expect(new Set(days).size).toBe(42);
  expect(shiftCalendarMonth("2024-01-31", 1)).toBe("2024-02-29");
  expect(shiftCalendarMonth("2026-01-31", 1)).toBe("2026-02-28");
  expect(shiftCalendarMonth("2026-01-01", -1)).toBe("2025-12-01");
  expect(shiftCalendarDate("2026-03-08", 1)).toBe("2026-03-09");
  expect(shiftCalendarDate("2024-02-28", 1)).toBe("2024-02-29");
  expect(shiftCalendarDate("0001-01-01", -1)).toBe("0001-01-01");
  expect(shiftCalendarDate("9999-12-31", 1)).toBe("9999-12-31");
  expect(calendarDays(9999, 12).filter(Boolean).at(-1)).toBe("9999-12-31");
});
