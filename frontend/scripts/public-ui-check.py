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
SECOND_TICKET_ID = "11111111111111111111111111111111"
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
        "id": TICKET_ID, "code": "ET-0123456789ABCDEF0123456789ABCDEF", "attendeeName": "Nadia Pembeli",
        "orderReference": "TO-0123456789abcdef0123", "eventId": "nusa-malam",
        "eventArtist": "Nusa Malam", "eventCity": "Jakarta", "eventVenue": "Ruang Selatan",
        "eventAddress": "Jl. Musik Raya, Jakarta", "eventStartsAt": EVENT_START,
        "tierName": "Festival", "gate": "Gate B", "issuedAt": "2026-10-01T10:00:00Z",
    }


def second_ticket():
    return {**ticket(), "id": SECOND_TICKET_ID, "code": "ET-11111111111111111111111111111111", "attendeeName": "Raka Pembeli"}


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
        if scenario == "phase26-search":
            base = events[0]
            events = [
                base,
                {**base, "id": "jazz-mahal", "artist": "Jazz Mahal", "city": "Makassar", "genre": "Jazz", "startsAt": "2027-08-10T19:30:00+07:00", "price": 2500000},
                {**base, "id": "melodi-medan", "artist": "Melodi Medan", "city": "Medan", "genre": "Dangdut", "startsAt": "2027-09-01T19:30:00+07:00", "price": 100000},
                *[{**base, "id": f"tambahan-{index}", "artist": f"Konser Tambahan {index}", "startsAt": f"2027-10-{index:02d}T19:30:00+07:00"} for index in range(1, 7)],
            ]
        elif scenario == "phase21-search":
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
        if scenario == "phase27-share-long":
            result.update({"artist": "Musisi " * 24, "venue": "Gedung Konser Internasional " * 10, "address": "Jalan Musik & Kenangan #2, Kecamatan " * 8, "city": "Yogyakarta"})
        if scenario == "phase27-share-no-location":
            result.update({"venue": "", "address": "", "city": ""})
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
        if scenario == "phase27-partial-tickets":
            return 200, {"tickets": [ticket(), second_ticket()]}
        return 200, {"tickets": [ticket()]}
    if endpoint.startswith("/api/v1/orders/"):
        status = {"pending": "PENDING", "cancelled": "CANCELLED", "expired": "EXPIRED", "refund-pending": "REFUND_PENDING", "refund-unknown": "REFUND_PENDING", "refund-manual": "REFUND_PENDING", "refund-failed": "REFUND_PENDING", "refunded": "REFUNDED"}.get(state, "PAID")
        payment_status = "PENDING" if status == "PENDING" else "SUCCEEDED"
        event_state = "POSTPONED" if state in ("changed", "refund-pending", "refund-unknown", "refund-manual", "refund-failed", "refunded", "no-deadline") else "RESCHEDULED" if state == "past-deadline" else "SCHEDULED"
        refund_state = {"refund-unknown": "UNKNOWN", "refund-manual": "MANUAL_REQUIRED", "refund-failed": "FAILED"}.get(state)
        refund_deadline = None if state == "no-deadline" else "2020-08-30T23:59:00+07:00" if state == "past-deadline" else "2027-08-30T23:59:00+07:00"
        result = order(status, payment_status, event_state, refund_state, refund_deadline)
        if scenario == "phase27-partial-tickets":
            result["subtotal"] = 550000
            result["total"] = 555000
            result["items"] = [{"tierId": "festival", "name": "Festival", "quantity": 2, "unitPrice": 275000, "lineTotal": 550000}]
            result["attendees"] = [{"tierId": "festival", "ticketNumber": 1, "name": "Nadia Pembeli"}, {"tierId": "festival", "ticketNumber": 2, "name": "Raka Pembeli"}]
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
    if endpoint in ("/api/v1/tickets/" + TICKET_ID, "/api/v1/tickets/" + SECOND_TICKET_ID):
        result = ticket() if endpoint.endswith(TICKET_ID) else second_ticket()
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


async def capture(page, name, description, full_page=True):
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
    await page.screenshot(path=str(OUTPUT / filename), full_page=full_page, animations="disabled")
    return {"file": filename, "capture": description, "title": await page.title()}


async def capture_admin(browser, suffix):
    token = "s" * 43
    admin = {"id": "admin-fixture", "name": "Admin Uji", "email": "admin@example.test", "role": "ADMIN", "active": True, "assignments": []}
    staff = {"id": "staff-fixture", "name": "Petugas Uji", "email": "staff@example.test", "role": "STAFF", "active": True, "assignments": [{"eventId": "fixture-event", "gate": "Gate A"}]}
    for viewport_name, viewport in (("desktop", {"width": 1440, "height": 900}), ("mobile", {"width": 390, "height": 844})):
        login_context = await browser.new_context(viewport=viewport, device_scale_factor=1)
        login_page = await login_context.new_page()
        await login_page.goto(f"{BASE_URL}/admin/login", wait_until="domcontentloaded")
        await login_page.wait_for_timeout(250)
        await login_page.screenshot(path=str(OUTPUT / f"admin-{suffix}-{viewport_name}-login.png"), full_page=True, animations="disabled")
        await login_context.close()

        context = await browser.new_context(viewport=viewport, device_scale_factor=1)
        await context.add_init_script(f"sessionStorage.setItem('ticket-online:staff-session', JSON.stringify({json.dumps({'accessToken': token, 'expiresAt': '2099-01-01T00:00:00.000Z'})}));")
        page = await context.new_page()

        async def admin_api(route):
            path = urlparse(route.request.url).path
            if path.endswith("/staff/me"):
                payload = {"staff": staff if "/admin/scan" in page.url else admin}
            elif path.endswith("/admin/staff"):
                payload = {"staff": []}
            elif path.endswith("/events"):
                payload = {"events": []}
            else:
                payload = {}
            await route.fulfill(status=200, content_type="application/json", body=json.dumps(payload))

        await context.route("**/api/v1/**", admin_api)
        for route, slug in (("/admin/staff", "staff"), ("/admin/scan", "scan")):
            await page.goto(f"{BASE_URL}{route}", wait_until="domcontentloaded")
            await page.wait_for_timeout(350)
            await page.screenshot(path=str(OUTPUT / f"admin-{suffix}-{viewport_name}-{slug}.png"), full_page=True, animations="disabled")
        await context.close()


