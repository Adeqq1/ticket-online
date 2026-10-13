"""Capture admin/staff UI states using local API fixtures.

Run from frontend/: python3 scripts/admin-ui-check.py
Requires the Vite app at http://127.0.0.1:5173, Playwright, and Google Chrome.
"""
import argparse
import asyncio
import json
import re
import subprocess
from copy import deepcopy
from runpy import run_path
from datetime import datetime, timezone
from pathlib import Path
from urllib.parse import urlparse, parse_qs

from playwright.async_api import async_playwright, expect


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
        "/admin/reports/sales.csv", "/admin/reports/conversion", "/admin/reports/attendance",
        "/admin/reports/attendance.csv", "/admin/check-ins", "/staff/ticket-status")
} | {("POST", "/staff/login"), ("POST", "/staff/check-ins"), ("POST", "/staff/logout")}


def known_api_request(method, path):
    if (method, path) in KNOWN_API:
        return True
    return (method == "GET" and bool(re.fullmatch(r"/admin/orders/[0-9a-f]{32}|/admin/payment-cases/[0-9]+|/admin/email-jobs/[0-9a-f]{32}|/admin/events/[^/]+/changes", path))) or \
        (method == "POST" and bool(re.fullmatch(r"/admin/orders/[0-9a-f]{32}/refund(?:/manual)?|/admin/payment-cases/[0-9]+/(?:recheck|notes|resolve)|/admin/email-jobs/[0-9a-f]{32}/retry", path)))


def body(path, mode, role):
    endpoint = urlparse(path).path.removeprefix("/api/v1")
    if mode == "rejected" and endpoint == "/staff/check-ins":
        return 409, {"error": {"code": "TICKET_ALREADY_USED", "message": "Tiket sudah digunakan."}, "ticket": TICKET}
    if mode == "unknown-checkedin" and endpoint == "/staff/check-ins":
        return 503, {"error": {"code": "SERVICE_UNAVAILABLE", "message": "Layanan sedang tidak tersedia."}}
    if mode == "expired-camera" and endpoint == "/staff/check-ins":
        return 401, {"error": {"code": "UNAUTHORIZED", "message": "Sesi petugas sudah berakhir."}}
    if mode == "unknown-checkedin" and endpoint == "/staff/ticket-status":
        return 200, {"status": "CHECKED_IN", "ticket": TICKET, "orderStatus": "PAID", "checkedInAt": "2026-10-10T03:00:00Z"}
    if endpoint == "/staff/logout": return 204, {}
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
    if endpoint == f"/admin/orders/{ORDER}/refund": return 200, {"status": "REQUESTED", "amount": 280000, "reason": "Permintaan admin"}
    if endpoint == f"/admin/orders/{ORDER}/refund/manual": return 200, {"status": "SUCCEEDED", "amount": 280000, "reason": "Transfer dicatat"}
    if endpoint == "/admin/payment-cases":
        case = {"id": "17", "status": "OPEN", "orderId": ORDER, "reference": "TO-0123456789abcdef0123",
            "eventName": EVENT["artist"], "orderStatus": "PENDING", "paymentStatus": "PENDING", "providerStatus": "pending",
            "reason": "Status pembayaran perlu diperiksa", "amount": 280000, "createdAt": "2026-10-10T02:00:00Z",
            "updatedAt": "2026-10-10T02:00:00Z", "lastCheckedAt": None, "lastCheckError": "", "checkInProgress": False}
        return 200, {"items": [] if empty else [case], "nextCursor": None}
    if endpoint == "/admin/payment-cases/17":
        return 200, {"id": "17", "status": "OPEN", "orderId": ORDER, "reference": "TO-0123456789abcdef0123",
            "eventName": EVENT["artist"], "orderStatus": "PENDING", "paymentStatus": "PENDING", "providerStatus": "pending",
            "reason": "Status pembayaran perlu diperiksa", "amount": 280000, "createdAt": "2026-10-10T02:00:00Z",
            "updatedAt": "2026-10-10T02:00:00Z", "lastCheckedAt": None, "lastCheckError": "", "checkInProgress": False,
            "total": 280000, "expiresAt": "2026-10-10T02:10:00Z", "gatewayOrderId": "fixture-order", "canRecheck": True,
            "canResolve": True, "notes": [], "history": [{"action": "RECHECK", "actorName": "Admin Event", "createdAt": "2026-10-10T02:00:00Z", "data": {"result": "STARTED"}}]}
    if endpoint == "/admin/payment-cases/17/recheck": return 200, {"caseId": "17", "providerStatus": "settlement"}
    if endpoint in ("/admin/payment-cases/17/notes", "/admin/payment-cases/17/resolve"): return 204, {}
    if endpoint == "/admin/email-jobs":
        job = {"id": "e" * 32, "kind": "TICKETS", "status": "FAILED", "reference": "TO-0123456789abcdef0123",
            "recipient": "n***@example.test", "attempts": 2, "lastError": "SMTP koneksi gagal", "updatedAt": "2026-10-10T02:00:00Z", "supersededBy": None}
        other_job = {**job, "id": "g" * 32, "reference": "TO-9876543210fedcba9876", "recipient": "raka@example.test"}
        return 200, {"items": [] if empty else [job, other_job], "nextCursor": None}
    if endpoint == f"/admin/email-jobs/{'e' * 32}":
        return 200, {"id": "e" * 32, "kind": "TICKETS", "status": "FAILED", "reference": "TO-0123456789abcdef0123",
            "recipient": "nadia@example.test", "attempts": 2, "lastError": "SMTP belum merespons", "updatedAt": "2026-10-10T02:00:00Z",
            "supersededBy": None, "orderStatus": "PAID", "canRetry": True, "retryReason": "", "retryJobId": None,
            "history": [{"action": "RETRY", "actorName": "Admin Event", "createdAt": "2026-10-10T02:00:00Z", "data": {"retryJobId": "f" * 32, "kind": "TICKETS"}}]}
    if endpoint == f"/admin/email-jobs/{'e' * 32}/retry": return 200, {"jobId": "e" * 32, "retryJobId": "f" * 32, "status": "PENDING"}
    if endpoint == "/admin/operations":
        return 200, {"collectedAt": "2026-10-10T03:00:00Z", "api5xxLast5m": 0 if empty else 2, "failedEmailJobs": 0 if empty else 1,
            "oldestPendingEmailSeconds": 0 if empty else 420, "openPaymentCases": 0 if empty else 1, "openRefunds": 0 if empty else 1,
            "heldTickets": 0 if empty else 3, "pendingPayments": 0 if empty else 2,
            "workers": [] if empty else [{"name": "payment-reconcile", "running": False, "lastFinishedAt": "2026-10-10T02:58:00Z", "lastSuccessAt": "2026-10-10T02:55:00Z", "consecutiveFailures": 1}],
            "alerts": [] if empty else ["Pembayaran tertunda perlu diperiksa."]}
    if endpoint == "/admin/reports/sales":
        amount = ZERO if empty else {"successfulTransactions": 2, "paymentAmount": 560000, "refundAmount": 280000,
            "netAmount": 280000, "unfinishedRefunds": 1, "openReconciliationCases": 1}
        return 200, {"period": {"eventId": None, "dateFrom": "2026-09-11", "dateTo": "2026-10-10", "timeZone": "Asia/Jakarta"},
            "summary": amount, "daily": [] if empty else [{"date": "2026-10-10", **amount}],
            "byEvent": [] if empty else [{"id": EVENT["id"], "name": EVENT["artist"], **amount}],
            "filterOptions": {"events": [{"id": EVENT["id"], "name": EVENT["artist"]}]}, "dataUpdatedAt": "2026-10-10T03:00:00Z"}
    if endpoint == "/admin/reports/conversion":
        stages = STAGE if empty else {"detail": 47, "reservation": 19, "order": 15, "paymentStarted": 11, "paymentSucceeded": 8}
        pending = 0 if empty else 3
        breakdown = {"total": stages, "matured": stages, "pendingObservation": pending,
            "lost": {"detailToReservation": 28 if not empty else 0, "reservationToOrder": 4 if not empty else 0, "orderToPayment": 4 if not empty else 0, "paymentToSuccess": 3 if not empty else 0}}
        return 200, {"period": {"eventId": "", "device": "", "dateFrom": "2026-09-11", "dateTo": "2026-10-10",
            "timeZone": "Asia/Jakarta", "observationHours": 24}, "summary": breakdown,
            "byEvent": [] if empty else [{**breakdown, "eventId": EVENT["id"], "eventName": EVENT["artist"]}],
            "byDevice": [] if empty else [{**breakdown, "device": "mobile"}, {**breakdown, "device": "desktop"}],
            "blockers": [] if empty else [{"kind": "PAYMENT_FAILURE", "reason": "provider_timeout", "count": 2}],
            "unattributedReservations": None if mode == "missing-data" else 4, "unattributedPayments": None if mode == "missing-data" else 2, "dataUpdatedAt": "2026-10-10T03:00:00Z"}
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


async def check_help(page, width, desktop=False):
    buttons = page.locator(".admin-help-button:visible" if desktop else ".admin-help-button")
    description = page.locator(".mobile-help-description")
    if width >= 768 and not desktop:
        for button in await buttons.all(): await expect(button).not_to_be_visible()
        for paragraph in await description.all(): await expect(paragraph).to_be_visible()
        return
    if width < 768: await expect(description).not_to_be_visible()
    state = """() => [...document.querySelectorAll('main input, main select, main textarea, .mobile-list-viewport, .order-list-panel .history-table-wrap')].map(el => [el.value, el.checked, el.scrollLeft])"""
    before = await page.evaluate(state)
    for button in await buttons.all():
        assert await button.evaluate("el => getComputedStyle(el).backgroundColor") == "rgba(0, 0, 0, 0)", "Help icons must have a transparent background"
        popup = page.locator("#" + await button.get_attribute("aria-controls"))
        if width < 768: await button.tap()
        else: await button.click()
        await expect(popup).to_be_visible()
        await expect(button).to_have_attribute("aria-expanded", "true")
        assert await popup.inner_text()
        assert await button.evaluate("el => { const r = el.getBoundingClientRect(); return r.width >= 44 && r.height >= 44; }")
        await button.click()
        await expect(popup).not_to_be_visible()
        await button.focus()
        await page.keyboard.press("Tab")
        await page.keyboard.press("Shift+Tab")
        await expect(button).to_be_focused()
        await expect(popup).to_be_visible()
        await page.keyboard.press("Escape")
        await expect(popup).not_to_be_visible()
        await page.mouse.move(0, 0)
        await button.hover()
        await expect(popup).to_be_visible()
        await popup.hover()
        await expect(popup).to_be_visible()
        await page.get_by_role("heading", level=1).click()
        await expect(popup).not_to_be_visible()
    assert await page.evaluate(state) == before, "Help changed filters or the selected card"
    if await buttons.count() > 1:
        await buttons.first.click()
        await buttons.last.click()
        await expect(page.locator('.admin-help-popup:visible')).to_have_count(1)
        await page.keyboard.press("Escape")


