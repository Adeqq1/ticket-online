import { ApiError, checkInTicket, getStaffProfile, getStaffTicketStatus, type CheckInRequest, type CheckInTicket, type Staff, type StaffAssignment, type TicketCheckInStatus } from "./api.ts";

export type ScanStatus = "idle" | "valid" | "used" | "wrong-gate" | "not-found" | "unpaid" | "denied" | "unknown" | "unavailable" | "error";
export type ScannerState = {
  busy: boolean;
  status: ScanStatus;
  resultTicket: CheckInTicket | null;
  expectedGate: string;
  checkedInAt: string;
  resultDetail: string;
  scanCount: number;
  lastScan: string;
  locked: boolean;
  lastAttempt: CheckInRequest | null;
  statusChecking: boolean;
  ticketStatus: TicketCheckInStatus | null;
  statusCheckError: string;
};

export function normalizeScanCode(value: string) {
  return value.trim().toUpperCase();
}

export function parseCameraScanCode(value: unknown): string | null {
  if (typeof value !== "string") return null;
  const code = normalizeScanCode(value);
  return /^ET-[0-9A-F]{32}$/.test(code) ? code : null;
}

export function continueToNextScan(state: ScannerState, confirmUnknown: boolean): boolean {
  if (!state.locked || state.busy || state.statusChecking) return false;
  if (state.status === "unknown" && state.ticketStatus?.status !== "CHECKED_IN" && !confirmUnknown) return false;
  state.locked = false;
  state.status = "idle";
  state.resultTicket = null;
  state.expectedGate = "";
  state.checkedInAt = "";
  state.resultDetail = "";
  state.lastAttempt = null;
  state.ticketStatus = null;
  state.statusCheckError = "";
  return true;
}

export async function checkTicketStatus(state: ScannerState, accessToken: string, onUnauthorized: () => void): Promise<void> {
  const attempt = state.lastAttempt;
  if (!state.locked || state.status !== "unknown" || !attempt || state.busy || state.statusChecking) return;
  state.statusChecking = true;
  state.ticketStatus = null;
  state.statusCheckError = "";
  try {
    const result = await getStaffTicketStatus(accessToken, attempt.code);
    if (result.ticket.code !== attempt.code || result.ticket.eventId !== attempt.eventId || result.ticket.gate !== attempt.gate) {
      state.statusCheckError = "Respons status tidak sesuai dengan tiket dan gate pada percobaan ini.";
      return;
    }
    state.ticketStatus = result;
  } catch (cause) {
    const apiError = cause instanceof ApiError ? cause : undefined;
    if (apiError?.status === 401) { onUnauthorized(); return; }
    state.statusCheckError = apiError?.message ?? "Status tiket belum dapat diperiksa. Gate tetap ditahan.";
  } finally {
    state.statusChecking = false;
  }
}

function failed(state: ScannerState, cause: unknown, checkInSent: boolean, onUnauthorized: () => void) {
  const apiError = cause instanceof ApiError ? cause : undefined;
  state.resultTicket = apiError?.ticket ?? null;
  state.expectedGate = apiError?.expectedGate ?? "";
  state.checkedInAt = apiError?.checkedInAt ?? "";
  state.resultDetail = apiError?.message ?? "Tidak dapat menghubungi server.";
  if (apiError?.status === 401) { onUnauthorized(); return; }
  if (apiError?.status === 403) state.status = "denied";
  else if (apiError?.code === "TICKET_ALREADY_USED") state.status = "used";
  else if (apiError?.code === "WRONG_GATE") state.status = "wrong-gate";
  else if (apiError?.code === "TICKET_NOT_FOUND") state.status = "not-found";
  else if (apiError?.code === "ORDER_NOT_PAID") state.status = "unpaid";
  else if (checkInSent && (apiError?.code === "NETWORK_ERROR" || apiError?.code === "UNKNOWN_OUTCOME" || (apiError?.status ?? 0) >= 500)) state.status = "unknown";
  else if (apiError?.code === "NETWORK_ERROR" || apiError?.code === "UNKNOWN_OUTCOME") {
    state.status = "unavailable";
    state.resultDetail = "Sesi atau penugasan belum dapat diverifikasi. Check-in tidak dikirim.";
  } else state.status = "error";
}

export async function submitScan(
  state: ScannerState,
  accessToken: string,
  assignment: StaffAssignment | undefined,
  value: string,
  onProfile: (staff: Staff) => void,
  onUnauthorized: () => void,
): Promise<void> {
  if (state.busy || state.locked || !assignment) return;
  const code = normalizeScanCode(value);
  if (!code) return;
  const selected = { ...assignment };
  state.locked = true;
  state.lastAttempt = { ...selected, code };
  state.ticketStatus = null;
  state.statusCheckError = "";
  state.busy = true;
  state.status = "idle";
  state.resultTicket = null;
  state.expectedGate = "";
  state.checkedInAt = "";
  state.resultDetail = "";
  state.scanCount += 1;
  state.lastScan = new Date().toLocaleTimeString("id-ID", { hour: "2-digit", minute: "2-digit" });
  let checkInSent = false;
  try {
    const current = await getStaffProfile(accessToken);
    onProfile(current);
    if (current.role !== "STAFF" || !current.assignments.some((item) => item.eventId === selected.eventId && item.gate === selected.gate)) {
      state.status = "unavailable";
      state.resultDetail = "Penugasan ini sudah berubah. Pilih penugasan terbaru sebelum mengirim check-in.";
      return;
    }
    checkInSent = true;
    const result = await checkInTicket(accessToken, { ...selected, code });
    state.status = "valid";
    state.resultTicket = result.ticket;
    state.checkedInAt = result.checkedInAt;
  } catch (cause) {
    failed(state, cause, checkInSent, onUnauthorized);
  } finally {
    state.busy = false;
  }
}