async def main():
    global MODE, OUTPUT, BASE_URL
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--current", action="store_true", help="Capture with the current UI instead of baseline tokens.")
    parser.add_argument("--phase25", action="store_true", help="Run the responsive, text-zoom, target-size, and contrast matrix.")
    parser.add_argument("--admin-before", action="store_true", help="Capture login, staff, and scanner screens before public fixes.")
    parser.add_argument("--admin-after", action="store_true", help="Capture login, staff, and scanner screens after public fixes.")
    parser.add_argument("--output-dir", type=Path, help="Write captures to a directory separate from the Phase 20 baseline.")
    parser.add_argument("--base-url", default=BASE_URL, help="Use a running local UI server at this URL.")
    args = parser.parse_args()
    BASE_URL = args.base_url.rstrip("/")
    MODE = "current" if args.current else "baseline"
    if args.admin_before:
        MODE = "admin-before"
    elif args.admin_after:
        MODE = "admin-after"
    if args.output_dir:
        OUTPUT = args.output_dir if args.output_dir.is_absolute() else ROOT / args.output_dir
    commit = subprocess.run(["git", "rev-parse", "--short", "HEAD"], cwd=ROOT, check=True, capture_output=True, text=True).stdout.strip()
    captures = []
    layout_checks = []
    responsive_checks = []
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
        browser = await playwright.chromium.launch(channel="chrome", headless=True, ignore_default_args=["--disable-back-forward-cache"])
        if args.admin_before or args.admin_after:
            await capture_admin(browser, "before" if args.admin_before else "after")
            await browser.close()
            print(f"Captured six admin comparison screenshots in {OUTPUT}")
            return
        for viewport_name, viewport in (("desktop", {"width": 1440, "height": 900}), ("mobile", {"width": 390, "height": 844})):
            context = await browser.new_context(viewport=viewport, device_scale_factor=1)

            async def api(route):
                url = urlparse(route.request.url)
                page_params = dict(parse_qsl(urlparse(page.url).query))
                scenario = page_params.get("scenario", "standard")
                if scenario == "phase27-partial-tickets" and url.path.startswith("/api/v1/orders/") and url.path.endswith("/tickets"):
                    request_count = getattr(api, "phase27_ticket_requests", 0) + 1
                    api.phase27_ticket_requests = request_count
                    if request_count == 1:
                        await route.fulfill(status=503, content_type="application/json", body=json.dumps({"error": {"code": "SERVICE_UNAVAILABLE", "message": "Daftar tiket belum dapat dimuat."}}))
                        return
                    tickets = [ticket()] if request_count == 2 else [ticket(), second_ticket()]
                    await route.fulfill(status=200, content_type="application/json", body=json.dumps({"tickets": tickets}))
                    return
                if scenario == "phase27-detail-retry" and url.path == "/api/v1/events/nusa-malam":
                    request_count = getattr(api, "phase27_detail_requests", 0) + 1
                    api.phase27_detail_requests = request_count
                    if request_count == 1:
                        await route.fulfill(status=503, content_type="application/json", body=json.dumps({"error": {"code": "SERVICE_UNAVAILABLE", "message": "Layanan sedang tidak tersedia."}}))
                        return
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
            await context.add_init_script("""Object.defineProperty(Navigator.prototype, 'share', { configurable:true, get(){const scenario=new URL(location.href).searchParams.get('scenario');if(!scenario?.startsWith('phase27-share'))return undefined;return async payload=>{window.__sharedPayload=payload;if(scenario==='phase27-share-cancel')throw new DOMException('Canceled','AbortError');if(scenario==='phase27-share-fail')throw new Error('Share unavailable')}}});Object.defineProperty(Navigator.prototype, 'clipboard', { configurable:true, get(){const scenario=new URL(location.href).searchParams.get('scenario');if(!scenario?.startsWith('phase27-copy'))return undefined;if(scenario==='phase27-copy-unavailable')return undefined;return{writeText:async value=>{if(scenario==='phase27-copy-fail')throw new Error('Clipboard unavailable');window.__copiedText=value}}}});""")
            page = await context.new_page()
            for route_name, url, label in routes:
                await page.goto(BASE_URL + url, wait_until="domcontentloaded")
                await page.wait_for_timeout(350)
                if route_name == "order":
                    assert await page.locator(".order-status").is_visible(), "The payment or order status should lead the order page."
                    assert await page.locator(".order-ticket-section").evaluate("element => element.compareDocumentPosition(document.querySelector('#refund-right-title').closest('.recovery-panel')) & Node.DOCUMENT_POSITION_FOLLOWING"), "Tickets should appear before refund details."
                    assert await page.locator(".order-breakdown").evaluate("element => element.compareDocumentPosition(document.querySelector('#refund-right-title').closest('.recovery-panel')) & Node.DOCUMENT_POSITION_PRECEDING"), "Order item details should follow refund information."
                if route_name == "my-tickets":
                    assert await page.get_by_role("link", name="Sudah membeli, tetapi tiket tidak muncul?").is_visible(), "Ticket recovery should remain easy to find above the ticket list."
                    assert await page.locator(".my-ticket-card").count() == 1, "A saved order should render a concise ticket card."
                    footer_visibility = await page.locator(".site-footer").evaluate("element => getComputedStyle(element).visibility")
                    assert footer_visibility == "visible", f"The footer should remain visible with an empty live region; got {footer_visibility}."
                    await page.locator(".site-footer a").first.focus()
                    assert await page.evaluate("document.activeElement.closest('.site-footer') !== null"), "Footer links should remain focusable after My Tickets loads."
                if route_name == "order":
                    footer_visibility = await page.locator(".site-footer").evaluate("element => getComputedStyle(element).visibility")
                    assert footer_visibility == "visible", f"The footer should remain visible with an order status live region; got {footer_visibility}."
                if route_name == "ticket":
                    assert await page.locator(".pass-details").evaluate("element => !element.open"), "Secondary ticket details should start collapsed."
                    assert await page.locator(".qr-code").is_visible(), "The usable ticket should show its QR code."
                    assert await page.locator(".pass-row").filter(has=page.get_by_text("Nadia Pembeli")).is_visible(), "Attendee details should remain visible beside the ticket QR."
                    await page.evaluate("window.dispatchEvent(new Event('beforeprint'))")
                    assert await page.locator(".pass-details").evaluate("element => element.open"), "Ticket details should open for printing."
                    assert await page.get_by_text("Jl. Musik Raya, Jakarta").is_visible(), "The printed ticket should include the event address."
                    await page.evaluate("window.dispatchEvent(new Event('afterprint'))")
                    assert await page.locator(".pass-details").evaluate("element => !element.open"), "The on-screen ticket should restore the details state after printing."
                if route_name == "guide":
                    guide_copy = await page.locator("main").inner_text()
                    assert "simulasi development" not in guide_copy and "Kode belum mendukung pemindaian QR" not in guide_copy, "Guide copy should reflect the current payment and ticket flow."
                    await page.locator("#cara-kerja").get_by_role("button").first.click()
                    assert await page.locator("#cara-kerja").get_by_role("button").first.get_attribute("aria-expanded") == "true", "Guide accordions should open with keyboard-accessible controls."
                    assert await page.locator("#cara-kerja").get_by_role("region").count() == 1, "Expanded guide answers should be available to assistive technology."
                if route_name == "recovery":
                    await page.get_by_role("button", name="Kirim tautan pemulihan").click()
                    assert await page.locator("#recovery-email").get_attribute("aria-invalid") == "true", "Recovery validation should identify a missing buyer email."
                    assert await page.locator("#recovery-reference").get_attribute("aria-invalid") == "true", "Recovery validation should identify a missing order reference."
                if route_name == "catalog" and viewport_name == "desktop":
                    await page.wait_for_function("document.querySelector('.filters')?.open === true")
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
            if viewport_name == "mobile":
                await page.goto(f"{BASE_URL}/pesanan/{ORDER_ID}?scenario=phase27-partial-tickets", wait_until="domcontentloaded")
                await page.locator(".order-ticket-section").get_by_role("button", name="Muat ulang tiket").wait_for()
                assert await page.locator(".my-order-card").count() == 0, "The first ticket-list error should leave the order's ticket list empty."
                await page.locator(".order-ticket-section").get_by_role("button", name="Muat ulang tiket").click()
                await page.locator(".my-order-card").wait_for()
                assert await page.locator(".my-order-card").count() == 1, "A partial retry should keep its successfully loaded ticket visible."
                assert await page.locator(".order-ticket-section").get_by_role("alert").is_visible(), "A partial retry should expose its error beside the loaded ticket."
                assert await page.locator(".order-ticket-section").get_by_role("button", name="Muat ulang tiket").is_visible(), "A partial retry should remain available."
                await page.locator(".order-ticket-section").get_by_role("button", name="Muat ulang tiket").click()
                await page.wait_for_function("document.querySelectorAll('.my-order-card').length === 2")
                assert await page.locator(".order-ticket-section").get_by_role("alert").count() == 0, "A complete retry should clear the ticket-list error."
                assert await page.locator(".order-ticket-section").get_by_role("button", name="Muat ulang tiket").count() == 0, "A complete retry should remove the reload action."
                await page.locator(".my-order-card").first.get_by_role("link", name="Buka e-ticket").click()
                await page.wait_for_url(f"**/tiket/{TICKET_ID}")
                for width in (1440, 390):
                    await page.set_viewport_size({"width": width, "height": 844})
                    api.phase27_detail_requests = 0
                    await page.goto(f"{BASE_URL}/konser/nusa-malam?scenario=phase27-detail-retry", wait_until="domcontentloaded")
                    await page.get_by_role("button", name="Coba lagi").wait_for()
                    await page.get_by_role("button", name="Coba lagi").click()
                    await page.get_by_role("heading", name="Nusa Malam").wait_for()
                    details = page.locator(".zone-details")
                    expected_open = width >= 768
                    assert await details.evaluate("element => element.open") == expected_open, f"Detail retry should initialize the area panel at {width}px."
                    if width == 1440:
                        await details.locator("summary").click()
                        await page.get_by_role("button", name="Tambah tiket VIP A").click()
                        assert not await details.evaluate("element => element.open"), "Changing ticket quantity should not reopen a manually closed desktop area panel."
                        await page.set_viewport_size({"width": 390, "height": 844})
                        await page.wait_for_function("!document.querySelector('.zone-details').open")
                    else:
                        await details.locator("summary").click()
                        await page.get_by_role("button", name="Tambah tiket VIP A").click()
                        assert await details.evaluate("element => element.open"), "Changing ticket quantity should not close a manually opened mobile area panel."
                        await page.wait_for_function("""() => {const root=document.documentElement,cart=document.querySelector('.mobile-cart');return parseInt(root.style.getPropertyValue('--detail-mobile-cart-height'))===Math.ceil(cart.getBoundingClientRect().height)&&Boolean(root.style.scrollPaddingBottom)}""")
                        await page.locator(".mobile-cart").evaluate("element => element.style.fontSize = '24px'")
                        await page.wait_for_function("""() => parseInt(document.documentElement.style.getPropertyValue('--detail-mobile-cart-height'))===Math.ceil(document.querySelector('.mobile-cart').getBoundingClientRect().height)""")
                        assert await page.evaluate("document.documentElement.style.scrollPaddingBottom.length > 0"), "Mobile cart measurement should install scroll padding after detail retry."
                        await page.locator(".mobile-cart").evaluate("element => element.style.fontSize = ''")
            for state in ("changed", "inactive"):
                await page.goto(f"{BASE_URL}/tiket/{TICKET_ID}?snapshot={state}", wait_until="domcontentloaded")
                await page.wait_for_timeout(350)
                if state == "inactive":
                    assert await page.locator(".qr-code").count() == 0, "Backend-inactive tickets should not render a usable QR."
                    assert await page.get_by_text("Tidak aktif", exact=True).first.is_visible(), "Backend-inactive tickets should show their status."
                if state == "changed":
                    assert await page.get_by_role("status", name="Informasi terbaru acara").is_visible(), "Event changes should remain visible on the e-ticket."
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
                    assert await page.locator(".order-summary").evaluate("summary => Boolean(summary.compareDocumentPosition(document.querySelector('.checkout-panel')) & Node.DOCUMENT_POSITION_FOLLOWING)"), "The mobile reading and tab order should place the cost summary before checkout confirmation."
                    if viewport_name == "desktop":
                        desktop_columns = await page.evaluate("""() => {const summary=document.querySelector('.order-summary').getBoundingClientRect(),panel=document.querySelector('.checkout-panel').getBoundingClientRect();return summary.left>panel.left&&Math.abs(summary.top-panel.top)<2}""")
                        assert desktop_columns, "The checkout summary should remain in the right desktop column."
                    else:
                        for width in (320, 360, 390):
                            await page.set_viewport_size({"width": width, "height": 844})
                            positions = await page.evaluate("""() => {const costs=document.querySelector('.summary-breakdown').getBoundingClientRect(),terms=document.querySelector('.terms-trigger').getBoundingClientRect(),button=document.querySelector('.step-actions').getBoundingClientRect();return{costsBottom:costs.bottom,termsTop:terms.top,buttonTop:button.top}}""")
                            assert positions["costsBottom"] <= positions["termsTop"] <= positions["buttonTop"], f"Mobile cost details should precede approval and payment at {width}px: {positions}."
                        await page.set_viewport_size({"width": 390, "height": 844})
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
                assert await page.locator(".results-count").inner_text() == "5 konser ditemukan, menampilkan 4 pertama", "The home result count should describe all matches while showing only four cards."
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

                await page.set_viewport_size({"width": 390, "height": 844})
                await page.goto(BASE_URL + "/konser/nusa-malam?scenario=phase27-share&festival=2&token=private#TO-secret", wait_until="domcontentloaded")
                await page.get_by_role("heading", name="Nusa Malam").wait_for()
                assert await page.locator(".detail-actions .detail-action").all_text_contents() == ["Bagikan konser", "Buka lokasi ↗"], "Share and location should appear as secondary actions below the concert summary."
                assert await page.locator(".mobile-cart").is_visible(), "Secondary actions should leave the mobile purchase bar visible."
                assert await page.locator("#tier-festival .tier-controls output").inner_text() == "2", "The checkout quantity should stay selected while using secondary actions."
                map_link = page.get_by_role("link", name="Buka lokasi")
                map_url = urlparse(await map_link.get_attribute("href"))
                map_query = dict(parse_qsl(map_url.query))
                assert map_url.netloc == "www.google.com" and map_url.path == "/maps/search/" and map_query.get("api") == "1", "The location action should open a Google Maps search in a new tab."
                assert map_query.get("query") == "Ruang Selatan, Jl. Musik Raya, Jakarta", "The map search should contain only the public location fields."
                assert await map_link.get_attribute("target") == "_blank" and {"noopener", "noreferrer"} <= set((await map_link.get_attribute("rel")).split()), "External maps links should open safely in another tab."
                await page.get_by_role("button", name="Bagikan konser").click()
                payload = await page.evaluate("window.__sharedPayload")
                assert payload == {"title": "Nusa Malam", "url": BASE_URL + "/konser/nusa-malam"}, "Sharing should use the clean public URL without query, fragment, token, or cart values."

                await page.goto(BASE_URL + "/konser/nusa-malam?scenario=phase27-share-cancel", wait_until="domcontentloaded")
                await page.get_by_role("button", name="Bagikan konser").click()
                assert await page.locator(".detail-share-feedback").count() == 0, "Canceling native share should not report an error or trigger a fallback."
                await page.goto(BASE_URL + "/konser/nusa-malam?scenario=phase27-share-fail", wait_until="domcontentloaded")
                await page.get_by_role("button", name="Bagikan konser").click()
                await page.get_by_role("alert").filter(has_text="Berbagi tidak tersedia").wait_for()
                assert await page.get_by_role("button", name="Salin tautan konser").is_visible(), "A failed native share should offer a separate copy action."
                await page.get_by_role("button", name="Salin tautan konser").click()
                await page.get_by_role("textbox", name="Tautan konser").wait_for()
                assert await page.locator(".detail-manual-link input").evaluate("element => element === document.activeElement && element.selectionStart === 0 && element.selectionEnd === element.value.length"), "Missing clipboard access should select the clean URL for manual copying."

                await page.goto(BASE_URL + "/konser/nusa-malam?scenario=phase27-copy", wait_until="domcontentloaded")
                await page.get_by_role("button", name="Salin tautan konser").click()
                assert await page.evaluate("window.__copiedText") == BASE_URL + "/konser/nusa-malam", "The clipboard fallback should copy only the public concert URL."
                await page.get_by_role("status").filter(has_text="Tautan konser disalin").wait_for()
                await page.goto(BASE_URL + "/konser/nusa-malam?scenario=phase27-copy-fail", wait_until="domcontentloaded")
                await page.get_by_role("button", name="Salin tautan konser").click()
                await page.get_by_role("textbox", name="Tautan konser").wait_for()
                await page.goto(BASE_URL + "/konser/nusa-malam?scenario=phase27-copy-unavailable", wait_until="domcontentloaded")
                await page.get_by_role("button", name="Salin tautan konser").click()
                await page.get_by_role("textbox", name="Tautan konser").wait_for()

                await page.set_viewport_size({"width": 320, "height": 740})
                await page.goto(BASE_URL + "/konser/nusa-malam?scenario=phase27-share-long", wait_until="domcontentloaded")
                await page.get_by_role("heading", name="Musisi").wait_for()
                await page.locator(".detail-info").filter(has_text="Lokasi").locator("summary").click()
                assert await page.evaluate("document.documentElement.scrollWidth <= innerWidth"), "Long concert and location names should wrap without horizontal overflow at 320px."
                await page.goto(BASE_URL + "/konser/nusa-malam?scenario=phase27-share-no-location", wait_until="domcontentloaded")
                await page.locator(".detail-heading").wait_for(state="visible")
                assert await page.locator(".detail-location-unavailable").is_visible() and await page.locator(".detail-actions-row .detail-action[href]").count() == 0, "Missing API location details should not create an empty map link."

                await page.set_viewport_size({"width": 390, "height": 844})
                await page.evaluate("sessionStorage.removeItem('ticket-online:catalog:v1')")
                await page.goto(BASE_URL + "/konser?scenario=phase26-search", wait_until="domcontentloaded")
                await page.get_by_role("heading", name="Jazz Mahal").wait_for()
                assert await page.locator(".concert-card").count() == 9, "Every catalog event should be visible before the user applies filters."
                assert await page.locator("#concert-city option").all_text_contents() == ["Semua kota", "Jakarta", "Makassar", "Medan"], "City filters should follow unique API catalog values."
                assert await page.locator(".genre-chips span").all_text_contents() == ["Dangdut", "Indie", "Jazz"], "Genre filters should follow unique API catalog values."
                assert (await page.locator(".concert-card h3").all_text_contents())[0] == "Jazz Mahal", "The default catalog order should use the nearest event date."
                await page.locator(".filters summary").click()
                await page.locator("#concert-sort").select_option("price")
                assert (await page.locator(".concert-card h3").all_text_contents())[0] == "Melodi Medan", "Price ordering should use the loaded ticket prices."
                await page.get_by_label("Batasi harga").check()
                assert await page.locator("#concert-price").input_value() == "2500000", "Price controls should initialize from the catalog maximum."
                await page.locator("#concert-price").fill("200000")
                assert await page.locator(".concert-card h3").all_text_contents() == ["Melodi Medan"], "An active maximum price should include all matching catalog events."
                await page.locator("#concert-city").select_option(label="Medan")
                await page.locator("#concert-query").fill("Melodi")
                assert await page.locator(".filter-chip").count() == 3, "Each active catalog filter should have its own removable chip."
                await page.evaluate("window.__catalogRestoredFromBFCache=false;addEventListener('pageshow',event=>window.__catalogRestoredFromBFCache=event.persisted)")
                await page.evaluate("window.scrollTo(0, document.documentElement.scrollHeight)")
                await page.wait_for_timeout(100)
                saved_metrics = await page.evaluate("({y:window.scrollY,height:document.documentElement.scrollHeight,cards:document.querySelectorAll('.concert-card').length,session:sessionStorage.getItem('ticket-online:catalog:v1')})")
                await page.evaluate("addEventListener('unload',()=>{})")
                await page.locator(".concert-card-link").first.click()
                await page.wait_for_url("**/konser/melodi-medan")
                await page.go_back(wait_until="commit")
                assert not await page.evaluate("window.__catalogRestoredFromBFCache"), "This return should use a fresh document to verify session restoration without BFCache."
                await page.wait_for_function("target => Math.abs(window.scrollY - target) < 100", arg=saved_metrics["y"])
                assert await page.locator("#concert-query").input_value() == "Melodi", "Catalog search should survive returning from detail."
                restored_metrics = await page.evaluate("({y:window.scrollY,height:document.documentElement.scrollHeight,cards:document.querySelectorAll('.concert-card').length,session:sessionStorage.getItem('ticket-online:catalog:v1')})")
                assert abs(restored_metrics["y"] - saved_metrics["y"]) < 100, f"Catalog scroll position should be restored after returning from detail: saved {saved_metrics}, restored {restored_metrics}."
                assert await page.locator(".filter-chip").count() == 3, "Catalog filters should survive returning from detail."
                await page.locator(".filter-chip").filter(has_text="Kota: Medan").click()
                assert await page.locator(".filter-chip").count() == 2, "A filter chip should remove only its own filter."

                await page.evaluate("sessionStorage.removeItem('ticket-online:catalog:v1')")
                bfcache_context = await browser.new_context(viewport={"width": 390, "height": 844}, device_scale_factor=1)
                bfcache_events = reply(urlparse("/api/v1/events?scenario=phase26-search"), "phase26-search")[1]["events"]
                for fixture_event in bfcache_events:
                    fixture_event["image"] = "data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 4 5'%3E%3Crect width='4' height='5' fill='%231358d8'/%3E%3C/svg%3E"
                await bfcache_context.add_init_script(f"""const fixtureEvents={json.dumps(bfcache_events)};const nativeFetch=window.fetch.bind(window);window.fetch=(input,init)=>{{const url=new URL(typeof input==='string'?input:input.url,location.href);if(url.pathname==='/api/v1/events')return Promise.resolve(new Response(JSON.stringify({{events:fixtureEvents}}),{{status:200,headers:{{'Content-Type':'application/json'}}}}));if(url.pathname.startsWith('/api/v1/events/')){{const item=fixtureEvents.find(event=>event.id===decodeURIComponent(url.pathname.split('/').pop()));return Promise.resolve(new Response(JSON.stringify(item),{{status:item?200:404,headers:{{'Content-Type':'application/json'}}}}))}}return nativeFetch(input,init)}}""")
                bfcache_page = await bfcache_context.new_page()
                await bfcache_page.goto(BASE_URL + "/konser?scenario=phase26-search", wait_until="domcontentloaded")
                await bfcache_page.locator(".concert-card").nth(8).wait_for(state="visible")
                await bfcache_page.evaluate("window.scrollTo(0, 500)")
                await bfcache_page.wait_for_function("window.scrollY > 100")
                await bfcache_page.evaluate("window.__catalogPagehideY=null;window.__catalogPageshowPersisted=false;addEventListener('pagehide',()=>window.__catalogPagehideY=window.scrollY);addEventListener('pageshow',event=>{if(event.persisted)window.__catalogPageshowPersisted=true})")
                await bfcache_page.locator(".concert-card-link").nth(4).click()
                await bfcache_page.wait_for_url("**/konser/**")
                await bfcache_page.go_back(wait_until="commit")
                await bfcache_page.wait_for_function("window.__catalogPageshowPersisted === true")
                bfcache_metrics = await bfcache_page.evaluate("({saved:window.__catalogPagehideY,restored:window.scrollY})")
                assert bfcache_metrics["saved"] > 100 and abs(bfcache_metrics["restored"] - bfcache_metrics["saved"]) < 100, f"BFCache should restore a nonzero position without a preexisting catalog session: {bfcache_metrics}."
                await bfcache_context.close()
                await page.goto(BASE_URL + "/konser", wait_until="domcontentloaded")
                await page.locator("#concert-query").fill("Nusa")
                await page.locator("#concert-query").press("Enter")
                if not await page.locator(".filters").evaluate("element => element.open"):
                    await page.locator(".filters summary").click()
                await page.locator("#concert-city").select_option(label="Jakarta")
                if await page.locator(".filters").evaluate("element => element.open"):
                    await page.locator(".filters summary").click()
                assert "Jakarta" in await page.locator(".filters summary").inner_text(), "The closed filter summary should show the active city."
                await page.locator(".reset-filters").click()
                assert await page.locator("#concert-city").input_value() == "", "Reset should clear the city filter."
                assert await page.locator("#concert-query").input_value() == "", "Reset should clear the catalog search."
                assert await page.evaluate("document.documentElement.scrollWidth <= innerWidth"), "The catalog should not overflow at 360px."
                captures.append(await capture(page, "mobile360-katalog", "mobile 360×800, katalog"))

                for width in (768, 844, 900, 1024, 1440):
                    await page.set_viewport_size({"width": width, "height": 390 if width == 844 else 900})
                    if not await page.locator(".filters").evaluate("element => element.open"):
                        await page.locator(".filters summary").click()
                    await page.get_by_label("Batasi harga").check()
                    filter_bounds = await page.evaluate("""() => ({viewport:innerWidth,document:document.documentElement.scrollWidth,body:document.body.scrollWidth,fields:document.querySelector('.filter-fields').getBoundingClientRect().toJSON(),reset:document.querySelector('.reset-filters').getBoundingClientRect().toJSON(),controls:[...document.querySelectorAll('.filter-fields select,.filter-fields input[type=text]')].filter(el=>el.getClientRects().length).map(el=>el.getBoundingClientRect().toJSON())})""")
                    assert filter_bounds["document"] <= width, f"Open catalog filters should not create horizontal overflow at {width}px: {filter_bounds}."
                    assert all(control["left"] >= 0 and control["right"] <= width for control in filter_bounds["controls"]), f"Filter controls should remain within the viewport at {width}px: {filter_bounds}."
                    assert filter_bounds["reset"]["left"] >= filter_bounds["fields"]["right"] or filter_bounds["reset"]["right"] <= filter_bounds["fields"]["left"], f"Reset should not overlap the open filter panel at {width}px: {filter_bounds}."
                    await page.get_by_label("Batasi harga").uncheck()
                    await page.locator(".filters summary").click()
                    layout_checks.append({"route": "catalog-open-filters", "width": width, **filter_bounds})

                widths = (320, 360, 390, 768, 1024, 1440) if args.phase25 else (320,)
                for width in widths:
                    await page.set_viewport_size({"width": width, "height": 740 if width <= 390 else 900})
                    for route_name, url, label in routes:
                        await page.goto(BASE_URL + url, wait_until="domcontentloaded")
                        await page.wait_for_timeout(100)
                        metrics = await page.evaluate("""() => ({viewport:innerWidth,document:document.documentElement.scrollWidth,body:document.body.scrollWidth,font:getComputedStyle(document.querySelector('.public-site')).fontFamily,targets:[...document.querySelectorAll('.public-site button:not(:disabled),.public-site a,.public-site summary,.public-site select,.public-site input:not([type=hidden]):not([type=checkbox]):not([type=radio]),.public-site textarea,[role=button]')].filter(el=>el.getClientRects().length).map(el=>{const r=el.getBoundingClientRect();return{name:(el.getAttribute('aria-label')||el.innerText||el.getAttribute('type')||el.tagName).trim().slice(0,42),tag:el.tagName,classes:el.className,width:Math.round(r.width),height:Math.round(r.height),font:parseFloat(getComputedStyle(el).fontSize)}}),images:[...document.querySelectorAll('.public-site .concert-poster,.public-site .detail-poster')].map(el=>({width:el.getAttribute('width'),height:el.getAttribute('height'),loading:el.getAttribute('loading'),fetchpriority:el.getAttribute('fetchpriority')}))})""")
                        layout_checks.append({"route": label, **metrics})
                        if args.phase25 and width <= 390:
                            small = [target for target in metrics["targets"] if target["width"] < 44 or target["height"] < 44]
                            small_inputs = [target for target in metrics["targets"] if target["font"] < 16 and target["tag"] in ("INPUT", "SELECT", "TEXTAREA")]
                            responsive_checks.append({"route": label, "width": width, "smallTargets": small[:15], "smallTargetCount": len(small), "smallInputs": small_inputs, "imageDimensionsMissing": [img for img in metrics["images"] if not img["width"] or not img["height"]]})
                        if args.phase25:
                            await capture(page, f"phase25-{width}-{label}", f"Phase 25, viewport {width}px, {label}")

                if args.phase25:
                    for width, height, mode in ((844, 390, "landscape"), (390, 844, "text-zoom")):
                        await page.set_viewport_size({"width": width, "height": height})
                        for route_name, url, label in routes:
                            await page.goto(BASE_URL + url, wait_until="domcontentloaded")
                            await page.wait_for_timeout(100)
                            if mode == "text-zoom":
                                await page.evaluate("""() => { const elements=[...document.querySelectorAll('.public-site *')]; const sizes=elements.map(el=>el.getClientRects().length?parseFloat(getComputedStyle(el).fontSize):0); elements.forEach((el,index)=>{if(sizes[index])el.style.setProperty('font-size',`${sizes[index]*2}px`,'important')}); }""")
                            metrics = await page.evaluate("""() => ({viewport:innerWidth,document:document.documentElement.scrollWidth,body:document.body.scrollWidth,offenders:[...document.querySelectorAll('.public-site *')].map(el=>{const r=el.getBoundingClientRect();return{tag:el.tagName,classes:typeof el.className==='string'?el.className:'',text:(el.innerText||'').slice(0,50),right:Math.round(r.right),width:Math.round(r.width),scroll:el.scrollWidth,client:el.clientWidth}}).filter(el=>el.right>innerWidth||el.scroll>el.client+2).sort((a,b)=>b.right-a.right).slice(0,12)})""")
                            responsive_checks.append({"route": label, "mode": mode, **metrics})
                            if label in ("beranda", "detail-konser", "checkout-data", "pesanan", "tiket-saya", "e-ticket", "pemulihan"):
                                await capture(page, f"phase25-{mode}-{label}", f"Phase 25, {mode}, {label}", full_page=mode != "text-zoom")
                    for scheme in ("light", "dark"):
                        await page.emulate_media(color_scheme=scheme)
                        await page.goto(BASE_URL + "/", wait_until="domcontentloaded")
                        ratios = await page.evaluate("""() => {const root=document.querySelector('.public-site'),s=getComputedStyle(root);const rgb=v=>{const a=v.match(/[\\d.]+/g)?.map(Number);return a?.length>=3?a.slice(0,3).map(n=>{n/=255;return n<=.04045?n/12.92:((n+.055)/1.055)**2.4}):null};const color=n=>{const e=document.createElement('span');e.style.color=`var(${n})`;root.append(e);const value=getComputedStyle(e).color;e.remove();return value};const contrast=(a,b)=>{a=rgb(color(a));b=rgb(color(b));if(!a||!b)return null;const lum=x=>.2126*x[0]+.7152*x[1]+.0722*x[2];const q=[lum(a),lum(b)].sort((x,y)=>y-x);return Number(((q[0]+.05)/(q[1]+.05)).toFixed(2))};return{canvas:contrast('--ink','--canvas'),muted:contrast('--muted','--canvas'),surface:contrast('--ink','--surface'),accent:contrast('--accent-ink','--accent'),control:contrast('--control-line','--surface')}}""")
                        responsive_checks.append({"mode": f"contrast-{scheme}", **ratios})
                await page.goto(BASE_URL + "/panduan", wait_until="domcontentloaded")
                await page.evaluate("document.fonts.ready")
                isolation_checks.append(await page.evaluate("""() => ({kind:'public-font', loaded:document.fonts.check('400 16px Manrope'), family:getComputedStyle(document.querySelector('.public-site')).fontFamily})"""))
                await page.emulate_media(color_scheme="dark")
                isolation_checks.append(await page.evaluate("""() => ({kind:'dark-mode', accent:getComputedStyle(document.querySelector('.public-site')).getPropertyValue('--accent').trim()})"""))
                await page.emulate_media(reduced_motion="reduce")
                reduced_motion = await page.evaluate("""() => {const style=getComputedStyle(document.querySelector('.public-site'));const durations=[style.transitionDuration,style.animationDuration].flatMap(value=>value.split(',')).map(value=>parseFloat(value));return{kind:'reduced-motion',durations}}""")
                isolation_checks.append(reduced_motion)
                assert all(duration <= 0.001 for duration in reduced_motion["durations"]), f"Reduced motion should shorten public transitions and animations: {reduced_motion}"
                await page.goto(BASE_URL + "/admin/login", wait_until="domcontentloaded")
                isolation_checks.append(await page.evaluate("""() => ({kind:'admin-isolation', publicWrapper:Boolean(document.querySelector('.public-site')), font:getComputedStyle(document.body).fontFamily, adminMain:Boolean(document.querySelector('.admin-main'))})"""))
            await context.close()
        await browser.close()
    if MODE == "current":
        overflowing = [check for check in layout_checks if check["document"] > check["viewport"] or check["body"] > check["viewport"]]
        assert not overflowing, f"Public pages overflow the 320px viewport: {overflowing}"
        assert any(check.get("kind") == "public-font" and check["loaded"] for check in isolation_checks), "The locally hosted Manrope font did not load."
        assert any(check.get("kind") == "dark-mode" and check["accent"] == "#769dff" for check in isolation_checks), "The public dark-mode accent token is missing."
        assert any(check.get("kind") == "admin-isolation" and not check["publicWrapper"] and check["adminMain"] for check in isolation_checks), "Admin was included in the public visual wrapper."
        if args.phase25:
            overflow = [check for check in layout_checks if check["document"] > check["viewport"] or check["body"] > check["viewport"]]
            contrast_failures = [check for check in responsive_checks if check.get("mode", "").startswith("contrast-") and any(value is None or value < (3 if key == "control" else 4.5) for key, value in check.items() if key != "mode")]
            target_failures = [check for check in responsive_checks if check.get("smallTargetCount", 0)]
            input_failures = [check for check in responsive_checks if check.get("smallInputs")]
            responsive_overflow = [check for check in responsive_checks if check.get("mode") in ("landscape", "text-zoom") and (check["document"] > check["viewport"] or check["body"] > check["viewport"])]
            assert not overflow, f"Responsive matrix overflow: {overflow}"
            assert not contrast_failures, f"Public token contrast fell below WCAG thresholds: {contrast_failures}"
            assert not target_failures, f"Public touch targets fell below 44px: {target_failures}"
            assert not input_failures, f"Public input text fell below 16px: {input_failures}"
            assert not responsive_overflow, f"Public pages overflow in responsive mode: {responsive_overflow}"
    manifest = {"captureMode": MODE, "phase25": args.phase25, "capturedAt": datetime.now(timezone.utc).isoformat(), "sourceCommit": commit, "tool": "Python Playwright, Google Chrome", "viewports": {"desktop": "1440x900", "mobile": "390x844", "checkpoint": "360x800", "narrow": "320x740", "phase25": [320, 360, 390, 768, 1024, 1440, "844x390 landscape", "200% text simulation"]}, "api": "Responses are intercepted and generated from the current OpenAPI contract; no live buyer data is used.", "photoFixtures": "Existing Picsum URLs are served from docs/phase20/fixtures for repeatable captures.", "captures": captures, "layoutChecks": layout_checks, "responsiveChecks": responsive_checks, "isolationChecks": isolation_checks}
    OUTPUT.mkdir(parents=True, exist_ok=True)
    index = json.dumps(manifest, indent=2, ensure_ascii=False) + "\n"
    (OUTPUT / f"index-{MODE}.json").write_text(index)
    if MODE == "current":
        (OUTPUT / "index.json").write_text(index)
    print(f"Captured {len(captures)} {MODE} public UI screenshots in {OUTPUT}")


asyncio.run(main())
