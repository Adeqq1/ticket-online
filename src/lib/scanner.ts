export type ScanStatus = "idle" | "valid" | "used" | "not-found";

export type ScanTicket = {
  id: string;
  attendee: string;
  ticketType: string;
  event: string;
  gate: string;
  isUsed: boolean;
};

export const demoTickets: ScanTicket[] = [
  { id: "TO-ALPHA-2027", attendee: "Ayu Pertiwi", ticketType: "Festival", event: "Nusa Malam", gate: "Gate A", isUsed: false },
  { id: "TO-BETA-2027", attendee: "Raka Pratama", ticketType: "Tribune", event: "Nusa Malam", gate: "Gate C", isUsed: false },
  { id: "TO-GAMMA-2027", attendee: "Sinta Lestari", ticketType: "VIP", event: "Nusa Malam", gate: "Gate A", isUsed: true },
];

export function normalizeScanCode(value: string) {
  return value.trim().toUpperCase();
}

export function scanTicket(tickets: ScanTicket[], code: string): { status: Exclude<ScanStatus, "idle">; ticket: ScanTicket | null } {
  const ticket = tickets.find((candidate) => candidate.id === normalizeScanCode(code));
  if (!ticket) return { status: "not-found", ticket: null };
  if (ticket.isUsed) return { status: "used", ticket };
  ticket.isUsed = true;
  return { status: "valid", ticket };
}
