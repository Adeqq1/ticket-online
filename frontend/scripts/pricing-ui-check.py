"""Phase 41-42 Chrome checks with dummy APIs; desktop and mobile emulation.

Run from frontend/: python3 scripts/pricing-ui-check.py [Vite URL]
"""
import sys
sys.dont_write_bytecode = True

import asyncio
import json
from copy import deepcopy
from importlib.util import spec_from_file_location, module_from_spec
from pathlib import Path
from urllib.parse import urlparse

from playwright.async_api import async_playwright, expect
from runpy import run_path

confirmation_helpers = run_path(str(Path(__file__).with_name('admin-confirm.py')))
answer_confirmation = confirmation_helpers['answer_confirmation']
auto_confirm = confirmation_helpers['auto_confirm']


def fixture(name, filename):
    spec = spec_from_file_location(name, Path(__file__).with_name(filename))
    module = module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


admin = fixture("admin_fixture", "admin-ui-check.py")
public = fixture("public_fixture", "public-ui-check.py")
staff_audit = fixture("staff_audit", "staff-review-check.py").STYLE_AUDIT


async def check_fee_choices(browser, base, width):
    context = await browser.new_context(viewport={"width": width, "height": 900})
    await context.add_init_script(admin.SESSION_INIT)
    page = await context.new_page()
    errors, saves = [], []
    page.on("pageerror", lambda error: errors.append(str(error)))
    settings = {"adminFee": 1210000, "revision": 1}

    async def api(route):
        path = urlparse(route.request.url).path
        if path.endswith("/admin/pricing"):
            if route.request.method != "GET":
                payload = route.request.post_data_json
                saves.append(payload)
                assert payload["revision"] == settings["revision"]
                settings.update(adminFee=payload["adminFee"], revision=settings["revision"] + 1)
                return await route.fulfill(json=settings)
            return await route.fulfill(json={"settings": settings, "vouchers": [], "history": []})
        status, payload = admin.body(path, "standard", "ADMIN")
        await route.fulfill(status=status, json=payload)

    await page.route("**/api/v1/**", api)
    await page.goto(base + "/admin/pricing")
    preset = page.get_by_label("Nominal tetap (Rp)", exact=True)
    custom = page.get_by_label("Nominal kustom (Rp)", exact=True)
    sample = page.get_by_label("Subtotal tiket untuk pratinjau (Rp)", exact=True)
    submit = page.get_by_role("button", name="Simpan biaya admin", exact=True)
    preview = page.get_by_text("Pratinjau tanpa voucher:", exact=False)
    await expect(preset).to_have_value("custom")
    await expect(custom).to_have_value("1210000")
    await expect(submit).to_be_disabled()
    await expect(preset.locator("option")).to_have_text(["Rp 0", "Rp 2.500", "Rp 5.000", "Rp 7.500", "Rp 10.000", "Isi sendiri"])
    # A custom amount matching a preset keeps the input mounted while typing.
    await custom.fill("7500")
    await expect(preset).to_have_value("custom")
    await expect(custom).to_have_value("7500")
    for amount in (0, 2500, 5000, 7500, 10000):
        await preset.select_option(str(amount))
        await expect(custom).to_have_count(0)
        await expect(submit).to_be_enabled()
        total = f"{100000 + amount:,}".replace(",", ".")
        await expect(preview).to_contain_text(total)
    await preset.select_option("0")
    await submit.click(); await answer_confirmation(page)
    await expect(page.get_by_role("status").filter(has_text="Biaya admin tersimpan")).to_be_visible()
    assert saves == [{"adminFee": 0, "revision": 1}], saves
    await page.reload()
    await expect(preset).to_have_value("0")
    await expect(custom).to_have_count(0)
    await preset.focus(); await preset.select_option("custom")
    await expect(custom).to_be_focused()
    await expect(custom).to_have_value("0")
    for invalid in ("", "-1", "1.5", "1000000001"):
        await custom.fill(invalid)
        await expect(submit).to_be_disabled()
        await page.locator(".fee-panel form").dispatch_event("submit")
        await expect(page.locator(".admin-confirm[open]")).to_have_count(0)
        assert len(saves) == 1
    await custom.fill("1000000000")
    await expect(submit).to_be_enabled()
    await custom.fill("12345")
    await expect(sample).to_have_attribute("readonly", "")
    await sample.focus(); await sample.press("Control+A"); await sample.press("9")
    await expect(sample).to_have_value("100.000")
    await expect(preview).to_contain_text("112.345")
    await submit.click(); await answer_confirmation(page, False)
    assert len(saves) == 1
    await expect(custom).to_have_value("12345")
    await submit.click(); await answer_confirmation(page)
    await expect(page.get_by_role("status").filter(has_text="Biaya admin tersimpan")).to_be_visible()
    assert saves[-1] == {"adminFee": 12345, "revision": 2}, saves
    await page.reload()
    await expect(preset).to_have_value("custom")
    await expect(custom).to_have_value("12345")
    await expect(preview).to_contain_text("112.345")
    output = Path('/tmp/phase51-55/pricing')
    output.mkdir(parents=True, exist_ok=True)
    await page.screenshot(path=str(output / f"fee-choices-{width}.png"), full_page=True)
    await custom.focus()
    await page.evaluate("""() => {
      const fonts = [...document.querySelectorAll('.fee-panel *')].map(el => [el, getComputedStyle(el).fontSize]);
      for (const [el, size] of fonts) el.style.fontSize = parseFloat(size) * 2 + 'px';
    }""")
    audit = await page.evaluate(staff_audit)
    assert not errors and not audit["failures"] and not audit["overflow"], (errors, audit)
    await page.screenshot(path=str(output / f"fee-choices-{width}-text200.png"), full_page=True)
    await context.close()
    print(f"PASS {width}: fee presets/custom, zero, validation, fixed subtotal, confirmation, reload, focus and text200")


