"""Capture reproducible public-page visual baselines with fake API responses.

Run from frontend/: python3 scripts/public-ui-check.py
Requires the installed Python Playwright package and Google Chrome.
"""
import asyncio
import argparse
import json
import subprocess
from datetime import datetime, timedelta, timezone
from pathlib import Path
from urllib.parse import parse_qsl, urlparse

from playwright.async_api import async_playwright


ROOT = Path(__file__).resolve().parents[2]
OUTPUT = ROOT / "docs" / "phase20" / "screenshots"
FIXTURES = ROOT / "docs" / "phase20" / "fixtures"
MODE = "baseline"
BASE_URL = "http://127.0.0.1:5173"
ORDER_ID = "0123456789abcdef0123456789abcdef"
RESERVATION_ID = "abcdef0123456789abcdef0123456789"
TICKET_ID = "0123456789abcdefabcdef0123456789"
TOKEN = "t" * 43
EVENT_START = "2027-08-24T19:30:00+07:00"
EXPIRY = (datetime.now(timezone.utc) + timedelta(minutes=10)).isoformat()


def event(state="SCHEDULED"):
    changed = state != "SCHEDULED"
    return {
        "id": "nusa-malam", "artist": "Nusa Malam", "city": "Jakarta",
        "venue": "Ruang Selatan", "address": "Jl. Musik Raya, Jakarta",
        "startsAt": EVENT_START, "genre": "Indie", "status": "Presale",
        "publicationStatus": "PUBLISHED", "image": "https://picsum.photos/seed/nusa-malam/900/1100",
        "description": "Konser Nusa Malam dengan pilihan tiket untuk area panggung dan penonton.",
        "lineup": ["Nusa Malam", "Pembuka Sore", "Tamu Spesial"], "price": 275000,
        "zones": [
            {"id": "vip-a", "name": "VIP A", "description": "Area depan panggung"},
            {"id": "vip-b", "name": "VIP B", "description": "Area samping panggung"},
            {"id": "festival", "name": "Festival", "description": "Area berdiri umum"},
            {"id": "tribune", "name": "Tribune", "description": "Area duduk bertingkat"},
        ],
        "ticketTiers": [
            {"id": "vip-a", "name": "VIP A", "zoneId": "vip-a", "price": 650000, "availableQuantity": 5, "maxPerOrder": 4, "benefit": "Area depan panggung", "gate": "Gate A", "seating": "free-standing"},
            {"id": "vip-b", "name": "VIP B", "zoneId": "vip-b", "price": 475000, "availableQuantity": 18, "maxPerOrder": 4, "benefit": "Area samping panggung", "gate": "Gate A", "seating": "free-standing"},
            {"id": "festival", "name": "Festival", "zoneId": "festival", "price": 275000, "availableQuantity": 42, "maxPerOrder": 6, "benefit": "Area berdiri umum", "gate": "Gate B", "seating": "free-standing"},
            {"id": "tribune", "name": "Tribune", "zoneId": "tribune", "price": 350000, "availableQuantity": 24, "maxPerOrder": 4, "benefit": "Area duduk", "gate": "Gate C", "seating": "assigned"},
        ],
        "scheduleLocked": False,
        **({"currentEvent": {"id": "nusa-malam", "status": state, "version": 1, "salesPaused": state in ("POSTPONED", "CANCELLED"), "startsAt": "2027-09-01T19:30:00+07:00", "announcement": "Jadwal acara telah diperbarui. Periksa detail pesanan untuk informasi refund.", "refundDeadline": "2027-08-30T23:59:00+07:00"}} if changed else {}),
    }


def ticket():
    return {
        "id": TICKET_ID, "code": "ET-0123456789ABCDEF", "attendeeName": "Nadia Pembeli",
        "orderReference": "TO-0123456789abcdef0123", "eventId": "nusa-malam",
        "eventArtist": "Nusa Malam", "eventCity": "Jakarta", "eventVenue": "Ruang Selatan",
        "eventAddress": "Jl. Musik Raya, Jakarta", "eventStartsAt": EVENT_START,
        "tierName": "Festival", "gate": "Gate B", "issuedAt": "2026-10-01T10:00:00Z",
    }


