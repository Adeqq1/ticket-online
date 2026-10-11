import { expect, test } from "bun:test";
import { clampMobileGroup, mobileListItems } from "./mobile-list.ts";

test("mobile groups retain order, cover every item once, and clamp after filtering", () => {
  for (const count of [0, 1, 5, 6, 20, 22]) {
    const items = Array.from({ length: count }, (_, id) => id);
    const groups = Array.from({ length: Math.ceil(count / 5) }, (_, group) => mobileListItems(items, group, true));
    expect(groups.flat()).toEqual(items);
    expect(groups.every(group => group.length <= 5)).toBe(true);
    expect(mobileListItems(items, 4, false)).toBe(items);
    expect(mobileListItems(items, 99, true)).toEqual(items.slice(clampMobileGroup(99, count) * 5));
  }
  expect(clampMobileGroup(3, 6)).toBe(1);
  expect(clampMobileGroup(-1, 20)).toBe(0);
});
