"""Run against Vite: python3 scripts/staff-review-check.py [base URL].

Requires the installed Python Playwright package and Google Chrome.
"""
import asyncio
import sys

from playwright.async_api import async_playwright, expect


async def main():
    base_url = sys.argv[1] if len(sys.argv) > 1 else "http://127.0.0.1:5173"
    async with async_playwright() as playwright:
        browser = await playwright.chromium.launch(channel="chrome", headless=True)
        context = await browser.new_context()
        await context.add_init_script("sessionStorage.setItem('ticket-online:staff-session', JSON.stringify({accessToken:'s'.repeat(43),expiresAt:new Date(Date.now()+3600000).toISOString()}))")
        page = await context.new_page()
        admin = dict(id="c" * 32, name="Admin", email="admin@example.com", role="ADMIN", active=True, assignments=[])
        people = [dict(id=letter * 32, name=name, email=name.lower() + "@example.com", role="STAFF", active=True, assignments=[dict(eventId="event", gate=gate)]) for letter, name, gate in [("a", "Alice", "Gate A"), ("b", "Bob", "Gate B")]]
        started, release = asyncio.Event(), asyncio.Event()
        mutations = []

        async def admin_api(route):
            path = route.request.url.split("/api/v1/")[1]
            if path == "staff/me":
                await route.fulfill(json=dict(staff=admin))
            elif path == "events":
                await route.fulfill(json=dict(events=[]))
            elif path == "admin/staff":
                await route.fulfill(json=dict(staff=people))
            elif path.endswith("/assignments"):
                mutations.append(dict(path=path, body=route.request.post_data_json))
                started.set()
                await release.wait()
                await route.fulfill(status=204)
            else:
                raise AssertionError(path)

        await page.route("**/api/v1/**", admin_api)
        await page.goto(base_url + "/admin/staff")
        await page.get_by_role("button", name="Kelola", exact=True).first.click()
        editor = page.locator(".staff-edit-panel")
        await editor.get_by_label("Nama", exact=True).fill("Alice draft")
        await editor.get_by_label("Akun aktif", exact=True).uncheck()
        await editor.get_by_role("button", name="Simpan penugasan", exact=True).click()
        await asyncio.wait_for(started.wait(), 5)
        bob = page.get_by_role("button", name="Kelola", exact=True)
        await expect(bob).to_be_disabled()
        await expect(page.get_by_role("button", name="Tutup", exact=True)).to_be_disabled()
        await expect(page.get_by_role("button", name="Muat ulang", exact=True)).to_be_disabled()
        await bob.dispatch_event("click")
        await expect(editor.locator("h2")).to_have_text("Alice")
        release.set()
        await page.get_by_text("Penugasan berhasil diperbarui.", exact=True).wait_for()
        await expect(bob).to_be_enabled()
        await expect(editor.get_by_label("Nama", exact=True)).to_have_value("Alice draft")
        await expect(editor.get_by_label("Akun aktif", exact=True)).not_to_be_checked()
        await bob.click()
        await expect(editor.get_by_label("Nama", exact=True)).to_have_value("Bob")
        await editor.get_by_role("button", name="Simpan penugasan", exact=True).click()
        await page.get_by_text("Penugasan berhasil diperbarui.", exact=True).wait_for()
        assert mutations == [dict(path=f"admin/staff/{person['id']}/assignments", body=dict(assignments=person["assignments"])) for person in people]
        print("PASS: delayed assignment save preserves selection and profile draft")
        await page.unroute("**/api/v1/**", admin_api)

        mode = "success"
        posts = []
        started.clear()
        release.clear()

        async def scanner_api(route):
            if route.request.url.endswith("/me"):
                await route.fulfill(json=dict(staff=people[0]))
                return
            assert route.request.url.endswith("/check-ins")
            payload = route.request.post_data_json
            posts.append(payload)
            if mode == "success":
                started.set()
                await release.wait()
                await route.fulfill(status=201, json=dict(status="CHECKED_IN", checkedInAt="2026-10-05T12:00:00Z", ticket=dict(id=payload["code"][3:].lower(), code=payload["code"], attendeeName="Guest", tierName="Festival", eventId="event", gate="Gate A")))
            elif mode == "network":
                await route.abort("failed")
            elif mode == "truncated":
                await route.fulfill(status=201, content_type="application/json", body='{"status":"CHECKED_IN",')
            elif mode == "500":
                await route.fulfill(status=500, json=dict(error=dict(code="INTERNAL_ERROR", message="Server error")))
            else:
                raise AssertionError(mode)

        await page.route("**/api/v1/**", scanner_api)
        await page.goto(base_url + "/admin/scan")
        await page.get_by_label("Penugasan aktif").select_option("event:Gate A")
        code_input = page.get_by_label("Kode e-ticket", exact=True)
        first, second = "ET-" + "A" * 32, "ET-" + "B" * 32
        await code_input.fill(first)
        await code_input.press("Enter")
        await asyncio.wait_for(started.wait(), 5)
        await expect(code_input).to_be_disabled()
        await page.locator(".scan-form").dispatch_event("submit")
        assert len(posts) == 1
        release.set()
        await page.get_by_text("Gate Masuk Terbuka", exact=True).wait_for()
        await expect(code_input).to_be_focused()
        await expect(code_input).to_have_value("")
        await page.keyboard.type(second)
        await page.keyboard.press("Enter")
        await expect(page.locator(".ticket-detail-card")).to_contain_text(second)
        await expect(code_input).to_be_focused()
        assert [post["code"] for post in posts] == [first, second]
        print("PASS: two keyboard scans without mouse and concurrent submit blocked")

        for mode in ["network", "truncated", "500"]:
            await page.goto(base_url + "/admin/scan")
            await page.get_by_label("Penugasan aktif").select_option("event:Gate A")
            await code_input.fill(first)
            count = len(posts)
            await code_input.press("Enter")
            await page.get_by_text("Hasil belum diketahui", exact=True).wait_for()
            await expect(page.locator(".result-main .result-detail").first).to_contain_text("Gate tetap ditahan; periksa tiket atau minta bantuan admin sebelum mencoba lagi.")
            await expect(page.locator(".result-footer")).to_contain_text("Akses ditahan")
            await expect(code_input).to_have_value(first)
            await expect(code_input).not_to_be_focused()
            await page.wait_for_load_state("networkidle")
            assert len(posts) == count + 1
            print(f"PASS: {mode} keeps unknown result, recovery instructions, ticket code, and no retry")
        await browser.close()


asyncio.run(main())