def order(status="PAID", payment_status="SUCCEEDED", event_state="SCHEDULED", refund_state=None, refund_deadline="2027-08-30T23:59:00+07:00"):
    result = {
        "id": ORDER_ID, "reference": "TO-0123456789abcdef0123", "reservationId": RESERVATION_ID,
        "status": status, "expiresAt": EXPIRY, "subtotal": 275000, "adminFee": 5000,
        "discount": 0, "total": 280000,
        "items": [{"tierId": "festival", "name": "Festival", "quantity": 1, "unitPrice": 275000, "lineTotal": 275000}],
        "buyer": {"name": "Nadia Pembeli", "email": "nadia@example.test", "phone": "08123456789", "identity": "1234567890123456"},
        "attendees": [{"tierId": "festival", "ticketNumber": 1, "name": "Nadia Pembeli"}],
        "payment": {"id": "payment-demo", "method": "QRIS", "amount": 280000, "status": payment_status},
        "createdAt": "2026-10-01T10:00:00Z", "updatedAt": "2026-10-01T10:01:00Z",
        "eventStartsAt": EVENT_START, "accessExpiresAt": None,
    }
    if event_state != "SCHEDULED":
        result["currentEvent"] = event(event_state)["currentEvent"]
    if event_state in ("POSTPONED", "RESCHEDULED"):
        result["refundRight"] = {"requested": status in ("REFUND_PENDING", "REFUNDED"), "deadline": refund_deadline}
    if status in ("REFUND_PENDING", "REFUNDED"):
        result["refund"] = {"status": refund_state or ("PROCESSING" if status == "REFUND_PENDING" else "SUCCEEDED"), "amount": 280000}
    return result


def reservation():
    return {"id": RESERVATION_ID, "status": "ACTIVE", "expiresAt": EXPIRY,
            "event": {"id": "nusa-malam", "artist": "Nusa Malam"},
            "items": [{"tierId": "festival", "name": "Festival", "quantity": 1, "unitPrice": 275000, "lineTotal": 275000}], "subtotal": 275000}


