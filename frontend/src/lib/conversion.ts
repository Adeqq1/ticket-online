const storageKey = "ticket-online:conversion:";
const idPattern = /^[a-f0-9]{32}$/;

export type ConversionKind = "DETAIL_VIEWED" | "CHECKOUT_STARTED" | "VALIDATION_FAILED" | "SERVICE_FAILURE";
export type ConversionReason = "BUYER_DATA" | "ATTENDEE_DATA" | "RESERVATION" | "PAYMENT_PROVIDER" | "SERVICE";

export function journeyId(eventId: string): string {
  try {
    const key = `${storageKey}${encodeURIComponent(eventId)}`;
    const now = Date.now();
    const stored = JSON.parse(sessionStorage.getItem(key) ?? "null") as { id?: unknown; lastActivityAt?: unknown } | null;
    const current = stored?.id;
    const active = typeof current === "string" && idPattern.test(current) && typeof stored?.lastActivityAt === "number" && now - stored.lastActivityAt < 30 * 60 * 1_000;
    const id = active ? current : crypto.randomUUID().replaceAll("-", "");
    sessionStorage.setItem(key, JSON.stringify({ id, lastActivityAt: now }));
    return id;
  } catch { return crypto.randomUUID().replaceAll("-", ""); }
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
