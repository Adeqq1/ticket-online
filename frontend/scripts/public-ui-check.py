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


def order(status="PAID", payment_status="SUCCEEDED", event_state="SCHEDULED"):
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
        result["refundRight"] = {"requested": status in ("REFUND_PENDING", "REFUNDED"), "deadline": "2027-08-30T23:59:00+07:00"}
    if status in ("REFUND_PENDING", "REFUNDED"):
        result["refund"] = {"status": "PROCESSING" if status == "REFUND_PENDING" else "SUCCEEDED", "amount": 280000}
    return result


def reservation():
    return {"id": RESERVATION_ID, "status": "ACTIVE", "expiresAt": EXPIRY,
            "event": {"id": "nusa-malam", "artist": "Nusa Malam"},
            "items": [{"tierId": "festival", "name": "Festival", "quantity": 1, "unitPrice": 275000, "lineTotal": 275000}], "subtotal": 275000}


def reply(path, scenario, page_state="standard"):
    query = dict(item.split("=", 1) for item in path.query.split("&") if "=" in item)
    state = query.get("snapshot", page_state)
    endpoint = path.path
    if scenario == "error":
        return 503, {"error": {"code": "SERVICE_UNAVAILABLE", "message": "Layanan sedang tidak tersedia."}}
    events = [event("POSTPONED" if state == "changed" else "SCHEDULED")]
    if endpoint == "/api/v1/events":
        return 200, {"events": [] if scenario == "empty" else events}
    if endpoint.startswith("/api/v1/events/"):
        return 200, event("POSTPONED" if state == "changed" else "SCHEDULED")
    if endpoint.endswith("/event") and "/reservations/" in endpoint:
        return 200, event()
    if endpoint == "/api/v1/reservations":
        return 201, reservation()
    if endpoint.endswith("/checkout") and "/reservations/" in endpoint:
        result = order("PENDING", "PENDING")
        return 201, {**result, "accessToken": TOKEN}
    if endpoint.startswith("/api/v1/reservations/"):
        return 200, reservation()
    if endpoint.startswith("/api/v1/orders/") and endpoint.endswith("/tickets"):
        return 200, {"tickets": [ticket()]}
    if endpoint.startswith("/api/v1/orders/"):
        status = {"pending": "PENDING", "cancelled": "CANCELLED", "expired": "EXPIRED", "refund-pending": "REFUND_PENDING", "refunded": "REFUNDED"}.get(state, "PAID")
        payment_status = "PENDING" if status == "PENDING" else "SUCCEEDED"
        event_state = "POSTPONED" if state in ("changed", "refund-pending", "refunded") else "SCHEDULED"
        return 200, order(status, payment_status, event_state)
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
    global MODE
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--current", action="store_true", help="Capture with the current UI instead of baseline tokens.")
    MODE = "current" if parser.parse_args().current else "baseline"
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
                if scenario == "loading":
                    await asyncio.sleep(1.5)
                status, body = reply(url, scenario, page_params.get("snapshot", "standard"))
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
                captures.append(await capture(page, f"{viewport_name}-{label}", f"{viewport_name}, {label}, standard"))
            for page_name, url, label in (("home", "/?scenario=loading", "beranda"), ("catalog", "/konser?scenario=empty", "katalog-kosong"), ("catalog", "/konser?scenario=error", "katalog-error"), ("detail", "/konser/nusa-malam?scenario=error", "detail-error")):
                await page.goto(BASE_URL + url, wait_until="domcontentloaded")
                await page.wait_for_timeout(100 if "loading" in url else 350)
                captures.append(await capture(page, f"{viewport_name}-{label}-{page_name}", f"{viewport_name}, {label}"))
            for state in ("pending", "cancelled", "expired", "changed", "refund-pending", "refunded"):
                await page.goto(f"{BASE_URL}/pesanan/{ORDER_ID}?snapshot={state}", wait_until="domcontentloaded")
                await page.wait_for_timeout(350)
                captures.append(await capture(page, f"{viewport_name}-pesanan-{state}", f"{viewport_name}, status pesanan {state}"))
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
                captures.append(await capture(page, f"{viewport_name}-checkout-{label}", f"{viewport_name}, checkout langkah {step}"))
            if viewport_name == "mobile":
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
    manifest = {"captureMode": MODE, "capturedAt": datetime.now(timezone.utc).isoformat(), "sourceCommit": commit, "tool": "Python Playwright, Google Chrome", "viewports": {"desktop": "1440x900", "mobile": "390x844", "narrow": "320x740"}, "api": "Responses are intercepted and generated from the current OpenAPI contract; no live buyer data is used.", "photoFixtures": "Existing Picsum URLs are served from docs/phase20/fixtures for repeatable captures.", "captures": captures, "layoutChecks": layout_checks, "isolationChecks": isolation_checks}
    OUTPUT.mkdir(parents=True, exist_ok=True)
    index = json.dumps(manifest, indent=2, ensure_ascii=False) + "\n"
    (OUTPUT / f"index-{MODE}.json").write_text(index)
    if MODE == "current":
        (OUTPUT / "index.json").write_text(index)
    print(f"Captured {len(captures)} {MODE} public UI screenshots in {OUTPUT}")


asyncio.run(main())
