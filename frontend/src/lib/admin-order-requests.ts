export async function applyIfCurrent<T>(current: () => boolean, request: () => Promise<T>, apply: (value: T) => void | Promise<void>) {
  const value = await request();
  if (!current()) return false;
  await apply(value);
  return true;
}
