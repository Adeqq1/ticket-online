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
            "recipient": "nadia@example.test", "attempts": 2, "lastError": "SMTP belum merespons", "updatedAt": "2026-10-10T02:00:00Z", "supersededBy": None}
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


async def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--base-url", default="http://127.0.0.1:5173")
    parser.add_argument("--only", help="Capture a single route key for debugging, e.g. events")
    parser.add_argument("--output-dir", type=Path, help="Screenshot output directory; defaults to docs/phase29/screenshots")
    parser.add_argument("--phase", type=int, default=29, help="Phase number recorded in the manifest")
    parser.add_argument("--phase35", action="store_true", help="Capture tablet, landscape, reduced-height keyboard, and enlarged-text layouts")
    parser.add_argument("--inject-pageerror", action="store_true", help="Inject a non-blocking browser exception to verify failure reporting")
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
                        await page.get_by_role("heading", name="Penanganan yang tersedia").wait_for()
                        await page.get_by_role("button", name="Muat ulang", exact=True).click()
                        await page.get_by_role("alert").wait_for()
                        await page.get_by_text("Snapshot terakhir yang berhasil dimuat.", exact=False).wait_for()
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
                        await email_panel.get_by_role("button", name="Detail", exact=True).first.click()
                        await page.get_by_role("heading", name="Email TO-0123456789abcdef0123").wait_for()
                        await page.get_by_role("button", name="Kirim ulang email", exact=True).click()
                        await page.get_by_role("button", name="Periksa hasil tindakan", exact=True).wait_for()
                        if fixture_calls.get("POST retry") != 1:
                            raise AssertionError("uncertain retry fixture did not send exactly one POST")
                        other_detail = email_panel.get_by_role("button", name="Detail", exact=True).nth(1)
                        if await other_detail.is_enabled():
                            raise AssertionError("another email detail remained navigable while retry result was uncertain")
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
                    if name == "orders" and mode == "standard" and viewport == "mobile":
                        search_input = page.locator("#order-search-form input").first
                        await search_input.fill("TO-0123456789abcdef0123")
                        await page.locator("#order-search-form").get_by_role("button", name="Cari", exact=True).click()
                        await page.get_by_role("button", name="Buka pesanan TO-0123456789abcdef0123").wait_for()
                        mobile_order_list = "orders-filtered-list-mobile.png"
                        await page.locator(".order-list-panel").scroll_into_view_if_needed()
                        await page.screenshot(path=str(output / mobile_order_list), full_page=False, animations="disabled")
                        await page.get_by_role("button", name="Buka pesanan TO-0123456789abcdef0123").click()
                        await page.get_by_role("heading", name="Detail pesanan").wait_for()
                        if await search_input.input_value() != "TO-0123456789abcdef0123":
                            raise AssertionError("mobile order selection discarded the active search filter")
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
                            if mobile_order_list:
                                capture["files"].append(mobile_order_list)
                                capture["audit"]["mobileOrderSelection"] = {"filteredListCaptured": True, "selectedDetailOpened": True, "searchValuePreserved": True}
                            if role == "ADMIN":
                                menu_button = page.get_by_role("button", name="Menu", exact=True)
                                await menu_button.focus()
                                await page.keyboard.press("Enter")
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


asyncio.run(main())