async def check_help_pages(browser, base, output, width):
    context = await browser.new_context(viewport={"width": width, "height": 900}, is_mobile=width < 768, has_touch=width < 768)
    await context.add_init_script(SESSION_INIT)
    page = await context.new_page()
    errors, requests = [], []
    editor_mode = ""
    page.on("pageerror", lambda error: errors.append(str(error)))
    audit_js = run_path(str(Path(__file__).with_name("staff-review-check.py")))["STYLE_AUDIT"]

    async def fixture(route):
        requests.append((route.request.method, urlparse(route.request.url).path))
        status, data = body(route.request.url, "standard", "ADMIN")
        if editor_mode and urlparse(route.request.url).path == "/api/v1/admin/events":
            event = {**deepcopy(EVENT), "scheduleLocked": editor_mode == "locked", "locationLocked": editor_mode == "locked"}
            if editor_mode == "changed":
                event["currentEvent"] = {"id": EVENT["id"], "version": 1, "status": "RESCHEDULED", "salesPaused": False,
                                         "startsAt": EVENT["startsAt"], "announcement": "Jadwal diperbarui.", "refundDeadline": None}
            data = {"events": [event]}
        await route.fulfill(status=status, json=data)

    await page.route("**/api/v1/**", fixture)
    for name in ("events", "staff", "orders", "issues"):
        await page.goto(base + "/admin/" + name)
        await page.wait_for_load_state("networkidle")
        await expect(page.locator(".admin-help-button")).to_have_count(2 if name == "staff" else 1)
        await check_help(page, width)
        if name == "staff":
            await expect(page.locator(".staff-list-explanation")).to_be_visible(visible=width >= 768)
        if width < 768:
            await page.locator(".admin-help-button").first.click()
            popup = page.locator('.admin-help-popup:visible')
            await expect(popup).to_have_count(1)
            await page.screenshot(path=str(output / f"help-{name}-{width}.png"), full_page=True)
            await page.evaluate("""() => {
                window.helpFonts = [...document.querySelectorAll('.admin-layout *')].map(el => [el, el.style.fontSize, getComputedStyle(el).fontSize]);
                for (const [el, , size] of window.helpFonts) el.style.fontSize = parseFloat(size) * 2 + 'px';
            }""")
            result = await page.evaluate(audit_js)
            assert not result["failures"] and not result["overflow"], (name, width, result)
            assert await page.locator(".admin-help h1, .admin-help h2").evaluate_all("headings => headings.every(el => el.getBoundingClientRect().height <= parseFloat(getComputedStyle(el).fontSize) * 3)"), "Enlarged headings collapsed into narrow columns"
            assert await popup.evaluate("el => { const r = el.getBoundingClientRect(); return r.left >= 0 && r.right <= innerWidth; }")
            await page.evaluate("scrollTo({top: 0, behavior: 'instant'})")
            await page.screenshot(path=str(output / f"help-{name}-text200-{width}.png"), full_page=True)
            await expect(popup).to_be_visible()
            await page.evaluate("() => { for (const [el, size] of window.helpFonts) el.style.fontSize = size; }")
            await page.mouse.move(0, 0)
            await page.set_viewport_size({"width": 768, "height": 900})
            await expect(page.locator(".admin-help-button").first).to_have_attribute("aria-expanded", "false")
            await expect(page.locator('.admin-help-popup:visible')).to_have_count(0)
            await page.set_viewport_size({"width": width, "height": 900})
            await expect(page.locator('.admin-help-popup:visible')).to_have_count(0)
        else:
            await page.screenshot(path=str(output / f"help-{name}-{width}.png"), full_page=True)
    for editor_mode in ("locked", "unlocked", "changed"):
        await page.goto(base + "/admin/events")
        await page.get_by_role("button", name="Kelola konser Nusa Malam", exact=True).click()
        editor = page.locator(".event-editor-panel")
        await expect(page.locator("#event-editor-title")).to_be_focused()
        await expect(editor.locator(".admin-help-button")).to_have_count({"locked": 2, "unlocked": 0, "changed": 1}[editor_mode])
        await editor.locator(".event-section-nav").get_by_role("button", name="Jadwal & lokasi", exact=True).click()
        if editor_mode == "unlocked": await expect(page.get_by_label("Jadwal (WIB)", exact=True)).to_be_enabled()
        else: await expect(page.get_by_label("Jadwal (WIB)", exact=True)).to_be_disabled()
        for label in ("Kota", "Venue", "Alamat"):
            if editor_mode == "locked": await expect(page.get_by_label(label, exact=True)).to_be_disabled()
            else: await expect(page.get_by_label(label, exact=True)).to_be_enabled()
        await expect(editor.locator("label button, .admin-help-popup:visible")).to_have_count(0)
        if editor_mode != "unlocked":
            await expect(page.get_by_label("Jadwal (WIB)", exact=True)).to_have_attribute("aria-describedby", "schedule-lock-note")
            await expect(page.locator("#schedule-lock-note")).to_have_text("Jadwal terkunci setelah pesanan dibuat atau perubahan acara dicatat. Gunakan panel Pembatalan dan perubahan jadwal di bawah untuk menjaga hak pembeli dan riwayat keputusan.")
            await check_help(page, width, desktop=True)
            button = editor.locator(".admin-help-button").last
            await button.click()
            popup = page.locator("#" + await button.get_attribute("aria-controls"))
            await expect(popup).to_be_visible()
            await page.screenshot(path=str(output / f"help-editor-{editor_mode}-{width}.png"), full_page=True)
            await page.evaluate("""() => {
                window.helpFonts = [...document.querySelectorAll('.admin-layout *')].map(el => [el, el.style.fontSize, getComputedStyle(el).fontSize]);
                for (const [el, , size] of window.helpFonts) el.style.fontSize = parseFloat(size) * 2 + 'px';
            }""")
            assert await page.evaluate("document.documentElement.scrollWidth <= innerWidth"), (editor_mode, width, "200% text overflow")
            assert await popup.evaluate("el => { const r = el.getBoundingClientRect(); return r.left >= 0 && r.right <= innerWidth; }")
            await page.screenshot(path=str(output / f"help-editor-{editor_mode}-text200-{width}.png"), full_page=True)
            await page.evaluate("() => { for (const [el, size] of window.helpFonts) el.style.fontSize = size; }")
            await page.keyboard.press("Escape")
        if editor_mode == "locked":
            for label in ("Kota", "Venue", "Alamat"):
                await expect(page.get_by_label(label, exact=True)).to_have_attribute("aria-describedby", "location-lock-note")
            await expect(page.locator("#location-lock-note")).to_have_text("Kota, venue, dan alamat terkunci setelah reservasi pertama untuk menjaga informasi pada pesanan pembeli. Perubahan lokasi tidak tersedia di panel perubahan acara.")
            await editor.locator(".event-section-nav").get_by_role("button", name="Informasi", exact=True).click()
            await page.get_by_label("Nama konser / artis", exact=True).fill("Nusa Malam diperbarui")
            await editor.locator(".event-section-nav").get_by_role("button", name="Jadwal & lokasi", exact=True).click()
            await check_help(page, width, desktop=True)
            await expect(page.get_by_label("Nama konser / artis", exact=True)).to_have_value("Nusa Malam diperbarui")
            page.once("dialog", lambda dialog: dialog.dismiss())
            await page.get_by_role("button", name="Kembali ke daftar", exact=True).click()
            await expect(editor).to_be_visible()
            page.once("dialog", lambda dialog: dialog.accept())
        await page.get_by_role("button", name="Kembali ke daftar", exact=True).click()
        await expect(page.get_by_role("button", name="Kelola konser Nusa Malam", exact=True)).to_be_focused()
    assert all(method == "GET" for method, _ in requests) and not errors, (requests, errors)
    (output / f"help-{width}.json").write_text(json.dumps({"automated": True, "humanUAT": "not performed", "width": width, "pages": ["events", "staff", "orders", "issues"], "requests": requests, "browserErrors": errors}, indent=2))
    await context.close()
    print(f"PASS {width}: help tap/hover/keyboard, one popup, 200% text, desktop descriptions, locked/unlocked/event-change editor help, preserved drafts/focus")


async def check_event_forms(browser, base, output, width, record_baseline=False, baseline=None):
    """Compare every concert section with one fixed category, including enlarged text."""
    context = await browser.new_context(viewport={"width": width, "height": 900}, timezone_id="Asia/Jakarta")
    await context.add_init_script(SESSION_INIT)
    page = await context.new_page()
    errors, decisions, measurements = [], [], {}
    page.on("pageerror", lambda error: errors.append(str(error)))
    event = deepcopy(EVENT)
    event.update(scheduleLocked=True, locationLocked=True, image=base + "/fixtures/nusa-malam.jpg")

    async def fixture(route):
        path, method = urlparse(route.request.url).path, route.request.method
        if path == "/fixtures/nusa-malam.jpg":
            await route.fulfill(path=str(ROOT / "docs/phase20/fixtures/nusa-malam.jpg"))
        elif path.endswith("/changes/preview"):
            await route.fulfill(json={"event": {}, "snapshot": "preview", "impact": {"activeReservations": 1234, "pendingOrders": 2, "pendingPayments": 1, "paidOrders": 5, "issuedTickets": 10, "checkIns": 3, "existingRefunds": 1, "automaticAmount": 1250000, "manualAmount": 0}})
        elif path.endswith("/changes"):
            if method == "POST":
                decisions.append(route.request.post_data_json)
                await route.fulfill(json={"id": "change-1"})
            else: await route.fulfill(json=[])
        elif path.endswith("/admin/events"):
            if method == "POST":
                event.update(route.request.post_data_json)
                event.update(zones=[], ticketTiers=[])
                await route.fulfill(status=201, json=event)
            else: await route.fulfill(json={"events": [event]})
        else:
            status, data = body(route.request.url, "standard", "ADMIN")
            await route.fulfill(status=status, json=data)

    await page.route("**/api/v1/**", fixture)
    await page.route("**/fixtures/nusa-malam.jpg", fixture)
    await page.goto(base + "/admin/events")
    await page.get_by_role("button", name="Kelola konser Nusa Malam", exact=True).click()

    async def capture(name, enlarged=False):
        content = page.locator(".event-section-content")
        if enlarged:
            await page.evaluate("""() => {
                window.formFonts = [...document.querySelectorAll('.admin-layout *')].map(el => [el, el.style.fontSize, getComputedStyle(el).fontSize]);
                for (const [el,,size] of window.formFonts) el.style.fontSize = parseFloat(size)*2+'px';
            }""")
        assert await page.evaluate("document.documentElement.scrollWidth <= innerWidth"), (width, name, enlarged, "overflow")
        for control in await content.locator("input:not([type=radio]):not([type=checkbox]), select, textarea, button").all():
            if await control.is_visible():
                assert await control.evaluate("el => el.getBoundingClientRect().height >= 44"), (width, name, "touch height")
        if not record_baseline:
            if not enlarged:
                for control in await content.locator("input:not([type=radio]):not([type=checkbox]), select, textarea").all():
                    assert await control.evaluate("el => getComputedStyle(el).fontSize === '16px'")
            for grid in await content.locator(".event-form-row, .event-group, #zone-form, #tier-form, .event-change-panel fieldset").all():
                if await grid.is_visible():
                    tracks = await grid.evaluate("el => getComputedStyle(el).gridTemplateColumns")
                    # Chromium reports authored minmax()/repeat() for fieldset's anonymous grid.
                    count = 2 if tracks.startswith("repeat(2,") else len(re.findall(r"minmax\([^)]*\)|[\d.]+px", tracks))
                    two = await content.evaluate("el => el.clientWidth >= 35 * parseFloat(getComputedStyle(el).fontSize)")
                    assert count == (2 if two else 1), (name, width, enlarged, count, two, await grid.evaluate("el => [el.className, getComputedStyle(el).gridTemplateColumns]"))
            for textarea in await content.locator("textarea").all():
                assert await textarea.get_attribute("rows") == "3"
                assert await textarea.evaluate("el => getComputedStyle(el).resize === 'vertical'")
            image = content.locator(".poster-preview img")
            if await image.is_visible():
                assert await image.evaluate("el => el.getBoundingClientRect().height <= 240 && getComputedStyle(el).objectFit === 'contain'")
        await content.screenshot(path=str(output / f"forms-{name}{'-text200' if enlarged else ''}-{width}.png"), animations="disabled")
        if enlarged:
            await page.evaluate("() => { for(const [el, original] of window.formFonts) el.style.fontSize = original; }")

    for section in range(7):
        await page.locator(".event-section-nav button").nth(section).click()
        if section == 1:
            for label in ("Jadwal (WIB)", "Kota", "Venue", "Alamat"):
                await expect(page.get_by_label(label, exact=True)).to_be_disabled()
        if section in (3, 4):
            await page.locator(f'[data-section="{section}"] > label select').select_option("festival")
            if section == 4:
                await expect(page.get_by_label("Gate", exact=True)).to_be_disabled()
                measurements["ticketHeight"] = await page.evaluate("document.querySelector('#tier-form button[type=submit]').getBoundingClientRect().bottom - document.querySelector('#event-section-title').getBoundingClientRect().top")
        if section == 6:
            await page.get_by_label("Keputusan").select_option("RESCHEDULED")
            await page.get_by_label("Alasan untuk audit").fill("Alasan audit " + "panjang" * 30)
            await page.get_by_label("Pengumuman untuk pembeli").fill("Jadwal diperbarui untuk pembeli.")
            await page.get_by_label("Jadwal baru (WIB)").fill("2027-12-01T19:00")
            await page.get_by_label("Tenggat permintaan refund (WIB)").fill("2027-11-01T19:00")
            await page.get_by_role("button", name="Pratinjau dampak", exact=True).click()
            await expect(page.get_by_role("heading", name="Dampak keputusan")).to_be_focused()
            await expect(page.get_by_role("button", name="Terapkan keputusan")).to_be_disabled()
            await page.get_by_role("checkbox").check()
            await expect(page.get_by_role("button", name="Terapkan keputusan")).to_be_enabled()
        await capture(f"edit-{section}")
        await capture(f"edit-{section}", True)
    await page.get_by_role("button", name="Terapkan keputusan").click()
    assert len(decisions) == 1 and decisions[0]["reason"].startswith("Alasan audit"), decisions
    page.once("dialog", lambda dialog: dialog.accept())
    await page.get_by_role("button", name="Kembali ke daftar", exact=True).click()
    await page.get_by_role("button", name="Tambah konser", exact=True).click()
    await page.get_by_label("Nama konser / artis", exact=True).fill("Konser baru " + "panjang" * 15)
    for section in range(6):
        await capture(f"create-{section}")
        await capture(f"create-{section}", True)
        if section < 5:
            name = "Simpan draf & lanjutkan" if section == 2 else "Berikutnya"
            await page.get_by_role("button", name=name, exact=True).click()
    if baseline and width == 1440:
        old = json.loads((baseline / f"forms-{width}.json").read_text())["ticketHeight"]
        measurements["reduction"] = 1 - measurements["ticketHeight"] / old
        assert measurements["reduction"] >= .25, measurements
    assert not errors, errors
    (output / f"forms-{width}.json").write_text(json.dumps(measurements, indent=2))
    await context.close()
    print(f"PASS forms {width}px: all sections, 200% text, locks, preview/confirmation; {measurements}", flush=True)


