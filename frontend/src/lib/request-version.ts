export function createRequestVersion() {
  let current = 0;
  return {
    next: () => ++current,
    value: () => current,
    isCurrent: (version: number) => version === current,
  };
}
