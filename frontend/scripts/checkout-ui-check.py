"""Phase 69–73 checkout browser fixtures; no real payment or emails.
Run from frontend/: python3 scripts/checkout-ui-check.py [base_url]
"""
import asyncio
import json
import sys
from datetime import datetime, timedelta, timezone
from pathlib import Path
from urllib.parse import urlparse

from playwright.async_api import async_playwright, expect

BASE = (sys.argv[1] if len(sys.argv) > 1 else "http://127.0.0.1:5175").rstrip("/")
OUTPUT = Path(__file__).resolve().parents[2] / "docs" / "phase69-73"
ORDER_ID = "0123456789abcdef0123456789abcdef"
RESERVATION_ID = "abcdef0123456789abcdef0123456789"
ACCESS = "a" * 43


def fixtures():
    now = datetime.now(timezone.utc)
    deadline = (now + timedelta(minutes=10)).isoformat()
    starts = (now + timedelta(days=40)).isoformat()
    tiers = [
        {"id": "festival", "name": "Festival", "zoneId": "festival", "price": 275000, "availableQuantity": 42, "maxPerOrder": 6, "benefit": "Area berdiri", "gate": "Gate B", "seating": "free-standing"},
        {"id": "tribune", "name": "Tribune", "zoneId": "tribune", "price": 350000, "availableQuantity": 24, "maxPerOrder": 4, "benefit": "Area duduk", "gate": "Gate C", "seating": "assigned"},
    ]
    event = {"id": "nusa-malam", "artist": "Nusa Malam", "city": "Jakarta", "venue": "Ruang Selatan", "address": "Jl. Musik Raya", "startsAt": starts, "genre": "Indie", "status": "Presale", "publicationStatus": "PUBLISHED", "image": "https://example.test/poster.jpg", "description": "Konser Nusa Malam.", "lineup": ["Nusa Malam"], "price": 275000, "zones": [{"id": t["id"], "name": t["name"], "description": t["benefit"]} for t in tiers], "ticketTiers": tiers, "scheduleLocked": False, "currentEvent": {"id": "nusa-malam", "status": "SCHEDULED", "version": 0, "startsAt": starts, "salesPaused": False, "announcement": "", "refundDeadline": None}}
    return {"now": now, "deadline": deadline, "event": event, "tiers": tiers, "reservation": {"id": RESERVATION_ID, "status": "ACTIVE", "expiresAt": deadline, "event": {"id": "nusa-malam", "artist": "Nusa Malam"}, "items": [], "subtotal": 0}, "order": None, "orders": [], "posts": [], "payments": [], "quote": {"adminFee": 7500, "discount": 0, "voucher": None}, "uncertainCreate": False, "uncertainPayment": False, "changedTotal": False, "createDelay": 0}


def order_for(state, payload):
    lines = []
    subtotal = 0
    for item in payload["attendees"]:
        tier = next(t for t in state["tiers"] if t["id"] == item["tierId"])
        reservation_item = next((i for i in state["reservation"]["items"] if i["tierId"] == tier["id"]), None)
        quantity = len(item["names"])
        price = tier["price"]
        total = price * quantity
        subtotal += total
        lines.append({"tierId": tier["id"], "name": tier["name"], "quantity": quantity, "unitPrice": price, "lineTotal": total})
    discount = state["quote"]["discount"]
    total = subtotal + state["quote"]["adminFee"] - discount + (5000 if state["changedTotal"] else 0)
    state["order"] = {"id": ORDER_ID, "reference": "TO-0123456789abcdef0123", "reservationId": RESERVATION_ID, "status": "PENDING", "expiresAt": state["reservation"]["expiresAt"], "subtotal": subtotal, "adminFee": state["quote"]["adminFee"], "discount": discount, "total": total, "items": lines, "buyer": {**payload["buyer"], "identity": ""}, "attendees": [{"tierId": a["tierId"], "ticketNumber": n + 1, "name": name} for a in payload["attendees"] for n, name in enumerate(a["names"])], "payment": None, "createdAt": state["now"].isoformat(), "updatedAt": state["now"].isoformat(), "eventStartsAt": state["event"]["startsAt"], "accessExpiresAt": None, "currentEvent": state["event"]["currentEvent"], "accessToken": ACCESS}
    state["reservation"]["status"] = "CONVERTED"
    return state["order"]


