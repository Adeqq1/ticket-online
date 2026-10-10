"""Capture admin/staff UI states using local API fixtures.

Run from frontend/: python3 scripts/admin-ui-check.py
Requires the Vite app at http://127.0.0.1:5173, Playwright, and Google Chrome.
"""
import argparse
import asyncio
import json
import re
import subprocess
from datetime import datetime, timezone
from pathlib import Path
from urllib.parse import urlparse

from playwright.async_api import async_playwright


ROOT = Path(__file__).resolve().parents[2]
OUTPUT = ROOT / "docs" / "phase29" / "screenshots"
TOKEN = "s" * 43
ORDER = "0123456789abcdef0123456789abcdef"
SESSION_INIT = f"sessionStorage.setItem('ticket-online:staff-session', JSON.stringify({{accessToken:'{TOKEN}',expiresAt:new Date(Date.now()+3600000).toISOString()}}))"
EVENT = {
    "id": "nusa-malam", "artist": "Nusa Malam", "city": "Jakarta", "venue": "Ruang Selatan",
    "address": "Jl. Musik Raya, Jakarta", "startsAt": "2027-08-24T12:30:00Z", "genre": "Indie",
    "status": "Presale", "publicationStatus": "PUBLISHED", "image": "/fixtures/nusa-malam.jpg",
    "description": "Konser Nusa Malam dengan pilihan tiket untuk area panggung dan penonton.",
    "lineup": ["Nusa Malam", "Pembuka Sore"], "price": 275000, "scheduleLocked": False,
    "locationLocked": False,
    "zones": [{"id": "festival", "name": "Festival", "description": "Area berdiri umum"}],
    "ticketTiers": [{"id": "festival", "name": "Festival", "zoneId": "festival", "price": 275000,
                     "availableQuantity": 42, "capacity": 100, "boundQuantity": 58, "maxPerOrder": 6,
                     "benefit": "Area berdiri umum", "gate": "Gate B", "gateLocked": True,
                     "seating": "free-standing"}],
}
ADMIN = {"id": "c" * 32, "name": "Admin Event", "email": "admin@example.test", "role": "ADMIN", "active": True, "assignments": []}
STAFF = {"id": "a" * 32, "name": "Dina Petugas", "email": "dina@example.test", "role": "STAFF", "active": True,
         "assignments": [{"eventId": "nusa-malam", "gate": "Gate B"}]}
PERSON = {**STAFF, "id": "b" * 32, "name": "Raka Petugas", "email": "raka@example.test"}
TICKET = {"id": "d" * 32, "code": "ET-" + "D" * 32, "attendeeName": "Nadia Pembeli", "tierName": "Festival",
          "eventId": "nusa-malam", "gate": "Gate B"}
ZERO = {"successfulTransactions": 0, "paymentAmount": 0, "refundAmount": 0, "netAmount": 0,
        "unfinishedRefunds": 0, "openReconciliationCases": 0}
STAGE = {"detail": 0, "reservation": 0, "order": 0, "paymentStarted": 0, "paymentSucceeded": 0}
BREAKDOWN = {"total": STAGE, "matured": STAGE, "pendingObservation": 0,
             "lost": {"detailToReservation": 0, "reservationToOrder": 0, "orderToPayment": 0, "paymentToSuccess": 0}}
KNOWN_API = {
    ("GET", path) for path in ("/staff/me", "/events", "/admin/events", "/admin/staff", "/admin/orders",
        "/admin/payment-cases", "/admin/email-jobs", "/admin/operations", "/admin/reports/sales",
        "/admin/reports/conversion", "/admin/reports/attendance", "/admin/check-ins", "/staff/ticket-status")
} | {("POST", "/staff/login"), ("POST", "/staff/check-ins")}


def known_api_request(method, path):
    if (method, path) in KNOWN_API:
        return True
    return method == "GET" and bool(re.fullmatch(r"/admin/orders/[0-9a-f]{32}|/admin/events/[^/]+/changes", path))