async def check(browser, base, width):
    context = await browser.new_context(viewport={"width": width, "height": 900}, timezone_id="Asia/Jakarta", is_mobile=width < 768, has_touch=width < 768)
    await context.add_init_script(admin.SESSION_INIT)
    page = await context.new_page()
    errors = []
    page.on("pageerror", lambda e: errors.append(str(e)))
    voucher = {"code": "HEMAT10", "kind": "PERCENT", "value": 10, "startsAt": "2020-01-01T00:00:00Z", "endsAt": "2099-01-01T00:00:00Z", "active": True, "eventId": "", "minimumSubtotal": 0, "quota": 1000000000, "revision": 1, "usage": {"held": 0, "consumed": 0, "released": 0}}
    state = {"settings": {"adminFee": 7500, "revision": 1}, "vouchers": [voucher, {**voucher, "code": "PROMO_LIBURAN_AKHIR_TAHUN_2026", "kind": "FIXED", "value": 100000, "eventId": "nusa-malam", "active": False}], "history": []}
    state["vouchers"] += [{**voucher, "code": f"PROMO_{i:02d}", "active": False} for i in range(10)]
    state["history"] = [
        {"actor": "Admin Event", "objectId": "admin-fee", "before": {"adminFee": 5000}, "after": {"adminFee": 7500}, "createdAt": "2026-10-11T17:00:07Z"},
        {"actor": "Admin Event", "objectId": "HEMAT10", "before": voucher, "after": {**voucher, "active": False}, "createdAt": "2026-10-12T03:00:07Z"},
    ]
    mode = {"save": 422, "hold": None, "role": "ADMIN", "quote": 200, "changed": False}
    saves = []

    async def api(route):
        path = urlparse(route.request.url).path
        method = route.request.method
        if path == "/api/v1/staff/me":
            await route.fulfill(json={"staff": admin.ADMIN if mode["role"] == "ADMIN" else admin.STAFF})
        elif path == "/api/v1/admin/events":
            await route.fulfill(json={"events": [{**admin.EVENT, "artist": "Konser Nusa Malam dan Perayaan Akhir Tahun Bersama"}]})
        elif path == "/api/v1/admin/pricing" and method == "GET":
            await route.fulfill(json={**state, "vouchers": [v for v in state["vouchers"] if not mode.get("hideSaved") or v["code"] != "DUMMY50"]})
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
            state["vouchers"] = [item for item in state["vouchers"] if item["code"] != saved["code"]]
            state["vouchers"].insert(8 if method == "POST" else 0, {**saved, "usage": {"held": 1, "consumed": 2, "released": 1}})
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
    if width >= 768:
        await expect(page.locator(".voucher-table th")).to_have_text(["Kode voucher", "Diskon", "Konser", "Masa berlaku (WIB)", "Kuota order", "Penggunaan", "Status", "Aksi"])
        await expect(page.locator(".voucher-table tbody tr")).to_have_count(12)
        assert await page.locator(".voucher-table").evaluate("el => getComputedStyle(el).display") == "table"
        await expect(page.locator(".voucher-table tbody tr").first.locator("td").nth(4)).to_have_text("1.000.000.000")
        await expect(page.locator(".voucher-table .admin-order-status")).to_have_text(["Aktif"] + ["Nonaktif"] * 11)
        assert await page.locator(".voucher-table button, .voucher-table .admin-order-status").evaluate_all("els => els.every(el => getComputedStyle(el).whiteSpace === 'nowrap')"), "Actions and status labels must stay on one line"
        await expect(page.locator(".voucher-table tbody tr").first.locator("time")).to_have_text(["1 Jan 202007.00 WIB", "1 Jan 209907.00 WIB"])
        manage = page.get_by_role("button", name="Kelola voucher HEMAT10", exact=True)
        await expect(manage.locator("svg[aria-hidden=true]")).to_have_count(1)
        await expect(manage).to_have_attribute("title", "Kelola voucher HEMAT10")
        assert await manage.evaluate("el => { const r = el.getBoundingClientRect(); return !el.textContent.trim() && r.width >= 44 && r.height >= 44; }")
        history = page.locator(".pricing-history-table")
        await expect(history.locator("th")).to_have_text(["Waktu (WIB)", "Admin", "Sebelum", "Sesudah"])
        await expect(history.locator("tbody tr")).to_have_count(2)
        await expect(history.locator("tbody tr").first.locator("td").first).to_have_text("12 Okt 202600.00.07 WIB")
        await expect(history.locator("tbody tr").first.locator("dt")).to_have_text(["Biaya admin", "Biaya admin"])
        await expect(history.locator("tbody tr").first.locator("dd")).to_have_text(["Rp 5.000", "Rp 7.500"])
        await expect(history.locator("tbody tr").last.locator(".admin-order-status")).to_have_text(["Aktif", "Nonaktif"])
        await expect(history.locator("tbody tr").last.locator("td").nth(3).locator("dd").nth(3)).to_have_text("1.000.000.000")
        if width == 768:
            scroll = page.locator('.pricing-history .history-table-wrap')
            await scroll.focus()
            await page.keyboard.press("ArrowRight")
            await page.wait_for_function("document.querySelector('.pricing-history .history-table-wrap').scrollLeft > 0")
            await scroll.evaluate("el => el.scrollLeft = 0")
        await page.locator(".pricing-history").screenshot(path=f"/tmp/phase51-55/pricing/desktop-history-{width}.png")
        await page.locator(".voucher-list-panel").screenshot(path=f"/tmp/phase51-55/pricing/desktop-vouchers-{width}.png")
    help_buttons = page.locator(".admin-help-button")
    explanations = page.locator(".pricing-explanation")
    if width < 768:
        await page.locator(".pricing-history > summary").click()
        history = page.locator(".pricing-history .history")
        await expect(history.locator("li").first.locator("dd")).to_have_text(["Rp 5.000", "Rp 7.500"])
        await expect(history.locator("li").last.locator(".admin-order-status")).to_have_text(["Aktif", "Nonaktif"])
        await page.locator(".pricing-history").screenshot(path=f"/tmp/phase51-55/pricing/mobile-history-{width}.png")
        await page.evaluate("""() => {
          window.__historyFonts = [...document.querySelectorAll('.pricing-history *')].map(el => [el, el.style.fontSize, getComputedStyle(el).fontSize]);
          for (const [el, , size] of window.__historyFonts) el.style.fontSize = parseFloat(size) * 2 + 'px';
        }""")
        audit = await page.evaluate(staff_audit)
        assert not audit["failures"] and not audit["overflow"], (width, "history-text200", audit)
        await page.locator(".pricing-history").screenshot(path=f"/tmp/phase51-55/pricing/mobile-history-text200-{width}.png")
        await page.evaluate("() => { for (const [el, size] of window.__historyFonts) el.style.fontSize = size; }")
        await page.locator(".pricing-history > summary").click()
        cards = page.locator(".voucher-list li")
        await expect(cards).to_have_count(5)
        first = cards.first
        await expect(first.locator(".admin-order-status")).to_have_text("Aktif")
        await expect(cards.nth(1).locator(".admin-order-status")).to_have_text("Nonaktif")
        assert await first.locator(".admin-order-status").evaluate("el => el.tagName === 'SPAN' && el.tabIndex === -1")
        await expect(first.locator(".voucher-card-details dt")).to_have_text(["Diskon", "Konser", "Minimum subtotal", "Mulai berlaku", "Berakhir", "Kuota order"])
        await expect(first.locator(".voucher-card-details dd").last).to_have_text("1.000.000.000")
        await expect(first.locator("time")).to_have_text(["1 Jan 202007.00 WIB", "1 Jan 209907.00 WIB"])
        await expect(cards.nth(1).locator(".voucher-card-details dd").nth(1)).to_have_text("Konser Nusa Malam dan Perayaan Akhir Tahun Bersama")
        await expect(first.get_by_role("button", name="Kelola voucher HEMAT10", exact=True)).to_have_text("Kelola voucher")
        assert await first.evaluate("el => { const button = el.querySelector('button').getBoundingClientRect(); return button.height >= 44 && Math.abs(button.width - el.getBoundingClientRect().width + 34) < 2; }")
        await page.locator(".voucher-list-panel").screenshot(path=f"/tmp/phase51-55/pricing/mobile-vouchers-{width}.png")
        await expect(help_buttons).to_have_count(2)
        for explanation in await explanations.all(): await expect(explanation).not_to_be_visible()
        opener = page.locator("#voucher-create-opener")
        await expect(opener.locator("svg[aria-hidden=true]")).to_have_count(1)
        assert await opener.evaluate("el => { const icon = el.querySelector('svg').getBoundingClientRect(); const text = el.querySelector('span').getBoundingClientRect(); const button = el.getBoundingClientRect(); return Math.abs((icon.left + text.right) / 2 - (button.left + button.right) / 2) < 2 && Math.abs(icon.y + icon.height / 2 - text.y - text.height / 2) < 2; }")
        for help_id in ("fee-title", "list-title"):
            trigger = page.locator(f'[aria-controls="{help_id}-help"]')
            popup = page.locator(f"#{help_id}-help")
            await expect(popup).not_to_be_visible()
            await trigger.tap()
            await expect(trigger).to_have_attribute("aria-expanded", "true")
            await expect(popup).to_be_visible()
            await trigger.tap()
            await expect(popup).not_to_be_visible()
            await trigger.focus()
            await page.keyboard.press("Tab")
            await page.keyboard.press("Shift+Tab")
            await expect(trigger).to_be_focused()
            await expect(popup).to_be_visible()
            await page.keyboard.press("Escape")
            await expect(popup).not_to_be_visible()
            await trigger.hover()
            await expect(popup).to_be_visible()
            await popup.hover()
            await expect(popup).to_be_visible()
            await page.mouse.move(0, 0)
            await fee.focus()
            await expect(popup).not_to_be_visible()
            await trigger.tap()
            await page.get_by_role("heading", level=1).tap()
            await expect(popup).not_to_be_visible()
        assert not saves, "Help must not submit the fee or voucher forms"
        await expect(fee).to_have_value("7500")
        await page.locator('[aria-controls="list-title-help"]').tap()
        await page.evaluate("""() => {
          window.__helpFonts = [...document.querySelectorAll('.admin-layout *')].map(el => [el, el.style.fontSize, getComputedStyle(el).fontSize]);
          for (const [el, , size] of window.__helpFonts) el.style.fontSize = parseFloat(size) * 2 + 'px';
        }""")
        await expect(page.locator("#list-title-help")).to_be_visible()
        audit = await page.evaluate(staff_audit)
        assert not audit["failures"] and not audit["overflow"], (width, "help-text200", audit)
        assert await page.locator("#list-title-help").evaluate("el => { const r = el.getBoundingClientRect(); return r.left >= 0 && r.right <= innerWidth; }")
        await page.evaluate("scrollTo({top: 0, behavior: 'instant'})")
        await page.screenshot(path=f"/tmp/phase51-55/pricing/mobile-help-text200-{width}.png", full_page=True)
        await expect(page.locator("#list-title-help")).to_be_visible()
        await page.evaluate("() => { for (const [el, size] of window.__helpFonts) el.style.fontSize = size; }")
        await page.keyboard.press("Escape")
    else:
        await expect(page.locator(".voucher-card-details")).to_have_count(0)
        for trigger in await help_buttons.all(): await expect(trigger).not_to_be_visible()
        for explanation in await explanations.all(): await expect(explanation).to_be_visible()
    await fee.select_option("custom")
    fee = page.get_by_label("Nominal kustom (Rp)", exact=True)
    await expect(fee).to_be_focused()
    await fee.fill("9000")
    await expect(page.get_by_text("Perubahan belum disimpan.", exact=True)).to_be_visible()
    await expect(page.get_by_text("Pratinjau tanpa voucher:", exact=False)).to_contain_text("109.000")
    await page.get_by_role("button", name="Simpan biaya admin", exact=True).click()
    await answer_confirmation(page, False)
    assert not saves
    await page.get_by_role("button", name="Simpan biaya admin", exact=True).click()
    await answer_confirmation(page, True)
    await expect(page.get_by_role("alert")).to_contain_text("Simpan belum berhasil")
    await expect(fee).to_have_value("9000")
    mode["save"] = 200
    mode["hold"] = asyncio.Event()
    await page.get_by_role("button", name="Simpan biaya admin", exact=True).click()
    await answer_confirmation(page, True)
    await expect(fee).to_be_disabled()
    await expect(page.get_by_role("button", name="Muat ulang", exact=True)).to_be_disabled()
    mode["hold"].set()
    await expect(page.get_by_role("status").filter(has_text="Biaya admin tersimpan")).to_be_visible()
    assert len(saves) == 2 and state["settings"]["adminFee"] == 9000
    # An unchanged stored interval may differ only by seconds; review must preserve it.
    original_times = voucher["startsAt"], voucher["endsAt"]
    voucher.update(startsAt="2026-10-01T00:00:10Z", endsAt="2026-10-01T00:00:50Z")
    await page.get_by_role("button", name="Muat ulang", exact=True).click()
    await page.get_by_role("button", name="Kelola voucher HEMAT10", exact=True).click()
    await page.get_by_role("button", name="3. Tinjau", exact=True).click()
    await expect(page.locator("#voucher-section-title")).to_have_text("Tinjau")
    await page.get_by_role("button", name="Kembali ke daftar", exact=True).click()
    voucher.update(startsAt=original_times[0], endsAt=original_times[1])
    await page.get_by_role("button", name="Muat ulang", exact=True).click()
    if width < 768:
        await expect(page.locator(".voucher-editor")).not_to_be_visible()
        await expect(fee).to_be_visible()
        await page.locator("#voucher-create-opener").click()
        await expect(page.locator("#voucher-title")).to_be_focused()
        await page.get_by_label("Kode voucher", exact=True).fill("DRAFT")
        await page.evaluate("""() => {
          window.__voucherFonts = [...document.querySelectorAll('.admin-layout *')].map(el => [el, el.style.fontSize, getComputedStyle(el).fontSize]);
          for (const [el, , size] of window.__voucherFonts) el.style.fontSize = parseFloat(size) * 2 + 'px';
        }""")
        audit = await page.evaluate(staff_audit)
        assert not audit["failures"] and not audit["overflow"], (width, "voucher-editor-text200", audit)
        await page.screenshot(path=f"/tmp/phase51-55/pricing/voucher-editor-text200-{width}.png", full_page=True)
        await page.evaluate("() => { for (const [el, size] of window.__voucherFonts) el.style.fontSize = size; }")
        await page.set_viewport_size({"width": width, "height": 420})
        await page.get_by_label("Kode voucher", exact=True).focus()
        assert await page.evaluate("document.documentElement.scrollWidth <= innerWidth")
        await page.set_viewport_size({"width": width, "height": 900})
        await page.get_by_role("button", name="Batal", exact=True).click()
        await answer_confirmation(page, False)
        await expect(page.get_by_label("Kode voucher", exact=True)).to_have_value("DRAFT")
        await page.get_by_role("button", name="Batal", exact=True).click()
        await answer_confirmation(page, True)
        await expect(page.locator("#voucher-create-opener")).to_be_focused()
        await page.locator("#voucher-create-opener").click()
        await expect(page.get_by_label("Kode voucher", exact=True)).to_have_value("")
    if not await page.locator(".voucher-editor").is_visible(): await page.locator("#voucher-create-opener").click()
    await expect(fee).not_to_be_visible()
    await expect(page.locator(".voucher-list-panel")).not_to_be_visible()
    await expect(page.locator(".pricing-history")).not_to_be_visible()
    await page.get_by_label("Kode voucher", exact=True).fill("dummy50")
    before = len(saves)
    await page.get_by_label("Kode voucher", exact=True).press("Enter")
    await expect(page.locator("#voucher-section-title")).to_have_text("Ketentuan")
    assert len(saves) == before
    await page.get_by_role("button", name="1. Kode & diskon", exact=True).click()
    await page.get_by_label("Jenis diskon").select_option("FIXED")
    await page.get_by_label("Nilai diskon (Rp)", exact=True).fill("50000")
    for step in (0, 1, 2):
        await page.get_by_role("button", name=f"{step + 1}. " + ["Kode & diskon", "Ketentuan", "Tinjau"][step], exact=True).click()
        assert len(saves) == before
        await admin.check_wizard(page, width, stress=True)
        audit = await page.evaluate(staff_audit)
        assert not audit["failures"] and not audit["overflow"], (width, step, audit)
        await page.screenshot(path=f"/tmp/phase51-55/pricing/wizard-{step}-{width}.png", full_page=True)
        await page.evaluate("""() => {
          window.__stepFonts = [...document.querySelectorAll('.admin-layout *')].map(el => [el, el.style.fontSize, getComputedStyle(el).fontSize]);
          for (const [el, , size] of window.__stepFonts) el.style.fontSize = parseFloat(size) * 2 + 'px';
        }""")
        await admin.check_wizard(page, width, stress=True)
        audit = await page.evaluate(staff_audit)
        assert not audit["failures"] and not audit["overflow"], (width, step, "text200", audit)
        await page.screenshot(path=f"/tmp/phase51-55/pricing/wizard-{step}-text200-{width}.png", full_page=True)
        await page.evaluate("() => { for (const [el, size] of window.__stepFonts) el.style.fontSize = size; }")
    await page.locator("#voucher-code").evaluate("el => el.value = ''")
    await page.locator("#voucher-form").dispatch_event("submit")
    await expect(page.locator("#voucher-section-title")).to_have_text("Kode & diskon")
    await expect(page.locator("#voucher-code")).to_be_focused()
    assert len(saves) == before
    await page.get_by_label("Kode voucher", exact=True).fill("dummy50")
    await page.get_by_role("button", name="2. Ketentuan", exact=True).click()
    await admin.fill_admin_date(page, "Akhir masa berlaku", "2000-01-01T00:00")
    await page.get_by_role("button", name="3. Tinjau", exact=True).click()
    await expect(page.locator("#voucher-section-title")).to_have_text("Ketentuan")
    await expect(page.get_by_label("Akhir masa berlaku", exact=True)).to_be_focused()
    await admin.fill_admin_date(page, "Akhir masa berlaku", "2099-01-01T00:00")
    await page.get_by_role("button", name="2. Ketentuan", exact=True).click()
    await page.get_by_label("Cakupan konser").select_option("nusa-malam")
    await page.get_by_role("button", name="2. Ketentuan", exact=True).click()
    await page.get_by_label("Minimum subtotal tiket (Rp)", exact=True).fill("100000")
    await page.get_by_role("button", name="2. Ketentuan", exact=True).click()
    await page.get_by_label("Kuota order", exact=True).fill("5")
    await page.get_by_role("button", name="2. Ketentuan", exact=True).click()
    await page.get_by_label("Voucher aktif", exact=True).check()
    await page.get_by_role("button", name="3. Tinjau", exact=True).click()
    await page.get_by_role("button", name="Buat voucher", exact=True).click()
    await answer_confirmation(page, True)
    await expect(page.get_by_role("status").filter(has_text="Voucher tersimpan")).to_be_visible()
    if width < 768:
        await expect(page.locator("#voucher-manage-DUMMY50")).to_be_focused()
        await expect(page.locator("li").filter(has=page.locator("#voucher-manage-DUMMY50")).locator('[aria-label="Penggunaan voucher"] dd')).to_have_text(["1", "2", "1"])
        await expect(page.locator('.voucher-list-panel [role="status"]')).to_contain_text("6 sampai 10")
        assert await page.locator("#voucher-manage-DUMMY50").evaluate("el => { const r = el.getBoundingClientRect(); return r.left >= 0 && r.right <= innerWidth; }")
        await expect(page.locator("li").filter(has=page.locator("#voucher-manage-DUMMY50")).locator(".voucher-card-details dd").last).to_have_text("5")
        await page.locator("#voucher-manage-DUMMY50").click()
    else:
        await expect(page.locator(".voucher-table tr").filter(has=page.locator("#voucher-manage-DUMMY50")).locator(".admin-table-pairs dd")).to_have_text(["1", "2", "1"])
        await expect(page.locator(".voucher-table tr").filter(has=page.locator("#voucher-manage-DUMMY50")).locator("td").nth(4)).to_have_text("5")
    if not await page.locator(".voucher-editor").is_visible(): await page.locator("#voucher-manage-DUMMY50").click()
    await expect(page.get_by_label("Kode voucher", exact=True)).to_have_attribute("readonly", "")
    await page.get_by_role("button", name="2. Ketentuan", exact=True).click()
    await page.get_by_label("Voucher aktif", exact=True).uncheck()
    await page.get_by_role("button", name="3. Tinjau", exact=True).click()
    await page.get_by_role("button", name="Simpan perubahan voucher", exact=True).click()
    await answer_confirmation(page, True)
    await expect(page.locator(".voucher-list, .voucher-table")).to_contain_text("Nonaktif")
    if width < 768:
        await expect(page.locator("#voucher-manage-DUMMY50")).to_be_focused()
        await expect(page.locator('.voucher-list-panel [role="status"]')).to_contain_text("1 sampai 5")
        # A successful save can precede the refreshed list: focus the result instead of a missing card.
        await page.locator("#voucher-manage-DUMMY50").click()
        await page.get_by_role("button", name="2. Ketentuan", exact=True).click()
        await page.get_by_label("Voucher aktif", exact=True).check()
        mode["hideSaved"] = True
        await page.get_by_role("button", name="3. Tinjau", exact=True).click()
        await page.get_by_role("button", name="Simpan perubahan voucher", exact=True).click()
        await answer_confirmation(page, True)
        await expect(page.locator("#pricing-feedback")).to_be_focused()
        await expect(page.locator("#pricing-feedback")).to_contain_text("belum tersedia di daftar")
        mode["hideSaved"] = False
        await page.get_by_role("button", name="Muat ulang", exact=True).click()
        await expect(page.locator("#voucher-manage-DUMMY50")).to_be_visible()
    assert await page.evaluate("document.documentElement.scrollWidth <= innerWidth"), "admin overflow"
    assert await page.locator("#voucher-create-opener").evaluate("el => el.getBoundingClientRect().height >= 44")

    await page.goto(base + "/checkout/nusa-malam?festival=1")
    await expect(page.locator(".grand-total")).to_contain_text("284.000")
    if not await page.get_by_label("Kode promo", exact=True).is_visible():
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


