import type { OrderAccess } from "./order-access.ts";

export type RecoveryResult = { orderId: string; accessToken: string; reference: string; reservationId: string; expiresAt: string; accessExpiresAt: string; ticketIds: string[] };

const tokenPattern = /^[A-Za-z0-9_-]{43}$/;
const idPattern = /^[0-9a-f]{32}$/;

/** Reads exactly one recovery token from a URL fragment such as `#token=...`. */
export function recoveryToken(hash: string): string | null {
  const values = new URLSearchParams(hash.startsWith("#") ? hash.slice(1) : hash).getAll("token");
  return values.length === 1 && tokenPattern.test(values[0] ?? "") ? values[0]! : null;
}

/** Validates the verify response before it is trusted as order access. */
export function isRecoveryResult(value: unknown): value is RecoveryResult {
  if (!value || typeof value !== "object") return false;
  const result = value as Partial<RecoveryResult>;
  return typeof result.orderId === "string" && idPattern.test(result.orderId) && typeof result.accessToken === "string" && tokenPattern.test(result.accessToken) &&
    typeof result.reference === "string" && /^TO-[0-9a-f]{20}$/.test(result.reference) && typeof result.reservationId === "string" && idPattern.test(result.reservationId) &&
    typeof result.expiresAt === "string" && Number.isFinite(Date.parse(result.expiresAt)) && typeof result.accessExpiresAt === "string" && Number.isFinite(Date.parse(result.accessExpiresAt)) &&
    Array.isArray(result.ticketIds) && result.ticketIds.length > 0 && result.ticketIds.every((id) => typeof id === "string" && idPattern.test(id));
}

/** Converts a recovery result to a stored record; checkout metadata stays absent so checkout never reuses it. */
export function recoveredOrderAccess(result: RecoveryResult): OrderAccess {
  const { orderId, accessToken, expiresAt, accessExpiresAt, reservationId, reference, ticketIds } = result;
  return { orderId, accessToken, expiresAt, accessExpiresAt, reservationId, reference, ticketIds };
}