async def check_workflows(browser, base, output, width):
    context = await browser.new_context(viewport={"width": width, "height": 900}, timezone_id="Asia/Jakarta")
    await context.add_init_script(SESSION_INIT)
    page = await context.new_page()
    errors, requests, mutations, audits = [], [], [], []
    event_failures = {"create": "", "refresh": False}
    refund_calls = []
    refund_found = False
    detail_failed = False
    page.on("pageerror", lambda error: errors.append(str(error)))
    events = [{**deepcopy(EVENT), "id": f"konser-{i}", "artist": f"Konser {i:02d}" + (" dengan nama sangat panjang untuk operasional acara" if i == 19 else ""), "publicationStatus": ["DRAFT", "PUBLISHED", "ARCHIVED"][i % 3]} for i in range(20)]
    _, detail = body(base + f"/api/v1/admin/orders/{ORDER}", "standard", "ADMIN")
    orders = [{"id": f"{i + 100:032x}", "reference": f"TO-{i:020d}", "buyerName": "Pembeli dengan nama panjang " + str(i), "eventName": events[i % 20]["artist"], "eventId": events[i % 20]["id"], "status": ["PAID", "PENDING", "EXPIRED", "CANCELLED", "REFUND_PENDING", "REFUNDED"][i % 6], "total": 100000 + i, "createdAt": "2026-10-01T10:00:00Z", "ticketCount": 2} for i in range(22)]
    audit_js = run_path(str(Path(__file__).with_name("staff-review-check.py")))["STYLE_AUDIT"]

    async def fixture(route):
        url = urlparse(route.request.url)
        path, method = url.path.removeprefix("/api/v1/"), route.request.method
        requests.append((method, path, parse_qs(url.query)))
        if path == "staff/me": await route.fulfill(json={"staff": ADMIN})
        elif path == "admin/events" and method == "GET":
            if event_failures["refresh"]:
                event_failures["refresh"] = False
                await route.fulfill(status=503, json={"error": {"message": "Penyegaran gagal"}})
            else: await route.fulfill(json={"events": events})
        elif path.endswith("/changes") and method == "GET": await route.fulfill(json=[])
        elif path.startswith("admin/events"):
            value = route.request.post_data_json
            mutations.append((method, path, value))
            if path == "admin/events":
                if event_failures["create"]:
                    code = event_failures["create"]
                    event_failures["create"] = ""
                    await route.fulfill(status=409 if code == "DUPLICATE_EVENT" else 503, json={"error": {"code": code, "message": "Simpan gagal. Coba lagi."}})
                    return
                event = {**deepcopy(EVENT), **value, "zones": [], "ticketTiers": []}
                events.append(event)
                await route.fulfill(status=201, json=event)
            else:
                event = next(e for e in events if e["id"] == path.split("/")[2])
                if path.endswith("/zones"):
                    event["zones"].append(value)
                    await route.fulfill(status=201, json=value)
                elif path.endswith("/ticket-tiers"):
                    tier = {**value, "availableQuantity": value["capacity"], "boundQuantity": 0, "gateLocked": False}
                    event["ticketTiers"].append(tier)
                    await route.fulfill(status=201, json=tier)
                elif "/zones/" in path or "/ticket-tiers/" in path:
                    rows = event["zones" if "/zones/" in path else "ticketTiers"]
                    row = next(row for row in rows if row["id"] == path.split("/")[-1])
                    row.update(value)
                    await route.fulfill(json=row)
                else:
                    event.update(value)
                    await route.fulfill(json=event)
        elif path == "admin/orders":
            query = parse_qs(url.query)
            if width < 768 and "cursor" in query and sum(1 for method, path, params in requests if path == "admin/orders" and "cursor" in params) == 1:
                await route.fulfill(status=503, json={"error": {"code": "SERVICE_UNAVAILABLE", "message": "Halaman berikutnya gagal dimuat."}})
                return
            rows = orders[10:] if "cursor" in query else orders[:10]
            if query.get("q") == ["absent"]: rows = []
            await route.fulfill(json={"items": rows, "nextCursor": "cursor-two" if len(rows) == 10 else None, "filterOptions": {"events": [{"id": e["id"], "name": e["artist"]} for e in events]}})
        elif path.startswith("admin/orders/"):
            if method == "POST":
                refund_calls.append(route.request.post_data_json)
                await route.abort("connectionreset")
            elif detail_failed:
                await route.fulfill(status=503, json={"error": {"message": "Detail gagal dimuat."}})
            else:
                value = {**detail, **next(o for o in orders if o["id"] == path.split("/")[-1])}
                if refund_found: value.update(status="REFUND_PENDING", refund={"status": "REQUESTED", "amount": value["total"], "reason": "Permintaan admin"})
                await route.fulfill(json=value)
        else: raise AssertionError((method, path))

    async def capture(name):
        result = await page.evaluate(audit_js)
        if result["overflow"]:
            result["overflowElements"] = await page.evaluate("() => [...document.querySelectorAll('body *')].filter(el => el.getBoundingClientRect().right > innerWidth + 1).slice(-20).map(el => ({tag: el.tagName, classes: el.className, text: el.textContent.slice(0, 60), right: el.getBoundingClientRect().right}))")
        assert not result["failures"] and not result["overflow"], (name, width, result)
        audits.append({"state": name, **result})
        await page.screenshot(path=str(output / f"workflow-{name}-{width}.png"), full_page=True, animations="disabled")

    async def text200(name):
        await page.evaluate("""() => {
          window.__phase55Fonts = [...document.querySelectorAll('.admin-layout *')].map(el => [el, el.style.fontSize, getComputedStyle(el).fontSize]);
          for (const [el, , size] of window.__phase55Fonts) el.style.fontSize = parseFloat(size) * 2 + 'px';
        }""")
        await capture(name + "-text200")
        await page.evaluate("() => { for (const [el, original] of window.__phase55Fonts) el.style.fontSize = original; }")

    await page.route("**/api/v1/**", fixture)
    await page.goto(base + "/admin/events")
    try: await expect(page.locator(".event-admin-list li")).to_have_count(5 if width < 768 else 20, timeout=15000)
    except AssertionError: raise AssertionError({"url": page.url, "errors": errors, "requests": requests, "text": await page.locator("body").inner_text()})
    await expect(page.locator("#event-form")).to_have_count(0)
    await expect(page.get_by_role("heading", level=1)).to_have_text("Daftar konser.")
    if width < 768:
        opener_box = await page.locator("#event-create-opener").bounding_box()
        panel_box = await page.locator('section[aria-labelledby="event-list-title"]').bounding_box()
        assert abs(opener_box["x"] - panel_box["x"]) < 2 and abs(opener_box["width"] - panel_box["width"]) < 2, "Tambah konser must fill the mobile content width"
        assert await page.locator("#event-create-opener").evaluate("el => getComputedStyle(el).textAlign") == "center"
    await capture("events-list")
    await text200("events-list")
    if width < 768:
        await page.get_by_role("button", name="Buka pencarian dan filter", exact=True).click()
    await page.get_by_label("Cari konser", exact=True).fill("Konser 19")
    await page.get_by_role("button", name="Kelola konser Konser 19", exact=False).click()
    await expect(page.locator("#event-editor-title")).to_be_focused()
    await expect(page.locator(".event-admin-list")).to_have_count(0)
    await expect(page.locator(".event-list-filters")).to_have_count(0)
    await expect(page.locator("#event-create-opener")).to_have_count(0)
    await expect(page.get_by_role("heading", level=1)).to_have_text("Edit konser.")
    await page.get_by_label("Nama konser / artis", exact=True).fill("Draft tidak disimpan")
    page.once("dialog", lambda d: d.dismiss())
    await page.get_by_role("button", name="Kembali ke daftar", exact=True).click()
    await expect(page.locator("#event-form")).to_have_count(1)
    page.once("dialog", lambda d: d.accept())
    await page.get_by_role("button", name="Kembali ke daftar", exact=True).click()
    await expect(page.get_by_label("Cari konser", exact=True)).to_have_value("Konser 19")
    await expect(page.locator("#event-manage-konser-19")).to_be_focused()
    await page.locator("#event-create-opener").click()
    await expect(page.locator("#event-editor-title")).to_be_focused()
    await expect(page.get_by_role("heading", level=1)).to_have_text("Tambah konser.")
    await expect(page.locator(".event-list-filters")).to_have_count(0)
    name = page.get_by_label("Nama konser / artis", exact=True)
    await name.fill("Konser Uji Baru")
    await expect(page.locator("#event-id")).to_have_value("konser-uji-baru")
    await page.locator("#event-id").fill("id-manual")
    await name.fill("Nama setelah ID manual")
    await expect(page.locator("#event-id")).to_have_value("id-manual")
    async def part(index):
        if width < 768 and await page.locator(".event-progress").count():
            await page.get_by_role("combobox", name="Pilih langkah", exact=True).select_option(str(index))
        else:
            await page.locator(".event-section-nav button").nth(index).click()
        await expect(page.locator("#event-section-title")).to_be_focused()

    await capture("events-create")
    await text200("events-create")
    await name.press("Enter")
    await expect(page.locator("#event-section-title")).to_have_text("Jadwal & lokasi")
    await page.get_by_label("Jadwal (WIB)", exact=True).fill("2027-12-01T19:00")
    for label, value in [("Kota", "Jakarta"), ("Venue", "Venue Uji"), ("Alamat", "Alamat Uji")]:
        await page.get_by_label(label, exact=True).fill(value)
    await page.get_by_role("button", name="Sebelumnya", exact=True).click()
    await expect(name).to_have_value("Nama setelah ID manual")
    await page.get_by_role("button", name="Berikutnya", exact=True).click()
    await expect(page.get_by_label("Kota", exact=True)).to_have_value("Jakarta")
    await page.get_by_role("button", name="Berikutnya", exact=True).click()
    for label, value in [("Deskripsi", "Deskripsi konser uji"), ("Lineup", "Artis Uji")]:
        await page.get_by_label(label, exact=label != "Lineup").fill(value)
    await page.get_by_role("radio", name="Gunakan URL gambar", exact=True).check()
    await page.get_by_label("URL poster", exact=True).fill("https://example.test/poster.jpg")
    await capture("events-materials")
    await text200("events-materials")
    event_failures["create"] = "SERVICE_UNAVAILABLE"
    await page.get_by_role("button", name="Simpan draf & lanjutkan", exact=True).click()
    await expect(page.get_by_role("alert")).to_contain_text("Simpan gagal")
    await expect(page.locator("#event-section-title")).to_have_text("Materi")
    event_failures["create"] = "DUPLICATE_EVENT"
    await page.get_by_role("button", name="Simpan draf & lanjutkan", exact=True).click()
    await expect(page.locator("#event-id")).to_be_focused()
    await page.locator("#event-id").fill("id-manual-baru")
    await part(2)
    await expect(page.get_by_label("URL poster", exact=True)).to_have_value("https://example.test/poster.jpg")
    await page.get_by_role("button", name="Simpan draf & lanjutkan", exact=True).click()
    await expect(page.locator("#event-editor-title")).to_have_text("Tambah konser")
    await expect(page.locator("#event-section-title")).to_have_text("Zona")
    await expect(page.get_by_text("Draf konser dibuat.", exact=False)).to_be_visible()
    await expect(page.locator("#event-id")).to_have_count(0)
    await page.get_by_label("Nama zona", exact=True).fill("Tidak disimpan")
    await page.get_by_role("button", name="Berikutnya", exact=True).click()
    await expect(page.get_by_role("button", name="Simpan & lanjutkan", exact=True)).to_be_visible()
    await page.get_by_role("button", name="Tetap di sini", exact=True).click()
    if width < 768:
        await page.get_by_role("combobox", name="Pilih langkah", exact=True).select_option("2")
        await expect(page.get_by_role("combobox", name="Pilih langkah", exact=True)).to_have_value("3")
        await page.get_by_role("button", name="Tetap di sini", exact=True).click()
    await expect(page.get_by_label("Nama zona", exact=True)).to_have_value("Tidak disimpan")
    await page.get_by_role("button", name="Berikutnya", exact=True).click()
    await page.get_by_role("button", name="Buang & lanjutkan", exact=True).click()
    await expect(page.get_by_text("Tambahkan zona pada langkah Zona", exact=False)).to_be_visible()
    await expect(page.get_by_role("button", name="Tambah kategori", exact=True)).to_be_disabled()
    await page.get_by_role("button", name="Berikutnya", exact=True).click()
    await expect(page.get_by_role("button", name="Terbitkan konser", exact=True)).to_be_disabled()
    await part(3)
    await page.get_by_label("Nama zona", exact=True).fill("Zona Uji")
    event_failures["refresh"] = True
    await page.get_by_role("button", name="Berikutnya", exact=True).click()
    await page.get_by_role("button", name="Simpan & lanjutkan", exact=True).click()
    await expect(page.locator("#event-section-title")).to_have_text("Kategori tiket")
    await expect(page.get_by_text("Zona tersimpan; daftar belum dapat dimuat ulang.", exact=True)).to_be_visible()
    await part(3)
    await page.locator('[data-section="3"] > label select').select_option("")
    await page.get_by_label("Nama zona", exact=True).fill("Zona Kedua")
    await expect(page.locator("#zone-id")).to_have_value("zona-kedua")
    await page.get_by_role("button", name="Tambah zona", exact=True).click()
    await expect(page.get_by_text("Zona tersimpan.", exact=True)).to_be_visible()
    await page.get_by_label("Deskripsi zona", exact=True).fill("Deskripsi zona diperbarui")
    await page.get_by_role("button", name="Simpan zona", exact=True).click()
    await expect(page.get_by_text("Zona tersimpan.", exact=True)).to_be_visible()
    assert events[-1]["zones"][-1]["description"] == "Deskripsi zona diperbarui"
    await page.get_by_role("button", name="Berikutnya", exact=True).click()
    for suffix in ("Uji", "Kedua"):
        if suffix == "Kedua": await page.locator('[data-section="4"] > label select').select_option("")
        await page.get_by_label("Nama kategori", exact=True).fill("Kategori " + suffix)
        await expect(page.locator("#tier-id")).to_have_value("kategori-" + suffix.lower())
        await page.locator("#tier-form").get_by_role("combobox").first.select_option("zona-uji")
        await page.get_by_label("Harga (rupiah)", exact=True).fill("100000")
        await page.get_by_label("Kapasitas", exact=True).fill("20")
        await page.get_by_label("Gate", exact=True).fill("Gate " + suffix)
        await page.get_by_role("button", name="Tambah kategori", exact=True).click()
        await expect(page.get_by_text("Kategori tiket tersimpan.", exact=True)).to_be_visible()
    await page.get_by_label("Benefit", exact=True).fill("Benefit kategori diperbarui")
    await page.get_by_role("button", name="Simpan kategori", exact=True).click()
    await expect(page.get_by_text("Kategori tiket tersimpan.", exact=True)).to_be_visible()
    assert events[-1]["ticketTiers"][-1]["benefit"] == "Benefit kategori diperbarui"
    await capture("events-tickets")
    await text200("events-tickets")
    await page.get_by_role("button", name="Berikutnya", exact=True).click()
    await capture("events-review")
    await text200("events-review")
    await page.get_by_role("button", name="Terbitkan konser", exact=True).click()
    await expect(page.get_by_text("Konser berhasil diterbitkan.", exact=True)).to_be_visible()
    await expect(page.locator("#event-list-title")).to_be_focused()  # Existing search excludes the new concert.
    assert events[-1]["publicationStatus"] == "PUBLISHED" and len(events[-1]["ticketTiers"]) == 2
    assert sum(1 for method, path, value in mutations if path == "admin/events" and value["id"] == "id-manual-baru") == 1
    # An incomplete concert can finish as a draft and resume through the feature editor.
    await page.locator("#event-create-opener").click()
    await page.get_by_role("button", name="Berikutnya", exact=True).click()
    await expect(page.get_by_label("Nama konser / artis", exact=True)).to_be_focused()
    await page.get_by_label("Nama konser / artis", exact=True).fill("Draf lanjutan")
    for _ in range(2): await page.get_by_role("button", name="Berikutnya", exact=True).click()
    await page.get_by_role("button", name="Simpan draf & lanjutkan", exact=True).click()
    for _ in range(2): await page.get_by_role("button", name="Berikutnya", exact=True).click()
    await page.get_by_role("button", name="Simpan sebagai draf", exact=True).click()
    await expect(page.get_by_text("Draf konser tersimpan.", exact=False)).to_be_visible()
    await page.get_by_label("Cari konser", exact=True).fill("Draf lanjutan")
    await page.get_by_role("button", name="Kelola konser Draf lanjutan", exact=True).click()
    await expect(page.locator('.event-section-nav svg')).to_have_count(7)
    await part(1)
    await page.get_by_label("Kota", exact=True).fill("Belum lengkap")
    await part(0)
    await expect(page.get_by_label("Kota", exact=True)).to_have_value("Belum lengkap")
    await part(5)
    await page.get_by_role("combobox", name="Status publikasi", exact=True).select_option("PUBLISHED")
    await page.get_by_role("button", name="Simpan perubahan", exact=True).click()
    await expect(page.locator("#event-section-title")).to_have_text("Jadwal & lokasi")
    await expect(page.get_by_label("Jadwal (WIB)", exact=True)).to_be_focused()
    page.once("dialog", lambda d: d.accept())
    await page.goto(base + "/admin/orders")
    await expect(page.locator(".order-list-panel li, .order-list-panel tbody tr")).to_have_count(5 if width < 768 else 10)
    await expect(page.get_by_role("button", name="Cari", exact=True)).to_have_count(0 if width < 768 else 1)
    if width >= 768: await expect(page.get_by_role("columnheader", name="Nomor pesanan", exact=True)).to_be_visible()
    if width < 768:
        await expect(page.locator("#order-search-form")).to_be_hidden()
        await capture("orders-search-collapsed")
        await text200("orders-search-collapsed")
        toggle = page.locator(".order-search-toggle")
        await toggle.focus()
        await page.keyboard.press("Enter")
        await expect(toggle).to_have_attribute("aria-expanded", "true")
        await page.get_by_label("Nomor pesanan", exact=True).fill("Draft pencarian")
        await capture("orders-search-expanded")
        await toggle.focus()
        await page.keyboard.press("Enter")
        await expect(page.locator("#order-search-form")).to_be_hidden()
        await expect(toggle).to_be_focused()
        await toggle.click()
        await expect(page.get_by_label("Nomor pesanan", exact=True)).to_have_value("Draft pencarian")
        await page.get_by_label("Nomor pesanan", exact=True).fill("")
        await toggle.click()
        await expect(page.get_by_role("button", name="Sebelumnya", exact=True)).to_be_disabled()
        await expect(page.locator(".order-list-panel .admin-mobile-card")).to_have_count(5)
        await expect(page.locator(".order-list-panel li").first.locator("dt").filter(has_text="Tanggal")).to_have_count(1)
        await capture("orders-mobile-first-group")
        await text200("orders-mobile-first-group")
        seen = []
        for index, count in enumerate((5, 5, 5, 5, 2)):
            await expect(page.locator(".order-list-panel li, .order-list-panel tbody tr")).to_have_count(count)
            references = await page.locator(".admin-mobile-card-header strong").all_text_contents()
            assert references == [o["reference"] for o in orders[index * 5:index * 5 + count]]
            seen.extend(references)
            if index == 1:
                # A failed cursor request keeps the current page; retry cannot submit twice.
                await page.get_by_role("button", name="Berikutnya", exact=True).click()
                await expect(page.get_by_role("alert")).to_contain_text("Halaman berikutnya gagal dimuat")
                assert await page.locator(".admin-mobile-card-header strong").all_text_contents() == references
                await page.get_by_role("button", name="Berikutnya", exact=True).evaluate("button => { button.click(); button.click(); }")
                await expect(page.locator(".order-list-panel [role=status]").filter(has_text="Menampilkan")).to_contain_text("Menampilkan 11 sampai 15")
                assert sum(1 for method, path, params in requests if path == "admin/orders" and "cursor" in params) == 2
            elif index < 4:
                await page.get_by_role("button", name="Berikutnya", exact=True).click()
                await expect(page.locator("#order-results-title")).to_be_focused()
        assert len(seen) == len(set(seen)) == 22
        await expect(page.get_by_role("button", name="Berikutnya", exact=True)).to_be_disabled()
        await expect(page.locator(".order-list-panel [role=status]").filter(has_text="pesanan dimuat")).to_contain_text("22 pesanan dimuat")
        table = page.locator(".order-list-panel .mobile-list-viewport")
        assert await page.locator(".order-list-panel ul").evaluate("el => getComputedStyle(el).display") == "flex"
        assert await page.locator(".order-list-panel li, .order-list-panel tbody tr").first.evaluate("el => getComputedStyle(el).display") == "grid"
        assert await table.evaluate("el => el.scrollWidth > el.clientWidth")
        await table.focus()
        await page.keyboard.press("ArrowRight")
        await page.wait_for_function("document.querySelector('.order-list-panel .mobile-list-viewport').scrollLeft > 0")
        await table.evaluate("el => el.scrollLeft = el.scrollWidth")
        table_scroll = await table.evaluate("el => el.scrollLeft")
    else:
        await page.get_by_role("button", name="Muat berikutnya", exact=True).click()
        await expect(page.locator(".order-list-panel li, .order-list-panel tbody tr")).to_have_count(22)
        await expect(page.get_by_text("22 pesanan dimuat", exact=True)).to_be_visible()
        assert await page.locator(".order-list-panel li, .order-list-panel tbody tr").evaluate_all("rows => new Set(rows.map(r => r.querySelector('code').textContent)).size") == 22
    list_count = 2 if width < 768 else 22
    await capture("orders-list")
    await text200("orders-list")
    order_trigger = page.locator(f"#order-detail-{orders[-1]['id']}")
    await order_trigger.scroll_into_view_if_needed()
    list_scroll = await page.evaluate("scrollY")
    await order_trigger.click()
    await expect(page.locator("#order-detail-title")).to_be_focused()
    await expect(page.locator(".order-list-panel")).to_have_count(0)
    await expect(page.locator("#order-search-form")).to_have_count(0)
    await expect(page.locator(".order-detail-grid")).to_be_visible()
    await capture("orders-detail")
    await text200("orders-detail")
    await page.get_by_role("button", name="Kembali ke daftar", exact=True).click()
    await expect(order_trigger).to_be_focused()
    assert abs(await page.evaluate("scrollY") - list_scroll) <= 2
    await expect(page.locator(".order-list-panel li, .order-list-panel tbody tr")).to_have_count(list_count)
    # Detail load failures still provide a focused title and a way back to the retained list.
    detail_failed = True
    await order_trigger.click()
    await expect(page.locator("#order-detail-title")).to_be_focused()
    await expect(page.get_by_role("alert")).to_contain_text("Detail gagal dimuat")
    await page.get_by_role("button", name="Kembali ke daftar", exact=True).click()
    detail_failed = False
    await expect(page.locator(".order-list-panel li, .order-list-panel tbody tr")).to_have_count(list_count)

    if width < 768:
        assert abs(await table.evaluate("el => el.scrollLeft") - table_scroll) <= 2
        for _ in range(4):
            await page.get_by_role("button", name="Sebelumnya", exact=True).click()
        await expect(page.locator(".order-list-panel li, .order-list-panel tbody tr")).to_have_count(5)
        await expect(page.get_by_role("button", name="Sebelumnya", exact=True)).to_be_disabled()

    # Refund drafts require confirmation; unknown mutations stay locked across list/detail switches.
    await page.locator(f"#order-detail-{orders[0]['id']}").click()
    reason = page.get_by_label("Alasan refund", exact=True)
    await reason.fill("Permintaan admin")
    page.once("dialog", lambda d: d.dismiss())
    await page.get_by_role("button", name="Kembali ke daftar", exact=True).click()
    await expect(reason).to_have_value("Permintaan admin")
    page.once("dialog", lambda d: d.accept())
    await page.get_by_role("button", name="Ajukan refund", exact=True).click()
    await expect(page.get_by_role("button", name="Ajukan refund", exact=True)).to_be_disabled()
    page.once("dialog", lambda d: d.accept())
    await page.get_by_role("button", name="Kembali ke daftar", exact=True).click()
    await page.locator(f"#order-detail-{orders[0]['id']}").click()
    await expect(reason).to_have_value("")
    await reason.fill("Permintaan ulang ditahan")
    await expect(page.get_by_role("button", name="Ajukan refund", exact=True)).to_be_disabled()
    await page.locator(".recovery-panel").dispatch_event("submit")
    assert len(refund_calls) == 1
    await page.get_by_role("button", name="Periksa detail refund", exact=True).click()
    await expect(page.get_by_role("button", name="Ajukan refund", exact=True)).to_be_disabled()
    refund_found = True
    await page.get_by_role("button", name="Periksa detail refund", exact=True).click()
    await expect(page.locator(".order-selected-panel")).to_contain_text("Permintaan admin")
    page.once("dialog", lambda d: d.accept())
    await page.get_by_role("button", name="Kembali ke daftar", exact=True).click()
    await expect(page.locator(".order-list-panel li, .order-list-panel tbody tr")).to_have_count(5 if width < 768 else 22)
    await expect(page.locator(".order-list-panel li, .order-list-panel tbody tr").first).to_contain_text("Refund diproses")
    if width < 768: await page.get_by_role("button", name="Buka pencarian pesanan", exact=True).click()
    await page.get_by_label("Nomor pesanan", exact=True).fill("absent")
    await page.get_by_role("button", name="Cari", exact=True).click()
    await expect(page.locator(".order-list-panel li, .order-list-panel tbody tr")).to_have_count(0)
    await capture("orders-empty")
    assert requests[-1][2].get("q") == ["absent"]
    await page.get_by_role("button", name="Reset", exact=True).click()
    await expect(page.locator(".order-list-panel li, .order-list-panel tbody tr")).to_have_count(5 if width < 768 else 10)
    if width < 768:
        await expect(page.get_by_role("button", name="Sebelumnya", exact=True)).to_be_disabled()
    assert not errors, errors
    (output / f"workflow-{width}.json").write_text(json.dumps({"automated": True, "humanUAT": "not performed", "events": 20, "orders": 22, "audits": audits, "browserErrors": errors, "mutations": mutations}, indent=2))
    await context.close()
    print(f"PASS {width}: 20 concerts, create/inventory/publish, preserved drafts/focus, 2 cursor pages, 22 unique orders, table/mobile detail")


