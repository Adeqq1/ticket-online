"""Custom admin calendars with existing API fixtures. Run: python3 scripts/admin-date-check.py [URL]."""
import asyncio
import sys
sys.dont_write_bytecode = True
from copy import deepcopy
from datetime import datetime, timezone
from importlib.util import module_from_spec, spec_from_file_location
from pathlib import Path
from urllib.parse import parse_qs, urlparse

from playwright.async_api import async_playwright, expect

spec = spec_from_file_location("admin", Path(__file__).with_name("admin-ui-check.py"))
admin = module_from_spec(spec)
spec.loader.exec_module(admin)


async def calendar(page, field_id, required=False, seconds=False):
    field = page.locator("#" + field_id)
    wrapper = field.locator("xpath=../..")
    trigger = wrapper.locator(".date-trigger")
    popup = page.locator("#" + field_id + "-calendar")
    today = "2026-10-12" if field_id.startswith("voucher-") else "2026-10-13"
    today_display = "12/10/2026" if field_id.startswith("voucher-") else "13/10/2026"
    await expect(field).to_have_attribute("inputmode", "text")
    original = await field.input_value()
    await trigger.click()
    await expect(popup).to_be_visible()
    await popup.get_by_label("Tahun", exact=True).fill("2024")
    await popup.get_by_label("Tahun", exact=True).press("Tab")
    await popup.get_by_role("combobox", name="Bulan", exact=True).select_option("2")
    await expect(field).to_have_value(original)
    leap = popup.locator('[data-date="2024-02-29"]')
    await leap.focus()
    await leap.press("ArrowRight")
    await expect(popup.locator('[data-date="2024-03-01"]')).to_be_focused()
    await page.keyboard.press("ArrowLeft")
    await page.keyboard.press("Home")
    await expect(popup.locator('[data-date="2024-02-26"]')).to_be_focused()
    await page.keyboard.press("End")
    await expect(popup.locator('[data-date="2024-03-03"]')).to_be_focused()
    await page.keyboard.press("PageUp")
    await expect(popup.locator('[data-date="2024-02-03"]')).to_be_focused()
    await page.keyboard.press("PageDown")
    await page.keyboard.press("Space")
    await expect(field).to_have_value("03/03/2024")
    await expect(popup).not_to_be_visible()
    await expect(trigger).to_be_focused()
    await trigger.click()
    await page.keyboard.press("Escape")
    await expect(field).to_have_value("03/03/2024")
    await expect(trigger).to_be_focused()
    await trigger.click()
    await page.mouse.click(0, 0)
    await expect(popup).not_to_be_visible()
    await trigger.click()
    await popup.get_by_role("button", name="Kosongkan", exact=True).click()
    await expect(field).to_have_value("")
    assert await field.evaluate("el => el.checkValidity()") == (not required)
    await trigger.click()
    await expect(popup.locator(f'[data-date="{today}"][aria-current="date"]')).to_be_focused()
    await popup.get_by_role("button", name="Hari ini", exact=True).click()
    await expect(field).to_have_value(today_display)
    assert await field.evaluate("el => el.checkValidity()")
    if await wrapper.locator(".date-time").count():
        for part in ("Jam", "Menit", "Detik") if seconds else ("Jam", "Menit"):
            await expect(wrapper.get_by_label(part + " ", exact=False)).to_have_value("00")
        time = ("23", "47", "58") if seconds else ("23", "47")
        for i, value in enumerate(time):
            await wrapper.locator(".date-time input").nth(i).fill(value)
        await trigger.click()
        await popup.get_by_role("button", name="Hari ini", exact=True).click()
        await expect(field).to_have_value(today_display)
        for i, value in enumerate(time):
            await expect(wrapper.locator(".date-time input").nth(i)).to_have_value(value)
        await field.fill("29/02/2024")
        await trigger.click()
        await popup.locator('[data-date="2024-02-28"]').click()
        await expect(wrapper.locator(".date-time input").first).to_have_value("23")
        await wrapper.locator(".date-time input").first.fill("24")
        assert not await field.evaluate("el => el.checkValidity()")
        await wrapper.locator(".date-time input").first.fill("23")
    await field.fill("29/02/2026")
    assert not await field.evaluate("el => el.checkValidity()")
    await field.fill("12/")
    assert not await field.evaluate("el => el.checkValidity()")
    await field.fill("29/02/2024")
    assert await field.evaluate("el => el.checkValidity()")


