export type Route =
  | { name: "home" }
  | { name: "concerts" }
  | { name: "concert-detail"; id: string }
  | { name: "checkout"; id: string }
  | { name: "ticket"; id: string }
  | { name: "my-tickets" }
  | { name: "guide" }
  | { name: "not-found" };

export function matchRoute(pathname: string): Route {
  const path = pathname !== "/" ? pathname.replace(/\/+$/, "") : pathname;
  if (path === "/") return { name: "home" };
  if (path === "/konser") return { name: "concerts" };
  if (path === "/tiket-saya") return { name: "my-tickets" };
  if (path === "/panduan") return { name: "guide" };
  for (const [prefix, name] of [["/konser/", "concert-detail"], ["/checkout/", "checkout"], ["/tiket/", "ticket"]] as const) {
    if (!path.startsWith(prefix)) continue;
    const encoded = path.slice(prefix.length);
    if (!encoded || encoded.includes("/")) return { name: "not-found" };
    try { return { name, id: decodeURIComponent(encoded) }; } catch { return { name: "not-found" }; }
  }
  return { name: "not-found" };
}