async def check_manual_refund(browser, base, output, width):
    context = await browser.new_context(viewport={"width": width, "height": 900}, timezone_id="America/Los_Angeles")
    await context.add_init_script(SESSION_INIT)
    page = await context.new_page()
    errors, calls = [], []
    page.on("pageerror", lambda e: errors.append(str(e)))
    _, detail = body(base + f"/api/v1/admin/orders/{ORDER}", "standard", "ADMIN")
    refund = {"status": "MANUAL_REQUIRED", "amount": 123456, "reason": "Transfer penuh diperlukan", "manualReference": "", "manualPaidAt": None}
    mode = "success"
    started, release = asyncio.Event(), asyncio.Event()
    async def fixture(route):
        path = urlparse(route.request.url).path
        if path.endswith("/staff/me"): await route.fulfill(json={"staff": ADMIN})
        elif path.endswith("/refund/manual"):
            calls.append(route.request.post_data_json)
            if mode == "delayed":
                started.set(); await release.wait()
            if mode == "unknown": await route.abort("connectionreset")
            else:
                refund.update(status="SUCCEEDED", manualReference=calls[-1]["reference"], manualPaidAt=calls[-1]["paidAt"])
                await route.fulfill(json=refund)
        elif path.endswith("/admin/orders"):
            await route.fulfill(json={"items": [], "nextCursor": None, "filterOptions": {"events": []}})
        elif "/admin/orders/" in path:
            await route.fulfill(json={**detail, "id": path.split("/")[-1], "status": "REFUND_PENDING", "refund": refund})
        else: raise AssertionError(path)
    await page.route("**/api/v1/**", fixture)
    await page.goto(base + "/admin/orders?orderId=" + ORDER)
    form = page.locator("#manual-refund-form")
    next_step = form.get_by_role("button", name="Lanjutkan", exact=True)
    final = form.get_by_role("button", name="Catat refund manual selesai", exact=True)
    evidence = [("Referensi transfer", "BANK-" + "X" * 140), ("Waktu transfer (WIB)", "2026-10-13T14:23:45"), ("Catatan bukti", "Transfer sudah diperiksa. " * 15)]
    audit_js = run_path(str(Path(__file__).with_name("staff-review-check.py")))["STYLE_AUDIT"]
    async def capture(label):
        audit = await page.evaluate(audit_js)
        assert not audit["failures"] and not audit["overflow"], (width, label, audit)
        targets = await form.locator('button, input:not([type="checkbox"]), textarea').evaluate_all("els => els.filter(el => el.getClientRects().length).every(el => el.getBoundingClientRect().height >= 44)")
        assert targets
        await page.screenshot(path=str(output / f"manual-{label}-{width}.png"), full_page=True)
        await page.evaluate("""() => {
          window.__manualFonts = [...document.querySelectorAll('.admin-layout *')].map(el => [el, el.style.fontSize, getComputedStyle(el).fontSize]);
          for (const [el, , size] of window.__manualFonts) el.style.fontSize = parseFloat(size) * 2 + 'px';
        }""")
        audit = await page.evaluate(audit_js)
        if audit["overflow"]:
            audit["elements"] = await page.evaluate("() => [...document.querySelectorAll('.admin-layout *')].filter(el => el.getClientRects().length && el.getBoundingClientRect().right > innerWidth + 1).map(el => [el.tagName, el.className, el.textContent.slice(0,70), el.getBoundingClientRect().right]).slice(-15)")
        assert not audit["failures"] and not audit["overflow"], (width, label, "text200", audit)
        await page.screenshot(path=str(output / f"manual-{label}-text200-{width}.png"), full_page=True)
        await page.evaluate("() => { for (const [el, size] of window.__manualFonts) el.style.fontSize = size; }")
    await next_step.click()
    await expect(form.get_by_label("Referensi transfer", exact=True)).to_be_focused()
    assert not calls
    for label, value in evidence: await form.get_by_label(label, exact=True).fill(value)
    first, second = form.get_by_label(evidence[0][0], exact=True), form.get_by_label(evidence[1][0], exact=True)
    is_two = await second.evaluate("el => el.getBoundingClientRect().left") > await first.evaluate("el => el.getBoundingClientRect().left") + 1
    metrics = await form.evaluate("el => [el.clientWidth, getComputedStyle(el).fontSize, getComputedStyle(el).containerType, getComputedStyle(el.querySelector('.manual-evidence')).gridTemplateColumns]")
    assert is_two == (metrics[0] >= float(metrics[1].removesuffix("px")) * 35), (width, is_two, metrics)
    await capture("evidence")
    await first.press("Enter")
    await expect(page.locator("#manual-step-title")).to_have_text("Tinjau & konfirmasi")
    await expect(form.locator(".manual-review")).to_contain_text("Rp 123.456")
    await expect(form.locator(".manual-review")).to_contain_text(evidence[0][1])
    await expect(form).to_contain_text("tidak mengirim uang")
    await expect(final).to_be_disabled()
    assert not calls
    await capture("review")
    checkbox = form.get_by_role("checkbox")
    await checkbox.check()
    await form.get_by_role("button", name="Sebelumnya", exact=True).click()
    for label, value in evidence: await expect(form.get_by_label(label, exact=True)).to_have_value(value)
    await next_step.click()
    await expect(checkbox).not_to_be_checked()
    await checkbox.check()
    page.once("dialog", lambda d: d.dismiss())
    await final.click()
    assert not calls
    mode = "delayed"
    page.once("dialog", lambda d: d.accept())
    await final.click()
    await asyncio.wait_for(started.wait(), 10)
    await form.dispatch_event("submit")
    assert len(calls) == 1
    release.set()
    await expect(page.locator(".order-selected-panel")).to_contain_text("Refund manual tercatat")
    assert calls[0] == {"reference": evidence[0][1], "paidAt": "2026-10-13T14:23:45+07:00", "note": evidence[2][1].strip()}, calls
    refund.update(status="MANUAL_REQUIRED", manualReference="", manualPaidAt=None)
    await page.goto(base + "/admin/orders?orderId=" + "b" * 32)
    await expect(form.get_by_label("Referensi transfer", exact=True)).to_have_value("")
    for label, value in evidence: await form.get_by_label(label, exact=True).fill(value)
    await next_step.click(); await checkbox.check()
    mode = "unknown"
    page.once("dialog", lambda d: d.accept())
    await final.click()
    await expect(final).to_be_disabled()
    await form.dispatch_event("submit")
    assert len(calls) == 2
    await form.get_by_role("button", name="Periksa detail refund", exact=True).click()
    await expect(final).to_be_disabled()
    assert len(calls) == 2
    refund.update(status="SUCCEEDED", manualReference=evidence[0][1], manualPaidAt=calls[-1]["paidAt"])
    await form.get_by_role("button", name="Periksa detail refund", exact=True).click()
    await expect(page.locator(".order-selected-panel")).to_contain_text("Bukti transfer sudah tercatat.")
    assert len(calls) == 2 and not errors, errors
    page.once("dialog", lambda d: d.accept())
    await page.get_by_role("button", name="Kembali ke daftar", exact=True).click()
    await context.close()
    print(f"PASS {width}: manual refund evidence/review, authoritative amount, confirmation reset, single mutation, unknown outcome lock, 200% text")


