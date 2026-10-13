export type DateMode = "date" | "datetime";
export type TimePrecision = "minute" | "second";
export const calendarMonths = ["Januari", "Februari", "Maret", "April", "Mei", "Juni", "Juli", "Agustus", "September", "Oktober", "November", "Desember"];
export const calendarWeekdays = ["Senin", "Selasa", "Rabu", "Kamis", "Jumat", "Sabtu", "Minggu"];
const pad = (n: number) => String(n).padStart(2, "0");

export function daysInMonth(year: number, month: number) {
  return month === 2 ? (year % 4 === 0 && (year % 100 !== 0 || year % 400 === 0) ? 29 : 28) : [4, 6, 9, 11].includes(month) ? 30 : 31;
}
export function isoDate(year: number, month: number, day: number) {
  return `${String(year).padStart(4, "0")}-${pad(month)}-${pad(day)}`;
}
export function parseISODate(value: string) {
  const match = /^(\d{4})-(\d{2})-(\d{2})$/.exec(value);
  if (!match) return null;
  const year = Number(match[1]), month = Number(match[2]), day = Number(match[3]);
  return year >= 1 && year <= 9999 && month >= 1 && month <= 12 && day >= 1 && day <= daysInMonth(year, month) ? { year, month, day } : null;
}
export function parseDisplayDate(value: string) {
  const match = /^(\d{2})\/(\d{2})\/(\d{4})$/.exec(value);
  if (!match) return null;
  const result = `${match[3]}-${match[2]}-${match[1]}`;
  return parseISODate(result) ? result : null;
}
export function displayDate(value: string) {
  const date = parseISODate(value);
  return date ? `${pad(date.day)}/${pad(date.month)}/${String(date.year).padStart(4, "0")}` : "";
}
export function dateFieldValue(date: string, hour: string, minute: string, second: string, mode: DateMode, precision: TimePrecision) {
  if (!date && (mode === "date" || !hour && !minute && !second)) return "";
  const parsed = parseDisplayDate(date);
  if (!parsed) return null;
  if (mode === "date") return parsed;
  const time = precision === "second" ? [hour, minute, second] : [hour, minute];
  if (time.some((part, i) => !/^\d{1,2}$/.test(part) || Number(part) > (i === 0 ? 23 : 59))) return null;
  return `${parsed}T${time.map(part => pad(Number(part))).join(":")}`;
}
function utcDate(value: string) {
  const date = new Date(0);
  const parsed = parseISODate(value)!;
  date.setUTCFullYear(parsed.year, parsed.month - 1, parsed.day);
  return date;
}
export function shiftCalendarDate(value: string, days: number) {
  const date = utcDate(value);
  date.setUTCDate(date.getUTCDate() + days);
  const year = date.getUTCFullYear();
  return year < 1 ? "0001-01-01" : year > 9999 ? "9999-12-31" : isoDate(year, date.getUTCMonth() + 1, date.getUTCDate());
}
export function shiftCalendarMonth(value: string, months: number) {
  const date = parseISODate(value)!;
  const index = Math.max(0, Math.min(9999 * 12 - 1, (date.year - 1) * 12 + date.month - 1 + months));
  const year = Math.floor(index / 12) + 1, month = index % 12 + 1;
  return isoDate(year, month, Math.min(date.day, daysInMonth(year, month)));
}
export function mondayIndex(value: string) { return (utcDate(value).getUTCDay() + 6) % 7; }
export function calendarDays(year: number, month: number) {
  const first = isoDate(year, month, 1), offset = mondayIndex(first);
  // Keep out-of-range cells empty at the supported year boundaries.
  return Array.from({ length: 42 }, (_, i) => {
    const delta = i - offset;
    if (year === 1 && month === 1 && delta < 0 || year === 9999 && month === 12 && delta >= 31) return "";
    return shiftCalendarDate(first, delta);
  });
}
export function calendarDateLabel(value: string) {
  const date = parseISODate(value)!;
  return `${calendarWeekdays[mondayIndex(value)]}, ${date.day} ${calendarMonths[date.month - 1]} ${date.year}`;
}