def body(path, mode, role):
    endpoint = urlparse(path).path.removeprefix("/api/v1")
    if mode == "error" and endpoint != "/staff/me":
        return 503, {"error": {"code": "SERVICE_UNAVAILABLE", "message": "Layanan sedang tidak tersedia."}}
    empty = mode == "empty"
    if endpoint == "/staff/me":
        profile = ADMIN if role == "ADMIN" else STAFF
        if empty and role == "STAFF": profile = {**STAFF, "assignments": []}
        return 200, {"staff": profile}
    if endpoint in ("/events", "/admin/events"):
        return 200, {"events": [] if empty else [EVENT]}
    if endpoint == "/admin/staff":
        return 200, {"staff": [] if empty else [STAFF, PERSON]}
    if endpoint == "/admin/orders":
        rows = [] if empty else [{"id": ORDER, "reference": "TO-0123456789abcdef0123", "status": "PAID",
            "eventId": EVENT["id"], "eventName": EVENT["artist"], "buyerName": "Nadia Pembeli",
            "createdAt": "2026-10-01T10:00:00Z", "ticketCount": 1, "total": 280000}]
        return 200, {"items": rows, "nextCursor": None, "filterOptions": {"events": [{"id": EVENT["id"], "name": EVENT["artist"]}]}}
    if endpoint == f"/admin/orders/{ORDER}":
        return 200, {"id": ORDER, "reference": "TO-0123456789abcdef0123", "status": "PAID", "eventId": EVENT["id"],
            "eventName": EVENT["artist"], "buyerName": "Nadia Pembeli", "createdAt": "2026-10-01T10:00:00Z",
            "ticketCount": 1, "total": 280000, "buyer": {"name": "Nadia Pembeli", "email": "nadia@example.test",
            "phone": "08123456789", "identityMasked": "************3456"}, "subtotal": 275000, "adminFee": 5000,
            "discount": 0, "expiresAt": "2026-10-01T10:10:00Z", "items": [{"tierId": "festival", "name": "Festival",
            "quantity": 1, "unitPrice": 275000, "lineTotal": 275000}], "payment": {"method": "QRIS", "amount": 280000,
            "status": "SUCCEEDED", "paidAt": "2026-10-01T10:01:00Z"}, "tickets": [{**TICKET, "status": "NOT_CHECKED_IN", "checkedInAt": None, "checkedInBy": None}]}
    if endpoint == "/admin/payment-cases":
        case = {"id": "17", "status": "OPEN", "orderId": ORDER, "reference": "TO-0123456789abcdef0123",
            "eventName": EVENT["artist"], "orderStatus": "PENDING", "paymentStatus": "PENDING", "providerStatus": "pending",
            "reason": "Status pembayaran perlu diperiksa", "amount": 280000, "createdAt": "2026-10-10T02:00:00Z",
            "updatedAt": "2026-10-10T02:00:00Z", "lastCheckedAt": None, "lastCheckError": "", "checkInProgress": False}
        return 200, {"items": [] if empty else [case], "nextCursor": None}
    if endpoint == "/admin/email-jobs":
        job = {"id": "e" * 32, "kind": "TICKETS", "status": "FAILED", "reference": "TO-0123456789abcdef0123",
            "recipient": "nadia@example.test", "attempts": 2, "lastError": "SMTP belum merespons", "updatedAt": "2026-10-10T02:00:00Z", "supersededBy": None}
        return 200, {"items": [] if empty else [job], "nextCursor": None}
    if endpoint == "/admin/operations":
        return 200, {"collectedAt": "2026-10-10T03:00:00Z", "api5xxLast5m": 0, "failedEmailJobs": 0,
            "oldestPendingEmailSeconds": 0, "openPaymentCases": 0, "openRefunds": 0, "heldTickets": 0,
            "pendingPayments": 0, "workers": [], "alerts": []}
    if endpoint == "/admin/reports/sales":
        amount = ZERO if empty else {"successfulTransactions": 2, "paymentAmount": 560000, "refundAmount": 280000,
            "netAmount": 280000, "unfinishedRefunds": 1, "openReconciliationCases": 1}
        return 200, {"period": {"eventId": None, "dateFrom": "2026-09-11", "dateTo": "2026-10-10", "timeZone": "Asia/Jakarta"},
            "summary": amount, "daily": [] if empty else [{"date": "2026-10-10", **amount}],
            "byEvent": [] if empty else [{"id": EVENT["id"], "name": EVENT["artist"], **amount}],
            "filterOptions": {"events": [{"id": EVENT["id"], "name": EVENT["artist"]}]}, "dataUpdatedAt": "2026-10-10T03:00:00Z"}
    if endpoint == "/admin/reports/conversion":
        stages = STAGE if empty else {"detail": 47, "reservation": 19, "order": 15, "paymentStarted": 11, "paymentSucceeded": 8}
        breakdown = {"total": stages, "matured": stages, "pendingObservation": 3,
            "lost": {"detailToReservation": 28, "reservationToOrder": 4, "orderToPayment": 4, "paymentToSuccess": 3}}
        return 200, {"period": {"eventId": "", "device": "", "dateFrom": "2026-09-11", "dateTo": "2026-10-10",
            "timeZone": "Asia/Jakarta", "observationHours": 24}, "summary": breakdown,
            "byEvent": [] if empty else [{**breakdown, "eventId": EVENT["id"], "eventName": EVENT["artist"]}],
            "byDevice": [] if empty else [{**breakdown, "device": "mobile"}, {**breakdown, "device": "desktop"}],
            "blockers": [] if empty else [{"kind": "PAYMENT_FAILURE", "reason": "provider_timeout", "count": 2}],
            "unattributedReservations": 4, "unattributedPayments": 2, "dataUpdatedAt": "2026-10-10T03:00:00Z"}
    if endpoint == "/admin/reports/attendance":
        amounts = {"capacity": 0, "available": 0, "issued": 0, "eligible": 0, "heldForRefund": 0, "checkedIn": 0, "attendanceRate": None}
        counts = amounts if empty else {"capacity": 100, "available": 42, "issued": 58, "eligible": 55, "heldForRefund": 3, "checkedIn": 21, "attendanceRate": 38.2}
        row = {"id": 1, "name": "Festival", **counts}
        gate_row = {"gate": "Gate B", **counts}
        return 200, {"event": {"id": EVENT["id"], "name": EVENT["artist"]}, "gate": None, "timeZone": "Asia/Jakarta",
            "summary": counts, "byCategory": [] if empty else [row], "byGate": [] if empty else [gate_row],
            "hourly": [] if empty else [{"hour": "2026-10-10T03:00:00Z", "checkedIn": 21}], "filterOptions": {"gates": ["Gate B"]},
            "dataUpdatedAt": "2026-10-10T03:00:00Z"}
    if endpoint == "/admin/check-ins":
        history = {"id": "1", "code": TICKET["code"], "eventId": EVENT["id"], "eventName": EVENT["artist"],
            "gate": "Gate B", "staff": {"id": STAFF["id"], "name": STAFF["name"]}, "outcome": "CHECKED_IN",
            "recordedAt": "2026-10-10T03:00:00Z", "checkedInAt": "2026-10-10T03:00:00Z"}
        return 200, {"items": [] if empty else [history], "nextCursor": None,
            "filterOptions": [{"id": EVENT["id"], "name": EVENT["artist"], "gates": ["Gate B"]}]}
    if endpoint == "/staff/ticket-status":
        return 200, {"status": "NOT_CHECKED_IN", "ticket": TICKET, "orderStatus": "PAID", "checkedInAt": None}
    if endpoint == "/admin/events/nusa-malam/changes": return 200, []
    if endpoint == "/staff/check-ins":
        return 201, {"status": "CHECKED_IN", "ticket": TICKET, "checkedInAt": "2026-10-10T03:00:00Z"}
    return 200, {}