async def check_mobile_lists(browser, base, output, width):
    context = await browser.new_context(viewport={"width": width, "height": 900}, is_mobile=width < 768, has_touch=width < 768, timezone_id="America/Los_Angeles")
    await context.add_init_script(SESSION_INIT)
    page = await context.new_page()
    errors, cursor_calls, requests, audits = [], {}, [], []
    page.on("pageerror", lambda error: errors.append(str(error)))
    endpoints = {
        "/admin/events": ("events", 20), "/admin/staff": ("staff", 20),
        "/admin/payment-cases": ("items", 22), "/admin/email-jobs": ("items", 22),
        "/admin/check-ins": ("items", 22), "/admin/orders": ("items", 22),
    }

    async def fixture(route):
        url = urlparse(route.request.url)
        endpoint = url.path.removeprefix("/api/v1")
        requests.append((route.request.method, endpoint, url.query))
        assert route.request.method == "GET", "List navigation must not mutate data"
        status, data = body(route.request.url, "standard", "ADMIN")
        if endpoint == "/admin/pricing":
            voucher = {"code": "HEMAT10", "kind": "PERCENT", "value": 10, "startsAt": "2020-01-01T00:00:00Z", "endsAt": "2099-01-01T00:00:00Z", "active": True, "eventId": "", "minimumSubtotal": 0, "quota": 1000000000, "revision": 1, "usage": {"held": 1000, "consumed": 2000, "released": 3000}}
            status, data = 200, {"settings": {"adminFee": 7500, "revision": 1}, "history": [], "vouchers": [{**voucher, "code": f"PROMO_LIBURAN_AKHIR_TAHUN_{i:02d}"} for i in range(12)]}
        elif endpoint.startswith("/admin/payment-cases/"):
            status, data = body(base + "/api/v1/admin/payment-cases/17", "standard", "ADMIN")
            data["id"] = endpoint.split("/")[-1]
        elif endpoint.startswith("/admin/email-jobs/"):
            status, data = body(base + "/api/v1/admin/email-jobs/" + "e" * 32, "standard", "ADMIN")
            data["id"] = endpoint.split("/")[-1]
        elif endpoint.startswith("/admin/orders/"):
            status, data = body(base + "/api/v1/admin/orders/" + ORDER, "standard", "ADMIN")
            data["id"] = endpoint.split("/")[-1]
        if endpoint in endpoints:
            key, total = endpoints[endpoint]
            seed = data[key][0]
            rows = [{**deepcopy(seed), "id": (str(i + 100) if endpoint in ("/admin/check-ins", "/admin/payment-cases") else f"concert-{i}" if endpoint == "/admin/events" else f"{i + 100:032x}"), **({"name": f"Petugas {i:02d}", "role": "STAFF"} if key == "staff" else {})} for i in range(total)]
            if endpoint == "/admin/events":
                rows[0]["artist"] = "Konser dengan nama panjang untuk pemeriksaan kartu mobile"
                rows[0]["ticketTiers"][0].update(capacity=1000000000, availableQuantity=999999942)
            elif endpoint == "/admin/staff":
                for i, person in enumerate(rows):
                    person.update(role="ADMIN" if i == 0 else "STAFF", active=i % 4 != 3,
                                  email="nama.petugas.yang.sangat.panjang@contoh-domain-pengujian.example.test" if i == 1 else f"petugas{i}@example.test",
                                  assignments=[{"eventId": "concert-0", "gate": "Gate B"}] if i % 3 == 2 else [])
            elif endpoint == "/admin/check-ins":
                outcomes = ["CHECKED_IN", "TICKET_ALREADY_USED", "INVALID_REQUEST", "TICKET_NOT_FOUND", "ORDER_NOT_PAID", "WRONG_GATE", "FORBIDDEN", "EVENT_CHANGED"]
                for i, item in enumerate(rows):
                    item.update(outcome=outcomes[i % len(outcomes)], code=None if i == 2 else TICKET["code"],
                                eventName="Konser dengan nama panjang untuk pemeriksaan kartu riwayat check-in",
                                recordedAt="2026-10-09T18:00:37Z", checkedInAt="2026-10-09T18:00:37Z" if i % len(outcomes) in (0, 1) else None)
            if endpoint in ("/admin/events", "/admin/staff"):
                data[key] = rows
            else:
                cursor = bool(url.query)
                if cursor:
                    cursor_calls[endpoint] = cursor_calls.get(endpoint, 0) + 1
                    if cursor_calls[endpoint] == 1:
                        await route.fulfill(status=503, json={"error": {"message": "Halaman berikutnya gagal dimuat."}})
                        return
                data[key] = rows[10:] if cursor else rows[:11]
                data["nextCursor"] = None if cursor else "100" if endpoint == "/admin/check-ins" else "next-page"
        await route.fulfill(status=status, json=data)

    await page.route("**/api/v1/**", fixture)
    specs = [("events", "event-list-title", ".event-admin-list li", 20), ("staff", "staff-list-title", "[id^=staff-account-]", 20),
             ("orders", "order-results-title", ".admin-mobile-list > li, tbody tr", 11),
             ("pricing", "list-title", ".voucher-list > li, .voucher-table tbody tr", 12),
             ("issues", "case-list-title", ".admin-mobile-list > li, tbody tr", 11), ("issues", "email-list-title", ".admin-mobile-list > li, tbody tr", 11),
             ("check-ins", "history-results-title", ".admin-mobile-list > li, tbody tr", 11)]
    for route, heading, row_selector, count in specs:
        await page.goto(base + "/admin/" + route)
        section = page.locator(f'section[aria-labelledby="{heading}"]')
        rows = section.locator(row_selector)
        await expect(rows).to_have_count(5 if width < 768 else count)
        if width < 768:
            await expect(section.locator(".admin-mobile-card")).to_have_count(5)
            first = rows.first
            assert await first.evaluate("el => getComputedStyle(el).borderTopStyle") == "solid"
            assert await first.locator("dl.admin-card-fields").evaluate_all("els => els.every(el => getComputedStyle(el).gridTemplateColumns.split(' ').length === 1)")
            await expect(first.locator("dl.admin-card-fields")).to_have_count(2 if route == "pricing" else 1)
            assert await first.evaluate("el => el.tagName === 'LI' && el.parentElement.tagName === 'UL'")
            assert await first.evaluate("el => getComputedStyle(el).padding === '16px'")
            assert await first.locator("dt").count() == await first.locator("dd").count()
            if route == "events":
                await expect(first.locator("dd").nth(4)).to_have_text("1.000.000.000")
                await expect(first.locator("dd").nth(5)).to_have_text("999.999.942")
            if heading == "email-list-title":
                await expect(first.locator("time .admin-card-date")).to_have_count(2)
            if route != "check-ins":
                action = rows.nth(1).locator("button").first
                assert await action.evaluate("el => { const button = el.getBoundingClientRect(), card = el.closest('li').getBoundingClientRect(); return button.height >= 44 && Math.abs(button.width - card.width + 34) <= 2; }")
            else:
                await expect(rows.locator("button, a")).to_have_count(0)
                await expect(first.locator("time .admin-card-date")).to_have_text(["10 Okt 2026", "01.00.37 WIB", "10 Okt 2026", "01.00.37 WIB"])
                await expect(first.locator(".admin-order-status.is-paid")).to_have_text("Check-in berhasil")
                await expect(rows.nth(1).locator(".admin-order-status.is-expired")).to_have_text("Tiket sudah digunakan")
                await expect(rows.nth(2).locator("dd").last).to_have_text("Belum tersedia")
                await expect(rows.nth(2).locator("strong")).to_have_text("Kode tidak valid")
            if route == "staff":
                assert await first.locator(".admin-card-fields > div").last.evaluate("el => { const label = el.querySelector('dt').getBoundingClientRect(), value = el.querySelector('dd').getBoundingClientRect(); return Math.abs(label.y - value.y) <= 1 || getComputedStyle(el).gridTemplateColumns.split(' ').length === 1; }")
                await expect(first.locator("button")).to_have_count(0)
                await expect(first.locator("dt")).to_contain_text(["Email", "Peran", "Penugasan"])
            if route == "pricing":
                await expect(first.locator('[aria-label="Penggunaan voucher"] dd')).to_have_text(["1.000", "2.000", "3.000"])
            badges = first.locator(".admin-order-status, .staff-status")
            assert await badges.count() > 0
            assert await badges.evaluate_all("els => els.every(el => el.tabIndex < 0 && getComputedStyle(el).cursor !== 'pointer')")
            # Text enlargement must keep label/value pairs and actions inside each card.
            await page.evaluate("""() => {
              window.__cardFonts = [...document.querySelectorAll('.admin-layout *')].map(el => [el, el.style.fontSize, getComputedStyle(el).fontSize]);
              for (const [el, , size] of window.__cardFonts) el.style.fontSize = parseFloat(size) * 2 + 'px';
            }""")
            assert await first.evaluate("el => el.scrollWidth <= el.clientWidth + 1"), (heading, width, "200% text overflows card")
            assert await page.evaluate("document.documentElement.scrollWidth <= innerWidth"), (heading, width, "200% text overflows viewport")
            await page.screenshot(path=str(output / f"mobile-cards-{heading}-text200-{width}.png"), full_page=True, animations="disabled")
            await page.evaluate("() => { for (const [el, original] of window.__cardFonts) el.style.fontSize = original; }")
            viewport = section.locator(".mobile-list-viewport")
            await expect(viewport).to_have_attribute("tabindex", "0")
            assert await page.evaluate("innerWidth === document.documentElement.clientWidth && document.documentElement.scrollWidth <= innerWidth"), "Cards expanded the mobile viewport"
            await rows.first.locator("strong, code, td").first.scroll_into_view_if_needed()
            box = await viewport.bounding_box()
            target = await rows.first.locator("strong, code, td").first.bounding_box()
            start, end, y = box["x"] + box["width"] - 20, box["x"] + 20, target["y"] + target["height"] / 2
            # Mouse dragging in mobile simulators must work as well as native touch.
            await page.mouse.move(start, y)
            await page.mouse.down()
            await page.mouse.move(end, y, steps=20)
            await page.mouse.up()
            await page.wait_for_function("id => document.querySelector(`section[aria-labelledby='${id}'] .mobile-list-viewport`).scrollLeft > 0", arg=heading)
            await expect(page.locator("#event-editor-title, #staff-edit-title, #case-detail-title, #email-detail-title")).to_have_count(0)
            await viewport.evaluate("el => el.scrollTo({left: 0, behavior: 'instant'})")
            client = await context.new_cdp_session(page)
            await client.send("Input.dispatchTouchEvent", {"type": "touchStart", "touchPoints": [{"x": start, "y": y}]})
            for step in range(1, 21):
                await client.send("Input.dispatchTouchEvent", {"type": "touchMove", "touchPoints": [{"x": start + (end - start) * step / 20, "y": y}]})
                await page.wait_for_timeout(15)
            await client.send("Input.dispatchTouchEvent", {"type": "touchEnd", "touchPoints": []})
            await page.wait_for_function("id => document.querySelector(`section[aria-labelledby='${id}'] .mobile-list-viewport`).scrollLeft > 0", arg=heading)
            await client.detach()
            await viewport.evaluate("el => el.scrollTo({left: 0, behavior: 'instant'})")
            await viewport.focus()
            await page.keyboard.press("ArrowRight")
            await page.wait_for_function("id => document.querySelector(`section[aria-labelledby='${id}'] .mobile-list-viewport`).scrollLeft > 0", arg=heading)
            await section.get_by_role("button", name="Berikutnya", exact=True).click()
            await expect(page.locator("#" + heading)).to_be_focused()
            await expect(section.get_by_role("status").filter(has_text="Menampilkan")).to_contain_text("6 sampai 10")
            await expect(rows).to_have_count(5)
            if route != "check-ins":
                trigger = rows.nth(2).get_by_role("button")
                await trigger.scroll_into_view_if_needed()
                await page.wait_for_timeout(150)
                left = await viewport.evaluate("el => el.scrollLeft")
                top = await page.evaluate("scrollY")
                await trigger.click()
                title = {"events": "event-editor-title", "staff": "staff-edit-title", "orders": "order-detail-title", "issues": "issue-detail-title", "pricing": "voucher-title"}[route]
                await expect(page.locator("#" + title)).to_be_focused()
                await page.get_by_role("button", name="Kembali ke daftar", exact=True).click()
                await expect(trigger).to_be_focused()
                assert abs(await viewport.evaluate("el => el.scrollLeft") - left) <= 2, "Returning from editor moved the selected card"
                assert abs(await page.evaluate("scrollY") - top) <= 2, "Returning from detail moved the page"
            if route == "pricing":
                await section.get_by_role("button", name="Berikutnya", exact=True).click()
                await expect(rows).to_have_count(2)
                await expect(section.get_by_role("button", name="Berikutnya", exact=True)).to_be_disabled()
            if count == 11:
                await section.get_by_role("button", name="Berikutnya", exact=True).click()
                await expect(rows).to_have_count(1)
                await expect(section.get_by_role("status").filter(has_text="Menampilkan")).to_contain_text("11 sampai 11")
                endpoint = "/admin/" + ({"case-list-title": "payment-cases", "email-list-title": "email-jobs", "order-results-title": "orders"}.get(heading, "check-ins"))
                assert endpoint not in cursor_calls, "Remaining loaded cards must not require a request"
                await section.get_by_role("button", name="Berikutnya", exact=True).click()
                await expect(page.get_by_role("alert")).to_contain_text("gagal dimuat")
                await expect(rows).to_have_count(1)
                await expect(section.get_by_role("status").filter(has_text="Menampilkan")).to_contain_text("11 sampai 11")
                await section.get_by_role("button", name="Berikutnya", exact=True).click()
                await expect(rows).to_have_count(5)
                await expect(section.get_by_role("status").filter(has_text="Menampilkan")).to_contain_text("11 sampai 15")
                await expect(page.get_by_role("alert")).to_have_count(0)
            await section.get_by_role("button", name="Sebelumnya", exact=True).click()
        else:
            assert await section.locator(".history-table-wrap" if route == "pricing" else ".mobile-list-viewport").evaluate("el => getComputedStyle(el).scrollSnapType === 'none'")
        assert await page.evaluate("document.documentElement.scrollWidth <= innerWidth"), (route, width)
        await page.evaluate("scrollTo({top: 0, behavior: 'instant'})")
        if width < 768:
            await page.wait_for_function("id => { const viewport = document.querySelector(`section[aria-labelledby='${id}'] .mobile-list-viewport`); return viewport.scrollLeft === 0; }", arg=heading)
            await expect(rows.first).to_be_visible()
        await page.screenshot(path=str(output / f"mobile-lists-{heading}-{width}.png"), full_page=True, animations="disabled")
        audits.append(heading)
    assert not errors, errors
    (output / f"mobile-lists-{width}.json").write_text(json.dumps({"width": width, "automated": True, "humanUAT": "not performed", "lists": audits, "requests": requests, "browserErrors": errors}, indent=2))
    await context.close()
    print(f"PASS {width}: " + ("five-card groups, mouse dragging, native touch swipe, keyboard scrolling, focus restoration, partial groups, failed cursor retry, viewport containment" if width < 768 else "full desktop lists, no carousel snapping, viewport containment"))

    await check_worker_cards(browser, base, output, width)


