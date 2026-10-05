export type OrderAccess = { orderId: string; accessToken: string; expiresAt: string; accessExpiresAt: string; reservationId: string; idempotencyKey: string; basketKey: string; reference: string; ticketIds: string[] };

const prefix = "ticket-online:order:";
const memory = new Map<string, OrderAccess>();
function storage() { try { return globalThis.localStorage; } catch { return null; } }
function key(orderId: string) { return `${prefix}${orderId}`; }

export function parseOrderAccess(value: string | null): OrderAccess | null {
  try {
    const record = JSON.parse(value ?? "") as Partial<OrderAccess>;
    if (typeof record.orderId !== "string" || !record.orderId || typeof record.accessToken !== "string" || !record.accessToken || typeof record.expiresAt !== "string" || !Number.isFinite(Date.parse(record.expiresAt)) || typeof record.accessExpiresAt !== "string" || !Number.isFinite(Date.parse(record.accessExpiresAt)) || typeof record.reservationId !== "string" || !record.reservationId || typeof record.idempotencyKey !== "string" || !record.idempotencyKey || typeof record.basketKey !== "string" || !record.basketKey || typeof record.reference !== "string" || !record.reference || !Array.isArray(record.ticketIds) || record.ticketIds.some((id) => typeof id !== "string")) return null;
    return record as OrderAccess;
  } catch { return null; }
}

export function saveOrderAccess(record: OrderAccess) {
  memory.set(record.orderId, record);
  try { const local = storage(); if (!local) return false; local.setItem(key(record.orderId), JSON.stringify(record)); return true; } catch { return false; }
}

export function removeOrderAccess(orderId: string) {
  memory.delete(orderId);
  try { storage()?.removeItem(key(orderId)); } catch { /* storage may be unavailable */ }
}

export function getOrderAccess(orderId: string): OrderAccess | null {
  const inMemory = memory.get(orderId);
  if (inMemory) return inMemory;
  const local = storage();
  try {
    const saved = parseOrderAccess(local?.getItem(key(orderId)) ?? null);
    if (saved) { memory.set(orderId, saved); return saved; }
  } catch { /* storage may be unavailable */ }
  return memory.get(orderId) ?? null;
}

export function hasPersistentOrderAccess(orderId: string) {
  try { return Boolean(parseOrderAccess(storage()?.getItem(key(orderId)) ?? null)); } catch { return false; }
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
  return [...records.values()];
}

export function getOrderAccessForTicket(ticketId: string) {
  return listOrderAccess().find((record) => record.ticketIds.includes(ticketId)) ?? null;
}
