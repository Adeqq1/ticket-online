export type Route =
  | { name: "home" }
  | { name: "concerts" }
  | { name: "concert-detail"; id: string }
  | { name: "checkout"; id: string }
  | { name: "ticket"; id: string }
  | { name: "my-tickets" }
  | { name: "guide" }
  | { name: "admin-login" }
  | { name: "admin-staff" }
  | { name: "admin-check-ins" }
  | { name: "admin-scan" }
  | { name: "not-found" };

export function matchRoute(pathname: string): Route {
  const path = pathname !== "/" ? pathname.replace(/\/+$/, "") : pathname;
  if (path === "/") return { name: "home" };
  if (path === "/konser") return { name: "concerts" };
  if (path === "/tiket-saya") return { name: "my-tickets" };
  if (path === "/panduan") return { name: "guide" };
  if (path === "/admin/login") return { name: "admin-login" };
  if (path === "/admin/staff") return { name: "admin-staff" };
  if (path === "/admin/check-ins") return { name: "admin-check-ins" };
  if (path === "/admin/scan") return { name: "admin-scan" };
  for (const [prefix, name] of [["/konser/", "concert-detail"], ["/checkout/", "checkout"], ["/tiket/", "ticket"]] as const) {
    if (!path.startsWith(prefix)) continue;
    const encoded = path.slice(prefix.length);
    if (!encoded || encoded.includes("/")) return { name: "not-found" };
    try { return { name, id: decodeURIComponent(encoded) }; } catch { return { name: "not-found" }; }
  }
  return { name: "not-found" };
}