async def main():
    OUTPUT.mkdir(parents=True, exist_ok=True)
    results, errors = [], []
    async with async_playwright() as p:
        browser = await p.chromium.launch(channel="chrome", headless=True)
        for width in (320, 390, 768, 1440):
            state = fixtures()
            context = await browser.new_context(viewport={"width": width, "height": 900}, timezone_id="Asia/Jakarta", reduced_motion="reduce")
            await context.add_init_script("localStorage.clear(); sessionStorage.clear();")

            async def api(route):
                path = urlparse(route.request.url).path
                method = route.request.method
                if method == "POST" and path == "/api/v1/reservations":
                    body = route.request.post_data_json
                    assert body["eventId"] == "nusa-malam"
                    state["reservation"]["items"] = [{"tierId": item["tierId"], "name": next(t["name"] for t in state["tiers"] if t["id"] == item["tierId"]), "quantity": item["quantity"], "unitPrice": next(t["price"] for t in state["tiers"] if t["id"] == item["tierId"]), "lineTotal": item["quantity"] * next(t["price"] for t in state["tiers"] if t["id"] == item["tierId"])} for item in body["items"]]
                    state["reservation"]["subtotal"] = sum(item["lineTotal"] for item in state["reservation"]["items"])
                    return await route.fulfill(status=201, json=state["reservation"])
                if method == "POST" and path.endswith("/quote"):
                    code = route.request.post_data_json.get("voucherCode", "")
                    subtotal = state["reservation"]["subtotal"]
                    if code == "SALAH": return await route.fulfill(status=422, json={"error": {"code": "INVALID_VOUCHER", "message": "technical invalid voucher"}})
                    discount = subtotal // 10 if code == "HEMAT10" else 0
                    state["quote"] = {"adminFee": 7500, "discount": discount, "voucher": {"code": code, "kind": "PERCENT", "value": 10} if code else None}
                    return await route.fulfill(json={"subtotal": subtotal, **state["quote"], "total": subtotal + 7500 - discount})
                if method == "POST" and path.endswith("/checkout"):
                    body = route.request.post_data_json
                    state["posts"].append({"body": body, "key": route.request.headers.get("idempotency-key")})
                    assert "identity" not in body["buyer"], body
                    if state["createDelay"]: await asyncio.sleep(state["createDelay"])
                    if state["uncertainCreate"] and len(state["posts"]) == 1:
                        order_for(state, body)
                        return await route.abort("failed")
                    order = state["order"] or order_for(state, body)
                    return await route.fulfill(status=200 if len(state["posts"]) > 1 else 201, json={k: order[k] for k in ("id", "reference", "reservationId", "status", "expiresAt", "subtotal", "adminFee", "discount", "total", "items")} | {"accessToken": ACCESS, "accessExpiresAt": None})
                if method == "POST" and path.endswith("/payments"):
                    state["payments"].append(route.request.post_data_json)
                    assert path == f"/api/v1/orders/{ORDER_ID}/payments"
                    if state["uncertainPayment"] and len(state["payments"]) == 1:
                        state["order"]["payment"] = {"id": "payment-1", "method": route.request.post_data_json["method"], "status": "PENDING", "amount": state["order"]["total"]}
                        return await route.abort("failed")
                    return await route.fulfill(json={"redirectUrl": "https://app.sandbox.midtrans.com/snap/v2/fixture"})
                if method == "GET" and path == "/api/v1/events/nusa-malam": return await route.fulfill(json=state["event"])
                if method == "GET" and path == f"/api/v1/reservations/{RESERVATION_ID}": return await route.fulfill(json=state["reservation"])
                if method == "GET" and path == f"/api/v1/orders/{ORDER_ID}":
                    if not state["order"]: return await route.fulfill(status=404, json={"error": {"code": "ORDER_NOT_FOUND", "message": "not found"}})
                    return await route.fulfill(json={k: v for k, v in state["order"].items() if k != "accessToken"})
                if method == "GET" and path == "/api/v1/events": return await route.fulfill(json={"events": [state["event"]]})
                return await route.fulfill(status=404, json={"error": {"code": "NOT_FOUND", "message": "fixture not mapped: " + path}})

            await context.route("**/api/v1/**", api)
            await context.route("https://app.sandbox.midtrans.com/**", lambda route: route.fulfill(body="Sandbox fixture"))
            page = await context.new_page()
            page.on("pageerror", lambda error: errors.append(str(error)))
            query = "festival=2&tribune=1"
            await page.goto(f"{BASE}/checkout/nusa-malam?{query}")
            await expect(page.get_by_role("heading", name="Data pemesan")).to_be_visible()
            await expect(page.get_by_label("Nomor identitas", exact=False)).to_have_count(0)
            await expect(page.get_by_text("Biaya admin dihitung saat checkout.").first).to_be_visible()
            await expect(page.locator(".checkout-mobile-actions")).to_be_visible() if width < 768 else await expect(page.locator(".checkout-mobile-actions")).to_be_hidden()
            await expect(page.get_by_role("button", name="Lanjut ke pembayaran")).to_be_visible()
            await expect(page.locator(".summary-total")).to_contain_text("907.500")
            await expect(page.locator(".checkout-steps")).to_contain_text("Data pemesan")
            if width in (320, 390):
                await page.screenshot(path=str(OUTPUT / f"buyer-{width}.png"), full_page=True)
            # Invalid fields retain focus and one visible primary action.
            await page.locator("#buyer-form").evaluate("form => form.requestSubmit()")
            await expect(page.locator("#name")).to_be_focused()
            await page.locator("#name").fill("Nadia Pembeli")
            await page.locator("#email").fill("nadia@example.test")
            await page.locator("#phone").fill("08123456789")
            await page.get_by_role("button", name="Gunakan nama pemesan untuk semua tiket").scroll_into_view_if_needed()
            await page.get_by_role("button", name="Gunakan nama pemesan untuk semua tiket").dispatch_event("click")
            for label in ("Nama peserta 1", "Nama peserta 2", "Nama peserta 3"):
                await expect(page.get_by_label(label)).to_have_value("Nadia Pembeli")
            await page.locator("#attendee-tribune-0").fill("Raka Pembeli")
            await expect(page.locator("#attendee-tribune-0")).to_have_value("Raka Pembeli")
            await page.get_by_role("button", name="Gunakan nama pemesan untuk semua tiket").click()
            await expect(page.get_by_role("dialog", name="Ganti nama tiket?")).to_be_visible()
            await page.get_by_role("button", name="Batal").click()
            await expect(page.locator("#attendee-tribune-0")).to_have_value("Raka Pembeli")
            await page.get_by_role("button", name="Gunakan nama pemesan untuk semua tiket").click()
            await page.get_by_role("button", name="Ganti nama", exact=True).click()
            await expect(page.locator("#attendee-tribune-0")).to_have_value("Nadia Pembeli")
            await page.get_by_role("button", name="Lanjut ke pembayaran").last.click()
            await expect(page.get_by_role("heading", name="Metode pembayaran")).to_be_visible()
            await expect(page.locator(".reservation-banner")).to_contain_text("Tiket ditahan selama")
            await expect(page.locator(".reservation-banner")).to_contain_text("Batas ini mencakup pengisian data hingga pembayaran.")
            await page.get_by_label("QRIS", exact=False).check()
            await page.get_by_role("button", name="Tinjau pesanan").last.click()
            await expect(page.get_by_role("heading", name="Tinjau pesanan")).to_be_visible()
            for text in ("Nusa Malam", "Festival", "Tribune", "Nadia Pembeli", "nadia@example.test", "QRIS", "907.500"):
                await expect(page.locator("#confirmation")).to_contain_text(text)
            await page.locator("#confirmation").get_by_role("button", name="Ubah metode").click()
            await page.get_by_label("GoPay", exact=False).check()
            await page.get_by_role("button", name="Tinjau pesanan").last.click()
            await expect(page.locator("#confirmation")).to_contain_text("GoPay")
            if width in (320, 390):
                await page.screenshot(path=str(OUTPUT / f"confirmation-{width}.png"), full_page=True)
            if width == 1440:
                state["changedTotal"] = True
                await page.get_by_role("button", name="Buat pesanan dan bayar").last.click()
                await expect(page.locator("#accept-total")).to_be_visible()
                await expect(page.locator("#confirmation")).to_contain_text("berubah dari Rp 907.500 menjadi Rp 912.500")
                await expect(page.locator("#confirmation .step-actions .button")).to_be_disabled()
                await page.locator("#accept-total").evaluate("e => { e.checked = true; e.dispatchEvent(new Event('change', { bubbles: true })); }")
                await page.get_by_role("button", name="Lanjutkan pembayaran").last.click()
                await expect(page.get_by_role("heading", name="Status pembayaran")).to_be_visible()
                assert len(state["posts"]) == 1 and "identity" not in state["posts"][0]["body"]["buyer"]
            results.append({"width": width, "passed": True})
            await context.close()
        await browser.close()
    assert not errors, errors
    (OUTPUT / "checkout-automated.json").write_text(json.dumps({"status": "passed", "generatedAt": datetime.now(timezone.utc).isoformat(), "baseUrl": BASE, "results": results, "pageErrors": errors, "realSandbox": "not tested", "emailDelivery": "not tested", "humanUAT": "pending"}, indent=2) + "\n")
    print(json.dumps(results, indent=2))


asyncio.run(main())
