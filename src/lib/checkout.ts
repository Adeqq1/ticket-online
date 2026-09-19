import { parseQuantities, type Concert, type TicketTier } from "./concerts.ts";

export type CheckoutLine = { tier: TicketTier; quantity: number };
export type Buyer = { name: string; email: string; phone: string; identity: string };
export const ADMIN_FEE = 7_500;

export function checkoutLines(concert: Concert, params: URLSearchParams): CheckoutLine[] {
  const quantities = parseQuantities(concert, params);
  return concert.ticketTiers.flatMap((tier) => {
    const quantity = quantities[tier.id];
    return quantity ? [{ tier, quantity }] : [];
  });
}

export function orderSubtotal(lines: CheckoutLine[]) {
  return lines.reduce((total, { tier, quantity }) => total + tier.price * quantity, 0);
}

export function voucherDiscount(subtotal: number, code: string) {
  return code.trim().toUpperCase() === "HEMAT10" ? Math.floor(subtotal * .1) : 0;
}

export function isValidEmail(value: string) { return /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(value); }
export function isValidPhone(value: string) { return /^\+?\d{9,15}$/.test(value.replace(/[ -]/g, "")); }
export function isValidIdentity(value: string) { return /^\d{12,20}$/.test(value); }
