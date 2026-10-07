export type Route =
  | { name: "home" }
  | { name: "concerts" }
  | { name: "concert-detail"; id: string }
  | { name: "checkout"; id: string }
  | { name: "ticket"; id: string }
  | { name: "my-tickets" }
  | { name: "ticket-recovery" }
  | { name: "order"; id: string }
  | { name: "guide" }
  | { name: "admin-login" }
  | { name: "admin-staff" }
  | { name: "admin-events" }
  | { name: "admin-orders" }
  | { name: "admin-issues" }
  | { name: "admin-operations" }
  | { name: "admin-reports" }
  | { name: "admin-check-ins" }
  | { name: "admin-scan" }
  | { name: "not-found" };

export function matchRoute(pathname: string): Route {
  const path = pathname !== "/" ? pathname.replace(/\/+$/, "") : pathname;
  if (path === "/") return { name: "home" };
  if (path === "/konser") return { name: "concerts" };
  if (path === "/tiket-saya") return { name: "my-tickets" };
  if (path === "/pulihkan-tiket") return { name: "ticket-recovery" };
  if (path === "/panduan") return { name: "guide" };
  if (path === "/admin/login") return { name: "admin-login" };
  if (path === "/admin/staff") return { name: "admin-staff" };
  if (path === "/admin/events") return { name: "admin-events" };
  if (path === "/admin/orders") return { name: "admin-orders" };
  if (path === "/admin/issues") return { name: "admin-issues" };
  if (path === "/admin/operations") return { name: "admin-operations" };
  if (path === "/admin/reports") return { name: "admin-reports" };
  if (path === "/admin/check-ins") return { name: "admin-check-ins" };
  if (path === "/admin/scan") return { name: "admin-scan" };
  for (const [prefix, name] of [["/konser/", "concert-detail"], ["/checkout/", "checkout"], ["/tiket/", "ticket"], ["/pesanan/", "order"]] as const) {
    if (!path.startsWith(prefix)) continue;
    const encoded = path.slice(prefix.length);
    if (!encoded || encoded.includes("/")) return { name: "not-found" };
    try { return { name, id: decodeURIComponent(encoded) }; } catch { return { name: "not-found" }; }
  }
  return { name: "not-found" };
}
