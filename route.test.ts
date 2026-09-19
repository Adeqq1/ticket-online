import { expect, test } from "bun:test";
import { matchRoute } from "./src/lib/route.ts";

test("matches application paths without accepting extra segments", () => {
  expect(matchRoute("/")).toEqual({ name: "home" });
  expect(matchRoute("/konser/")).toEqual({ name: "concerts" });
  expect(matchRoute("/tiket-saya")).toEqual({ name: "my-tickets" });
  expect(matchRoute("/tiket-saya/")).toEqual({ name: "my-tickets" });
  expect(matchRoute("/panduan")).toEqual({ name: "guide" });
  expect(matchRoute("/panduan/")).toEqual({ name: "guide" });
  expect(matchRoute("/konser/nusa-malam")).toEqual({ name: "concert-detail", id: "nusa-malam" });
  expect(matchRoute("/checkout/nusa-malam")).toEqual({ name: "checkout", id: "nusa-malam" });
  expect(matchRoute("/tiket/a1b2c3d4")).toEqual({ name: "ticket", id: "a1b2c3d4" });
  expect(matchRoute("/konser/nusa-malam/extra")).toEqual({ name: "not-found" });
});