async def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--base-url", default="http://127.0.0.1:5173")
    parser.add_argument("--only", help="Capture a single route key for debugging, e.g. events")
    parser.add_argument("--output-dir", type=Path, help="Screenshot output directory; defaults to docs/phase29/screenshots")
    parser.add_argument("--phase", type=int, default=29, help="Phase number recorded in the manifest")
    args = parser.parse_args()
    output = args.output_dir if args.output_dir and args.output_dir.is_absolute() else ROOT / args.output_dir if args.output_dir else OUTPUT
    output.mkdir(parents=True, exist_ok=True)
    for previous in output.glob("*.png"):
        previous.unlink()
    commit = subprocess.run(["git", "rev-parse", "HEAD"], cwd=ROOT, text=True, capture_output=True, check=True).stdout.strip()
    captures, unexpected, observed_api, unhandled_api = [], [], set(), []
    routes = [
        ("login", "/admin/login", "PUBLIC"), ("events", "/admin/events", "ADMIN"),
        ("staff", "/admin/staff", "ADMIN"), ("orders", "/admin/orders", "ADMIN"),
        ("issues", "/admin/issues", "ADMIN"), ("operations", "/admin/operations", "ADMIN"),
        ("sales", "/admin/reports", "ADMIN"), ("attendance", "/admin/reports/attendance", "ADMIN"),
        ("conversion", "/admin/reports/conversion", "ADMIN"), ("check-ins", "/admin/check-ins", "ADMIN"),
        ("scanner", "/admin/scan", "STAFF"),
    ]
    if args.only:
        routes = [item for item in routes if item[0] == args.only]
        if not routes:
            parser.error(f"unknown route key: {args.only}")
    async with async_playwright() as playwright:
        browser = await playwright.chromium.launch(channel="chrome", headless=True)
        for viewport, size in (("desktop", {"width": 1440, "height": 900}), ("mobile", {"width": 390, "height": 844})):
            for name, route, role in routes:
                for mode in ("standard", "loading", "empty", "error"):
                    context = await browser.new_context(viewport=size, timezone_id="Asia/Jakarta", color_scheme="dark")
                    if role != "PUBLIC":
                        await context.add_init_script(SESSION_INIT)
                    page = await context.new_page()
                    page_errors = []
                    page.on("pageerror", lambda error: page_errors.append(str(error)))

                    async def fixture(request):
                        parsed = urlparse(request.request.url)
                        if not parsed.path.startswith("/api/v1/"):
                            return await request.continue_()
                        endpoint = parsed.path.removeprefix("/api/v1")
                        signature = (request.request.method, endpoint)
                        observed_api.add(f"{signature[0]} {signature[1]}")
                        if not known_api_request(*signature):
                            unhandled_api.append(f"{signature[0]} {signature[1]}")
                            await request.fulfill(status=501, json={"error": {"code": "UNHANDLED_FIXTURE", "message": "API fixture belum dipetakan."}})
                            return
                        if mode == "loading" and parsed.path != "/api/v1/staff/me":
                            await asyncio.sleep(1.2)
                        status, data = body(parsed.path + ("?" + parsed.query if parsed.query else ""), mode, role)
                        await request.fulfill(status=status, json=data)

                    await page.route("**/*", fixture)
                    target = route
                    if name == "orders" and mode == "standard": target += f"?orderId={ORDER}"
                    await page.goto(args.base_url + target, wait_until="domcontentloaded")
                    await page.locator("#konten").wait_for()
                    await page.wait_for_function("document.title !== 'Tiket Online'", timeout=15000)
                    await page.wait_for_timeout(80 if mode == "loading" else 400)
                    if name == "attendance" and mode == "standard":
                        await page.locator(".history-filter-form select").first.wait_for()
                        await page.locator(".history-filter-form select").first.select_option(EVENT["id"])
                        await page.get_by_role("button", name="Terapkan", exact=True).click()
                        await page.wait_for_timeout(400)
                    if name == "login" and mode in ("loading", "error"):
                        await page.get_by_label("Email", exact=True).fill("admin@example.test")
                        await page.get_by_label("Password", exact=True).fill("fixture-password")
                        await page.get_by_role("button", name="Masuk", exact=False).click()
                        if mode == "error": await page.get_by_role("alert").wait_for()
                    if name == "scanner" and mode in ("standard", "error"):
                        await page.get_by_label("Penugasan aktif").select_option(f"{EVENT['id']}:Gate B")
                        await page.get_by_label("Kode e-ticket", exact=True).fill(TICKET["code"])
                        await page.get_by_role("button", name="Verifikasi", exact=False).click()
                        await page.get_by_text("Hasil belum diketahui" if mode == "error" else "Gate Masuk Terbuka", exact=False).wait_for()
                    await page.wait_for_timeout(120)
                    suffix = "" if mode == "standard" else f"-{mode}"
                    base = f"{name}{suffix}-{viewport}"
                    dimensions = await page.evaluate("({height: Math.max(document.body.scrollHeight, document.documentElement.scrollHeight), width: innerWidth, view: innerHeight})")
                    paths = []
                    stride = max(1, dimensions["view"] - 48)
                    tops = [0] if dimensions["height"] <= dimensions["view"] else list(range(0, dimensions["height"] - dimensions["view"], stride)) + [dimensions["height"] - dimensions["view"]]
                    for index, top in enumerate(dict.fromkeys(tops)):
                        await page.evaluate("top => window.scrollTo(0, top)", top)
                        await page.wait_for_timeout(80)
                        path = f"{base}.png" if len(tops) == 1 else f"{base}-view-{index + 1:02d}.png"
                        await page.screenshot(path=str(output / path), full_page=False, animations="disabled", timeout=10000)
                        paths.append(path)
                    current_url = urlparse(page.url)
                    if mode == "standard" and name not in ("login", "scanner") and current_url.path != route:
                        unexpected.append({"route": route, "actual": current_url.path, "mode": mode, "viewport": viewport})
                    capture = {"route": route, "scenario": mode, "role": role, "viewport": viewport, "files": paths, "finalUrl": current_url.path + (("?" + current_url.query) if current_url.query else "")}
                    if mode == "standard":
                        capture["audit"] = await page.evaluate("""() => {
                          const visible = el => { const r = el.getBoundingClientRect(); const s = getComputedStyle(el); return r.width > 0 && r.height > 0 && s.visibility !== 'hidden' && s.display !== 'none'; };
                          const text = [...document.querySelectorAll('h1,h2,h3,p,label,small,button,a,th,td,strong,summary')].filter(visible);
                          const controls = [...document.querySelectorAll('a,button,input:not([type=hidden]),select,textarea,summary,[role=button]')].filter(visible).map(el => { const r=el.getBoundingClientRect(); return {name:(el.getAttribute('aria-label') || el.innerText || el.getAttribute('name') || el.tagName).trim().replace(/\\s+/g,' ').slice(0,48),width:Math.round(r.width),height:Math.round(r.height)}; });
                          return {
                            title: document.title,
                            headings: [...document.querySelectorAll('h1,h2,h3')].filter(visible).map(el => ({level:el.tagName,text:el.innerText.trim()})),
                            links: [...document.querySelectorAll('a[href]')].filter(visible).map(el => ({label:el.innerText.trim(),href:el.getAttribute('href'),current:el.getAttribute('aria-current')})),
                            fields: [...document.querySelectorAll('input,select,textarea')].filter(visible).map(el => ({tag:el.tagName,type:el.type||'',name:el.getAttribute('name'),id:el.id,label:el.labels?.[0]?.innerText.trim()||''})),
                            anchors: [...document.querySelectorAll('[id]')].filter(visible).map(el=>el.id),
                            textFontSizes: [...new Set(text.map(el=>parseFloat(getComputedStyle(el).fontSize)).filter(Number.isFinite))].sort((a,b)=>a-b),
                            undersizedTargets: controls.filter(el=>el.width<44||el.height<44),
                            liveRegionCount: document.querySelectorAll('[role=status],[role=alert],[aria-live]').length,
                            overflowElements: [...document.querySelectorAll('body *')].filter(el=>{const r=el.getBoundingClientRect();return r.right>innerWidth+1||r.left < -1}).slice(0,12).map(el=>({tag:el.tagName,id:el.id,classes:typeof el.className==='string'?el.className:'',parents:[el.parentElement?.tagName,el.parentElement?.className,el.parentElement?.parentElement?.className],left:Math.round(el.getBoundingClientRect().left),right:Math.round(el.getBoundingClientRect().right)})),
                            viewportWidth: innerWidth,documentWidth: document.documentElement.scrollWidth,
                            overflow: document.documentElement.scrollWidth>innerWidth
                          };
                        }""")
                        action = page.locator(".admin-page-heading .admin-heading-actions > button").first
                        if name not in ("login", "scanner"):
                            if not await action.count():
                                raise AssertionError(f"admin heading action missing on {route}")
                            action_info = await action.evaluate("button => ({label:button.innerText.trim(),type:button.type,formId:button.form?.id||'',disabled:button.disabled})")
                            if action_info["formId"] and not await page.locator(f"#{action_info['formId']}").count():
                                raise AssertionError(f"heading action points to a missing form on {route}")
                            capture["audit"]["primaryAction"] = action_info
                        await page.keyboard.press("Tab")
                        capture["audit"]["firstTabFocus"] = await page.evaluate("""() => ({tag:document.activeElement?.tagName,id:document.activeElement?.id||'',label:document.activeElement?.getAttribute('aria-label')||document.activeElement?.innerText?.trim()||''})""")
                        if viewport == "mobile":
                            if role == "ADMIN":
                                menu_button = page.get_by_role("button", name="Menu", exact=True)
                                await menu_button.focus()
                                await page.keyboard.press("Enter")
                                dialog = page.locator("#admin-mobile-menu")
                                if not await dialog.evaluate("dialog => dialog.open"):
                                    raise AssertionError(f"mobile admin menu did not open on {route}")
                                close_button = dialog.get_by_role("button", name="Tutup", exact=True)
                                if not await close_button.evaluate("button => button === document.activeElement"):
                                    raise AssertionError(f"mobile menu focus did not move to close button on {route}")
                                current_links = dialog.locator('a[aria-current="page"]')
                                if await current_links.count() != 1:
                                    raise AssertionError(f"mobile menu needs one active destination on {route}")
                                await dialog.get_by_role("link").last.focus()
                                await page.keyboard.press("Tab")
                                focus_trapped = await dialog.evaluate("dialog => dialog.contains(document.activeElement)")
                                if not focus_trapped:
                                    active = await page.evaluate("() => document.activeElement?.outerHTML")
                                    raise AssertionError(f"mobile menu focus escaped its dialog on {route}: {active}")
                                await page.keyboard.press("Escape")
                                focus_restored = await menu_button.evaluate("button => button === document.activeElement")
                                if not focus_restored:
                                    raise AssertionError(f"mobile menu did not restore focus on {route}")
                                await page.keyboard.press("Enter")
                                await page.set_viewport_size({"width": 1024, "height": size["height"]})
                                await page.wait_for_function("!document.querySelector('#admin-mobile-menu').open")
                                desktop_focus_restored = await page.locator(".admin-topbar .scan-brand").evaluate("brand => brand === document.activeElement")
                                if not desktop_focus_restored:
                                    raise AssertionError(f"desktop resize left focus on a hidden control on {route}")
                                await page.set_viewport_size(size)
                                capture["audit"]["mobileMenu"] = {"oneActiveDestination": True, "focusEntered": True, "focusTrapped": True, "escapeCloses": True, "focusRestored": True, "desktopResizeCloses": True}
                            await page.set_viewport_size({"width": 320, "height": 740})
                            await page.evaluate("window.scrollTo(0,0)")
                            capture["audit"]["narrow320"] = await page.evaluate("""() => ({viewportWidth:innerWidth,documentWidth:document.documentElement.scrollWidth,overflow:document.documentElement.scrollWidth>innerWidth})""")
                            narrow_path = f"{name}-320px.png"
                            await page.screenshot(path=str(output / narrow_path), full_page=False, animations="disabled")
                            capture["files"].append(narrow_path)
                    if page_errors:
                        capture["pageErrors"] = page_errors
                    captures.append(capture)
                    await context.close()

        # Preserve role-denial redirects for each administrator-only page.
        for viewport, size in (("desktop", {"width": 1440, "height": 900}), ("mobile", {"width": 390, "height": 844})):
            for name, route, role in routes:
                if role != "ADMIN": continue
                context = await browser.new_context(viewport=size, timezone_id="Asia/Jakarta")
                await context.add_init_script(SESSION_INIT)
                page = await context.new_page()

                async def denied_fixture(request):
                    if request.request.url.endswith("/staff/me"):
                        await request.fulfill(json={"staff": STAFF})
                    else:
                        await request.fulfill(json={"events": [EVENT]})

                await page.route("**/api/v1/**", denied_fixture)
                await page.goto(args.base_url + route, wait_until="domcontentloaded")
                await page.wait_for_function("document.title !== 'Tiket Online'", timeout=15000)
                await page.wait_for_timeout(250)
                if urlparse(page.url).path != "/admin/scan":
                    unexpected.append({"route": route, "actual": urlparse(page.url).path, "mode": "role-denied", "viewport": viewport})
                denied_path = f"{name}-role-denied-{viewport}.png"
                await page.screenshot(path=str(output / denied_path), full_page=False)
                captures.append({"route": route, "scenario": "role-denied", "role": "STAFF", "viewport": viewport, "files": [denied_path], "finalUrl": urlparse(page.url).path})
                await context.close()
        await browser.close()

    manifest = {"phase": args.phase, "capturedAt": datetime.now(timezone.utc).isoformat(), "sourceCommit": commit,
        "tool": "Python Playwright, Google Chrome", "viewports": {"desktop": "1440x900", "mobile": "390x844"},
        "theme": "dark", "timezone": "Asia/Jakarta", "api": "All API requests are intercepted; responses use local fixtures and unmapped requests fail the capture.",
        "apiRequests": sorted(observed_api), "unhandledApiRequests": unhandled_api,
        "pageErrors": [{"route": c["route"], "scenario": c["scenario"], "viewport": c["viewport"], "errors": c["pageErrors"]} for c in captures if c.get("pageErrors")],
        "captures": captures, "unexpectedRedirects": unexpected}
    (output / "manifest.json").write_text(json.dumps(manifest, ensure_ascii=False, indent=2) + "\n")
    print(f"Captured {len(captures)} admin/staff screenshots in {output}")
    if unexpected:
        raise AssertionError(f"Unexpected redirects: {unexpected}")
    if unhandled_api:
        raise AssertionError(f"API requests missing a fixture: {unhandled_api}")


asyncio.run(main())
