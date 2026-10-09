import type { EventState } from "./concerts.ts";

export function wibDateTime(value: string) {
  return `${value.length === 16 ? `${value}:00` : value}+07:00`;
}

export function canBuy(state?: EventState) {
  return !state || !state.salesPaused && (state.status === "SCHEDULED" || state.status === "RESCHEDULED");
}
export function currentStart(item: { currentEvent?: EventState; eventStartsAt: string }) {
  return item.currentEvent ? item.currentEvent.startsAt : item.eventStartsAt;
}
export function eventStatus(state?: EventState) {
  if (!state) return "";
  return ({ SCHEDULED: "", POSTPONED: "Acara ditunda", RESCHEDULED: "Jadwal diperbarui", CANCELLED: "Acara dibatalkan" })[state.status];
}