def reply(path, scenario, page_state="standard", refund_requested=False):
    query = dict(item.split("=", 1) for item in path.query.split("&") if "=" in item)
    state = query.get("snapshot", page_state)
    endpoint = path.path
    if scenario == "error":
        return 503, {"error": {"code": "SERVICE_UNAVAILABLE", "message": "Layanan sedang tidak tersedia."}}
    events = [event("POSTPONED" if state == "changed" else "SCHEDULED")]
    if endpoint == "/api/v1/events":
        if scenario == "phase21-search":
            events = [{**events[0], "id": f"fixture-{index}", "artist": f"Konser Contoh {index}", "venue": f"Venue {index}"} for index in range(1, 5)]
            events.append({**events[0], "id": "fixture-fifth", "artist": "Panggung Terakhir", "venue": "Gedung Musik", "image": "https://picsum.photos/seed/nusa-malam/900/1100"})
        elif scenario == "phase21-card-states":
            base = events[0]
            sold_out = {**base, "id": "sold-out", "artist": "Konser Habis", "status": "Sold Out", "ticketTiers": [{**base["ticketTiers"][0], "availableQuantity": 0}]}
            no_tickets = {**base, "id": "no-tickets", "artist": "Tiket Menyusul", "ticketTiers": []}
            changed = event("RESCHEDULED")
            changed.update({"id": "rescheduled", "artist": "Jadwal Baru"})
            broken_poster = {**base, "id": "broken-poster", "artist": "Poster Gagal", "image": "/missing-poster.jpg"}
            events = [sold_out, no_tickets, changed, broken_poster]
        return 200, {"events": [] if scenario == "empty" else events}
    if endpoint.startswith("/api/v1/events/"):
        result = event("POSTPONED" if state == "changed" else "SCHEDULED")
        if scenario in ("phase22-rescheduled", "phase22-postponed", "phase22-cancelled"):
            event_state = {"phase22-rescheduled": "RESCHEDULED", "phase22-postponed": "POSTPONED", "phase22-cancelled": "CANCELLED"}[scenario]
            result = event(event_state)
        if scenario == "phase22-sold-out":
            result["status"] = "Sold Out"
            result["ticketTiers"] = [{**tier, "availableQuantity": 0} for tier in result["ticketTiers"]]
        if scenario == "phase22-no-tiers":
            result["ticketTiers"] = []
        return 200, result
    if endpoint.endswith("/event") and "/reservations/" in endpoint:
        return 200, event()
    if endpoint == "/api/v1/reservations":
        return 201, reservation()
    if endpoint.endswith("/checkout") and "/reservations/" in endpoint:
        result = order("PENDING", "PENDING")
        return 201, {**result, "accessToken": TOKEN}
    if endpoint.startswith("/api/v1/reservations/"):
        return 200, reservation()
    if endpoint.endswith("/refund-request"):
        if scenario == "phase23-refund-unknown":
            return 503, {"error": {"code": "SERVICE_UNAVAILABLE", "message": "Status pengajuan belum dapat dipastikan."}}
        return 200, {"status": "REQUESTED"}
    if endpoint.startswith("/api/v1/orders/") and endpoint.endswith("/tickets"):
        if scenario == "phase23-ticket-error":
            return 503, {"error": {"code": "SERVICE_UNAVAILABLE", "message": "Daftar tiket belum dapat dimuat."}}
        return 200, {"tickets": [ticket()]}
    if endpoint.startswith("/api/v1/orders/"):
        status = {"pending": "PENDING", "cancelled": "CANCELLED", "expired": "EXPIRED", "refund-pending": "REFUND_PENDING", "refund-unknown": "REFUND_PENDING", "refund-manual": "REFUND_PENDING", "refund-failed": "REFUND_PENDING", "refunded": "REFUNDED"}.get(state, "PAID")
        payment_status = "PENDING" if status == "PENDING" else "SUCCEEDED"
        event_state = "POSTPONED" if state in ("changed", "refund-pending", "refund-unknown", "refund-manual", "refund-failed", "refunded", "no-deadline") else "RESCHEDULED" if state == "past-deadline" else "SCHEDULED"
        refund_state = {"refund-unknown": "UNKNOWN", "refund-manual": "MANUAL_REQUIRED", "refund-failed": "FAILED"}.get(state)
        refund_deadline = None if state == "no-deadline" else "2020-08-30T23:59:00+07:00" if state == "past-deadline" else "2027-08-30T23:59:00+07:00"
        result = order(status, payment_status, event_state, refund_state, refund_deadline)
        if scenario == "phase23-refund-eligible":
            result = order("REFUND_PENDING" if refund_requested else "PAID", "SUCCEEDED", "POSTPONED", "PROCESSING" if refund_requested else None)
        if scenario == "phase23-refund-eligible" and refund_requested:
            result["status"] = "REFUND_PENDING"
            result["refundRight"] = {"requested": True, "deadline": "2027-08-30T23:59:00+07:00"}
            result["refund"] = {"status": "PROCESSING", "amount": 280000}
        if scenario == "phase23-payment-accepted-pending":
            result = order("PENDING", "SUCCEEDED")
        if scenario == "phase23-checkout-retry":
            result = order("PENDING", "PENDING")
        if scenario == "phase23-refund-unknown":
            result = order("PAID", "SUCCEEDED", "RESCHEDULED")
        return 200, result
    if endpoint == "/api/v1/tickets/" + TICKET_ID:
        result = ticket()
        if state == "changed":
            result["currentEvent"] = event("POSTPONED")["currentEvent"]
        if state == "inactive":
            result["usable"] = False
        return 200, result
    if endpoint == "/api/v1/ticket-recovery":
        return 202, {"message": "Jika data cocok, tautan pemulihan akan dikirim ke email pembeli."}
    if endpoint == "/api/v1/ticket-recovery/verify":
        return 200, {"orderId": ORDER_ID, "accessToken": TOKEN, "reference": "TO-0123456789abcdef0123", "expiresAt": EXPIRY, "accessExpiresAt": None, "reservationId": RESERVATION_ID, "ticketIds": [TICKET_ID]}
    if endpoint.endswith("/simulate-payment"):
        return 200, {"id": "payment-demo", "orderId": ORDER_ID, "orderStatus": "PAID", "method": "QRIS", "amount": 280000, "status": "SUCCEEDED", "tickets": [ticket()]}
    return 200, {}


async def capture(page, name, description):
    OUTPUT.mkdir(parents=True, exist_ok=True)
    await page.evaluate("""async () => {
      const images = [...document.images];
      images.forEach(image => image.loading = 'eager');
      await Promise.all(images.map(image => image.decode().catch(() => undefined)));
    }""")
    if MODE == "baseline":
        await page.add_style_tag(content="""
          .public-site { --canvas:#f3f4f0!important; --surface:#e6e8e1!important; --ink:#17211f!important;
            --muted:#56615e!important; --line:#bdc5bd!important; --control-line:#7d8a84!important;
            --accent:#1358d8!important; --accent-ink:#f5f8ff!important; --danger:#8f2f22!important;
            --inverse-surface:#17211f!important; --inverse-text:#f3f4f0!important;
            font-family:Arial,Helvetica,sans-serif!important; }
        """)
    filename = f"{MODE}-{name}.png"
    await page.screenshot(path=str(OUTPUT / filename), full_page=True, animations="disabled")
    return {"file": filename, "capture": description, "title": await page.title()}


