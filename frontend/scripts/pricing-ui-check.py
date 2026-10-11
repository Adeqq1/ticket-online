"""Phase 41-42 Chrome checks with dummy APIs; desktop and mobile emulation.

Run from frontend/: python3 scripts/pricing-ui-check.py [Vite URL]
"""
import sys
sys.dont_write_bytecode = True

import asyncio
from copy import deepcopy
from importlib.util import spec_from_file_location, module_from_spec
from pathlib import Path
from urllib.parse import urlparse

from playwright.async_api import async_playwright, expect


def fixture(name, filename):
    spec = spec_from_file_location(name, Path(__file__).with_name(filename))
    module = module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


admin = fixture("admin_fixture", "admin-ui-check.py")
public = fixture("public_fixture", "public-ui-check.py")


async def check(browser, base, width):
    context = await browser.new_context(viewport={"width": width, "height": 900}, timezone_id="Asia/Jakarta", is_mobile=width < 600, has_touch=width < 600)
    await context.add_init_script(admin.SESSION_INIT)
    page = await context.new_page()
    errors = []
    page.on("pageerror", lambda e: errors.append(str(e)))
    state = {"settings": {"adminFee": 7500, "revision": 1}, "vouchers": [], "history": []}
    mode = {"save": 503, "hold": None, "role": "ADMIN", "quote": 200, "changed": False}
    saves = []

    async def api(route):
        path = urlparse(route.request.url).path
        method = route.request.method
        if path == "/api/v1/staff/me":
            await route.fulfill(json={"staff": admin.ADMIN if mode["role"] == "ADMIN" else admin.STAFF})
        elif path == "/api/v1/admin/events":
            await route.fulfill(json={"events": [admin.EVENT]})
        elif path == "/api/v1/admin/pricing" and method == "GET":
            await route.fulfill(json=state)
        elif path == "/api/v1/admin/pricing":
            assert route.request.headers["authorization"] == "Bearer " + admin.TOKEN
            saves.append(route.request.post_data_json)
            if mode["hold"]:
                await mode["hold"].wait()
            if mode["save"] != 200:
                await route.fulfill(status=mode["save"], json={"error": {"code": "PRICING_CONFLICT", "message": "Simpan belum berhasil. Muat ulang bila data berubah."}})
            else:
                state["settings"] = {**saves[-1], "revision": state["settings"]["revision"] + 1}
                await route.fulfill(json=state["settings"])
        elif path.startswith("/api/v1/admin/vouchers"):
            v = route.request.post_data_json
            assert route.request.headers["authorization"] == "Bearer " + admin.TOKEN
            assert v["eventId"] == "nusa-malam" and v["minimumSubtotal"] == 100000 and v["quota"] == 5
            saved = {**v, "revision": v["revision"] + 1}
            state["vouchers"] = [{**saved, "usage": {"held": 1, "consumed": 2, "released": 1}}]
            await route.fulfill(status=201 if method == "POST" else 200, json=saved)
        elif path.endswith("/quote"):
            assert route.request.headers.get("idempotency-key")
            code = route.request.post_data_json["voucherCode"]
            if mode["quote"] != 200:
                await route.fulfill(status=422, json={"error": {"code": "INVALID_VOUCHER", "message": "Kuota voucher habis."}})
            else:
                discount = 50000 if code else 0
                await route.fulfill(json={"subtotal": 275000, "adminFee": state["settings"]["adminFee"], "discount": discount, "total": 275000 + state["settings"]["adminFee"] - discount, "voucher": {"code": code, "kind": "FIXED", "value": 50000} if code else None})
        elif path.endswith("/checkout"):
            body = {**public.order("PENDING", "PENDING"), "adminFee": state["settings"]["adminFee"] + 1000, "total": 275000 + state["settings"]["adminFee"] + 1000, "accessToken": public.TOKEN}
            mode["changed"] = True
            await route.fulfill(status=201, json=body)
        else:
            assert not path.endswith("/payments"), "payment started before changed total was reviewed"
            status, body = public.reply(urlparse(route.request.url), "standard")
            await route.fulfill(status=status, json=body)

    await page.route("**/api/v1/**", api)
    await page.goto(base + "/admin/pricing")
    fee = page.get_by_label("Nominal tetap (Rp)", exact=True)
    await expect(fee).to_have_value("7500")
    await fee.fill("9000")
    await expect(page.get_by_text("Perubahan belum disimpan.", exact=True)).to_be_visible()
    await expect(page.get_by_text("Pratinjau tanpa voucher:", exact=False)).to_contain_text("109.000")
    page.once("dialog", lambda d: d.dismiss())
    await page.get_by_role("button", name="Simpan biaya admin", exact=True).click()
    assert not saves
    page.once("dialog", lambda d: d.accept())
    await page.get_by_role("button", name="Simpan biaya admin", exact=True).click()
    await expect(page.get_by_role("alert")).to_contain_text("Simpan belum berhasil")
    await expect(fee).to_have_value("9000")
    mode["save"] = 200
    mode["hold"] = asyncio.Event()
    page.once("dialog", lambda d: d.accept())
    await page.get_by_role("button", name="Simpan biaya admin", exact=True).click()
    await expect(fee).to_be_disabled()
    await expect(page.get_by_role("button", name="Muat ulang", exact=True)).to_be_disabled()
    mode["hold"].set()
    await expect(page.get_by_role("status").filter(has_text="Biaya admin tersimpan")).to_be_visible()
    assert len(saves) == 2 and state["settings"]["adminFee"] == 9000
    await page.get_by_label("Kode voucher", exact=True).fill("dummy50")
    await page.get_by_label("Jenis diskon").select_option("FIXED")
    await page.get_by_label("Nilai diskon", exact=True).fill("50000")
    await page.get_by_label("Cakupan konser").select_option("nusa-malam")
    await page.get_by_label("Minimum subtotal tiket (Rp)", exact=True).fill("100000")
    await page.get_by_label("Kuota order", exact=True).fill("5")
    await page.get_by_label("Voucher aktif", exact=True).check()
    page.once("dialog", lambda d: d.accept())
    await page.get_by_role("button", name="Simpan voucher", exact=True).click()
    await expect(page.get_by_role("status").filter(has_text="Voucher tersimpan")).to_be_visible()
    await expect(page.get_by_label("Kode voucher", exact=True)).to_have_attribute("readonly", "")
    await expect(page.locator(".voucher-list")).to_contain_text("Ditahan 1 · Dipakai 2 · Dilepas 1 · Kuota 5")
    await page.get_by_label("Voucher aktif", exact=True).uncheck()
    page.once("dialog", lambda d: d.accept())
    await page.get_by_role("button", name="Simpan voucher", exact=True).click()
    await expect(page.locator(".voucher-list")).to_contain_text("Nonaktif")
    assert await page.evaluate("document.documentElement.scrollWidth <= innerWidth"), "admin overflow"
    assert await page.get_by_role("button", name="Voucher baru", exact=True).evaluate("el => el.getBoundingClientRect().height >= 44")

    await page.goto(base + "/checkout/nusa-malam?festival=1")
    await expect(page.locator(".grand-total")).to_contain_text("284.000")
    if width < 600:
        await page.locator(".summary-breakdown summary").click()
    await page.get_by_label("Kode promo", exact=True).fill("DUMMY50")
    await page.get_by_role("button", name="Gunakan", exact=True).click()
    await expect(page.locator(".grand-total")).to_contain_text("234.000")
    mode["quote"] = 422
    await page.get_by_label("Kode promo", exact=True).fill("EXHAUSTED")
    await page.get_by_role("button", name="Gunakan", exact=True).click()
    await expect(page.get_by_role("alert")).to_contain_text("Kuota voucher habis")
    await expect(page.locator(".grand-total")).to_contain_text("Belum tersedia")
    mode["quote"] = 200
    await page.get_by_label("Kode promo", exact=True).fill("")
    await page.get_by_role("button", name="Gunakan", exact=True).click()
    await expect(page.locator(".grand-total")).to_contain_text("284.000")
    for selector, value in [("#buyer-name", "Dummy Buyer"), ("#buyer-email", "buyer@example.test"), ("#buyer-phone", "08123456789"), ("#buyer-identity", "1234567890123456"), ("#attendee-festival-0", "Dummy Guest")]:
        await page.locator(selector).fill(value)
    await page.get_by_role("button", name="Lanjut ke pembayaran").click()
    await page.locator('input[name="payment"][value="QRIS"]').check()
    await page.get_by_role("button", name="Tinjau pesanan").click()
    await page.get_by_role("button", name="Buat pesanan dan bayar", exact=True).click()
    await expect(page.locator(".form-error").filter(has_text="Nominal berubah sejak pratinjau")).to_be_visible()
    await expect(page.locator(".grand-total")).to_contain_text("285.000")
    assert mode["changed"] and not errors, errors
    mode["role"] = "STAFF"
    await page.goto(base + "/admin/pricing")
    await page.wait_for_url("**/admin/scan")
    await context.close()
    print(f"PASS {width}px: fee preview/confirmation/failure, voucher CRUD/usage, server quote/quota error, changed-total review, STAFF denied")


async def main():
    base = sys.argv[1] if len(sys.argv) > 1 else "http://127.0.0.1:5175"
    async with async_playwright() as p:
        browser = await p.chromium.launch(channel="chrome", headless=True)
        try:
            for width in (1440, 390):
                await check(browser, base, width)
        finally:
            await browser.close()


if __name__ == "__main__":
    asyncio.run(main())