async def check_worker_cards(browser, base, output, width):
    context = await browser.new_context(viewport={"width": width, "height": 900}, timezone_id="America/Los_Angeles")
    await context.add_init_script(SESSION_INIT)
    page = await context.new_page()
    mode, calls, errors = "standard", [], []
    page.on("pageerror", lambda error: errors.append(str(error)))

    async def fixture(route):
        assert route.request.method == "GET", "Operational checks must not mutate data"
        endpoint = urlparse(route.request.url).path.removeprefix("/api/v1")
        if endpoint == "/admin/operations":
            calls.append(mode)
            if mode == "stale":
                await route.fulfill(status=503, json={"error": {"message": "Snapshot belum tersedia."}})
                return
        status, data = body(route.request.url, "empty" if mode == "empty" else "standard", "ADMIN")
        if endpoint == "/admin/operations" and mode != "empty":
            worker = data["workers"][0]
            data.update(heldTickets=1234, workers=[
                {**worker, "name": "payment-reconcile-worker-dengan-nama-panjang-untuk-pemeriksaan-mobile", "running": True,
                 "lastFinishedAt": "2026-10-09T18:00:37Z", "lastSuccessAt": "2026-10-09T18:00:37Z", "consecutiveFailures": 1000},
                {**worker, "name": "email", "lastFinishedAt": None, "lastSuccessAt": None, "consecutiveFailures": 0},
            ])
        await route.fulfill(status=status, json=data)

    await page.route("**/api/v1/**", fixture)
    await page.goto(base + "/admin/operations")
    details = page.locator("details").filter(has=page.get_by_role("heading", name="Worker", exact=True))
    summary = details.locator("summary")
    await expect(details).not_to_have_attribute("open", "")
    await summary.focus()
    await page.keyboard.press("Enter")
    await expect(details).to_have_attribute("open", "")
    await expect(details.locator(".report-table-hint")).to_be_hidden()
    rows = details.locator("li.admin-mobile-card" if width < 768 else "tbody tr")
    await expect(rows).to_have_count(2)
    if width < 768:
        first = rows.first
        assert await first.evaluate("el => el.parentElement.tagName === 'UL' && getComputedStyle(el).padding === '16px' && getComputedStyle(el).borderTopStyle === 'solid'")
        assert await first.locator("dt").count() == await first.locator("dd").count() == 3
        await expect(first.locator("time .admin-card-date")).to_have_text(["10 Okt 2026", "01.00.37 WIB", "10 Okt 2026", "01.00.37 WIB"])
        await expect(first.locator("dd").last).to_have_text("1.000")
        await expect(rows.nth(1).locator("dd")).to_have_text(["Belum ada", "Belum ada", "0"])
        await expect(details.locator(".admin-order-status")).to_have_text(["Memproses", "Menunggu"])
        assert await details.locator(".admin-order-status").evaluate_all("els => els.every(el => el.tabIndex < 0 && getComputedStyle(el).cursor !== 'pointer')")
        assert await details.locator("ul").evaluate("el => getComputedStyle(el).display === 'grid' && getComputedStyle(el).gap === '16px' && el.scrollWidth <= el.clientWidth + 1")
        await expect(details.locator(".mobile-list-viewport, nav, button")).to_have_count(0)
    else:
        await expect(details.locator(".admin-mobile-card")).to_have_count(0)
        await expect(rows.first.locator("td").last).to_have_text("1.000")
    metrics = page.locator("details").filter(has=page.get_by_role("heading", name="Ringkasan metrik", exact=True))
    if not await metrics.evaluate("el => el.open"):
        await metrics.locator("summary").click()
    await expect(metrics.locator("dd").nth(5)).to_have_text("1.234")
    assert await summary.evaluate("el => el.getBoundingClientRect().height >= 44")
    assert await page.evaluate("document.documentElement.scrollWidth <= innerWidth")
    if width < 768:
        await page.screenshot(path=str(output / f"worker-cards-{width}.png"), full_page=True, animations="disabled")
    else:
        # Full-page capture changes media queries and resets desktop disclosure defaults.
        await details.screenshot(path=str(output / f"worker-cards-{width}.png"), animations="disabled")
    if width < 768:
        await page.evaluate("""() => {
          window.__cardFonts = [...document.querySelectorAll('.admin-layout *')].map(el => [el, el.style.fontSize, getComputedStyle(el).fontSize]);
          for (const [el, , size] of window.__cardFonts) el.style.fontSize = parseFloat(size) * 2 + 'px';
        }""")
        assert await rows.evaluate_all("els => els.every(el => el.scrollWidth <= el.clientWidth + 1)")
        assert await page.evaluate("document.documentElement.scrollWidth <= innerWidth"), (width, "200% worker text overflows viewport")
        await page.screenshot(path=str(output / f"worker-cards-text200-{width}.png"), full_page=True, animations="disabled")
        await page.evaluate("() => { for (const [el, original] of window.__cardFonts) el.style.fontSize = original; }")
    mode = "stale"
    await page.get_by_role("button", name="Muat ulang", exact=True).click()
    await expect(page.get_by_text("Pembaruan gagal. Data terakhir tetap ditampilkan.")).to_be_visible()
    await expect(rows).to_have_count(2)
    await expect(details).to_have_attribute("open", "")
    mode = "empty"
    await page.get_by_role("button", name="Muat ulang", exact=True).click()
    await expect(details.get_by_text("Belum ada data worker.")).to_be_visible()
    await expect(details.locator(".admin-mobile-card")).to_have_count(0)
    await expect(page.locator(".ops-actions")).to_have_count(0)
    mode = "standard"
    await page.clock.install()
    await page.reload()
    await expect(rows).to_have_count(2)
    await summary.click()
    await page.clock.run_for(50)
    await expect(details).to_have_attribute("open", "")
    previous_calls = len(calls)
    async with page.expect_response(lambda response: urlparse(response.url).path == "/api/v1/admin/operations"):
        await page.clock.run_for(60_000)
    await expect(page.get_by_text("Pembaruan gagal. Data terakhir tetap ditampilkan.")).to_have_count(0)
    assert len(calls) == previous_calls + 1, "Automatic refresh must still run every minute"
    await expect(details).to_have_attribute("open", "")
    assert not errors, errors
    (output / f"worker-cards-{width}.json").write_text(json.dumps({"width": width, "timezone": "America/Los_Angeles", "calls": calls, "browserErrors": errors}, indent=2))
    await context.close()
    print(f"PASS {width}: worker cards/table, WIB, localized counts, disclosure, failed refresh retains data, automatic refresh, empty state")


async def capture_wizard_baseline(browser, base, output, width):
    context = await browser.new_context(viewport={"width": width, "height": 900}, timezone_id="Asia/Jakarta")
    await context.add_init_script(SESSION_INIT)
    page = await context.new_page()
    _, detail = body(base + f"/api/v1/admin/orders/{ORDER}", "standard", "ADMIN")
    voucher = {"code": "HEMAT10", "kind": "PERCENT", "value": 10, "startsAt": "2026-10-01T00:00:00Z", "endsAt": "2099-01-01T00:00:00Z", "active": True, "eventId": "", "minimumSubtotal": 0, "quota": 100, "revision": 1, "usage": {"held": 0, "consumed": 0, "released": 0}}
    async def fixture(route):
        path = urlparse(route.request.url).path
        assert route.request.method == "GET", "Baseline capture must not mutate data"
        if path.endswith("/staff/me"): await route.fulfill(json={"staff": ADMIN})
        elif path.endswith("/admin/events"): await route.fulfill(json={"events": [EVENT]})
        elif path.endswith("/admin/staff"): await route.fulfill(json={"staff": [ADMIN, STAFF]})
        elif path.endswith("/admin/pricing"): await route.fulfill(json={"settings": {"adminFee": 7500, "revision": 1}, "vouchers": [voucher], "history": []})
        elif path.endswith("/admin/orders"): await route.fulfill(json={"items": [], "nextCursor": None, "filterOptions": {"events": []}})
        elif "/admin/orders/" in path: await route.fulfill(json={**detail, "status": "REFUND_PENDING", "refund": {"status": "MANUAL_REQUIRED", "amount": 123456, "reason": "Transfer penuh diperlukan", "manualReference": "", "manualPaidAt": None}})
        else: raise AssertionError(path)
    await page.route("**/api/v1/**", fixture)
    async def capture(name):
        await page.screenshot(path=str(output / f"before-{name}-{width}.png"), full_page=True)
        await page.evaluate("""() => {
          window.__baselineFonts = [...document.querySelectorAll('.admin-layout *')].map(el => [el, el.style.fontSize, getComputedStyle(el).fontSize]);
          for (const [el, , size] of window.__baselineFonts) el.style.fontSize = parseFloat(size) * 2 + 'px';
        }""")
        await page.screenshot(path=str(output / f"before-{name}-text200-{width}.png"), full_page=True)
        await page.evaluate("() => { for (const [el, size] of window.__baselineFonts) el.style.fontSize = size; }")
    await page.goto(base + "/admin/staff")
    await page.locator("#staff-create-opener").click()
    await capture("staff-create")
    await page.get_by_role("button", name="Kembali ke daftar", exact=True).click()
    await page.locator(f"#staff-manage-{STAFF['id']}").click()
    await capture("staff-edit")
    await page.goto(base + "/admin/pricing")
    await expect(page.get_by_label("Nominal tetap (Rp)", exact=True)).to_have_value("7500")
    if not await page.locator(".voucher-editor").is_visible(): await page.locator("#voucher-create-opener").click()
    await capture("voucher-create")
    await page.goto(base + "/admin/orders?orderId=" + ORDER)
    await expect(page.get_by_label("Referensi transfer", exact=True)).to_be_visible()
    await capture("manual-refund")
    await context.close()
    print(f"CAPTURE {width}: existing staff, voucher and refund layouts at normal/200% text")


