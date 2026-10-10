import { describe, expect, it } from "bun:test";
import { suggestAdminId } from "./admin-id.ts";

describe("suggestAdminId", () => {
  it("normalizes names to valid URL IDs", () => {
    expect(suggestAdminId("  Panggung Élite 2027! ")).toBe("panggung-elite-2027");
  });

  it("handles names with no usable characters and trims the length ceiling", () => {
    expect(suggestAdminId("🎵🎸")).toBe("");
    expect(suggestAdminId(`${"A".repeat(63)} B`)).toBe("a".repeat(63));
  });
});
