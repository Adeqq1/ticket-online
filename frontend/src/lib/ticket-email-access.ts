const prefix = "ticket-online:email-ticket:";
const memory = new Map<string, string>();

export function emailAccessToken(hash: string): string | null {
  const params = new URLSearchParams(hash.startsWith("#") ? hash.slice(1) : hash);
  const values = params.getAll("access_token");
  const token = values[0];
  return values.length === 1 && token !== undefined && /^[A-Za-z0-9_-]{43}$/.test(token) ? token : null;
}

export function saveEmailTicketAccess(ticketId: string, token: string) {
  memory.set(ticketId, token);
  try {
    globalThis.sessionStorage.setItem(prefix + ticketId, token);
  } catch { /* keep the current page usable when browser storage is blocked */ }
}

export function getEmailTicketAccess(ticketId: string) {
  try {
    const token = globalThis.sessionStorage.getItem(prefix + ticketId);
    if (token && /^[A-Za-z0-9_-]{43}$/.test(token)) return token;
  } catch { /* fall back to memory */ }
  return memory.get(ticketId) ?? null;
}
