import { formatRupiah, getConcertById, type Concert, type TicketTier } from "./concerts.ts";

type Tab = "description" | "lineup" | "location" | "terms";
export type DetailState = { activeTab: Tab; selectedZoneId: string | null; quantities: Record<string, number> };
export const initialState: DetailState = { activeTab: "description", selectedZoneId: null, quantities: {} };

export function changeQuantity(concert: Concert, state: DetailState, tierId: string, delta: -1 | 1): DetailState {
  const tier = concert.ticketTiers.find((item) => item.id === tierId);
  if (!tier) return state;
  const quantity = Math.max(0, Math.min(tier.stock, tier.maxPerOrder, (state.quantities[tierId] ?? 0) + delta));
  return { ...state, selectedZoneId: tier.zoneId, quantities: { ...state.quantities, [tierId]: quantity } };
}

export function cartTotal(concert: Concert, quantities: Record<string, number>) {
  return concert.ticketTiers.reduce((total, tier) => total + tier.price * (quantities[tier.id] ?? 0), 0);
}
export function ticketCount(quantities: Record<string, number>) { return Object.values(quantities).reduce((total, quantity) => total + quantity, 0); }

if (typeof document !== "undefined") {
  const app = document.querySelector<HTMLDivElement>("#detail-app");
  const id = location.pathname.split("/").at(-1);
  const concert = getConcertById(id);
  if (!app) throw new Error("Detail container tidak ditemukan.");
  if (!concert) {
    app.innerHTML = `<section class="detail-missing shell"><h1>Konser tidak ditemukan.</h1><p>Konser ini mungkin sudah tidak tersedia atau URL-nya tidak tepat.</p><a class="button" href="/konser">Kembali ke konser</a></section>`;
  } else {
    const detailConcert = concert;
    document.title = `${concert.artist} | Tiket Online`;
    let state = initialState;
    app.innerHTML = `<section class="detail-hero"><img class="detail-backdrop" src="${concert.image}" alt="" aria-hidden="true" /><div class="detail-hero-inner shell"><img class="detail-poster" src="${concert.image}" alt="Poster contoh konser ${concert.artist}" width="900" height="1100" fetchpriority="high" /><div><p class="detail-status">${concert.status}</p><h1>${concert.artist}</h1><p class="detail-summary">${concert.date}, ${concert.venue}, ${concert.city}</p><p class="detail-summary">${concert.genre} · Mulai ${formatRupiah.format(concert.price)}</p></div></div></section><section class="detail-layout shell"><div class="detail-main"><div class="tabs" role="tablist" aria-label="Informasi konser"><button role="tab" id="tab-description" aria-selected="true" aria-controls="panel-description" tabindex="0">Deskripsi</button><button role="tab" id="tab-lineup" aria-selected="false" aria-controls="panel-lineup" tabindex="-1">Line-up</button><button role="tab" id="tab-location" aria-selected="false" aria-controls="panel-location" tabindex="-1">Lokasi</button><button role="tab" id="tab-terms" aria-selected="false" aria-controls="panel-terms" tabindex="-1">Ketentuan</button></div><section id="panel-description" role="tabpanel" aria-labelledby="tab-description"><h2>Untuk malam yang panjang.</h2><p>${concert.description}</p></section><section id="panel-lineup" role="tabpanel" aria-labelledby="tab-lineup" hidden><h2>Line-up artis</h2><ul class="lineup-list">${concert.lineup.map((artist) => `<li>${artist}</li>`).join("")}</ul></section><section id="panel-location" role="tabpanel" aria-labelledby="tab-location" hidden><h2>Lokasi</h2><figure class="location-map"><div aria-hidden="true"><span></span><b>${concert.venue}</b></div><figcaption>${concert.address}</figcaption></figure></section><section id="panel-terms" role="tabpanel" aria-labelledby="tab-terms" hidden><h2>Syarat & ketentuan</h2><ul class="terms-list"><li>Tiket contoh ini tidak dapat digunakan untuk masuk ke event.</li><li>Jumlah tiket dibatasi sesuai stok yang ditampilkan.</li><li>Harga dan stok pada halaman ini adalah data demo.</li></ul></section><section class="ticket-selection" aria-labelledby="ticket-title"><h2 id="ticket-title">Pilih area dan tiket.</h2><div class="stage-layout" aria-label="Denah panggung"><div class="stage">Panggung</div>${concert.zones.map((zone) => `<button class="zone zone-${zone.id}" type="button" data-zone="${zone.id}" aria-pressed="false">${zone.name}<small>${zone.description}</small></button>`).join("")}</div><div class="tier-list" id="tier-list"></div></section></div><aside class="cart" aria-labelledby="cart-title"><h2 id="cart-title">Pesananmu</h2><div id="cart-lines"><p class="cart-empty">Belum ada tiket dipilih.</p></div><p class="cart-total" id="cart-total" aria-live="polite">0 tiket, ${formatRupiah.format(0)}</p><button class="button cart-checkout" id="checkout" type="button" disabled>Lanjut ke Pembayaran</button></aside></section><div class="mobile-cart" id="mobile-cart" hidden><span id="mobile-total">0 tiket</span><button class="button" id="mobile-checkout" type="button">Lanjut</button></div><dialog id="checkout-dialog" aria-labelledby="checkout-title"><h2 id="checkout-title">Checkout belum tersedia.</h2><p id="dialog-summary"></p><p>Pilihan tiket hanya tersimpan selama halaman ini terbuka.</p><button class="button" id="close-dialog" type="button">Tutup</button></dialog>`;

    const tiers = document.querySelector<HTMLDivElement>("#tier-list");
    const cartLines = document.querySelector<HTMLDivElement>("#cart-lines");
    const cartTotalElement = document.querySelector<HTMLParagraphElement>("#cart-total");
    const checkout = document.querySelector<HTMLButtonElement>("#checkout");
    const mobileCart = document.querySelector<HTMLDivElement>("#mobile-cart");
    const mobileTotal = document.querySelector<HTMLSpanElement>("#mobile-total");
    const dialog = document.querySelector<HTMLDialogElement>("#checkout-dialog");
    const dialogSummary = document.querySelector<HTMLParagraphElement>("#dialog-summary");

    function renderTiers() {
      if (!tiers) return;
      tiers.replaceChildren(...detailConcert.ticketTiers.map((tier) => tierRow(tier)));
      document.querySelectorAll<HTMLButtonElement>("[data-zone]").forEach((button) => button.setAttribute("aria-pressed", String(button.dataset.zone === state.selectedZoneId)));
    }
    function tierRow(tier: TicketTier) {
      const quantity = state.quantities[tier.id] ?? 0;
      const limit = Math.min(tier.stock, tier.maxPerOrder);
      const row = document.createElement("article");
      row.className = `ticket-tier${tier.zoneId === state.selectedZoneId ? " is-selected" : ""}`;
      row.innerHTML = `<div><h3>${tier.name}</h3><p>${tier.benefit}</p><strong>${formatRupiah.format(tier.price)}</strong></div><div class="tier-controls"><span class="stock ${tier.stock === 0 ? "sold-out" : tier.stock <= 5 ? "low-stock" : ""}">${tier.stock === 0 ? "Habis" : tier.stock <= 5 ? `Sisa ${tier.stock} tiket` : "Tersedia"}</span><div><button type="button" data-tier="${tier.id}" data-delta="-1" aria-label="Kurangi tiket ${tier.name}" ${quantity === 0 ? "disabled" : ""}>-</button><output aria-label="Jumlah tiket ${tier.name}">${quantity}</output><button type="button" data-tier="${tier.id}" data-delta="1" aria-label="Tambah tiket ${tier.name}" ${quantity >= limit ? "disabled" : ""}>+</button></div></div>`;
      return row;
    }
    function renderCart() {
      const count = ticketCount(state.quantities);
      const total = cartTotal(detailConcert, state.quantities);
      const selected = detailConcert.ticketTiers.filter((tier) => state.quantities[tier.id]);
      if (cartLines) cartLines.replaceChildren(...(selected.length ? selected.map((tier) => { const line = document.createElement("p"); line.textContent = `${state.quantities[tier.id]}x ${tier.name}: ${formatRupiah.format(tier.price * (state.quantities[tier.id] ?? 0))}`; return line; }) : [Object.assign(document.createElement("p"), { className: "cart-empty", textContent: "Belum ada tiket dipilih." })]));
      if (cartTotalElement) cartTotalElement.textContent = `${count} tiket, ${formatRupiah.format(total)}`;
      if (checkout) checkout.disabled = count === 0;
      if (mobileCart) mobileCart.hidden = count === 0;
      if (mobileTotal) mobileTotal.textContent = `${count} tiket, ${formatRupiah.format(total)}`;
      if (dialogSummary) dialogSummary.textContent = `${count} tiket dengan total ${formatRupiah.format(total)}.`;
    }
    function render() { renderTiers(); renderCart(); }
    function activateTab(tab: HTMLButtonElement) {
      document.querySelectorAll<HTMLButtonElement>('[role="tab"]').forEach((item) => { const active = item === tab; item.setAttribute("aria-selected", String(active)); item.tabIndex = active ? 0 : -1; document.querySelector(`#${item.getAttribute("aria-controls")}`)?.toggleAttribute("hidden", !active); });
    }
    document.querySelectorAll<HTMLButtonElement>('[role="tab"]').forEach((tab, index, tabs) => { tab.addEventListener("click", () => activateTab(tab)); tab.addEventListener("keydown", (event) => { const next = event.key === "ArrowRight" ? (index + 1) % tabs.length : event.key === "ArrowLeft" ? (index - 1 + tabs.length) % tabs.length : event.key === "Home" ? 0 : event.key === "End" ? tabs.length - 1 : index; if (next !== index || event.key === "Home" || event.key === "End") { event.preventDefault(); tabs[next]?.focus(); activateTab(tabs[next]!); } }); });
    document.addEventListener("click", (event) => { const target = event.target as HTMLElement; const zone = target.closest<HTMLButtonElement>("[data-zone]"); if (zone) { state = { ...state, selectedZoneId: zone.dataset.zone ?? null }; render(); } const control = target.closest<HTMLButtonElement>("[data-tier]"); if (control) { state = changeQuantity(detailConcert, state, control.dataset.tier ?? "", Number(control.dataset.delta) as -1 | 1); render(); } });
    [checkout, document.querySelector<HTMLButtonElement>("#mobile-checkout")].forEach((button) => button?.addEventListener("click", () => dialog?.showModal()));
    document.querySelector<HTMLButtonElement>("#close-dialog")?.addEventListener("click", () => dialog?.close());
    render();
  }
}
