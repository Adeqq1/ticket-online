import { describe, expect, test } from "bun:test";
import { normalizeScanCode } from "./scanner.ts";

describe("normalizeScanCode", () => {
  test("trims surrounding whitespace and normalizes case", () => {
    expect(normalizeScanCode("  et-0123abcd  ")).toBe("ET-0123ABCD");
  });
});