async def main():
    global MODE, OUTPUT
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--current", action="store_true", help="Capture with the current UI instead of baseline tokens.")
    parser.add_argument("--output-dir", type=Path, help="Write captures to a directory separate from the Phase 20 baseline.")
    args = parser.parse_args()
    MODE = "current" if args.current else "baseline"
    if args.output_dir:
        OUTPUT = args.output_dir if args.output_dir.is_absolute() else ROOT / args.output_dir
    commit = subprocess.run(["git", "rev-parse", "--short", "HEAD"], cwd=ROOT, check=True, capture_output=True, text=True).stdout.strip()
    captures = []
    layout_checks = []
    isolation_checks = []
    routes = [
        ("home", "/", "beranda"), ("catalog", "/konser", "katalog"),
        ("detail", "/konser/nusa-malam", "detail-konser"),
        ("checkout", "/checkout/nusa-malam?festival=1", "checkout-data"),
        ("order", f"/pesanan/{ORDER_ID}", "pesanan"), ("my-tickets", "/tiket-saya", "tiket-saya"),
        ("ticket", f"/tiket/{TICKET_ID}", "e-ticket"),
        ("recovery", "/pulihkan-tiket", "pemulihan"), ("guide", "/panduan", "panduan"),
        ("not-found", "/alamat-tidak-dikenal", "tidak-ditemukan"),
    ]
    async with async_playwright() as playwright:
        browser = await playwright.chromium.launch(channel="chrome", headless=True)
        for viewport_name, viewport in (("desktop", {"width": 1440, "height": 900}), ("mobile", {"width": 390, "height": 844})):
            context = await browser.new_context(viewport=viewport, device_scale_factor=1)

            async def api(route):
                url = urlparse(route.request.url)
                page_params = dict(parse_qsl(urlparse(page.url).query))
                scenario = page_params.get("scenario", "standard")
                if url.path.endswith("/refund-request") and route.request.method == "POST":
                    api.refund_post_count = getattr(api, "refund_post_count", 0) + 1
                    if scenario == "phase23-refund-eligible":
                        api.refund_requested = True
                        await route.fulfill(status=200, content_type="application/json", body=json.dumps({"status": "REQUESTED"}))
                        return
                if url.path.endswith("/checkout") and "/reservations/" in url.path and route.request.method == "POST" and scenario == "phase23-checkout-retry":
                    api.checkout_requests = getattr(api, "checkout_requests", [])
                    api.checkout_requests.append({"key": route.request.headers.get("idempotency-key"), "payload": route.request.post_data_json})
                    if len(api.checkout_requests) == 1:
                        await route.fulfill(status=503, content_type="application/json", body=json.dumps({"error": {"code": "SERVICE_UNAVAILABLE", "message": "Respons order belum dapat dipastikan."}}))
                        return
                    result = order("PENDING", "PENDING")
                    await route.fulfill(status=201, content_type="application/json", body=json.dumps({**result, "accessToken": TOKEN}))
                    return
                if scenario == "loading":
                    await asyncio.sleep(1.5)
                status, body = reply(url, scenario, page_params.get("snapshot", "standard"), getattr(api, "refund_requested", False))
                await route.fulfill(status=status, content_type="application/json", body=json.dumps(body))

            async def photo(route):
                path = urlparse(route.request.url).path
                fixture = next((file for seed, file in (("ticket-online-concert-crowd", "concert-crowd.jpg"), ("nusa-malam", "nusa-malam.jpg"), ("ticket-online-stage-lights", "stage-lights.jpg")) if seed in path), None)
                if fixture is None:
                    await route.abort()
                    return
                await route.fulfill(path=str(FIXTURES / fixture))

            await context.route("**/api/v1/**", api)
            await context.route("https://picsum.photos/**", photo)
            await context.add_init_script(f"""localStorage.setItem('ticket-online:order:{ORDER_ID}', JSON.stringify({json.dumps({"orderId": ORDER_ID, "accessToken": TOKEN, "expiresAt": EXPIRY, "accessExpiresAt": None, "reservationId": RESERVATION_ID, "reference": "TO-0123456789abcdef0123", "ticketIds": [TICKET_ID]})}));""")
            page = await context.new_page()
            for route_name, url, label in routes:
                await page.goto(BASE_URL + url, wait_until="domcontentloaded")
                await page.wait_for_timeout(350)
                if route_name == "order":
                    assert await page.locator(".order-status").is_visible(), "The payment or order status should lead the order page."
                    assert await page.locator(".order-ticket-section").evaluate("element => element.compareDocumentPosition(document.querySelector('#refund-right-title').closest('.recovery-panel')) & Node.DOCUMENT_POSITION_FOLLOWING"), "Tickets should appear before refund details."
                    assert await page.locator(".order-breakdown").evaluate("element => element.compareDocumentPosition(document.querySelector('#refund-right-title').closest('.recovery-panel')) & Node.DOCUMENT_POSITION_PRECEDING"), "Order item details should follow refund information."
                if route_name == "catalog" and viewport_name == "desktop":
                    assert await page.locator(".filters").evaluate("element => element.open"), "Catalog filters should start open on desktop."
                if route_name == "detail":
                    if viewport_name == "desktop":
                        assert await page.locator(".zone-details").evaluate("element => element.open"), "The zone map should start open on desktop."
                    else:
                        assert await page.locator(".mobile-cart").is_visible(), "The mobile summary bar should appear before a ticket is selected."
                        assert await page.get_by_role("button", name="Lanjut checkout").is_disabled(), "Checkout should remain disabled with an empty selection."
                captures.append(await capture(page, f"{viewport_name}-{label}", f"{viewport_name}, {label}, standard"))
            for page_name, url, label in (("home", "/?scenario=loading", "beranda"), ("catalog", "/konser?scenario=empty", "katalog-kosong"), ("catalog", "/konser?scenario=error", "katalog-error"), ("detail", "/konser/nusa-malam?scenario=error", "detail-error")):
                await page.goto(BASE_URL + url, wait_until="domcontentloaded")
                await page.wait_for_timeout(100 if "loading" in url else 350)
                captures.append(await capture(page, f"{viewport_name}-{label}-{page_name}", f"{viewport_name}, {label}"))
            for state in ("pending", "cancelled", "expired", "changed", "refund-pending", "refunded"):
                await page.goto(f"{BASE_URL}/pesanan/{ORDER_ID}?snapshot={state}", wait_until="domcontentloaded")
                await page.wait_for_timeout(350)
                captures.append(await capture(page, f"{viewport_name}-pesanan-{state}", f"{viewport_name}, status pesanan {state}"))
            for state in ("refund-unknown", "refund-manual", "refund-failed", "no-deadline", "past-deadline"):
                await page.goto(f"{BASE_URL}/pesanan/{ORDER_ID}?snapshot={state}", wait_until="domcontentloaded")
                await page.wait_for_timeout(350)
                captures.append(await capture(page, f"{viewport_name}-pesanan-{state}", f"{viewport_name}, refund {state}"))
            for scenario, label in (("phase23-ticket-error", "tiket-error"), ("phase23-payment-accepted-pending", "pembayaran-terkonfirmasi-pesanan-pending")):
                await page.goto(f"{BASE_URL}/pesanan/{ORDER_ID}?scenario={scenario}", wait_until="domcontentloaded")
                await page.wait_for_timeout(350)
                captures.append(await capture(page, f"{viewport_name}-pesanan-{label}", f"{viewport_name}, pesanan {label}"))
            for state in ("changed", "inactive"):
                await page.goto(f"{BASE_URL}/tiket/{TICKET_ID}?snapshot={state}", wait_until="domcontentloaded")
                await page.wait_for_timeout(350)
                captures.append(await capture(page, f"{viewport_name}-e-ticket-{state}", f"{viewport_name}, e-ticket {state}"))
            await page.goto(f"{BASE_URL}/pulihkan-tiket#token={TOKEN}", wait_until="domcontentloaded")
            await page.wait_for_timeout(350)
            captures.append(await capture(page, f"{viewport_name}-pemulihan-tautan", f"{viewport_name}, tautan pemulihan siap"))
            await page.evaluate("sessionStorage.clear()")
            await page.goto(f"{BASE_URL}/checkout/nusa-malam?festival=1", wait_until="domcontentloaded")
            await page.wait_for_timeout(350)
            for step, label in ((1, "data"), (2, "pembayaran"), (3, "konfirmasi")):
                if step == 2:
                    for selector, value in (("#buyer-name", "Nadia Pembeli"), ("#buyer-email", "nadia@example.test"), ("#buyer-phone", "08123456789"), ("#buyer-identity", "1234567890123456"), ("#attendee-festival-0", "Nadia Pembeli")):
                        await page.locator(selector).fill(value)
                    await page.get_by_role("button", name="Lanjut ke pembayaran").click()
                elif step == 3:
                    await page.locator('input[name="payment"][value="QRIS"]').check()
                    await page.get_by_role("button", name="Tinjau pesanan").click()
                await page.wait_for_timeout(150)
                if step == 1 and viewport_name == "mobile":
                    assert await page.locator(".checkout-overview").is_visible(), "The mobile checkout total should remain visible before confirmation."
                if step == 3:
                    assert await page.locator(".summary-breakdown").evaluate("element => element.open"), "Cost details should be open at final confirmation."
                    assert await page.get_by_text("Biaya admin", exact=True).is_visible(), "The admin fee should be visible before order confirmation."
                    assert await page.evaluate("document.activeElement?.id === 'confirmation-title'"), "The final checkout step should receive keyboard focus."
                    assert await page.locator(".skip-link").evaluate("element => getComputedStyle(element).top === '-64px'"), "The skip link should remain hidden unless it has keyboard focus."
                captures.append(await capture(page, f"{viewport_name}-checkout-{label}", f"{viewport_name}, checkout langkah {step}"))
            if viewport_name == "mobile":
                await page.goto(BASE_URL + "/checkout/nusa-malam?festival=1", wait_until="domcontentloaded")
                await page.get_by_role("button", name="Lanjut ke pembayaran").click()
                invalid_name = page.locator("#buyer-name")
                assert await invalid_name.get_attribute("aria-invalid") == "true" and await invalid_name.evaluate("element => document.activeElement === element"), "Invalid checkout fields should receive focus and expose their error state."
                assert await page.locator("#name-error").inner_text(), "The validation message should remain beside its field."

                await page.set_viewport_size({"width": 390, "height": 420})
                await page.goto(BASE_URL + "/checkout/nusa-malam?festival=1", wait_until="domcontentloaded")
                await page.get_by_label("Nama lengkap").focus()
                await page.wait_for_timeout(500)
                keyboard_layout = await page.evaluate("""() => { const input = document.querySelector('#buyer-name'); const bounds = input.getBoundingClientRect(); return {focused: document.activeElement === input, inputTop: bounds.top, inputBottom: bounds.bottom, stickyBottom: document.querySelector('.checkout-overview').getBoundingClientRect().bottom, viewport: innerHeight}; }""")
                assert keyboard_layout["focused"] and keyboard_layout["inputTop"] >= keyboard_layout["stickyBottom"] and keyboard_layout["inputBottom"] <= keyboard_layout["viewport"], f"The focused field should remain visible below the sticky total in a short viewport; got {keyboard_layout}."

                await page.goto(BASE_URL + "/checkout/nusa-malam?festival=1&scenario=phase23-checkout-retry", wait_until="domcontentloaded")
                for selector, value in (("#buyer-name", "Nadia Pembeli"), ("#buyer-email", "nadia@example.test"), ("#buyer-phone", "08123456789"), ("#buyer-identity", "1234567890123456"), ("#attendee-festival-0", "Nadia Pembeli")):
                    await page.locator(selector).fill(value)
                await page.get_by_role("button", name="Lanjut ke pembayaran").click()
                await page.locator('input[name="payment"][value="QRIS"]').check()
                await page.get_by_role("button", name="Tinjau pesanan").click()
                await page.get_by_role("button", name="Buat pesanan dan bayar").click()
                await page.get_by_role("button", name="Coba ulang checkout").wait_for()
                await page.reload(wait_until="domcontentloaded")
                await page.get_by_role("heading", name="Status pembayaran").wait_for()
                retries = getattr(api, "checkout_requests", [])
                assert len(retries) == 2 and retries[0]["key"] == retries[1]["key"] and retries[0]["payload"] == retries[1]["payload"], f"Checkout recovery must reuse the same idempotency key and payload; got {retries}."

                await page.goto(BASE_URL + f"/pesanan/{ORDER_ID}?scenario=phase23-refund-eligible", wait_until="domcontentloaded")
                await page.get_by_role("checkbox", name="Saya memahami refund mencakup seluruh order dan semua tiket akan dinonaktifkan.").check()
                await page.get_by_role("button", name="Ajukan refund penuh").click()
                await page.get_by_text("Pengajuan diterima. Dana belum dinyatakan kembali", exact=False).wait_for()
                assert getattr(api, "refund_post_count", 0) == 1, "A refund confirmation should submit one request."
                assert await page.get_by_text("Sedang diproses", exact=True).count() == 1, "A requested refund must remain visibly pending."

                await page.goto(BASE_URL + f"/pesanan/{ORDER_ID}?scenario=phase23-refund-unknown", wait_until="domcontentloaded")
                await page.get_by_role("checkbox", name="Saya memahami refund mencakup seluruh order dan semua tiket akan dinonaktifkan.").check()
                await page.get_by_role("button", name="Ajukan refund penuh").click()
                await page.get_by_role("button", name="Periksa status").wait_for()
                assert getattr(api, "refund_post_count", 0) == 2, "An uncertain refund request should not submit a duplicate automatically."
            if viewport_name == "mobile":
                await page.set_viewport_size({"width": 390, "height": 844})
                await page.goto(BASE_URL + "/konser/nusa-malam?scenario=phase22-normal", wait_until="domcontentloaded")
                await page.get_by_role("heading", name="Nusa Malam").wait_for()
                assert not await page.locator(".zone-details").evaluate("element => element.open"), "The zone map should start collapsed on mobile."
                await page.get_by_role("button", name="Tambah tiket VIP A").click()
                assert "1 tiket" in await page.locator(".mobile-cart").inner_text(), "The summary should update when a ticket is selected."
                subtotal_text = await page.locator(".mobile-cart").inner_text()
                assert "650.000" in subtotal_text, f"The mobile bar should show the selected subtotal; got {subtotal_text!r}."
                await page.locator(".ticket-tier:nth-child(3) .tier-controls button:last-child").evaluate("element => element.focus()")
                await page.keyboard.press("Tab")
                await page.evaluate("new Promise(resolve => requestAnimationFrame(resolve))")
                focus_layout = await page.evaluate("""() => ({focused: document.activeElement === document.querySelector('.ticket-tier:last-child .tier-controls button:last-child'), buttonBottom: document.querySelector('.ticket-tier:last-child .tier-controls button:last-child').getBoundingClientRect().bottom, barTop: document.querySelector('.mobile-cart').getBoundingClientRect().top})""")
                assert focus_layout["focused"] and focus_layout["buttonBottom"] <= focus_layout["barTop"], f"Focused controls should remain above the mobile summary bar; got {focus_layout}."
                await page.get_by_role("button", name="Lanjut checkout").click()
                await page.wait_for_url("**/checkout/nusa-malam?vip-a=1")

                await page.goto(BASE_URL + "/konser/nusa-malam?scenario=phase22-rescheduled", wait_until="domcontentloaded")
                await page.locator(".detail-status").get_by_text("Jadwal diperbarui", exact=True).wait_for()
                assert "September" in await page.locator(".detail-heading .detail-summary").first.inner_text(), "The detail heading should use the updated event date."
                assert "30 Agustus" in await page.locator(".refund-note").inner_text(), "The current refund deadline should be visible."
                captures.append(await capture(page, "mobile-jadwal-diperbarui", "mobile, konser dengan jadwal diperbarui"))

                await page.goto(BASE_URL + "/konser/nusa-malam?scenario=phase22-cancelled", wait_until="domcontentloaded")
                await page.locator(".detail-status").get_by_text("Acara dibatalkan", exact=True).wait_for()
                assert "refund penuh" in await page.locator(".refund-note").inner_text(), "Cancellation refund rights should remain visible."
                assert await page.locator(".mobile-cart .button").is_disabled(), "A cancelled event should disable checkout."
                await page.goto(BASE_URL + "/konser/nusa-malam?scenario=phase22-sold-out", wait_until="domcontentloaded")
                await page.locator(".detail-status").get_by_text("Tiket habis", exact=True).wait_for()
                assert await page.locator(".ticket-tier .tier-controls button:last-child").first.is_disabled(), "Sold-out tiers should disable quantity controls."
                await page.goto(BASE_URL + "/konser/nusa-malam?scenario=phase22-no-tiers", wait_until="domcontentloaded")
                await page.get_by_text("Tiket dan harga belum diumumkan.").wait_for()
                assert await page.locator(".mobile-cart .button").is_disabled(), "Checkout should remain disabled when ticket tiers are missing."
                await page.goto(BASE_URL + "/konser/nusa-malam?scenario=error", wait_until="domcontentloaded")
                await page.get_by_role("alert").wait_for()
                await page.get_by_role("button", name="Coba lagi").click()
                await page.get_by_role("alert").wait_for()

                await page.set_viewport_size({"width": 360, "height": 800})
                await page.goto(BASE_URL + "/?scenario=phase21-search", wait_until="domcontentloaded")
                await page.locator(".concert-card").first.wait_for()
                assert await page.locator(".concert-card").count() == 4, "The home page should initially show four events."
                await page.locator("#event-query").fill("Panggung Terakhir")
                await page.locator("#event-query").press("Enter")
                assert await page.get_by_role("heading", name="Panggung Terakhir").count() == 1, "Home search should find events beyond the first four."
                assert await page.locator(".concert-card").count() == 1, "The home search should show only matching events."
                await page.locator("#event-query").fill("tidak cocok")
                await page.locator("#event-query").press("Enter")
                assert await page.get_by_text("Tidak ada konser yang cocok dengan pencarianmu.").is_visible(), "Home search should announce an empty result."
                await page.locator("#event-query").fill("")
                await page.locator("#event-query").press("Enter")
                first_card = await page.locator(".concert-card").first.bounding_box()
                assert first_card and first_card["y"] + first_card["height"] <= 800, f"The first home result should fit within 360×800; got {first_card}."
                assert await page.evaluate("document.documentElement.scrollWidth <= innerWidth"), "The home page should not overflow at 360px."
                await page.locator("#event-query").evaluate("element => element.blur()")
                captures.append(await capture(page, "mobile360-beranda", "mobile 360×800, beranda"))

                await page.goto(BASE_URL + "/konser?scenario=phase21-card-states", wait_until="domcontentloaded")
                await page.get_by_role("heading", name="Konser Habis").wait_for()
                assert await page.get_by_text("Habis", exact=True).is_visible(), "Sold-out events should have a visible availability status."
                no_tickets = page.locator(".concert-card").filter(has=page.get_by_role("heading", name="Tiket Menyusul"))
                assert await no_tickets.locator(".concert-status").inner_text() == "Tiket belum tersedia", "Events without tiers should show ticket availability."
                assert await page.get_by_text("Jadwal diperbarui", exact=True).is_visible(), "Changed event schedules should be visible on the card."
                await page.locator(".poster-unavailable").wait_for(state="attached")
                assert await page.locator(".poster-unavailable").count() == 1, "A failed event poster should leave the neutral poster fallback."

                await page.goto(BASE_URL + "/konser", wait_until="domcontentloaded")
                await page.locator("#concert-query").fill("Nusa")
                await page.locator("#concert-query").press("Enter")
                await page.locator(".filters summary").click()
                await page.locator("#concert-city").select_option(label="Jakarta")
                await page.locator(".filters summary").click()
                assert "Jakarta" in await page.locator(".filters summary").inner_text(), "The closed filter summary should show the active city."
                await page.locator(".reset-filters").click()
                assert await page.locator("#concert-city").input_value() == "", "Reset should clear the city filter."
                assert await page.locator("#concert-query").input_value() == "", "Reset should clear the catalog search."
                assert await page.evaluate("document.documentElement.scrollWidth <= innerWidth"), "The catalog should not overflow at 360px."
                captures.append(await capture(page, "mobile360-katalog", "mobile 360×800, katalog"))

                await page.set_viewport_size({"width": 320, "height": 740})
                for route_name, url, label in routes:
                    await page.goto(BASE_URL + url, wait_until="domcontentloaded")
                    await page.wait_for_timeout(100)
                    metrics = await page.evaluate("({viewport: innerWidth, document: document.documentElement.scrollWidth, body: document.body.scrollWidth, font: getComputedStyle(document.querySelector('.public-site')).fontFamily})")
                    layout_checks.append({"route": label, **metrics})
                await page.goto(BASE_URL + "/panduan", wait_until="domcontentloaded")
                await page.evaluate("document.fonts.ready")
                isolation_checks.append(await page.evaluate("""() => ({kind:'public-font', loaded:document.fonts.check('400 16px Manrope'), family:getComputedStyle(document.querySelector('.public-site')).fontFamily})"""))
                await page.emulate_media(color_scheme="dark")
                isolation_checks.append(await page.evaluate("""() => ({kind:'dark-mode', accent:getComputedStyle(document.querySelector('.public-site')).getPropertyValue('--accent').trim()})"""))
                await page.goto(BASE_URL + "/admin/login", wait_until="domcontentloaded")
                isolation_checks.append(await page.evaluate("""() => ({kind:'admin-isolation', publicWrapper:Boolean(document.querySelector('.public-site')), font:getComputedStyle(document.body).fontFamily, adminMain:Boolean(document.querySelector('.admin-main'))})"""))
            await context.close()
        await browser.close()
    if MODE == "current":
        assert all(check["document"] <= check["viewport"] and check["body"] <= check["viewport"] for check in layout_checks), "A public page overflows the 320px viewport."
        assert any(check.get("kind") == "public-font" and check["loaded"] for check in isolation_checks), "The locally hosted Manrope font did not load."
        assert any(check.get("kind") == "dark-mode" and check["accent"] == "#769dff" for check in isolation_checks), "The public dark-mode accent token is missing."
        assert any(check.get("kind") == "admin-isolation" and not check["publicWrapper"] and check["adminMain"] for check in isolation_checks), "Admin was included in the public visual wrapper."
    manifest = {"captureMode": MODE, "capturedAt": datetime.now(timezone.utc).isoformat(), "sourceCommit": commit, "tool": "Python Playwright, Google Chrome", "viewports": {"desktop": "1440x900", "mobile": "390x844", "checkpoint": "360x800", "narrow": "320x740"}, "api": "Responses are intercepted and generated from the current OpenAPI contract; no live buyer data is used.", "photoFixtures": "Existing Picsum URLs are served from docs/phase20/fixtures for repeatable captures.", "captures": captures, "layoutChecks": layout_checks, "isolationChecks": isolation_checks}
    OUTPUT.mkdir(parents=True, exist_ok=True)
    index = json.dumps(manifest, indent=2, ensure_ascii=False) + "\n"
    (OUTPUT / f"index-{MODE}.json").write_text(index)
    if MODE == "current":
        (OUTPUT / "index.json").write_text(index)
    print(f"Captured {len(captures)} {MODE} public UI screenshots in {OUTPUT}")


asyncio.run(main())