async def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--base-url", default="http://127.0.0.1:5173")
    parser.add_argument("--only", help="Capture a single route key for debugging, e.g. events")
    parser.add_argument("--output-dir", type=Path, help="Screenshot output directory; defaults to docs/phase29/screenshots")
    parser.add_argument("--phase", type=int, default=29, help="Phase number recorded in the manifest")
    parser.add_argument("--phase35", action="store_true", help="Capture tablet, landscape, reduced-height keyboard, and enlarged-text layouts")
    parser.add_argument("--inject-pageerror", action="store_true", help="Inject a non-blocking browser exception to verify failure reporting")
    parser.add_argument("--mobile-lists", action="store_true", help="Check shared mobile list groups, native scroll, and cursor recovery")
    parser.add_argument("--mobile-help", action="store_true", help="Check mobile help on concert, staff, order, and issue lists")
    parser.add_argument("--workflows", action="store_true", help="Verify the phase 55 concert/order workflows with larger fixtures")
    parser.add_argument("--wizard-baseline", action="store_true", help="Record the existing staff/voucher/refund layouts before rebuilding the local frontend")
    parser.add_argument("--manual-refund", action="store_true", help="Check manual refund evidence and review steps")
    parser.add_argument("--event-forms", action="store_true", help="Check concert form density and all sections at normal/200%% text")
    parser.add_argument("--record-form-baseline", action="store_true")
    parser.add_argument("--form-baseline", type=Path, help="Compare ticket height against this screenshot directory")
    args = parser.parse_args()
    output = args.output_dir if args.output_dir and args.output_dir.is_absolute() else ROOT / args.output_dir if args.output_dir else OUTPUT
    output.mkdir(parents=True, exist_ok=True)
    for previous in output.glob("*.png"):
        previous.unlink()
    commit = subprocess.run(["git", "rev-parse", "HEAD"], cwd=ROOT, text=True, capture_output=True, check=True).stdout.strip()
    captures, unexpected, observed_api, observed_csv_queries, unhandled_api = [], [], set(), [], []
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
        if args.wizard_baseline:
            for width in (320, 390, 768, 1440): await capture_wizard_baseline(browser, args.base_url, output, width)
            await browser.close()
            return
        if args.manual_refund:
            for width in (320, 390, 768, 1440): await check_manual_refund(browser, args.base_url, output, width)
            await browser.close()
            return
        if args.event_forms:
            for width in (320, 390, 768, 1440):
                await check_event_forms(browser, args.base_url, output, width, args.record_form_baseline, args.form_baseline)
            await browser.close()
            return
        if args.mobile_help:
            for width in (320, 390, 768, 1440): await check_help_pages(browser, args.base_url, output, width)
            await browser.close()
            return
        if args.mobile_lists:
            for width in (320, 390, 768, 1440):
                if args.only == "operations": await check_worker_cards(browser, args.base_url, output, width)
                else: await check_mobile_lists(browser, args.base_url, output, width)
            await browser.close()
            return
        if args.workflows:
            for width in (1440, 320, 390, 768): await check_workflows(browser, args.base_url, output, width)
            await browser.close()
            return
        for viewport, size in (("desktop", {"width": 1440, "height": 900}), ("mobile", {"width": 390, "height": 844})):
            for name, route, role in routes:
                modes = ("standard", "loading", "empty", "error") + (("rejected", "unknown-checkedin", "camera-exit") if name == "scanner" else ())
                if args.phase35 and name == "scanner": modes += ("expired-camera", "concurrent")
                if args.phase35 and role != "PUBLIC": modes += ("expired-session",)
                if args.phase35 and name == "issues": modes += ("uncertain", "uncertain-note")
                if args.phase35 and name in ("sales", "attendance", "conversion"): modes += ("retry-failed",)
                if name == "conversion": modes += ("missing-data",)
                if name == "operations": modes += ("stale",)
                for mode in modes:
                    context = await browser.new_context(viewport=size, timezone_id="Asia/Jakarta", color_scheme="dark")
                    if role != "PUBLIC":
                        if mode == "expired-session":
                            await context.add_init_script(f"sessionStorage.setItem('ticket-online:staff-session', JSON.stringify({{accessToken:'{TOKEN}',expiresAt:'2000-01-01T00:00:00.000Z'}}))")
                        else:
                            await context.add_init_script(SESSION_INIT)
                    if name == "scanner" and mode in ("camera-exit", "expired-camera"):
                        await context.add_init_script("""if(!sessionStorage.getItem('cameraStops'))sessionStorage.setItem('cameraStops','0'); window.__cameraTrackStops=Number(sessionStorage.getItem('cameraStops')); const stream=new MediaStream(); stream.getTracks=()=>[{stop(){window.__cameraTrackStops+=1;sessionStorage.setItem('cameraStops',String(window.__cameraTrackStops))}}]; Object.defineProperty(navigator,'mediaDevices',{configurable:true,value:{getUserMedia:async()=>stream}}); HTMLMediaElement.prototype.play=async function(){return};""")
                    page = await context.new_page()
                    page_errors = []
                    fixture_calls = {}
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
                        if signature == ("POST", "/staff/check-ins"):
                            fixture_calls["POST /staff/check-ins"] = fixture_calls.get("POST /staff/check-ins", 0) + 1
                        if name == "issues" and mode == "uncertain" and signature == ("POST", f"/admin/email-jobs/{'e' * 32}/retry"):
                            fixture_calls["POST retry"] = fixture_calls.get("POST retry", 0) + 1
                            await request.fulfill(status=503, json={"error": {"code": "SERVICE_UNAVAILABLE", "message": "Hasil retry belum diketahui."}})
                            return
                        if name == "issues" and mode == "uncertain-note" and signature == ("POST", "/admin/payment-cases/17/notes"):
                            fixture_calls["POST note"] = fixture_calls.get("POST note", 0) + 1
                            await request.fulfill(status=503, json={"error": {"code": "SERVICE_UNAVAILABLE", "message": "Hasil catatan belum diketahui."}})
                            return
                        if name == "issues" and mode == "uncertain-note" and signature == ("GET", "/admin/payment-cases/17"):
                            fixture_calls["GET case detail"] = fixture_calls.get("GET case detail", 0) + 1
                        report_endpoint = {"sales": "/admin/reports/sales", "conversion": "/admin/reports/conversion", "attendance": "/admin/reports/attendance"}.get(name)
                        if report_endpoint and mode == "retry-failed" and signature == ("GET", report_endpoint):
                            fixture_calls["report queries"] = fixture_calls.get("report queries", []) + [parsed.query]
                            request_count = len(fixture_calls["report queries"])
                            should_fail = request_count == 2 if name in ("sales", "conversion") else request_count == 1
                            if should_fail:
                                await request.fulfill(status=503, json={"error": {"code": "SERVICE_UNAVAILABLE", "message": "Cakupan laporan belum tersedia."}})
                                return
                        if endpoint in ("/admin/reports/sales.csv", "/admin/reports/attendance.csv"):
                            observed_csv_queries.append({"endpoint": endpoint, "query": parsed.query})
                            await request.fulfill(status=200, body="laporan,fixture\n", headers={
                                "Content-Type": "text/csv; charset=utf-8",
                                "Content-Disposition": 'attachment; filename="laporan-fixture.csv"',
                            })
                            return
                        if mode == "loading" and parsed.path != "/api/v1/staff/me":
                            await asyncio.sleep(1.2)
                        if name == "operations" and mode == "stale" and endpoint == "/admin/operations":
                            fixture_calls[endpoint] = fixture_calls.get(endpoint, 0) + 1
                            if fixture_calls[endpoint] > 1:
                                await request.fulfill(status=503, json={"error": {"code": "SERVICE_UNAVAILABLE", "message": "Snapshot operasional belum tersedia."}})
                                return
                        if name == "scanner" and mode == "concurrent" and signature == ("POST", "/staff/check-ins"):
                            await asyncio.sleep(0.7)
                        status, data = body(parsed.path + ("?" + parsed.query if parsed.query else ""), mode, role)
                        if name == "issues" and mode == "uncertain-note" and endpoint == "/admin/payment-cases/17" and fixture_calls.get("GET case detail", 0) > 1:
                            data["history"].append({"action": "NOTE", "actorName": "Admin Event", "createdAt": "2026-10-10T03:01:00Z", "data": {"note": "Catatan A"}})
                        if status == 204: await request.fulfill(status=status)
                        else: await request.fulfill(status=status, json=data)

                    await page.route("**/*", fixture)
                    target = route
                    if name == "orders" and mode == "standard" and viewport == "desktop": target += f"?orderId={ORDER}"
                    await page.goto(args.base_url + target, wait_until="domcontentloaded")
                    await page.locator("#konten").wait_for()
                    await page.wait_for_function("document.title !== 'Tiket Online'", timeout=15000)
                    await page.wait_for_timeout(80 if mode == "loading" else 400)
                    if args.inject_pageerror and name == routes[0][0] and mode == "standard" and viewport == "desktop":
                        await page.evaluate("setTimeout(() => { throw new Error('intentional pageerror fixture') }, 0)")
                        await page.wait_for_timeout(50)
                    if mode == "expired-session":
                        await page.wait_for_url("**/admin/login")
                        await page.wait_for_load_state("domcontentloaded")
                        await page.wait_for_timeout(100)
                    if name == "attendance" and mode == "standard":
                        await page.locator(".history-filter-form select").first.wait_for()
                        await page.locator(".history-filter-form select").first.select_option(EVENT["id"])
                        await page.get_by_role("button", name="Terapkan", exact=True).click()
                        await page.wait_for_timeout(400)
                    if name == "operations" and mode == "stale":
                        await page.get_by_role("heading", name="Ringkasan metrik").wait_for()
                        await page.get_by_role("button", name="Muat ulang", exact=True).click()
                        await page.locator("p[role=alert]").wait_for()
                        await page.get_by_text("Pembaruan gagal. Data terakhir tetap ditampilkan.", exact=False).wait_for()
                    if name == "operations" and mode in ("standard", "empty", "stale"):
                        await expect(page.get_by_role("heading", name="Penanganan yang tersedia", exact=True)).to_have_count(0)
                        await expect(page.get_by_role("link", name="Periksa daftar email gagal", exact=False)).to_have_count(0)
                        await expect(page.locator(".ops-scope")).to_have_count(0)
                        if mode == "empty":
                            await expect(page.locator(".ops-actions")).to_have_count(0)
                    if name == "operations" and mode == "standard":
                        for href in ("/admin/issues", "/admin/orders"):
                            if not await page.locator(f".ops-action-list a[href='{href}']").count():
                                raise AssertionError(f"operations page is missing the existing follow-up link {href}")
                    if name == "issues" and mode == "standard":
                        await page.get_by_role("button", name="Detail", exact=True).first.click()
                        await page.get_by_role("heading", name="Kasus TO-0123456789abcdef0123").wait_for()
                    if name == "issues" and mode == "uncertain":
                        page.on("dialog", lambda dialog: dialog.accept())
                        email_panel = page.locator(".checkin-history-panel").nth(1)
                        await email_panel.get_by_role("button", name="Lihat detail email TO-0123456789abcdef0123", exact=True).click()
                        await page.get_by_role("heading", name="Email TO-0123456789abcdef0123").wait_for()
                        await page.get_by_role("button", name="Kirim ulang email", exact=True).click()
                        await page.get_by_role("button", name="Periksa hasil tindakan", exact=True).wait_for()
                        if fixture_calls.get("POST retry") != 1:
                            raise AssertionError("uncertain retry fixture did not send exactly one POST")
                        if viewport == "mobile":
                            await expect(page.get_by_role("button", name="Kembali ke daftar", exact=True)).to_be_disabled()
                        else:
                            await expect(page.get_by_role("button", name="Kembali ke daftar", exact=True)).to_have_count(0)
                        await expect(page.get_by_role("button", name="Kirim ulang email", exact=True)).to_be_disabled()
                        async with page.expect_response(lambda response: urlparse(response.url).path == f"/api/v1/admin/email-jobs/{'e' * 32}"):
                            await page.get_by_role("button", name="Periksa hasil tindakan", exact=True).click()
                        await page.get_by_text("belum membuktikan hasil tindakan", exact=False).wait_for()
                        if fixture_calls.get("POST retry") != 1:
                            raise AssertionError("checking an uncertain email retry sent another POST")
                    if name == "issues" and mode == "uncertain-note":
                        await page.get_by_role("button", name="Detail", exact=True).first.click()
                        await page.get_by_role("heading", name="Kasus TO-0123456789abcdef0123").wait_for()
                        await page.get_by_label("Catatan admin").fill("  Catatan A  ")
                        await page.get_by_role("button", name="Simpan catatan", exact=True).click()
                        await page.get_by_role("button", name="Periksa hasil tindakan", exact=True).wait_for()
                        await page.get_by_label("Catatan admin").fill("Catatan B")
                        async with page.expect_response(lambda response: urlparse(response.url).path == "/api/v1/admin/payment-cases/17"):
                            await page.get_by_role("button", name="Periksa hasil tindakan", exact=True).click()
                        await page.get_by_text("Hasil tindakan terlihat pada detail terbaru.", exact=True).wait_for()
                        if fixture_calls.get("POST note") != 1:
                            raise AssertionError("checking an uncertain note sent another POST")
                    if name in ("sales", "conversion", "attendance") and mode == "retry-failed":
                        if name == "attendance":
                            await page.get_by_label("Event wajib").select_option(EVENT["id"])
                            await page.get_by_role("button", name="Terapkan", exact=True).click()
                            await page.get_by_role("alert").wait_for()
                            failed_query = fixture_calls["report queries"][-1]
                            async with page.expect_response(lambda response: urlparse(response.url).path == "/api/v1/admin/reports/attendance" and urlparse(response.url).query == failed_query):
                                await page.get_by_role("button", name="Coba lagi", exact=True).click()
                            if fixture_calls["report queries"] != [failed_query, failed_query]:
                                raise AssertionError(f"attendance retry changed its failed event/gate query: {fixture_calls['report queries']}")
                        else:
                            if viewport == "mobile":
                                await page.locator(".mobile-filter-heading button").click()
                            if name == "sales":
                                await page.locator(".sales-report-filters select").first.select_option(EVENT["id"])
                            else:
                                await page.locator(".sales-report-filters select").nth(1).select_option("mobile")
                            dates = page.locator("input[type=date]")
                            await dates.nth(0).fill("2026-01-01")
                            await dates.nth(1).fill("2026-01-31")
                            await page.get_by_role("button", name="Terapkan", exact=True).click()
                            await page.get_by_role("alert").wait_for()
                            failed_query = fixture_calls["report queries"][-1]
                            await page.locator("input[type=date]").first.fill("2026-02-01")
                            async with page.expect_response(lambda response: urlparse(response.url).path == f"/api/v1/admin/reports/{name}" and urlparse(response.url).query == failed_query):
                                await page.get_by_role("button", name="Coba lagi", exact=True).click()
                            if fixture_calls["report queries"][-2:] != [failed_query, failed_query]:
                                raise AssertionError(f"{name} retry changed its failed filter query: {fixture_calls['report queries']}")
                    mobile_order_list = None
                    if name == "orders" and mode == "standard" and viewport == "desktop":
                        await page.get_by_role("button", name="Kembali ke daftar", exact=True).click()
                        cells = page.locator(".order-list-panel .history-table :is(th, td)")
                        await expect(page.locator(".order-list-panel .history-table tbody tr")).not_to_have_count(0)
                        wrapped = await cells.evaluate_all("""cells => cells.flatMap(cell => {
                            const walker = document.createTreeWalker(cell, NodeFilter.SHOW_TEXT);
                            for (let node; node = walker.nextNode();) {
                                if (!node.textContent.trim()) continue;
                                const range = document.createRange(); range.selectNodeContents(node);
                                if (range.getClientRects().length > 1) return [cell.textContent.trim()];
                            }
                            return [];
                        })""")
                        assert not wrapped, f"Order table text wrapped: {wrapped}"
                        assert await page.evaluate("document.documentElement.scrollWidth <= innerWidth"), "Order table overflows the page"
                        await page.screenshot(path=str(output / "orders-list-desktop.png"), full_page=True, animations="disabled")
                        await page.get_by_role("button", name="Lihat detail pesanan TO-0123456789abcdef0123", exact=True).click()
                        await page.get_by_role("heading", name="Detail pesanan", exact=True).wait_for()
                    if name == "orders" and mode == "standard" and viewport == "mobile":
                        await expect(page.locator("#order-search-form")).to_be_hidden()
                        await page.screenshot(path=str(output / "orders-search-collapsed-mobile.png"), full_page=True, animations="disabled")
                        toggle = page.locator(".order-search-toggle")
                        await toggle.focus()
                        await page.keyboard.press("Enter")
                        await expect(toggle).to_have_attribute("aria-expanded", "true")
                        search_input = page.locator("#order-search-form input").first
                        await search_input.fill("TO-0123456789abcdef0123")
                        await page.locator("#order-search-form").get_by_role("button", name="Cari", exact=True).click()
                        await page.get_by_role("button", name="Lihat detail pesanan TO-0123456789abcdef0123").wait_for()
                        mobile_order_list = "orders-filtered-list-mobile.png"
                        await page.locator(".order-list-panel").scroll_into_view_if_needed()
                        await page.screenshot(path=str(output / mobile_order_list), full_page=False, animations="disabled")
                        await page.get_by_role("button", name="Lihat detail pesanan TO-0123456789abcdef0123").click()
                        await page.get_by_role("heading", name="Detail pesanan", exact=True).wait_for()
                        await expect(search_input).to_have_count(0)
                        await page.get_by_role("button", name="Kembali ke daftar", exact=True).click()
                        if await search_input.input_value() != "TO-0123456789abcdef0123":
                            raise AssertionError("mobile order selection discarded the active search filter")
                        await page.get_by_role("button", name="Lihat detail pesanan TO-0123456789abcdef0123").click()
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
                        if mode == "error":
                            await page.get_by_role("button", name="Periksa status", exact=True).click()
                            await page.get_by_role("alert").wait_for()
                            await page.get_by_role("button", name="Konfirmasi penanganan", exact=True).click()
                    if name == "scanner" and mode in ("rejected", "unknown-checkedin"):
                        await page.get_by_label("Penugasan aktif").select_option(f"{EVENT['id']}:Gate B")
                        await page.get_by_label("Kode e-ticket", exact=True).fill(TICKET["code"])
                        await page.get_by_role("button", name="Verifikasi", exact=False).click()
                        await page.get_by_text("Tiket Sudah Digunakan" if mode == "rejected" else "Hasil belum diketahui", exact=True).wait_for()
                        if mode == "unknown-checkedin":
                            await page.get_by_role("button", name="Periksa status", exact=True).click()
                            await page.get_by_text("Check-in tiket sudah tercatat.", exact=True).wait_for()
                            if not await page.get_by_role("button", name="Scan berikutnya", exact=True).is_enabled():
                                raise AssertionError("confirmed unknown check-in should allow continuing after status lookup")
                    if name == "scanner" and mode == "camera-exit":
                        await page.get_by_label("Penugasan aktif").select_option(f"{EVENT['id']}:Gate B")
                        await page.get_by_role("button", name="Aktifkan kamera", exact=True).click()
                        await page.locator("video.camera-active").wait_for()
                        await page.get_by_role("button", name="Keluar", exact=True).click()
                        await page.wait_for_function("location.pathname === '/admin/login' && window.__cameraTrackStops > 0")
                        await page.wait_for_url("**/admin/login")
                        await page.wait_for_load_state("domcontentloaded")
                        await page.wait_for_timeout(100)
                    if name == "scanner" and mode == "expired-camera":
                        await page.get_by_label("Penugasan aktif").select_option(f"{EVENT['id']}:Gate B")
                        await page.get_by_role("button", name="Aktifkan kamera", exact=True).click()
                        await page.locator("video.camera-active").wait_for()
                        await page.get_by_label("Kode e-ticket", exact=True).fill(TICKET["code"])
                        await page.get_by_role("button", name="Verifikasi", exact=False).click()
                        await page.wait_for_function("location.pathname === '/admin/login' && window.__cameraTrackStops > 0")
                        await page.wait_for_url("**/admin/login")
                        await page.wait_for_load_state("domcontentloaded")
                        await page.wait_for_timeout(100)
                        if fixture_calls.get("POST /staff/check-ins") != 1:
                            raise AssertionError("expired session should send at most one check-in request before revoking access")
                    if name == "scanner" and mode == "concurrent":
                        await page.get_by_label("Penugasan aktif").select_option(f"{EVENT['id']}:Gate B")
                        await page.get_by_label("Kode e-ticket", exact=True).fill(TICKET["code"])
                        await page.locator(".scan-form").evaluate("form => { form.requestSubmit(); form.requestSubmit(); }")
                        await page.get_by_text("Gate Masuk Terbuka", exact=True).wait_for()
                        if fixture_calls.get("POST /staff/check-ins") != 1:
                            raise AssertionError(f"concurrent scanner submission sent {fixture_calls.get('POST /staff/check-ins', 0)} requests")
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
                            visibleEmDashCount: (document.body.innerText.match(/—/g) || []).length,
                            overflow: document.documentElement.scrollWidth>innerWidth
                          };
                        }""")
                        if capture["audit"]["visibleEmDashCount"]:
                            raise AssertionError(f"visible em-dash copy needs rewriting on {route}: {capture['audit']['visibleEmDashCount']}")
                        action = page.locator(".admin-page-heading .admin-heading-actions > button").first
                        if name == "orders":
                            await expect(page.get_by_role("button", name="Cari", exact=True)).to_have_count(1 if await page.locator("#order-search-form").is_visible() else 0)
                            assert not await action.count(), "Order search belongs in its filter form"
                        elif name == "check-ins":
                            assert not await action.count(), "Check-in search belongs in its filter form"
                        elif name == "issues" and await page.locator('#email-detail-title').count():
                            assert not await action.count(), "Email detail has no heading action"
                        elif name not in ("login", "scanner"):
                            if not await action.count():
                                raise AssertionError(f"admin heading action missing on {route}")
                            action_info = await action.evaluate("button => ({label:button.innerText.trim(),type:button.type,formId:button.form?.id||'',disabled:button.disabled})")
                            if action_info["formId"] and not await page.locator(f"#{action_info['formId']}").count():
                                raise AssertionError(f"heading action points to a missing form on {route}")
                            capture["audit"]["primaryAction"] = action_info
                        await page.keyboard.press("Tab")
                        capture["audit"]["firstTabFocus"] = await page.evaluate("""() => ({tag:document.activeElement?.tagName,id:document.activeElement?.id||'',label:document.activeElement?.getAttribute('aria-label')||document.activeElement?.innerText?.trim()||''})""")
                        if viewport == "mobile":
                            if mobile_order_list:
                                capture["files"].append(mobile_order_list)
                                capture["audit"]["mobileOrderSelection"] = {"filteredListCaptured": True, "selectedDetailOpened": True, "searchValuePreserved": True}
                            if role == "ADMIN":
                                menu_button = page.get_by_role("button", name="Menu", exact=True)
                                await expect(menu_button).to_have_attribute("aria-expanded", "false")
                                nav_controls = await page.locator(".admin-topbar :is(.scan-brand, .admin-menu-trigger, .admin-logout)").evaluate_all("elements => elements.map(el => { const r = el.getBoundingClientRect(); return {width:r.width, height:r.height, left:r.left, right:r.right, centerY:r.top+r.height/2}; })")
                                assert all(control["width"] >= 44 and control["height"] >= 44 for control in nav_controls), nav_controls
                                assert all(abs(control["centerY"] - nav_controls[0]["centerY"]) <= 1 for control in nav_controls), nav_controls
                                assert nav_controls[0]["right"] <= nav_controls[1]["left"] and nav_controls[1]["right"] <= nav_controls[2]["left"], nav_controls
                                await menu_button.focus()
                                await page.keyboard.press("Enter")
                                await expect(menu_button).to_have_attribute("aria-expanded", "true")
                                dialog = page.locator("#admin-mobile-menu")
                                if not await dialog.evaluate("dialog => dialog.open"):
                                    details = await dialog.evaluate("dialog => ({open:dialog.open,html:dialog.outerHTML,button:document.querySelector('.admin-menu-trigger')?.outerHTML})")
                                    raise AssertionError(f"mobile admin menu did not open on {route}: {details}")
                                close_button = dialog.get_by_role("button", name="Tutup", exact=True)
                                if not await close_button.evaluate("button => button === document.activeElement"):
                                    raise AssertionError(f"mobile menu focus did not move to close button on {route}")
                                current_links = dialog.locator('a[aria-current="page"]')
                                if await current_links.count() != 1:
                                    raise AssertionError(f"mobile menu needs one active destination on {route}")
                                if name == "orders":
                                    await page.screenshot(path=str(output / "orders-mobile-navigation.png"), animations="disabled")
                                await dialog.get_by_role("link").last.focus()
                                await page.keyboard.press("Tab")
                                focus_trapped = await dialog.evaluate("dialog => dialog.contains(document.activeElement)")
                                if not focus_trapped:
                                    active = await page.evaluate("() => document.activeElement?.outerHTML")
                                    raise AssertionError(f"mobile menu focus escaped its dialog on {route}: {active}")
                                await page.keyboard.press("Escape")
                                await expect(menu_button).to_have_attribute("aria-expanded", "false")
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
                            if capture["audit"]["narrow320"]["overflow"]:
                                raise AssertionError(f"{route} overflows at 320px: {capture['audit']['narrow320']}")
                            narrow_path = f"{name}-320px.png"
                            await page.screenshot(path=str(output / narrow_path), full_page=False, animations="disabled")
                            capture["files"].append(narrow_path)
                    if args.phase35 and mode == "standard" and viewport == "desktop":
                        responsive_checks = {}
                        variants = (
                            ("tablet", {"width": 768, "height": 1024}, False),
                            ("landscape", {"width": 844, "height": 390}, False),
                            ("keyboard-height", {"width": 390, "height": 500}, False),
                            ("text-200-percent", {"width": 390, "height": 844}, True),
                        )
                        for variant, variant_size, enlarge_text in variants:
                            await page.set_viewport_size(variant_size)
                            if enlarge_text:
                                await page.evaluate("""() => {
                                  window.__phase35FontOriginals = [...document.querySelectorAll('body *')]
                                    .filter(el => {
                                      if (el.closest('svg') || !el.getClientRects().length) return false;
                                      for (let parent = el; parent && parent !== document.body; parent = parent.parentElement) {
                                        const style = getComputedStyle(parent), rect = parent.getBoundingClientRect();
                                        if (style.clip !== 'auto' || style.clipPath !== 'none' || (style.position === 'absolute' && rect.width <= 1 && rect.height <= 1)) return false;
                                      }
                                      return true;
                                    })
                                    .map(el => [el, el.style.getPropertyValue('font-size'), el.style.getPropertyPriority('font-size'), parseFloat(getComputedStyle(el).fontSize)]);
                                  for (const [el, , , originalSize] of window.__phase35FontOriginals) {
                                    el.style.setProperty('font-size', `${originalSize * 2}px`, 'important');
                                  }
                                }""")
                            await page.evaluate("window.scrollTo(0, 0)")
                            result = await page.evaluate("""() => ({viewportWidth:innerWidth,viewportHeight:innerHeight,
                              documentWidth:document.documentElement.scrollWidth,overflow:document.documentElement.scrollWidth>innerWidth,
                              overflowingBoxes:[...document.querySelectorAll('body *')].filter(el=>el.scrollWidth>el.clientWidth+1).sort((a,b)=>{const depth=el=>{let n=0;while(el.parentElement){n++;el=el.parentElement}return n};return depth(b)-depth(a)}).slice(0,20).map(el=>({tag:el.tagName,id:el.id,classes:typeof el.className==='string'?el.className:'',text:(el.innerText||'').trim().slice(0,45),scrollWidth:el.scrollWidth,clientWidth:el.clientWidth,rect:el.getBoundingClientRect().toJSON()})),bodyWidth:document.body.scrollWidth,
                              offenders:[...document.querySelectorAll('body *')].filter(el=>{const r=el.getBoundingClientRect();return r.right>innerWidth+1||r.left < -1}).slice(0,8).map(el=>({tag:el.tagName,id:el.id,classes:typeof el.className==='string'?el.className:'',text:(el.innerText||'').trim().slice(0,40),left:Math.round(el.getBoundingClientRect().left),right:Math.round(el.getBoundingClientRect().right),parents:[el.parentElement,el.parentElement?.parentElement,el.parentElement?.parentElement?.parentElement].map(p=>p&&({tag:p.tagName,id:p.id,classes:typeof p.className==='string'?p.className:'',left:Math.round(p.getBoundingClientRect().left),right:Math.round(p.getBoundingClientRect().right)}))})),
                              focusableCount:[...document.querySelectorAll('a,button,input:not([type=hidden]),select,textarea,summary,[tabindex]')].filter(el=>!el.disabled&&el.tabIndex>=0).length,
                              headingCount:document.querySelectorAll('h1').length})""")
                            if result["overflow"]:
                                raise AssertionError(f"{route} overflows in {variant}: {result}")
                            if result["headingCount"] != 1:
                                raise AssertionError(f"{route} should keep one page heading in {variant}: {result}")
                            responsive_checks[variant] = result
                            responsive_path = f"{name}-{variant}.png"
                            await page.screenshot(path=str(output / responsive_path), full_page=False, animations="disabled")
                            capture["files"].append(responsive_path)
                            if enlarge_text:
                                await page.evaluate("""() => {
                                  for (const [el, value, priority] of window.__phase35FontOriginals || []) {
                                    if (value) el.style.setProperty('font-size', value, priority);
                                    else el.style.removeProperty('font-size');
                                  }
                                  delete window.__phase35FontOriginals;
                                }""")
                        capture["audit"]["phase35Responsive"] = responsive_checks
                    if args.phase35:
                        visible_dashes = await page.evaluate("(document.body.innerText.match(/[—–]/g) || []).length")
                        if visible_dashes:
                            raise AssertionError(f"visible em/en-dash copy needs rewriting on {route} ({mode}, {viewport}): {visible_dashes}")
                    if name in ("sales", "attendance") and mode == "standard":
                        async with page.expect_download() as download_info:
                            await page.get_by_role("button", name="Ekspor CSV", exact=True).click()
                        download = await download_info.value
                        if not download.suggested_filename.endswith(".csv"):
                            raise AssertionError(f"CSV export returned an unexpected filename: {download.suggested_filename}")
                        exported = observed_csv_queries[-1]
                        query = dict(part.split("=", 1) for part in exported["query"].split("&") if "=" in part)
                        if name == "sales" and query != {"dateFrom": "2026-09-11", "dateTo": "2026-10-10"}:
                            raise AssertionError(f"sales CSV did not use the displayed snapshot filters: {query}")
                        if name == "attendance" and query != {"eventId": "nusa-malam"}:
                            raise AssertionError(f"attendance CSV did not use the displayed snapshot filters: {query}")
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
                page_errors = []
                page.on("pageerror", lambda error: page_errors.append(str(error)))

                async def denied_fixture(request):
                    if request.request.url.endswith("/staff/me"):
                        await request.fulfill(json={"staff": STAFF})
                    elif "/api/v1/admin/" in request.request.url:
                        await request.fulfill(status=403, json={"error": {"code": "FORBIDDEN", "message": "Akses administrator diperlukan."}})
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
                denied_capture = {"route": route, "scenario": "role-denied", "role": "STAFF", "viewport": viewport, "files": [denied_path], "finalUrl": urlparse(page.url).path}
                if page_errors:
                    denied_capture["pageErrors"] = page_errors
                captures.append(denied_capture)
                await context.close()
        await browser.close()

    manifest = {"phase": args.phase, "capturedAt": datetime.now(timezone.utc).isoformat(), "sourceCommit": commit,
        "tool": "Python Playwright, Google Chrome", "viewports": {"desktop": "1440x900", "mobile": "390x844",
            **({"tablet": "768x1024", "landscape": "844x390", "keyboardHeightSimulation": "390x500", "textEnlargementSimulation": "200% text via injected CSS"} if args.phase35 else {})},
        "theme": "dark", "timezone": "Asia/Jakarta", "api": "All API requests are intercepted; responses use local fixtures and unmapped requests fail the capture.",
        "apiRequests": sorted(observed_api), "csvQueries": observed_csv_queries, "unhandledApiRequests": unhandled_api,
        "pageErrors": [{"route": c["route"], "scenario": c["scenario"], "viewport": c["viewport"], "errors": c["pageErrors"]} for c in captures if c.get("pageErrors")],
        "captures": captures, "unexpectedRedirects": unexpected}
    (output / "manifest.json").write_text(json.dumps(manifest, ensure_ascii=False, indent=2) + "\n")
    print(f"Captured {len(captures)} admin/staff screenshots in {output}")
    if unexpected:
        raise AssertionError(f"Unexpected redirects: {unexpected}")
    if unhandled_api:
        raise AssertionError(f"API requests missing a fixture: {unhandled_api}")
    if manifest["pageErrors"]:
        raise AssertionError(f"Browser runtime errors: {manifest['pageErrors']}")


if __name__ == "__main__":
    asyncio.run(main())