async def layout(page, field_id, width):
    trigger = page.locator("#" + field_id).locator("xpath=../..").locator(".date-trigger")
    popup = page.locator("#" + field_id + "-calendar")
    for height, enlarged in ((900, False), (260, False), (420, False), (390, True)):
        await page.set_viewport_size({"width": width, "height": height})
        await trigger.click()
        if enlarged:
            await popup.evaluate("el => { window.dateFonts = [...el.querySelectorAll('*')].map(n => [n, n.style.fontSize, getComputedStyle(n).fontSize]); for (const [n, , size] of window.dateFonts) n.style.fontSize = parseFloat(size) * 2 + 'px'; }")
            await page.dispatch_event("body", "resize")
            await page.evaluate("dispatchEvent(new Event('resize'))")
        if height == 420:
            await page.set_viewport_size({"width": width, "height": 260})
            await page.wait_for_function("document.querySelector('.admin-calendar[open]').getBoundingClientRect().bottom <= innerHeight")
            await page.set_viewport_size({"width": width, "height": height})
            await page.wait_for_function("document.querySelector('.admin-calendar[open]').getBoundingClientRect().bottom <= innerHeight")
        metrics = await popup.evaluate("""el => {
          const r = el.getBoundingClientRect();
          return {left:r.left, right:r.right, top:r.top, bottom:r.bottom, width:innerWidth, height:innerHeight,
            overflow:el.scrollWidth > el.clientWidth, pageOverflow:document.documentElement.scrollWidth > innerWidth,
            targets:[...el.querySelectorAll('button,select,input')].map(n => { const b=n.getBoundingClientRect(); return [b.width,b.height]; })};
        }""")
        assert metrics["left"] >= 0 and metrics["right"] <= width + 1, metrics
        assert metrics["top"] >= 0 and metrics["bottom"] <= height + 1, metrics
        assert not metrics["overflow"] and not metrics["pageOverflow"], metrics
        assert all(w >= 44 and h >= 44 for w, h in metrics["targets"]), metrics
        await popup.get_by_role("button", name="Tutup", exact=True).click()
        if enlarged: await page.evaluate("() => { for (const [n, size] of window.dateFonts) n.style.fontSize = size; }")
    await page.set_viewport_size({"width": width, "height": 900})
    await trigger.click()
    await page.screenshot(path=f"/tmp/admin-calendar-{width}.png")
    # A locked or hidden parent must close the top-layer dialog without changing the draft.
    field = page.locator("#" + field_id)
    before = await field.input_value()
    await field.evaluate("el => el.disabled = true")
    await expect(popup).not_to_be_visible()
    await field.evaluate("el => el.disabled = false")
    await trigger.click()
    await trigger.evaluate("el => el.parentElement.parentElement.hidden = true")
    await expect(popup).not_to_be_visible()
    await trigger.evaluate("el => el.parentElement.parentElement.hidden = false")
    await expect(field).to_have_value(before)


