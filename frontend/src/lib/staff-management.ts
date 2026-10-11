import type { Staff, StaffAssignment } from "./api.ts";

export type StaffFilters = { query: string; status: "all" | "active" | "inactive"; eventId: string; unassigned: boolean };

export function filterStaff(staff: Staff[], filters: StaffFilters): Staff[] {
  const query = filters.query.trim().toLowerCase();
  return staff.filter((person) =>
    (!query || person.name.toLowerCase().includes(query) || person.email.toLowerCase().includes(query)) &&
    (filters.status === "all" || person.active === (filters.status === "active")) &&
    (!filters.eventId || person.assignments.some((item) => item.eventId === filters.eventId)) &&
    (!filters.unassigned || person.assignments.length === 0)
  );
}

export function sameAssignments(a: StaffAssignment[], b: StaffAssignment[]): boolean {
  const keys = (items: StaffAssignment[]) => items.map(({ eventId, gate }) => JSON.stringify([eventId, gate])).sort();
  return JSON.stringify(keys(a)) === JSON.stringify(keys(b));
}
