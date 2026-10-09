const storageKey = "ticket-online:conversion:";
const idPattern = /^[a-f0-9]{32}$/;
const fallback = new Map<string, { id: string; lastActivityAt: number }>();

export type ConversionKind = "DETAIL_VIEWED" | "CHECKOUT_STARTED" | "VALIDATION_FAILED" | "SERVICE_FAILURE";
export type ConversionReason = "BUYER_DATA" | "ATTENDEE_DATA" | "RESERVATION" | "PAYMENT_PROVIDER" | "SERVICE";

export function journeyId(eventId: string): string {
  const now = Date.now();
  let stored = fallback.get(eventId);
  try {
    const key = `${storageKey}${encodeURIComponent(eventId)}`;
    const saved = JSON.parse(sessionStorage.getItem(key) ?? "null") as { id?: unknown; lastActivityAt?: unknown } | null;
    if (typeof saved?.id === "string" && idPattern.test(saved.id) && typeof saved.lastActivityAt === "number" && Number.isFinite(saved.lastActivityAt) && (!stored || saved.lastActivityAt > stored.lastActivityAt)) stored = { id: saved.id, lastActivityAt: saved.lastActivityAt };
  } catch { /* use document-scoped memory when storage is unavailable */ }
  const current = { id: stored && now >= stored.lastActivityAt && now - stored.lastActivityAt < 30 * 60 * 1_000 ? stored.id : crypto.randomUUID().replaceAll("-", ""), lastActivityAt: now };
  fallback.set(eventId, current);
  try { sessionStorage.setItem(`${storageKey}${encodeURIComponent(eventId)}`, JSON.stringify(current)); } catch { /* memory retains the selected ID */ }
  return current.id;
}

export function trackConversionActivity(eventId: string): () => void {
  const touch = () => { journeyId(eventId); };
  document.addEventListener("pointerdown", touch, { passive: true });
  document.addEventListener("keydown", touch);
  return () => { document.removeEventListener("pointerdown", touch); document.removeEventListener("keydown", touch); };
}

export function recordConversion(eventId: string, kind: ConversionKind, reason?: ConversionReason): string {
  const id = journeyId(eventId);
  const device = typeof matchMedia === "function" && matchMedia("(max-width: 767px)").matches ? "mobile" : typeof matchMedia === "function" ? "desktop" : "unknown";
  void fetch("/api/v1/conversion/events", {
    method: "POST", keepalive: true,
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ journeyId: id, eventId, device, kind, ...(reason ? { reason } : {}) }),
  }).catch(() => {});
  return id;
}
