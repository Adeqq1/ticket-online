export const mobileGroupSize = 5;

export function clampMobileGroup(group: number, count: number) {
  return Math.max(0, Math.min(group, Math.ceil(count / mobileGroupSize) - 1));
}

export function mobileListItems<T>(items: T[], group: number, mobile: boolean): T[] {
  const start = clampMobileGroup(group, items.length) * mobileGroupSize;
  return mobile ? items.slice(start, start + mobileGroupSize) : items;
}
