// Checkout metadata (idempotencyKey, basketKey) is absent on records restored through ticket recovery.
export type OrderAccess = { orderId: string; accessToken: string; expiresAt: string; accessExpiresAt: string | null; reservationId: string; idempotencyKey?: string; basketKey?: string; reference: string; ticketIds: string[] };
export type CheckoutOrderAccess = OrderAccess & { idempotencyKey: string; basketKey: string };

export function hasCheckoutMetadata(record: OrderAccess): record is CheckoutOrderAccess {
  return Boolean(record.idempotencyKey && record.basketKey);
}

const prefix = "ticket-online:order:";
const sessionPrefix = "ticket-online:order-session:";
const memory = new Map<string, OrderAccess>();
function storage() { try { return globalThis.localStorage; } catch { return null; } }
function sessionStorage() { try { return globalThis.sessionStorage; } catch { return null; } }
function key(orderId: string) { return `${prefix}${orderId}`; }
function sessionKey(orderId: string) { return `${sessionPrefix}${orderId}`; }

export function parseOrderAccess(value: string | null): OrderAccess | null {
  try {
    const record = JSON.parse(value ?? "") as Partial<OrderAccess>;
    if (typeof record.orderId !== "string" || !record.orderId || typeof record.accessToken !== "string" || !record.accessToken || typeof record.expiresAt !== "string" || !Number.isFinite(Date.parse(record.expiresAt)) || (record.accessExpiresAt !== null && (typeof record.accessExpiresAt !== "string" || !Number.isFinite(Date.parse(record.accessExpiresAt)))) || typeof record.reservationId !== "string" || !record.reservationId || !(record.idempotencyKey === undefined && record.basketKey === undefined || typeof record.idempotencyKey === "string" && record.idempotencyKey && typeof record.basketKey === "string" && record.basketKey) || typeof record.reference !== "string" || !record.reference || !Array.isArray(record.ticketIds) || record.ticketIds.some((id) => typeof id !== "string")) return null;
    return record as OrderAccess;
  } catch { return null; }
}

export function saveOrderAccess(record: OrderAccess) {
  memory.set(record.orderId, record);
  const encoded = JSON.stringify(record);
  try {
    const local = storage();
    if (local) { local.setItem(key(record.orderId), encoded); if (parseOrderAccess(local.getItem(key(record.orderId)))?.orderId === record.orderId) return true; }
  } catch { /* try tab-scoped storage below */ }
  try {
    const session = sessionStorage();
    if (!session) return false;
    session.setItem(sessionKey(record.orderId), encoded);
    return parseOrderAccess(session.getItem(sessionKey(record.orderId)))?.orderId === record.orderId;
  } catch { return false; }
}

export function saveOrderTicketAccess(record: OrderAccess, accessExpiresAt: string | null, ticketIds: string[]) {
  return saveOrderAccess({ ...record, accessExpiresAt, ticketIds: [...new Set([...record.ticketIds, ...ticketIds])] });
}

export function removeOrderAccess(orderId: string) {
  memory.delete(orderId);
  try { storage()?.removeItem(key(orderId)); } catch { /* storage may be unavailable */ }
  try { sessionStorage()?.removeItem(sessionKey(orderId)); } catch { /* storage may be unavailable */ }
}

export function getOrderAccess(orderId: string): OrderAccess | null {
  const inMemory = memory.get(orderId);
  if (inMemory) return inMemory;
  const local = storage();
  try {
    const saved = parseOrderAccess(local?.getItem(key(orderId)) ?? null);
    if (saved) { memory.set(orderId, saved); return saved; }
  } catch { /* storage may be unavailable */ }
  try {
    const saved = parseOrderAccess(sessionStorage()?.getItem(sessionKey(orderId)) ?? null);
    if (saved) { memory.set(orderId, saved); return saved; }
  } catch { /* storage may be unavailable */ }
  return memory.get(orderId) ?? null;
}

export function hasPersistentOrderAccess(orderId: string) {
  try { if (parseOrderAccess(storage()?.getItem(key(orderId)) ?? null)) return true; } catch { /* try tab-scoped storage */ }
  try { return Boolean(parseOrderAccess(sessionStorage()?.getItem(sessionKey(orderId)) ?? null)); } catch { return false; }
}

export function hasPersistentTicketAccess(orderId: string, ticketId: string) {
  try {
    const local = parseOrderAccess(storage()?.getItem(key(orderId)) ?? null);
    if (local?.orderId === orderId && local.ticketIds.includes(ticketId)) return true;
  } catch { /* try tab-scoped storage */ }
  try {
    const session = parseOrderAccess(sessionStorage()?.getItem(sessionKey(orderId)) ?? null);
    return session?.orderId === orderId && session.ticketIds.includes(ticketId);
  } catch { return false; }
}

export function listOrderAccess(): OrderAccess[] {
  const records = new Map(memory);
  const local = storage();
  if (local) try {
    for (let index = 0; index < local.length; index += 1) {
      const itemKey = local.key(index);
      if (!itemKey?.startsWith(prefix)) continue;
      const record = parseOrderAccess(local.getItem(itemKey));
      if (record && key(record.orderId) === itemKey && !records.has(record.orderId)) records.set(record.orderId, record);
    }
  } catch { /* storage may be unavailable */ }
  const session = sessionStorage();
  if (session) try {
    for (let index = 0; index < session.length; index += 1) {
      const itemKey = session.key(index);
      if (!itemKey?.startsWith(sessionPrefix)) continue;
      const record = parseOrderAccess(session.getItem(itemKey));
      if (record && sessionKey(record.orderId) === itemKey && !records.has(record.orderId)) records.set(record.orderId, record);
    }
  } catch { /* storage may be unavailable */ }
  return [...records.values()];
}

export function getOrderAccessForTicket(ticketId: string) {
  return listOrderAccess().find((record) => record.ticketIds.includes(ticketId)) ?? null;
}
