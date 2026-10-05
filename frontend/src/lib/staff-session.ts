import type { Staff, StaffRole } from "./api.ts";

export type StaffSessionRecord = { accessToken: string; expiresAt: string };
export const staffSessionKey = "ticket-online:staff-session";

function sessionStorageOrNull(): Storage | null {
  try { return globalThis.sessionStorage; } catch { return null; }
}

export function readStaffSession(storage = sessionStorageOrNull()): StaffSessionRecord | null {
  try {
    const record = JSON.parse(storage?.getItem(staffSessionKey) ?? "") as Partial<StaffSessionRecord>;
    if (typeof record.accessToken !== "string" || !/^[A-Za-z0-9_-]{43}$/.test(record.accessToken) || typeof record.expiresAt !== "string" || !Number.isFinite(Date.parse(record.expiresAt)) || Date.parse(record.expiresAt) <= Date.now()) return null;
    return record as StaffSessionRecord;
  } catch { return null; }
}

export function saveStaffSession(session: StaffSessionRecord, storage = sessionStorageOrNull()): boolean {
  try {
    if (!storage) return false;
    storage.setItem(staffSessionKey, JSON.stringify(session));
    return readStaffSession(storage)?.accessToken === session.accessToken;
  } catch { return false; }
}

export function clearStaffSession(storage = sessionStorageOrNull()): void {
  try { storage?.removeItem(staffSessionKey); } catch { /* storage may be unavailable */ }
}

export function staffHome(role: StaffRole): string {
  return role === "ADMIN" ? "/admin/staff" : "/admin/scan";
}

export function roleMatchesPage(page: "staff" | "scan", staff: Pick<Staff, "role">): boolean {
  return page === "staff" ? staff.role === "ADMIN" : staff.role === "STAFF";
}