async def check_recovery(browser, base, width, scenario):
    context = await browser.new_context(viewport={"width": width, "height": 900}, is_mobile=width < 768, has_touch=width < 768)
    page = await context.new_page()
    errors, checkouts, payments, quotes = [], [], [], []
    page.on("pageerror", lambda e: errors.append(str(e)))
    saved_order = None
    lost_response = scenario != "same-quote"

    async def api(route):
        nonlocal saved_order
        path = urlparse(route.request.url).path
        if path.endswith("/quote"):
            assert saved_order is None, "recovery must not requote a converted reservation"
            code = route.request.post_data_json["voucherCode"]
            quotes.append(code)
            await route.fulfill(json={"subtotal": 275000, "adminFee": 7500, "discount": 10000 if code else 0, "total": 272500 if code else 282500, "voucher": {"code": code, "kind": "FIXED", "value": 10000} if code else None})
        elif path.endswith("/checkout"):
            key, payload = route.request.headers["idempotency-key"], route.request.post_data_json
            checkouts.append((key, payload))
            if saved_order is None:
                fee = 10000 if lost_response else 7500
                saved_order = {**public.order("PENDING", "PENDING"), "adminFee": fee, "discount": 10000, "total": 265000 + fee, "voucher": {"code": "DUMMY10", "kind": "FIXED", "value": 10000}, "buyer": payload["buyer"], "attendees": [{"tierId": "festival", "ticketNumber": 1, "name": payload["attendees"][0]["names"][0]}], "payment": None, "accessToken": public.TOKEN}
            assert (key, payload) == checkouts[0], "replay changed the key or frozen payload"
            if lost_response and (len(checkouts) == 1 or scenario == "manual-replay" and len(checkouts) == 2):
                await route.abort("connectionreset")
            else:
                await route.fulfill(status=200 if len(checkouts) > 1 else 201, json=saved_order)
        elif path.endswith("/payments") or path.endswith("/simulate-payment"):
            assert path.split("/")[-2] == public.ORDER_ID
            assert route.request.headers["authorization"] == "Bearer " + public.TOKEN
            payments.append(path)
            if path.endswith("/simulate-payment"):
                await route.fulfill(json={"status": "FAILED", "orderStatus": "PENDING", "amount": saved_order["total"], "tickets": []})
            else:
                await route.fulfill(json={"redirectUrl": base + "/pesanan/" + public.ORDER_ID})
        elif path == "/api/v1/orders/" + public.ORDER_ID:
            await route.fulfill(json=saved_order)
        elif path == "/api/v1/reservations/" + public.RESERVATION_ID:
            await route.fulfill(json={**public.reservation(), "status": "CONVERTED" if saved_order else "ACTIVE"})
        else:
            status, body = public.reply(urlparse(route.request.url), "standard")
            await route.fulfill(status=status, json=body)

    await page.route("**/api/v1/**", api)
    await page.goto(base + "/checkout/nusa-malam?festival=1")
    await expect(page.locator(".grand-total")).to_contain_text("282.500")
    if width < 600:
        await page.locator(".summary-breakdown summary").click()
    await page.get_by_label("Kode promo", exact=True).fill("DUMMY10")
    await page.get_by_role("button", name="Gunakan", exact=True).click()
    await expect(page.locator(".grand-total")).to_contain_text("272.500")
    for selector, value in [("#buyer-name", "Dummy Buyer"), ("#buyer-email", "buyer@example.test"), ("#buyer-phone", "08123456789"), ("#buyer-identity", "1234567890123456"), ("#attendee-festival-0", "Dummy Guest")]:
        await page.locator(selector).fill(value)
    await page.get_by_role("button", name="Lanjut ke pembayaran").click()
    await page.locator('input[name="payment"][value="QRIS"]').check()
    await page.get_by_role("button", name="Tinjau pesanan").click()
    await page.get_by_role("button", name="Buat pesanan dan bayar", exact=True).click()
    if lost_response:
        await expect(page.get_by_role("button", name="Coba ulang checkout", exact=True)).to_be_enabled()
        assert len(checkouts) == 1 and not payments
        await page.reload()
        if scenario == "manual-replay":
            retry = page.get_by_role("button", name="Coba ulang checkout", exact=True)
            await expect(retry).to_be_enabled()
            assert len(checkouts) == 2 and not payments
            await retry.click()
            await expect(page.locator(".form-error").filter(has_text="Pesanan dipulihkan. Periksa rincian dan total akhir")).to_be_visible()
        await expect(page.get_by_role("button", name="Lanjutkan pembayaran", exact=True)).to_be_enabled()
        await expect(page.locator(".grand-total")).to_contain_text("275.000")
        assert len(checkouts) == (3 if scenario == "manual-replay" else 2) and not payments
        assert quotes == ["", "DUMMY10"], "recovery fetched a new quote"
        assert await page.evaluate("Object.keys(sessionStorage).every(key => !key.endsWith(':checkout-attempt'))")
        await page.get_by_role("button", name="Lanjutkan pembayaran", exact=True).click()
    await page.wait_for_function("location.pathname.startsWith('/pesanan/') || document.querySelector('.form-error')?.textContent.includes('Pembayaran simulasi gagal')")
    assert len(payments) == 1 and not errors, (payments, errors)
    assert checkouts[0][1]["voucherCode"] == "DUMMY10"
    await context.close()
    print(f"PASS {width}px {scenario}: original order/payload/key retained, payment starts once after review")