async def check(browser, base, width):
    context = await browser.new_context(viewport={"width": width, "height": 900}, timezone_id="America/New_York")
    await context.add_init_script(admin.SESSION_INIT)
    page = await context.new_page()
    await page.clock.set_fixed_time(datetime(2026, 10, 12, 19, 30, tzinfo=timezone.utc))
    errors, requests, previews, vouchers = [], [], [], []
    page.on("pageerror", lambda error: errors.append(str(error)))
    page.on("dialog", lambda dialog: dialog.accept())

    async def api(route):
        path = urlparse(route.request.url).path
        requests.append(route.request.url)
        if path.endswith("/admin/vouchers"):
            vouchers.append(route.request.post_data_json)
            return await route.fulfill(status=201, json={**vouchers[-1], "revision": 1})
        if path.endswith("/changes/preview"):
            previews.append(route.request.post_data_json)
            return await route.fulfill(json={"event": {}, "snapshot": "test", "impact": {"activeReservations": 0, "pendingOrders": 0, "pendingPayments": 0, "paidOrders": 0, "issuedTickets": 0, "checkIns": 0, "existingRefunds": 0, "automaticAmount": 0, "manualAmount": 0}})
        if path.endswith("/changes"): return await route.fulfill(json=[])
        if path.endswith("/admin/pricing"): return await route.fulfill(json={"settings": {"adminFee": 7500, "revision": 1}, "vouchers": [], "history": []})
        status, data = admin.body(route.request.url, "standard", "ADMIN")
        if path.endswith("/admin/orders/" + admin.ORDER):
            data = {**data, "status": "REFUND_PENDING", "refund": {"status": "MANUAL_REQUIRED", "amount": 280000, "reason": "Transfer diperlukan", "manualReference": "", "manualPaidAt": None}}
        await route.fulfill(status=status, json=data)

    await page.route("**/api/v1/**", api)
    await page.goto(base + "/admin/orders")
    if width < 768: await page.get_by_role("button", name="Buka pencarian pesanan").click()
    await calendar(page, "orders-date-from")
    await calendar(page, "orders-date-to")
    await layout(page, "orders-date-from", width)
    await page.get_by_role("button", name="Cari", exact=True).click()
    await page.wait_for_timeout(100)
    query = parse_qs(urlparse(requests[-1]).query)
    assert query["dateFrom"] == ["2024-02-29"] and query["dateTo"] == ["2024-02-29"], query
    await page.locator("#orders-date-from").fill("12/")
    await page.get_by_role("button", name="Reset", exact=True).click()
    await expect(page.locator("#orders-date-from")).to_have_value("")

    for route, prefix in (("/admin/reports", "sales"), ("/admin/reports/conversion", "conversion")):
        await page.goto(base + route)
        if width < 768: await page.get_by_role("button", name="Buka filter laporan").click()
        for suffix in ("from", "to"): await calendar(page, prefix + "-date-" + suffix, required=True)
        await page.get_by_role("button", name="Terapkan", exact=True).click()
        await page.wait_for_timeout(100)
        assert parse_qs(urlparse(requests[-1]).query)["dateFrom"] == ["2024-02-29"]
        for suffix in ("from", "to"):
            await page.locator(f"#{prefix}-date-{suffix}").locator("xpath=../..").locator(".date-trigger").click()
            await page.locator(f"#{prefix}-date-{suffix}-calendar").get_by_role("button", name="Hari ini", exact=True).click()
        await page.get_by_role("button", name="Terapkan", exact=True).click()
        await page.wait_for_timeout(100)
        query = parse_qs(urlparse(requests[-1]).query)
        assert query["dateFrom"] == ["2026-10-13"] and query["dateTo"] == ["2026-10-13"], query

    await page.goto(base + "/admin/events")
    await page.locator("#event-manage-nusa-malam").click()
    await page.get_by_role("button", name="Jadwal & lokasi", exact=True).click()
    await calendar(page, "event-starts-at", required=True)
    await page.locator("#event-starts-at").fill("12/")
    await page.get_by_role("button", name="Kembali ke daftar", exact=True).click()
    await expect(page.get_by_role("dialog", name="Buang perubahan?", exact=True)).to_be_visible()
    await admin.answer_confirmation(page, False)
    await expect(page.locator("#event-starts-at")).to_have_value("12/")
    await page.get_by_role("button", name="Perubahan acara", exact=True).click()
    await page.get_by_label("Keputusan").select_option("RESCHEDULED")
    for field_id in ("change-starts-at", "change-deadline"): await calendar(page, field_id, required=True)
    await page.get_by_label("Alasan untuk audit").fill("Perubahan jadwal")
    await page.get_by_label("Pengumuman untuk pembeli").fill("Jadwal diperbarui")
    await admin.fill_admin_date(page, "Jadwal baru (WIB)", "2027-12-01T19:30")
    await admin.fill_admin_date(page, "Tenggat permintaan refund (WIB)", "2027-11-01T18:00")
    await page.get_by_role("button", name="Pratinjau dampak", exact=True).click()
    await expect(page.get_by_role("heading", name="Dampak keputusan")).to_be_visible()
    assert previews[-1]["startsAt"] == "2027-12-01T19:30:00+07:00", previews
    await page.get_by_role("button", name="Buka kalender Jadwal baru (WIB)").click()
    await page.locator('#change-starts-at-calendar [data-date="2027-12-02"]').click()
    await expect(page.get_by_role("heading", name="Dampak keputusan")).to_have_count(0)
    await expect(page.get_by_label("Jam Jadwal baru (WIB)", exact=True)).to_have_value("19")

    await page.goto(base + "/admin/orders?orderId=" + admin.ORDER)
    await calendar(page, "manual-paid-at", required=True, seconds=True)
    await page.locator("#manual-paid-at").fill("12/")
    await page.get_by_role("button", name="Lanjutkan", exact=True).click()
    await expect(page.locator("#manual-step-title")).to_have_text("Bukti transfer")
    await page.goto(base + "/admin/pricing")
    await page.locator("#voucher-create-opener").click()
    await page.get_by_label("Kode voucher", exact=True).fill("TESTDATE")
    await page.get_by_role("button", name="Lanjutkan", exact=True).click()
    for field_id in ("voucher-starts", "voucher-ends"): await calendar(page, field_id, required=True)
    await page.locator("#voucher-starts").fill("12/")
    await page.get_by_role("button", name="Lanjutkan", exact=True).click()
    await expect(page.locator("#voucher-starts")).to_be_focused()
    await page.get_by_role("button", name="Kembali ke daftar", exact=True).click()
    await expect(page.get_by_role("dialog", name="Buang perubahan?", exact=True)).to_be_visible()
    await admin.answer_confirmation(page, False)
    await page.get_by_role("button", name="Kembali ke daftar", exact=True).click()
    await admin.answer_confirmation(page)
    await page.locator("#voucher-create-opener").click()
    await page.get_by_label("Kode voucher", exact=True).fill("TESTDATE")
    await page.get_by_role("button", name="Lanjutkan", exact=True).click()
    assert await page.locator("#voucher-starts").evaluate("el => el.checkValidity()")
    await admin.fill_admin_date(page, "Mulai berlaku", "2027-01-01T10:15")
    await admin.fill_admin_date(page, "Akhir masa berlaku", "2027-07-01T11:45")
    await page.get_by_role("button", name="Lanjutkan", exact=True).click()
    await page.get_by_role("button", name="Buat voucher", exact=True).click()
    await admin.answer_confirmation(page)
    await expect(page.locator("#voucher-create-opener")).to_be_visible()
    # Device timezone: New York is UTC-5 in January and UTC-4 in July.
    assert vouchers[-1]["startsAt"] == "2027-01-01T15:15:00.000Z", vouchers
    assert vouchers[-1]["endsAt"] == "2027-07-01T15:45:00.000Z", vouchers
    assert not errors, errors
    await context.close()
    print(f"PASS {width}: all 12 fields, keyboard, strict dates, time preservation, draft guards, WIB payload, preview invalidation, short screens and 200% text", flush=True)


async def main():
    async with async_playwright() as p:
        browser = await p.chromium.launch(channel="chrome", headless=True)
        try:
            for width in (320, 390, 767, 768, 1440): await check(browser, sys.argv[1] if len(sys.argv) > 1 else "http://localhost:5173", width)
        finally: await browser.close()


if __name__ == "__main__": asyncio.run(main())
