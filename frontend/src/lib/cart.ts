import type { Concert } from "./concerts.ts";

export type Tab = "description" | "lineup" | "location" | "terms";
export type DetailState = { selectedZoneId: string | null; quantities: Record<string, number> };
export const initialState: DetailState = { selectedZoneId: null, quantities: {} };

export function changeQuantity(concert: Concert, state: DetailState, tierId: string, delta: -1 | 1): DetailState {
  const tier = concert.ticketTiers.find((item) => item.id === tierId);
  if (!tier) return state;
  const quantity = Math.max(0, Math.min(tier.stock, tier.maxPerOrder, (state.quantities[tierId] ?? 0) + delta));
  return { ...state, selectedZoneId: tier.zoneId, quantities: { ...state.quantities, [tierId]: quantity } };
}

export function cartTotal(concert: Concert, quantities: Record<string, number>) {
  return concert.ticketTiers.reduce((total, tier) => total + tier.price * (quantities[tier.id] ?? 0), 0);
}

export function ticketCount(quantities: Record<string, number>) {
  return Object.values(quantities).reduce((total, quantity) => total + quantity, 0);
}

export function quantityParams(quantities: Record<string, number>) {
  const params = new URLSearchParams();
  Object.entries(quantities).forEach(([tierId, quantity]) => {
    if (quantity > 0) params.set(tierId, String(quantity));
  });
  return params;
}