async def check_loading(browser, base, width):
    context = await browser.new_context(viewport={"width": width, "height": 900}, timezone_id="Asia/Jakarta")
    await context.add_init_script(admin.SESSION_INIT)
    page = await context.new_page()
    errors, saves = [], []
    page.on("pageerror", lambda e: errors.append(str(e)))
    state = {"settings": {"adminFee": 7500, "revision": 1}, "vouchers": [], "history": []}
    mode = {"read": 503, "catalog": 503, "save": "success"}
    output = Path("/tmp/phase51-55/pricing")
    output.mkdir(parents=True, exist_ok=True)

    async def api(route):
        path = urlparse(route.request.url).path
        if path.endswith("/staff/me"): await route.fulfill(json={"staff": admin.ADMIN})
        elif path.endswith("/admin/events"):
            await route.fulfill(status=mode["catalog"], json={"events": [admin.EVENT]} if mode["catalog"] == 200 else {"error": {"message": "Katalog gagal"}})
        elif route.request.method == "GET":
            await route.fulfill(status=mode["read"], json=state if mode["read"] == 200 else {"error": {"message": "Route not found"}})
        else:
            payload = route.request.post_data_json
            saves.append(payload)
            if mode["save"] == "conflict":
                await route.fulfill(status=409, json={"error": {"code": "PRICING_CONFLICT", "message": "Revisi berubah. Muat ulang dan periksa data terbaru."}})
                return
            if path.endswith("/admin/pricing"): state["settings"] = {**payload, "revision": state["settings"]["revision"] + 1}
            else:
                assert "usage" not in payload
                state["vouchers"] = [{**payload, "revision": payload["revision"] + 1 if route.request.method == "PUT" else 0, "usage": {"held": 0, "consumed": 0, "released": 0}}]
            if mode["save"] == "lost": await route.abort("connectionreset")
            elif mode["save"] == "refresh-failed":
                mode["read"] = 503
                await route.fulfill(json=state["settings"])
            else: await route.fulfill(json=state["settings"] if path.endswith("/admin/pricing") else {k: v for k, v in state["vouchers"][0].items() if k != "usage"})

    await page.route("**/api/v1/**", api)
    await page.goto(base + "/admin/pricing")
    await expect(page.get_by_text("Pengaturan biaya dan voucher gagal dimuat. Coba lagi.", exact=False)).to_be_visible()
    await expect(page.locator("form")).to_have_count(0)
    assert not saves
    await page.screenshot(path=str(output / f"initial-error-{width}.png"), full_page=True)
    mode["read"] = 200
    await page.get_by_role("button", name="Coba lagi", exact=True).click()
    fee = page.get_by_label("Nominal tetap (Rp)", exact=True)
    await expect(fee).to_have_value("7500")
    await fee.select_option("custom")
    fee = page.get_by_label("Nominal kustom (Rp)", exact=True)
    await fee.fill("9000")
    await expect(page.get_by_role("button", name="Simpan biaya admin", exact=True)).to_be_enabled()
    await page.locator("#voucher-create-opener").click()
    await page.get_by_label("Kode voucher", exact=True).fill("CATALOG")
    await page.get_by_role("button", name="Lanjutkan", exact=True).click()
    await expect(page.locator(".voucher-editor").get_by_role("button", name="Lanjutkan", exact=True)).to_be_disabled()
    await page.screenshot(path=str(output / f"catalog-error-{width}.png"), full_page=True)
    mode["catalog"] = 200
    await page.get_by_role("button", name="Coba lagi katalog", exact=True).click()
    await expect(page.locator(".voucher-editor").get_by_role("button", name="Lanjutkan", exact=True)).to_be_enabled()
    await page.get_by_role("button", name="Batal", exact=True).click()
    await answer_confirmation(page, True)
    await expect(fee).to_have_value("9000")
    sample = page.get_by_label("Subtotal tiket untuk pratinjau (Rp)")
    await expect(sample).to_have_value("100.000")
    await expect(sample).to_have_attribute("readonly", "")
    await sample.focus(); await sample.press("Control+A"); await sample.press("9")
    await expect(sample).to_have_value("100.000")
    mode["read"] = 503
    await page.get_by_role("button", name="Muat ulang", exact=True).click()
    await expect(page.get_by_text("Pembaruan data biaya dan voucher gagal.", exact=False)).to_be_visible()
    await expect(fee).to_have_value("9000")
    assert not saves
    mode["read"] = 200
    await page.get_by_role("button", name="Coba lagi", exact=True).click()
    await expect(page.get_by_role("button", name="Muat ulang", exact=True)).to_be_enabled()
    mode["save"] = "conflict"
    await page.get_by_role("button", name="Simpan biaya admin", exact=True).click()
    await answer_confirmation(page, True)
    await expect(page.get_by_role("alert")).to_contain_text("Penyimpanan ditolak")
    await expect(fee).to_have_value("9000")
    mode["save"] = "refresh-failed"
    await page.get_by_role("button", name="Simpan biaya admin", exact=True).click()
    await answer_confirmation(page, True)
    await expect(page.get_by_role("status").filter(has_text="Biaya admin tersimpan")).to_be_visible()
    await expect(page.get_by_text("Pembaruan data biaya dan voucher gagal.", exact=False)).to_be_visible()
    mode["read"] = 200
    await page.get_by_role("button", name="Muat ulang", exact=True).click()
    await expect(page.get_by_role("button", name="Muat ulang", exact=True)).to_be_enabled()
    await fee.fill("9500")
    mode["save"] = "lost"
    await page.get_by_role("button", name="Simpan biaya admin", exact=True).click()
    await answer_confirmation(page, True)
    await expect(page.get_by_role("alert")).to_contain_text("belum pasti")
    count = len(saves)
    await page.locator("form").first.dispatch_event("submit")
    assert len(saves) == count
    async def check_navigation_locked():
        original_url = page.url
        if width < 1024:
            await page.get_by_role("button", name="Menu", exact=True).click()
            nav = page.locator("#admin-mobile-menu")
        else: nav = page.locator(".admin-sidebar")
        await nav.get_by_role("link", name="Konser", exact=True).click()
        await expect(page.locator(".admin-confirm[open]")).to_have_count(0)
        assert page.url == original_url and len(saves) == count
        if width < 1024: await nav.get_by_role("button", name="Tutup", exact=True).click()
        await expect(page.get_by_role("status").filter(has_text="Selesaikan pemeriksaan hasil penyimpanan")).to_be_visible()
    await check_navigation_locked()
    await page.get_by_role("button", name="Periksa hasil simpan", exact=True).click()
    await expect(page.get_by_role("status").filter(has_text="Biaya admin sesuai")).to_be_visible()
    await expect(fee).to_be_enabled()
    await page.locator("#voucher-create-opener").click()
    await page.get_by_label("Kode voucher", exact=True).fill("LOST50")
    await page.get_by_label("Nilai diskon (%)", exact=True).fill("15")
    await page.get_by_role("button", name="3. Tinjau", exact=True).click()
    await page.get_by_role("button", name="Buat voucher", exact=True).click()
    await answer_confirmation(page, True)
    await expect(page.get_by_role("alert")).to_contain_text("belum pasti")
    count = len(saves)
    await page.locator("form").nth(1).dispatch_event("submit")
    assert len(saves) == count
    await check_navigation_locked()
    await page.get_by_role("button", name="Periksa hasil simpan", exact=True).click()
    await expect(page.get_by_role("status").filter(has_text="Voucher sesuai")).to_be_visible()
    mode["save"] = "success"
    await page.get_by_role("button", name="2. Ketentuan", exact=True).click()
    await page.get_by_label("Voucher aktif", exact=True).check()
    await page.get_by_role("button", name="3. Tinjau", exact=True).click()
    await page.get_by_role("button", name="Simpan perubahan voucher", exact=True).click()
    await answer_confirmation(page, True)
    await expect(page.get_by_role("status").filter(has_text="Voucher tersimpan")).to_be_visible()
    assert not errors and await page.evaluate("document.documentElement.scrollWidth <= innerWidth"), errors
    await page.screenshot(path=str(output / f"saved-{width}.png"), full_page=True)
    await page.evaluate("""() => {
      const elements = [...document.querySelectorAll('.admin-layout *')];
      const sizes = elements.map(el => getComputedStyle(el).fontSize);
      elements.forEach((el, i) => el.style.fontSize = parseFloat(sizes[i]) * 2 + 'px');
    }""")
    audit = await page.evaluate(staff_audit)
    assert not audit["failures"] and not audit["overflow"], (width, audit)
    await page.screenshot(path=str(output / f"text200-{width}.png"), full_page=True)
    (output / f"checks-{width}.json").write_text(json.dumps({"automated": True, "fixture": True, "width": width, "saves": len(saves), "browserErrors": errors, "text200Audit": audit}))
    await context.close()
    print(f"PASS {width}: initial/partial/refresh failures, fixed preview, revision conflict, saved success, unknown fee/voucher verification")


