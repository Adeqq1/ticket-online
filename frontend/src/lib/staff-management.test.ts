import { expect, test } from "bun:test";
import type { Staff } from "./api.ts";
import { filterStaff, sameAssignments, type StaffFilters } from "./staff-management.ts";

const people: Staff[] = [
  { id: "b", name: "Bima Utama", email: "bima@example.test", role: "STAFF", active: true, assignments: [{ eventId: "musik", gate: "Utara" }] },
  { id: "a", name: "Ayu", email: "ayu.ops@example.test", role: "STAFF", active: false, assignments: [] },
  { id: "c", name: "Admin", email: "admin@example.test", role: "ADMIN", active: true, assignments: [] },
];
const all: StaffFilters = { query: "", status: "all", eventId: "", unassigned: false };

test("staff search trims query, ignores case, searches name/email, and preserves backend order", () => {
  expect(filterStaff(people, all)).toEqual(people);
  expect(filterStaff(people, { ...all, query: "  BIMA UTAMA  " })).toEqual([people[0]!]);
  expect(filterStaff(people, { ...all, query: "  OPS@EXAMPLE  " })).toEqual([people[1]!]);
  expect(filterStaff(people, { ...all, query: "EXAMPLE" }).map((p) => p.id)).toEqual(["b", "a", "c"]);
  expect(filterStaff(people, { ...all, query: "absent" })).toEqual([]);
});

test("staff filters combine with AND, including contradictory event/unassigned filters", () => {
  expect(filterStaff(people, { ...all, status: "active", eventId: "musik", query: "bima" })).toEqual([people[0]!]);
  expect(filterStaff(people, { ...all, status: "inactive", unassigned: true })).toEqual([people[1]!]);
  expect(filterStaff(people, { ...all, eventId: "musik", unassigned: true })).toEqual([]);
  expect(filterStaff(people, { ...all, status: "inactive", eventId: "musik" })).toEqual([]);
  expect(filterStaff(people, { ...all, unassigned: true }).map((p) => p.id)).toEqual(["a", "c"]);
  expect(people[0]!.assignments).toEqual([{ eventId: "musik", gate: "Utara" }]);
});

test("assignment reconciliation ignores order but distinguishes gate and event changes", () => {
  const a = [{ eventId: "musik", gate: "Utara" }, { eventId: "teater", gate: "Utara" }];
  expect(sameAssignments(a, [...a].reverse())).toBe(true);
  expect(sameAssignments(a, [{ eventId: "musik", gate: "Selatan" }, a[1]!])).toBe(false);
  expect(sameAssignments(a, a.slice(0, 1))).toBe(false);
  expect(sameAssignments([], [])).toBe(true);
  expect(sameAssignments([a[0]!, a[0]!], a)).toBe(false);
});
