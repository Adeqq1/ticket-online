"""Browser integration against an isolated local backend, without API fixtures.

Requires PHASE55_ISOLATED_TEST=true, PHASE55_EMAIL and PHASE55_PASSWORD.
Run: python3 scripts/admin-backend-check.py http://127.0.0.1:5175
"""
import asyncio
import json
import os
import secrets
import sys
from pathlib import Path
from urllib.parse import urlparse

from playwright.async_api import async_playwright, expect
from runpy import run_path

confirmation_helpers = run_path(str(Path(__file__).with_name('admin-confirm.py')))
auto_confirm = confirmation_helpers['auto_confirm']


async def main():
    base = sys.argv[1] if len(sys.argv) > 1 else "http://127.0.0.1:5175"
    assert os.environ.get("PHASE55_ISOLATED_TEST") == "true" and urlparse(base).hostname in ("127.0.0.1", "localhost"), "Use an explicitly isolated local test backend"
    output = Path("/tmp/phase51-55/backend")
    output.mkdir(parents=True, exist_ok=True)
    suffix = secrets.token_hex(4)
    async with async_playwright() as playwright:
        browser = await playwright.chromium.launch(channel="chrome", headless=True)
        context = await browser.new_context(viewport={"width": 1440, "height": 900}, timezone_id="Asia/Jakarta")
        response = await context.request.post(base + "/api/v1/staff/login", data={"email": os.environ["PHASE55_EMAIL"], "password": os.environ["PHASE55_PASSWORD"]})
        assert response.status == 200, await response.text()
        session = await response.json()
        assert session["staff"]["role"] == "ADMIN"
        await context.add_init_script("sessionStorage.setItem('ticket-online:staff-session', JSON.stringify(" + json.dumps({k: session[k] for k in ("accessToken", "expiresAt")}) + "))")
        page = await context.new_page()
        errors, responses = [], []
        page.on("pageerror", lambda error: errors.append(str(error)))
        page.on("response", lambda r: responses.append({"method": r.request.method, "path": urlparse(r.url).path, "status": r.status}) if "/api/v1/" in r.url else None)
        page.on("dialog", lambda dialog: dialog.accept())
        await auto_confirm(page)
        await page.goto(base + "/admin/pricing")
        preset = page.get_by_label("Nominal tetap (Rp)", exact=True)
        await preset.select_option("custom")
        fee = page.get_by_label("Nominal kustom (Rp)", exact=True)
        original_fee = int(await fee.input_value())
        await fee.fill(str(original_fee + 500))
        await page.get_by_role("button", name="Simpan biaya admin", exact=True).click()
        await expect(page.get_by_role("status").filter(has_text="Biaya admin tersimpan")).to_be_visible()
        await page.reload()
        await preset.select_option("custom")
        await expect(fee).to_have_value(str(original_fee + 500))
        code = "UJI_" + suffix.upper()
        await page.get_by_label("Kode voucher", exact=True).fill(code)
        await page.get_by_label("Nilai diskon (%)", exact=True).fill("15")
        await page.get_by_role("button", name="Buat voucher", exact=True).click()
        await expect(page.get_by_role("status").filter(has_text="Voucher tersimpan")).to_be_visible()
        await page.reload()
        await page.get_by_role("button", name="Kelola " + code, exact=True).click()
        await expect(page.get_by_label("Nilai diskon (%)", exact=True)).to_have_value("15")
        await page.screenshot(path=str(output / "pricing-reloaded.png"), full_page=True)
        await page.goto(base + "/admin/events")
        await page.locator("#event-create-opener").click()
        artist = "Konser integrasi " + suffix
        await page.get_by_label("Nama konser / artis", exact=True).fill(artist)
        event_id = await page.locator("#event-id").input_value()
        await page.get_by_role("button", name="Buat konser", exact=True).click()
        await expect(page.get_by_text("Draf konser dibuat.", exact=False)).to_be_visible()
        await page.get_by_label("Nama zona", exact=True).fill("Zona Integrasi")
        await page.get_by_role("button", name="Tambah zona", exact=True).click()
        await expect(page.get_by_text("Zona tersimpan.", exact=True)).to_be_visible()
        await page.get_by_label("Nama kategori", exact=True).fill("Kategori Integrasi")
        await page.locator("#tier-id").fill("kategori-" + suffix)
        await page.locator(".event-inventory-editor form").last.get_by_role("combobox").first.select_option("zona-integrasi")
        for label, value in [("Harga (rupiah)", "100000"), ("Kapasitas", "20"), ("Gate", "Gate Uji")]: await page.get_by_label(label, exact=True).fill(value)
        await page.get_by_role("button", name="Tambah kategori", exact=True).click()
        await expect(page.get_by_text("Kategori tiket tersimpan.", exact=True)).to_be_visible()
        await page.get_by_label("Jadwal (WIB)", exact=True).fill("01/12/2027")
        await page.get_by_label("Jam Jadwal (WIB)", exact=True).fill("19")
        await page.get_by_label("Menit Jadwal (WIB)", exact=True).fill("00")
        for label, value in [("Kota", "Jakarta"), ("Venue", "Venue Integrasi"), ("Alamat", "Alamat Integrasi"), ("Deskripsi", "Konser uji integrasi"), ("Lineup", "Artis Integrasi")]: await page.get_by_label(label, exact=label != "Lineup").fill(value)
        await page.get_by_role("radio", name="Gunakan URL gambar", exact=True).check()
        await page.get_by_label("URL poster", exact=True).fill("https://example.test/phase55.jpg")
        await page.get_by_role("combobox", name="Status publikasi", exact=True).select_option("PUBLISHED")
        await page.get_by_role("button", name="Simpan perubahan", exact=True).click()
        await expect(page.get_by_text("Perubahan konser tersimpan.", exact=True)).to_be_visible()
        await page.reload()
        await page.locator("#event-manage-" + event_id).click()
        await expect(page.get_by_label("Nama konser / artis", exact=True)).to_have_value(artist)
        await expect(page.get_by_role("combobox", name="Status publikasi", exact=True)).to_have_value("PUBLISHED")
        await expect(page.locator(".event-inventory-list")).to_contain_text("Kategori Integrasi")
        await page.screenshot(path=str(output / "concert-reloaded.png"), full_page=True)
        await page.goto(base + "/admin/staff")
        await page.locator("#staff-create-opener").click()
        panel = page.locator(".staff-create-panel")
        name = "Petugas integrasi " + suffix
        email = "phase55-" + suffix + "@example.test"
        await panel.get_by_label("Nama", exact=True).fill(name)
        await panel.get_by_label("Email", exact=True).fill(email)
        await panel.get_by_label("Password awal").fill(secrets.token_urlsafe(18))
        await panel.get_by_role("combobox", name="Event", exact=True).select_option(event_id)
        await panel.get_by_role("combobox", name="Gate", exact=True).select_option("Gate Uji")
        await panel.get_by_role("button", name="Tambah", exact=True).click()
        await panel.get_by_role("button", name="Buat akun petugas", exact=True).click()
        await expect(page.get_by_text("Akun petugas berhasil dibuat.", exact=True)).to_be_visible()
        await page.reload()
        row = page.locator(".staff-list li").filter(has_text=email)
        await expect(row).to_contain_text("Gate Uji")
        await row.get_by_role("button", name="Kelola", exact=False).click()
        editor = page.locator(".staff-edit-panel")
        await editor.get_by_label("Nama", exact=True).fill(name + " Utama")
        await editor.get_by_role("button", name="Simpan profil", exact=True).click()
        await expect(page.get_by_text("Profil petugas berhasil diperbarui.", exact=True)).to_be_visible()
        await editor.get_by_role("button", name="Hapus penugasan Gate Uji", exact=True).click()
        await editor.get_by_role("button", name="Simpan penugasan", exact=True).click()
        await expect(page.get_by_text("Penugasan berhasil diperbarui.", exact=True)).to_be_visible()
        await page.reload()
        await expect(row).to_contain_text("Belum ditugaskan")
        await expect(row).to_contain_text(name + " Utama")
        await row.get_by_role("button", name="Kelola", exact=False).click()
        await editor.get_by_label("Akun aktif", exact=True).uncheck()
        await editor.get_by_role("button", name="Simpan profil", exact=True).click()
        await expect(page.get_by_text("Profil petugas berhasil diperbarui.", exact=True)).to_be_visible()
        await page.reload()
        await expect(row).to_contain_text("Status: Nonaktif")
        await page.screenshot(path=str(output / "staff-reloaded.png"), full_page=True)
        await page.goto(base + "/admin/pricing")
        await preset.select_option("custom")
        await fee.fill(str(original_fee))
        await page.get_by_role("button", name="Simpan biaya admin", exact=True).click()
        await expect(page.get_by_role("status").filter(has_text="Biaya admin tersimpan")).to_be_visible()
        assert not errors, errors
        assert not [r for r in responses if r["status"] >= 400], responses
        (output / "integration.json").write_text(json.dumps({"realBackend": True, "apiFixtures": False, "humanUAT": "not performed", "database": "ticket_online_phase55_check", "eventId": event_id, "voucher": code, "responses": responses, "browserErrors": errors, "originalFeeRestored": True}, indent=2))
        await browser.close()
        print("PASS: real backend admin pricing/voucher, concert draft/inventory/publish, staff profile/assignments/status verified after reload")


if __name__ == "__main__":
    asyncio.run(main())