async def check_save_recovery(browser, base, width, operation, scenario):
    context = await browser.new_context(viewport={"width": width, "height": 900}, timezone_id="Asia/Jakarta")
    await context.add_init_script(admin.SESSION_INIT)
    page = await context.new_page()
    page.on("dialog", lambda dialog: dialog.accept())
    await auto_confirm(page)
    errors, requests = [], []
    page.on("pageerror", lambda error: errors.append(str(error)))
    voucher = {"code": "RECOVER15", "kind": "PERCENT", "value": 10, "startsAt": "2026-10-01T00:00:00Z", "endsAt": "2099-01-01T00:00:00Z", "active": False, "eventId": "", "minimumSubtotal": 0, "quota": 100, "revision": 1}
    state = {"settings": {"adminFee": 7500, "revision": 1}, "vouchers": [{**voucher, "usage": {"held": 0, "consumed": 0, "released": 0}}] if operation == "voucher-update" else [], "history": []}
    mode = {"read": 200, "hold": None, "failAgain": False, "commits": 0}

    def commit(payload):
        mode["commits"] += 1
        saved = {**payload, "revision": payload["revision"] + 1}
        if operation == "fee": state["settings"] = saved
        else: state["vouchers"] = [{**saved, "usage": {"held": 0, "consumed": 0, "released": 0}}]
        return saved

    async def api(route):
        path, method = urlparse(route.request.url).path, route.request.method
        if path.endswith("/staff/me"): return await route.fulfill(json={"staff": admin.ADMIN})
        if path.endswith("/admin/events"): return await route.fulfill(json={"events": [admin.EVENT]})
        if method == "GET":
            return await route.fulfill(status=mode["read"], json=state if mode["read"] == 200 else {"error": {"message": "Pemeriksaan gagal"}})
        payload = route.request.post_data_json
        requests.append((method, path, deepcopy(payload)))
        if mode["hold"]: await mode["hold"].wait()
        if len(requests) == 1:
            if scenario == "concurrent": commit(payload)
            if operation == "voucher-create": return await route.abort("connectionreset")
            return await route.fulfill(status=503, json={"error": {"message": "Koneksi penyimpanan terputus"}})
        if mode["failAgain"]:
            mode["failAgain"] = False
            return await route.abort("connectionreset")
        current = state["settings"] if operation == "fee" else next(iter(state["vouchers"]), None)
        if method == "POST" and current or method == "PUT" and (not current or current["revision"] != payload["revision"]):
            return await route.fulfill(status=409, json={"error": {"code": "PRICING_CONFLICT", "message": "Revisi atau kode telah digunakan"}})
        await route.fulfill(status=201 if method == "POST" else 200, json=commit(payload))

    await page.route("**/api/v1/**", api)
    await page.goto(base + "/admin/pricing")
    field = page.get_by_label("Nominal tetap (Rp)", exact=True)
    await expect(field).to_have_value("7500")
    if operation == "fee":
        await field.select_option("custom")
        field = page.get_by_label("Nominal kustom (Rp)", exact=True)
        await field.fill("9000")
        submit = page.get_by_role("button", name="Simpan biaya admin", exact=True)
        retry = page.get_by_role("button", name="Coba kirim perubahan biaya yang sama", exact=True)
        section = page.locator('[aria-labelledby="fee-recovery-title"]')
    else:
        if operation == "voucher-update": await page.get_by_role("button", name="Kelola voucher RECOVER15", exact=True).click()
        else: await page.locator("#voucher-create-opener").click()
        if operation == "voucher-create": await page.get_by_label("Kode voucher", exact=True).fill("RECOVER15")
        field = page.get_by_label("Nilai diskon (%)", exact=True)
        await field.fill("15")
        submit = page.locator(".voucher-editor").get_by_role("button", name="Buat voucher" if operation == "voucher-create" else "Simpan perubahan voucher", exact=True)
        retry = page.get_by_role("button", name="Coba kirim perubahan voucher yang sama", exact=True)
        section = page.locator('[aria-labelledby="voucher-recovery-title"]')
        await page.get_by_role("button", name="3. Tinjau", exact=True).click()
        assert not requests, "Only the final review action may mutate a voucher"
    await submit.click()
    await expect(retry).to_be_disabled()
    await expect(field).to_be_disabled()
    first = requests[0]
    if scenario == "concurrent":
        if operation == "fee": state["settings"] = {"adminFee": 10000, "revision": 3}
        else: state["vouchers"] = [{**state["vouchers"][0], "value": 20, "revision": 3}]
    verify = page.get_by_role("button", name="Periksa hasil simpan", exact=True)
    mode["read"] = 503
    await verify.click()
    await expect(page.get_by_text("Pembaruan data biaya dan voucher gagal.", exact=False)).to_be_visible()
    await expect(field).to_have_value("9000" if operation == "fee" else "15")
    await expect(retry).to_be_disabled()
    mode["read"] = 200
    await verify.click()
    await expect(retry).to_be_enabled()
    await expect(field).to_be_disabled()  # A GET mismatch alone cannot prove the original request failed.
    if scenario == "committed": commit(first[2])  # Original request finishes after the verification GET.
    if scenario == "no-commit":
        mode["failAgain"] = True
        await retry.click()
        await expect(retry).to_be_disabled()
        await expect(field).to_be_disabled()
        await verify.click()
        await expect(retry).to_be_enabled()
    mode["hold"] = asyncio.Event()
    await retry.click()
    await expect(retry).to_be_disabled()
    await retry.dispatch_event("click")
    mode["hold"].set()
    if scenario == "concurrent":
        accept = section.get_by_role("button", name="Gunakan data server sebagai baseline " + ("biaya" if operation == "fee" else "voucher") + " baru", exact=True)
        await expect(accept).to_be_enabled()
        await expect(field).to_be_disabled()
        await expect(section).to_contain_text("Rp 10.000" if operation == "fee" else "20%")
        assert mode["commits"] == 1, mode
        audit = await page.evaluate(staff_audit)
        assert not audit["failures"] and not audit["overflow"], audit
        await section.screenshot(path=f"/tmp/phase51-55/pricing/recovery-{operation}-{width}.png")
        await accept.click()
        await expect(field).to_be_enabled()
        await expect(field).to_have_value("9000" if operation == "fee" else "15")
        await expect(page.locator("#fee-amount" if operation == "fee" else "#voucher-title")).to_be_focused()
        assert requests[-1] == first, requests  # No automatic revision or operation change on retry.
        submit = page.get_by_role("button", name="Simpan biaya admin" if operation == "fee" else "Simpan perubahan voucher", exact=True)
        await submit.click()
        await expect(page.get_by_role("status").filter(has_text="Biaya admin tersimpan" if operation == "fee" else "Voucher tersimpan")).to_be_visible()
        await expect(section).to_have_count(0)
        assert requests[-1][0] == "PUT" and requests[-1][2]["revision"] == 3, requests
    else:
        await expect(section).to_have_count(0)
        assert all(request == first for request in requests), requests
        assert mode["commits"] == 1, mode
    assert len(requests) == (3 if scenario in ("no-commit", "concurrent") else 2), requests
    if operation == "fee": await expect(field).to_be_enabled()
    else:
        if scenario == "committed": await page.locator(".voucher-editor").get_by_role("button", name="Kembali ke daftar", exact=True).click()
        await expect(page.locator(".voucher-editor")).not_to_be_visible()
        await expect(page.locator("#voucher-create-opener")).to_be_enabled()
        await page.get_by_role("button", name="Kelola voucher RECOVER15", exact=True).click()
        await expect(page.get_by_label("Nilai diskon (%)", exact=True)).to_be_enabled()
    assert not errors, errors
    await context.close()
    print(f"PASS {width}px recovery {operation}/{scenario}: original payload, revision and method retained; no duplicate commit; conflict resolved explicitly")


async def main():
    base = sys.argv[1] if len(sys.argv) > 1 else "http://127.0.0.1:5175"
    async with async_playwright() as p:
        browser = await p.chromium.launch(channel="chrome", headless=True)
        try:
            for width in (1440, 320, 390, 767, 768):
                await check_fee_choices(browser, base, width)
                await check_loading(browser, base, width)
                await check(browser, base, width)
                for scenario in (("same-quote", "auto-replay", "manual-replay") if width in (1440, 390) else ()):
                    await check_recovery(browser, base, width, scenario)
                if width in (1440, 320):
                    for operation in ("fee", "voucher-create", "voucher-update"):
                        for scenario in ("no-commit", "committed", "concurrent"):
                            await check_save_recovery(browser, base, width, operation, scenario)
        finally:
            await browser.close()


if __name__ == "__main__":
    asyncio.run(main())
