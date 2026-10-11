"""Poster form regression checks. Run with Vite at 127.0.0.1:5174.

Requires the existing Playwright/Chrome tooling. Mobile runs emulate Chromium;
physical Android/iOS picker and HEIC checks remain manual.
"""
import sys
sys.dont_write_bytecode = True

import argparse
import asyncio
import re
from copy import deepcopy
from pathlib import Path
from urllib.parse import urlparse

from playwright.async_api import async_playwright, expect
from importlib.util import spec_from_file_location, module_from_spec

spec = spec_from_file_location("admin_fixtures", Path(__file__).with_name("admin-ui-check.py"))
fixtures = module_from_spec(spec)
spec.loader.exec_module(fixtures)
IMAGE = fixtures.ROOT / "docs/phase20/fixtures/nusa-malam.jpg"


async def check(browser, base, width):
    context = await browser.new_context(viewport={"width": width, "height": 900}, timezone_id="Asia/Jakarta")
    await context.add_init_script(fixtures.SESSION_INIT)
    await context.add_init_script("""window.posterURLs={created:[],revoked:[]};
      const create=URL.createObjectURL.bind(URL), revoke=URL.revokeObjectURL.bind(URL);
      URL.createObjectURL=(v)=>{const u=create(v);window.posterURLs.created.push(u);return u};
      URL.revokeObjectURL=(u)=>{window.posterURLs.revoked.push(u);revoke(u)};""")
    page = await context.new_page()
    page.on("dialog", lambda dialog: dialog.accept())
    errors = []
    page.on("pageerror", lambda error: errors.append(str(error)))
    event = deepcopy(fixtures.EVENT)
    event["image"] = base + "/api/v1/media/posters/" + "1" * 64 + ".jpg"
    other = {**event, "id": "konser-kedua", "artist": "Konser Kedua"}
    poster = base + "/api/v1/media/posters/" + "2" * 64 + ".jpg"
    calls = {"upload": 0, "save": 0}
    created = []
    state = {"upload_status": 201, "save_status": 503, "hold": None, "offline": False}

    async def api(route):
        path = urlparse(route.request.url).path
        method = route.request.method
        if path.startswith("/api/v1/media/posters/"):
            await route.fulfill(body=IMAGE.read_bytes(), content_type="image/jpeg")
        elif path == "/api/v1/admin/posters":
            calls["upload"] += 1
            assert route.request.headers.get("authorization") == "Bearer " + fixtures.TOKEN
            assert "multipart/form-data; boundary=" in route.request.headers["content-type"]
            if state["offline"]:
                await route.abort("internetdisconnected")
                return
            hold = state["hold"]
            if hold:
                await hold.wait()
            status = state["upload_status"]
            try:
                await route.fulfill(status=status, json={"url": poster} if status == 201 else {"error": {"code": "POSTER_STORAGE_FAILED", "message": "Upload gagal. Coba lagi."}})
            except Exception:
                pass  # The browser aborts requests when source, file, or concert changes.
        elif path == "/api/v1/admin/events" and method == "POST":
            value = {**event, **route.request.post_data_json, "zones": [], "ticketTiers": []}
            created.append(value)
            await route.fulfill(status=201, json=value)
        elif path == "/api/v1/admin/events" and method == "GET":
            await route.fulfill(json={"events": [event, other]})
        elif path == "/api/v1/admin/events/" + event["id"] and method == "PUT":
            calls["save"] += 1
            if state["save_status"] == 200:
                event.update(route.request.post_data_json)
                await route.fulfill(json=event)
            else:
                await route.fulfill(status=503, json={"error": {"code": "SERVICE_UNAVAILABLE", "message": "Simpan gagal. Coba lagi."}})
        elif path == "/api/v1/events":
            await route.fulfill(json={"events": [event]})
        elif path == "/api/v1/events/" + event["id"]:
            await route.fulfill(json=event)
        else:
            status, body = fixtures.body(route.request.url, "standard", "ADMIN")
            await route.fulfill(status=status, json=body)

    await page.route("**/api/v1/**", api)
    await page.goto(base + "/admin/events")
    await expect(page.get_by_role("radio", name="Unggah dari perangkat")).to_be_checked()
    await page.get_by_role("button", name="Nusa Malam").click()
    await expect(page.locator(".poster-preview img")).to_have_attribute("src", event["image"])
    picker = page.get_by_label("Pilih file poster")
    save = page.get_by_role("button", name="Simpan perubahan", exact=True)
    upload = page.get_by_role("button", name="Unggah poster", exact=True)
    await picker.set_input_files(str(IMAGE))
    await expect(save).to_be_disabled()
    await expect(page.locator(".poster-preview img")).to_have_attribute("src", re.compile("^blob:"))
    await expect(page.get_by_text("Perubahan belum disimpan", exact=True)).to_be_visible()
    state["upload_status"] = 503
    await upload.click()
    await expect(page.get_by_role("alert").filter(has_text="Upload gagal")).to_be_visible()
    await expect(save).to_be_disabled()
    assert event["image"].endswith("1" * 64 + ".jpg") and calls["save"] == 0
    state["offline"] = True
    await page.get_by_role("button", name="Coba lagi", exact=True).click()
    await expect(page.get_by_role("alert").filter(has_text="Tidak dapat terhubung ke server")).to_be_visible()
    await expect(save).to_be_disabled()
    await expect(page.locator(".poster-preview img")).to_have_attribute("src", re.compile("^blob:"))
    assert event["image"].endswith("1" * 64 + ".jpg") and calls["save"] == 0
    state["offline"] = False
    state["upload_status"] = 201
    await page.get_by_role("button", name="Coba lagi", exact=True).click()
    await expect(page.get_by_text("Poster terunggah. Simpan perubahan konser untuk menerapkannya.", exact=True)).to_be_visible()
    await expect(save).to_be_enabled()
    await save.click()
    await expect(page.get_by_role("alert").filter(has_text="Simpan gagal")).to_be_visible()
    await expect(page.get_by_text("Poster terunggah. Simpan perubahan konser untuk menerapkannya.", exact=True)).to_be_visible()
    await page.get_by_role("radio", name="Gunakan URL gambar").check()
    await expect(page.get_by_label("URL poster", exact=True)).to_have_value(poster)
    state["save_status"] = 200
    await save.click()
    await expect(page.get_by_text("Perubahan konser tersimpan.", exact=True)).to_be_visible()
    assert event["image"] == poster
    await expect(page.locator(".poster-preview img")).to_have_attribute("src", poster)

    # Duplicate clicks are blocked; switching concerts cancels the pending result.
    await picker.set_input_files(str(IMAGE))
    state["hold"] = asyncio.Event()
    before = calls["upload"]
    await upload.click()
    await expect(page.get_by_role("button", name="Mengunggah poster…", exact=True)).to_be_disabled()
    await page.get_by_role("button", name="Mengunggah poster…", exact=True).dispatch_event("click")
    await expect(save).to_be_disabled()
    await page.get_by_role("button", name="Konser Kedua").click()
    state["hold"].set()
    await expect(page.locator(".poster-preview img")).to_have_attribute("src", other["image"])
    assert calls["upload"] == before + 1
    await expect(page.get_by_text("Poster terunggah. Simpan perubahan konser untuk menerapkannya.", exact=True)).to_have_count(0)

    # Source change aborts an upload; the existing poster is retained.
    await picker.set_input_files(str(IMAGE))
    state["hold"] = asyncio.Event()
    await upload.click()
    await page.get_by_role("radio", name="Gunakan URL gambar").check()
    state["hold"].set()
    await expect(page.get_by_label("URL poster", exact=True)).to_have_value(other["image"])
    await expect(save).to_be_enabled()
    await page.get_by_label("URL poster", exact=True).fill("https://external.example.test/poster.png")
    await expect(page.locator(".poster-preview img")).to_have_attribute("src", "https://external.example.test/poster.png")

    # Validation feedback and responsive controls.
    await page.get_by_role("radio", name="Unggah dari perangkat").check()
    await picker.set_input_files({"name": "camera.heic", "mimeType": "image/heic", "buffer": b"heic"})
    await expect(page.get_by_role("alert").filter(has_text="Ekspor HEIC")).to_be_visible()
    await expect(save).to_be_disabled()
    await picker.set_input_files({"name": "large.png", "mimeType": "image/png", "buffer": bytes(5 * 1024 * 1024 + 1)})
    await expect(page.get_by_role("alert").filter(has_text="maksimal 5 MiB")).to_be_visible()
    await picker.set_input_files(str(IMAGE))
    assert await page.evaluate("document.documentElement.scrollWidth <= innerWidth"), "horizontal overflow"
    assert await upload.evaluate("el=>el.getBoundingClientRect().height>=44")
    await page.get_by_role("radio", name="Gunakan URL gambar").check()
    assert await page.evaluate("window.posterURLs.created.every(u=>window.posterURLs.revoked.includes(u))"), "object URL leak"

    # Replacing a file while its upload is running ignores the previous result.
    await page.get_by_role("radio", name="Unggah dari perangkat").check()
    await picker.set_input_files(str(IMAGE))
    state["hold"] = asyncio.Event()
    await upload.click()
    await picker.set_input_files({"name": "replacement.jpg", "mimeType": "image/jpeg", "buffer": IMAGE.read_bytes()})
    state["hold"].set()
    await expect(save).to_be_disabled()
    await expect(page.get_by_text("Poster terunggah. Simpan perubahan konser untuk menerapkannya.", exact=True)).to_have_count(0)
    state["hold"] = None
    await upload.click()
    await expect(save).to_be_enabled()

    # Both sources also work when creating a new draft concert.
    for source in ("file", "url"):
        await page.get_by_role("button", name="Tambah konser", exact=True).click()
        await expect(page.get_by_role("radio", name="Unggah dari perangkat")).to_be_checked()
        await page.get_by_label("Nama konser / artis", exact=True).fill("Konser Baru " + source)
        if source == "file":
            await picker.set_input_files(str(IMAGE))
            await expect(page.get_by_role("button", name="Buat konser", exact=True)).to_be_disabled()
            await upload.click()
            await expect(page.get_by_text("Poster terunggah. Simpan perubahan konser untuk menerapkannya.", exact=True)).to_be_visible()
            expected_url = poster
        else:
            await page.get_by_role("radio", name="Gunakan URL gambar").check()
            expected_url = base + "/api/v1/media/posters/" + "3" * 64 + ".jpg"
            await page.get_by_label("URL poster", exact=True).fill(expected_url)
        await page.get_by_role("button", name="Buat konser", exact=True).click()
        await expect(page.get_by_role("button", name="Simpan perubahan", exact=True)).to_be_enabled()
        await expect(page.locator(".poster-preview img")).to_have_attribute("src", expected_url)
        await expect(page.get_by_text("Perubahan belum disimpan", exact=True)).to_have_count(0)
        assert created[-1]["image"] == expected_url

    # The saved image contract reaches every public poster surface.
    for path in ("/", "/konser", "/konser/" + event["id"]):
        await page.goto(base + path)
        image = page.locator(f'img[src="{poster}"]').first
        await expect(image).to_be_visible()
        await expect(image).to_have_js_property("naturalWidth", 900)
    assert not errors, errors
    await context.close()
    print(f"PASS {width}px: picker, preview, disconnected upload, retry, save failure, stale upload, URL, cleanup, public pages")


async def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--base-url", default="http://127.0.0.1:5174")
    args = parser.parse_args()
    async with async_playwright() as playwright:
        browser = await playwright.chromium.launch(channel="chrome", headless=True)
        try:
            for width in (1440, 390, 320):
                await check(browser, args.base_url, width)
        finally:
            await browser.close()


if __name__ == "__main__":
    asyncio.run(main())
