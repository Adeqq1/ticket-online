"""Run against Vite: python3 scripts/staff-review-check.py [base URL] [width].

Requires the installed Python Playwright package and Google Chrome.
"""
import asyncio
import json
import os
import sys
from runpy import run_path
from pathlib import Path

from playwright.async_api import async_playwright, expect


STYLE_AUDIT = """() => {
    const rgb = value => {
        const canvas = document.createElement('canvas'); canvas.width = canvas.height = 1;
        const ctx = canvas.getContext('2d'); ctx.fillStyle = value; ctx.fillRect(0, 0, 1, 1);
        return [...ctx.getImageData(0, 0, 1, 1).data].map((v, i) => i === 3 ? v / 255 : v);
    };
    const background = el => {
        let result = [255, 255, 255];
        for (const node of [el, ...function*(n) { while(n.parentElement) { n = n.parentElement; yield n; } }(el)].reverse()) {
            const color = rgb(getComputedStyle(node).backgroundColor);
            result = result.map((v, i) => color[i] * color[3] + v * (1 - color[3]));
        }
        return result;
    };
    const luminance = color => color.slice(0, 3).map(v => v / 255).map(v => v <= .04045 ? v / 12.92 : ((v + .055) / 1.055) ** 2.4).reduce((sum, v, i) => sum + v * [.2126, .7152, .0722][i], 0);
    const ratio = (a, b) => (Math.max(luminance(a), luminance(b)) + .05) / (Math.min(luminance(a), luminance(b)) + .05);
    const failures = []; const ratios = []; const borders = []; const focus = [];
    for (const el of document.querySelectorAll('.admin-layout *')) {
        if (!el.checkVisibility() || el.getBoundingClientRect().width < 2 || el.getBoundingClientRect().height < 2 || el.matches('option, .scan-brand-mark')) continue;
        const style = getComputedStyle(el);
        if (el.matches('input, select, textarea') || [...el.childNodes].some(n => n.nodeType === Node.TEXT_NODE && n.textContent.trim())) {
            const contrast = ratio(rgb(style.color), background(el)); ratios.push(contrast);
            if (contrast < 4.5 || parseFloat(style.fontSize) < 14) failures.push({text: el.textContent.slice(0, 70), contrast, size: style.fontSize, color: style.color, background: background(el), disabled: el.matches(':disabled'), opacity: style.opacity});
        }
        if (el.matches('input:not([type="checkbox"]), select, textarea, .staff-secondary-button, .scan-submit:disabled')) {
            for (const bg of [background(el), background(el.parentElement)]) {
                const contrast = ratio(rgb(style.borderTopColor), bg);
                borders.push(contrast);
                if (contrast < 3) failures.push({control: el.tagName, contrast, border: style.borderTopColor});
            }
        }
        if (el.matches(':focus-visible') && parseFloat(style.outlineWidth) > 0) {
            const contrast = ratio(rgb(style.outlineColor), background(el.parentElement));
            focus.push(contrast);
            if (contrast < 3) failures.push({focus: el.tagName, contrast});
        }
    }
    return {failures, minTextContrast: Math.min(...ratios), minBorderContrast: borders.length ? Math.min(...borders) : null, minFocusContrast: focus.length ? Math.min(...focus) : null, overflow: document.documentElement.scrollWidth > innerWidth};
}"""


async def verify_create_flow(page, people, admin, width, base_url):
    mode = "success"
    catalog_mode = "success"
    reload_failed = False
    created = None
    posts = []
    logouts = []
    errors = []
    audits = []
    page.on("pageerror", lambda error: errors.append(str(error)))
    started, release = asyncio.Event(), asyncio.Event()
    event = dict(id="event", artist="Nusa Malam", city="Jakarta", ticketTiers=[dict(gate="Gate A")])
    # Include both roles, both statuses, and long content that must wrap intact.
    rows = [admin, *people, {**people[0], "id": "e" * 32, "name": "Petugas Nonaktif", "active": False,
            "email": "petugas.dengan.email.panjang@operasional-konser.example.com",
            "assignments": [dict(eventId="event-dengan-nama-penugasan-panjang", gate="Gate Selatan untuk penonton festival")]}]

    async def fixture(route):
        nonlocal created
        path = route.request.url.split("/api/v1/")[1]
        if path == "staff/me":
            await route.fulfill(json=dict(staff=admin))
        elif path == "staff/logout":
            logouts.append(True)
            await route.fulfill(status=204)
        elif path == "admin/events":
            if catalog_mode == "loading":
                started.set()
                await release.wait()
            if catalog_mode in ("error", "expired"):
                await route.fulfill(status=401 if catalog_mode == "expired" else 503, json=dict(error=dict(message="Katalog gagal.")))
            else:
                await route.fulfill(json=dict(events=[event]))
        elif path == "admin/staff" and route.request.method == "POST":
            posts.append(route.request.post_data_json)
            if mode == "delayed":
                started.set()
                await release.wait()
            if mode == "duplicate":
                await route.fulfill(status=409, json=dict(error=dict(code="EMAIL_ALREADY_EXISTS", message="Email sudah digunakan.")))
                return
            if mode == "expired":
                await route.fulfill(status=401, json=dict(error=dict(message="Sesi berakhir.")))
                return
            if mode == "network":
                await route.abort("failed")
                return
            created = dict(id="d" * 32, role="STAFF", active=True, **{k: v for k, v in posts[-1].items() if k != "password"})
            if mode == "truncated":
                await route.fulfill(status=201, content_type="application/json", body='{"id":')
            else:
                await route.fulfill(status=201, json=created)
        elif path == "admin/staff":
            if reload_failed and created:
                await route.fulfill(status=503, json=dict(error=dict(message="Daftar gagal dimuat.")))
            else:
                await route.fulfill(json=dict(staff=[*rows, *([created] if created else [])]))
        else:
            raise AssertionError(path)

    async def visit():
        nonlocal created
        created = None
        await page.goto(base_url + "/admin/staff")
        await expect(page.locator("[id^=staff-account-]")).to_have_count(len(rows))

    async def audit(label):
        result = await page.evaluate(STYLE_AUDIT)
        assert not result["failures"] and not result["overflow"], (width, label, result)
        audits.append(dict(state=label, **result))
        return result

    opener = page.locator("#staff-create-opener")
    panel = page.locator(".staff-create-panel")
    submit = panel.get_by_role("button", name="Buat akun petugas", exact=True)
    cancel = panel.get_by_role("button", name="Batal", exact=True)
    manage = page.locator(f"#staff-manage-{people[0]['id']}")
    output = Path(os.environ.get("STAFF_REVIEW_OUTPUT", "/tmp/phase51-55/staff"))
    output.mkdir(parents=True, exist_ok=True)

    async def draft():
        await opener.focus()
        await page.keyboard.press("Enter")
        await expect(page.locator("#staff-create-title")).to_be_focused()
        await page.keyboard.press("Tab")
        await expect(panel.get_by_role("button", name="Kembali ke daftar", exact=True)).to_be_focused()
        await page.keyboard.press("Tab")
        await expect(panel.get_by_label("Nama", exact=True)).to_be_focused()
        await panel.get_by_label("Nama", exact=True).fill("Nadia Petugas")
        await panel.get_by_label("Email", exact=True).fill("nadia@example.com")
        await panel.get_by_label("Password awal").fill("password-fixture-123")

    await page.route("**/api/v1/**", fixture)
    await visit()
    await expect(panel).to_have_count(0)
    await expect(page.locator(".staff-role").first).to_have_text("Admin")
    await expect(page.locator(".staff-status").last).to_have_text("Status: Nonaktif")
    await audit("list")
    await page.screenshot(path=str(output / f"list-{width}.png"), full_page=True)
    if width < 1024:
        await page.get_by_role("button", name="Menu", exact=True).click()
        await expect(page.locator("[data-menu-close]")).to_be_focused()
        await audit("mobile navigation")
        await page.keyboard.press("Escape")
        await expect(page.get_by_role("button", name="Menu", exact=True)).to_be_focused()
    await draft()
    assert not posts, "The top action must never create an account"
    await panel.get_by_role("combobox", name="Event", exact=True).select_option("event")
    await panel.get_by_role("combobox", name="Gate", exact=True).select_option("Gate A")
    await panel.get_by_role("button", name="Tambah", exact=True).click()
    await audit("create form")
    await cancel.hover()
    await audit("secondary hover")
    await cancel.focus()
    await page.keyboard.press("Shift+Tab")
    await expect(submit).to_be_focused()
    assert (await audit("focus"))["minFocusContrast"] is not None
    await page.screenshot(path=str(output / f"create-{width}.png"), full_page=True)
    for size, scale, label in [({"width": width, "height": 900}, 2, "text200"), ({"width": min(width, 390), "height": 420}, 1, "keyboard-height")]:
        await page.set_viewport_size(size)
        if scale == 2:
            await page.evaluate("""() => {
                window.originalSizes = [...document.querySelectorAll('.admin-layout *')].map(el => [el, el.style.fontSize, parseFloat(getComputedStyle(el).fontSize)]);
                for (const [el, , size] of window.originalSizes) el.style.setProperty('font-size', `${size * 2}px`, 'important');
            }""")
        await panel.get_by_label("Password awal").focus()
        await audit(label)
        await cancel.scroll_into_view_if_needed()
        await page.screenshot(path=str(output / f"{label}-{width}.png"), full_page=True)
        if scale == 2:
            await page.evaluate("() => { for (const [el, original] of window.originalSizes) { el.style.removeProperty('font-size'); if(original) el.style.fontSize = original; } }")
    await page.set_viewport_size({"width": width, "height": 900})
    page.once("dialog", lambda dialog: dialog.dismiss())
    await cancel.click()
    await expect(panel.get_by_label("Nama", exact=True)).to_have_value("Nadia Petugas")
    await expect(manage).to_have_count(0)
    await expect(opener).to_have_count(0)
    await expect(page.locator(".staff-list-filters")).to_have_count(0)
    await expect(panel).to_have_count(1)
    page.once("dialog", lambda dialog: dialog.dismiss())
    await page.get_by_role("button", name="Keluar", exact=True).click()
    assert not logouts
    # Native page-exit protection must cover new drafts as well as edits.
    async with page.expect_event("dialog") as prompt:
        await page.evaluate("setTimeout(() => location.reload(), 0)")
    dialog = await prompt.value
    assert dialog.type == "beforeunload"
    await dialog.dismiss()
    await expect(panel).to_have_count(1)
    page.once("dialog", lambda dialog: dialog.accept())
    await cancel.click()
    await expect(opener).to_be_focused()
    await opener.click()
    await expect(panel.get_by_label("Password awal")).to_have_value("")
    await cancel.click()
    await manage.click()
    await page.locator(".staff-edit-panel").get_by_label("Nama", exact=True).fill("Draft edit")
    await expect(opener).to_have_count(0)
    page.once("dialog", lambda dialog: dialog.dismiss())
    await page.get_by_role("button", name="Kembali ke daftar", exact=True).click()
    await expect(panel).to_have_count(0)
    await expect(page.locator(".staff-edit-panel")).to_have_count(1)
    page.once("dialog", lambda dialog: dialog.accept())
    await page.get_by_role("button", name="Kembali ke daftar", exact=True).click()
    await opener.click()
    await expect(page.locator(".staff-edit-panel")).to_have_count(0)
    await cancel.click()
    await draft()
    page.once("dialog", lambda dialog: dialog.accept())
    await cancel.click()
    await manage.click()
    await expect(panel).to_have_count(0)
    await expect(page.locator("#staff-edit-title")).to_be_focused()
    await page.get_by_role("button", name="Kembali ke daftar", exact=True).click()
    await expect(manage).to_be_focused()
    print(f"PASS {width}: list, badges, keyboard, draft cancellation, add/edit switching, rendered contrast, 200% text, reduced keyboard height")

    # A successful POST plus a failed refresh must never reopen the create form.
    for mode in ("delayed", "refresh-failed", "duplicate", "network", "truncated", "expired"):
        reload_failed = mode == "refresh-failed"
        await visit()
        await draft()
        count = len(posts)
        await submit.click()
        if mode == "delayed":
            await asyncio.wait_for(started.wait(), 15)
            await expect(opener).to_have_count(0)
            await expect(manage).to_have_count(0)
            for control in (panel.get_by_role("button", name="Kembali ke daftar", exact=True), cancel, page.get_by_role("button", name="Keluar", exact=True)):
                await expect(control).to_be_disabled()
            await audit("disabled controls")
            await panel.locator("form").dispatch_event("submit")
            await expect(page.locator(".staff-edit-panel")).to_have_count(0)
            assert len(posts) == count + 1
            release.set()
        if mode in ("delayed", "refresh-failed"):
            await expect(panel).to_have_count(0)
            await expect(page.locator("#staff-result")).to_contain_text("Akun petugas berhasil dibuat")
            if reload_failed:
                await expect(page.locator("#staff-result")).to_be_focused()
                reload_failed = False
                await page.get_by_role("button", name="Muat ulang", exact=True).click()
            else:
                await expect(page.locator(f"#staff-account-{created['id']}")).to_be_focused()
            await expect(page.locator(".staff-list, .staff-list-table")).to_contain_text("nadia@example.com")
            await opener.click()
            await expect(panel.get_by_label("Password awal")).to_have_value("")
            await cancel.click()
        elif mode == "expired":
            await page.wait_for_url("**/admin/login")
        else:
            await expect(page.get_by_role("alert").first).to_be_visible()
            await expect(panel.get_by_label("Password awal")).to_have_value("password-fixture-123")
            await audit(mode)
            if mode in ("network", "truncated"):
                await expect(submit).to_be_disabled()
                await expect(panel.get_by_label("Email", exact=True)).not_to_be_editable()
                await panel.locator("form").dispatch_event("submit")
                assert len(posts) == count + 1
                await panel.get_by_role("button", name="Periksa daftar petugas").click()
                if mode == "truncated":
                    await expect(panel).to_have_count(0)
                    await expect(page.locator(f"#staff-account-{created['id']}")).to_be_focused()
                else:
                    await expect(submit).to_be_enabled()
            if await panel.count():
                page.once("dialog", lambda dialog: dialog.accept())
                await cancel.click()
        assert len(posts) == count + 1
        print(f"PASS {width}: create {mode}")

    for catalog_mode in ("loading", "error", "expired"):
        started.clear(); release.clear()
        if catalog_mode == "expired":
            await page.goto(base_url + "/admin/staff")
            await page.wait_for_url("**/admin/login")
            continue
        await visit()
        await opener.click()
        if catalog_mode == "loading":
            await panel.get_by_text("Memuat pilihan event…").wait_for()
            await expect(submit).to_be_disabled()
            release.set()
            await expect(submit).to_be_enabled()
        elif catalog_mode == "error":
            await panel.get_by_text("Katalog event belum dapat dimuat.", exact=False).wait_for()
            await expect(submit).to_be_disabled()
            catalog_mode = "success"
            await panel.get_by_role("button", name="Coba lagi").click()
            await expect(submit).to_be_enabled()
        else:
            await page.wait_for_url("**/admin/login")
        if await panel.count(): await cancel.click()
    catalog_mode = "success"
    await visit()
    await draft()
    page.once("dialog", lambda dialog: dialog.accept())
    await page.get_by_role("button", name="Keluar", exact=True).click()
    await page.wait_for_url("**/admin/login")
    assert len(logouts) == 1
    assert not errors, errors
    (output / f"checks-{width}.json").write_text(json.dumps(dict(width=width, audits=audits, browserErrors=errors), indent=2))
    await page.unroute("**/api/v1/**", fixture)
    print(f"PASS {width}: catalog loading, failure/retry, expired session; no browser runtime errors")


async def verify_management_flow(page, admin, width, base_url):
    names = ["Ayu Satriana", "Bima Pradana", "Citra Nirmala", "Damar Santosa", "Eka Wulandari", "Farhan Ardi", "Gita Kusuma", "Hana Putri", "Indra Mahendra", "Jihan Laras", "Kirana Sari", "Laras Wening", "Maya Prameswari", "Nanda Prasetya", "Oki Saputra", "Putri Anindya", "Rani Paramita", "Satria Wibowo", "Tania Maharani dengan nama panjang untuk operasional konser"]
    events = [dict(id="musik", artist="Nusa Malam", city="Jakarta", ticketTiers=[dict(gate=f"Gate {letter}") for letter in "ABCDEFG"]),
              dict(id="teater", artist="Panggung Senja", city="Bandung", ticketTiers=[dict(gate="Gate Utama")]),
              dict(id="tanpa-gate", artist="Konser Baru", city="Bogor", ticketTiers=[])]
    rows = [admin, *[dict(id=f"{index:032x}", name=name, email=name.lower().replace(" ", ".") + "@operasional-konser.example.test", role="STAFF", active=index % 3 != 0,
                         assignments=[] if index % 4 == 0 else [dict(eventId="musik", gate="Gate A")] if index % 2 else [dict(eventId="teater", gate="Gate Utama")]) for index, name in enumerate(names, 1)]]
    rows[1]["assignments"] = [dict(eventId="musik", gate=f"Gate {letter}") for letter in "ABCDEFG"]
    output = Path(os.environ.get("STAFF_REVIEW_OUTPUT", "/tmp/phase51-55/staff"))
    output.mkdir(parents=True, exist_ok=True)
    (output / f"fixture20-{width}.json").write_text(json.dumps(dict(staff=rows, events=events), indent=2))
    mutations, errors, audits = [], [], []
    reads = 0
    mode = "success"
    delay = False
    started, release = asyncio.Event(), asyncio.Event()
    page.on("pageerror", lambda error: errors.append(str(error)))

    async def fixture(route):
        nonlocal reads
        path = route.request.url.split("/api/v1/")[1]
        method = route.request.method
        if path == "staff/me": await route.fulfill(json=dict(staff=admin))
        elif path == "admin/events": await route.fulfill(json=dict(events=events))
        elif path == "admin/staff" and method == "GET":
            reads += 1
            await route.fulfill(json=dict(staff=rows))
        else:
            payload = route.request.post_data_json
            mutations.append(dict(method=method, path=path, payload={k: v for k, v in payload.items() if k != "password"}))
            if delay:
                started.set()
                await release.wait()
            if mode == "network":
                await route.abort("failed")
                return
            if method == "POST" and path == "admin/staff":
                person = dict(id="d" * 32, role="STAFF", active=True, **{k: v for k, v in payload.items() if k != "password"})
                rows.append(person)
                await route.fulfill(status=201, json=person)
                return
            parts = path.split("/")
            person = next(row for row in rows if row["id"] == parts[2])
            if method == "PATCH" and len(parts) == 3: person.update(payload)
            elif method == "PUT" and parts[-1] == "assignments": person["assignments"] = payload["assignments"]
            elif method == "PUT" and parts[-1] == "password": assert len(payload["password"]) >= 12
            else: raise AssertionError((method, path))
            if mode == "truncated": await route.fulfill(status=200, content_type="application/json", body='{"result":')
            else: await route.fulfill(status=204)

    async def audit(label):
        result = await page.evaluate(STYLE_AUDIT)
        assert not result["failures"] and not result["overflow"], (width, label, result)
        audits.append(dict(state=label, **result))

    async def capture(name, target=None):
        if target is not None: await target.evaluate("element => element.scrollIntoView({block: 'start', behavior: 'instant'})")
        else: await page.evaluate("scrollTo({top: 0, behavior: 'instant'})")
        await page.evaluate("new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve)))")
        await audit(name)
        await page.screenshot(path=str(output / f"roster20-{name}-{width}.png"), animations="disabled")

    async def reset_filters():
        if await page.get_by_role("button", name="Reset filter", exact=True).count():
            await page.get_by_role("button", name="Reset filter", exact=True).click()

    async def reload_saved(person):
        await page.reload()
        if width < 768:
            await page.get_by_label("Cari nama atau email", exact=True).fill(person["email"])
        await expect(page.locator(f"#staff-account-{person['id']}")).to_contain_text(person["name"])
        await page.locator(f"#staff-manage-{person['id']}").click()
        await expect(page.locator("#staff-edit-title")).to_be_focused()
        await expect(page.locator(".staff-edit-panel").get_by_label("Nama", exact=True)).to_have_value(person["name"])

    await page.route("**/api/v1/**", fixture)
    await page.goto(base_url + "/admin/staff")
    await expect(page.locator("[id^=staff-account-]")).to_have_count(5 if width < 768 else 20)
    await run_path(str(Path(__file__).with_name("admin-ui-check.py")))["check_help"](page, width)
    assert not mutations, "Help must not mutate accounts"
    if width >= 768:
        await expect(page.locator(".staff-list-table th")).to_have_text(["Nama", "Email", "Peran", "Status", "Penugasan", "Aksi"])
        assert await page.locator(".staff-list-table").evaluate("el => getComputedStyle(el).display") == "table"
        assert await page.locator(".staff-list-table tbody tr").first.evaluate("el => getComputedStyle(el).display") == "table-row"
        icons = page.locator('.staff-list-table [id^="staff-manage-"]')
        await expect(icons).to_have_count(19)
        assert await icons.evaluate_all("buttons => buttons.every(el => { const r = el.getBoundingClientRect(); return !el.textContent.trim() && el.querySelector('svg[aria-hidden=true]') && el.getAttribute('aria-label') === el.title && r.width >= 44 && r.height >= 44; })")
    await expect(page.locator(".staff-filter-results")).to_contain_text("20 dari 20 akun")
    assert await page.locator("[id^=staff-account-]").evaluate_all("rows => rows.map(row => row.id.replace('staff-account-', ''))") == [row["id"] for row in (rows[:5] if width < 768 else rows)]
    await expect(page.get_by_role("button", name="Kelola", exact=False)).to_have_count(4 if width < 768 else 19)
    await expect(page.locator(f"#staff-account-{admin['id']}")).to_contain_text("Akun administrator tidak dapat diubah melalui halaman ini.")
    assert await page.locator(".staff-status").evaluate_all("badges => badges.every(b => b.tabIndex === -1 && b.tagName === 'SPAN' && getComputedStyle(b).cursor !== 'pointer' && !b.closest('button,a'))")
    badge = page.locator(".staff-status").first
    before = await badge.evaluate("b => [getComputedStyle(b).backgroundColor, getComputedStyle(b).color]")
    await badge.hover()
    assert await badge.evaluate("b => [getComputedStyle(b).backgroundColor, getComputedStyle(b).color]") == before
    await capture("list")
    if width == 1440:
        assert await page.get_by_role("button", name="Kelola", exact=False).first.evaluate("button => button.getBoundingClientRect().top >= 0 && button.getBoundingClientRect().bottom <= innerHeight"), "A STAFF manage button must be visible before scrolling"
    await page.screenshot(path=str(output / f"roster20-full-list-{width}.png"), full_page=True)
    summary = page.locator(f"#staff-account-{rows[1]['id']} summary")
    await expect(page.locator(f"#staff-account-{rows[1]['id']} details span").last).not_to_be_visible()
    await summary.focus(); await page.keyboard.press("Enter")
    await expect(page.locator(f"#staff-account-{rows[1]['id']} details span").last).to_be_visible()
    await page.keyboard.press("Enter")
    if width <= 600:
        await expect(page.get_by_role("combobox", name="Status akun")).not_to_be_visible()
        await page.get_by_role("button", name="Filter lainnya", exact=True).click()
    search = page.get_by_label("Cari nama atau email", exact=True)
    initial_reads = reads
    await search.fill("  AYU  ")
    await page.get_by_role("combobox", name="Status akun", exact=True).select_option("active")
    await page.get_by_role("combobox", name="Event penugasan", exact=True).select_option("musik")
    await expect(page.locator("[id^=staff-account-]")).to_have_count(1)
    await page.get_by_label("Hanya akun tanpa penugasan", exact=True).check()
    if width <= 600:
        await page.get_by_role("button", name="Filter lainnya (3 aktif)", exact=True).click()
        await expect(page.get_by_role("combobox", name="Status akun")).not_to_be_visible()
        await expect(search).to_be_visible()
        await page.get_by_role("button", name="Filter lainnya (3 aktif)", exact=True).click()
    await expect(page.locator(".staff-filter-results")).to_contain_text("0 dari 20 akun")
    await capture("empty-search")
    assert reads == initial_reads, "Filtering must use existing list data without API requests"
    await reset_filters()
    await expect(search).to_be_focused()
    await search.fill("  AYU  ")
    await page.locator(f"#staff-manage-{rows[1]['id']}").click()
    editor = page.locator(".staff-edit-panel")
    await expect(page.locator("#staff-edit-title")).to_be_focused()
    await expect(editor).to_contain_text(rows[1]["email"])
    await expect(page.locator(".staff-list, .staff-list-table")).to_have_count(0)
    await expect(search).to_have_count(0)
    await expect(page.locator("#staff-create-opener")).to_have_count(0)
    await editor.get_by_label("Nama", exact=True).fill("Draft Ayu")
    await editor.get_by_role("link", name="Penugasan", exact=True).click()
    await expect(page.locator("#staff-assignments")).to_be_focused()
    await editor.get_by_role("link", name="Profil dan akses", exact=True).click()
    await expect(editor.get_by_label("Nama", exact=True)).to_have_value("Draft Ayu")
    await capture("edit", editor.locator("h2"))
    page.once("dialog", lambda dialog: dialog.dismiss())
    await editor.get_by_role("button", name="Kembali ke daftar", exact=True).click()
    await expect(editor).to_have_count(1)
    page.once("dialog", lambda dialog: dialog.accept())
    await editor.get_by_role("button", name="Kembali ke daftar", exact=True).click()
    await expect(search).to_have_value("  AYU  ")
    await expect(page.locator(f"#staff-manage-{rows[1]['id']}")).to_be_focused()
    await reset_filters()

    # Returning to the list protects an unadded selection before another account can open.
    await page.locator(f"#staff-manage-{rows[1]['id']}").click()
    await editor.get_by_role("combobox", name="Event", exact=True).select_option("musik")
    page.once("dialog", lambda dialog: dialog.dismiss())
    await editor.get_by_role("button", name="Kembali ke daftar", exact=True).click()
    await expect(page.locator("#staff-edit-title")).to_have_text("Edit petugas: " + rows[1]["name"])
    page.once("dialog", lambda dialog: dialog.accept())
    await editor.get_by_role("button", name="Kembali ke daftar", exact=True).click()
    await page.locator(f"#staff-manage-{rows[2]['id']}").click()
    await expect(page.locator("#staff-edit-title")).to_be_focused()
    await expect(editor.get_by_role("combobox", name="Event", exact=True)).to_have_value("")
    await expect(editor.get_by_role("button", name="Simpan penugasan")).to_be_enabled()
    await editor.get_by_role("button", name="Kembali ke daftar", exact=True).click()

    # A distant account restores list scroll and its trigger.
    trigger = page.locator(f"#staff-manage-{rows[-1]['id']}")
    if width < 768:
        for _ in range(3):
            await page.get_by_role("button", name="Berikutnya", exact=True).click()
    await trigger.scroll_into_view_if_needed()
    scroll = await page.evaluate("scrollY")
    await trigger.click()
    await expect(page.locator(".staff-list, .staff-list-table")).to_have_count(0)
    await editor.get_by_role("button", name="Kembali ke daftar", exact=True).click()
    await expect(trigger).to_be_focused()
    assert abs(await page.evaluate("scrollY") - scroll) <= 2

    # End-to-end workflow uses one mutable API fixture so reloads verify persisted state.
    await page.locator("#staff-create-opener").click()
    panel = page.locator(".staff-create-panel")
    await capture("create-open", panel.locator("h2"))
    await panel.get_by_label("Nama", exact=True).fill("Nadia Petugas")
    await panel.get_by_label("Email", exact=True).fill("nadia.uat@example.test")
    await panel.get_by_label("Password awal").fill("fixture-password-123")
    event_select = panel.get_by_role("combobox", name="Event", exact=True)
    gate_select = panel.get_by_role("combobox", name="Gate", exact=True)
    submit = panel.get_by_role("button", name="Buat akun petugas", exact=True)
    await event_select.select_option("tanpa-gate")
    await expect(gate_select).to_be_disabled()
    await expect(submit).to_be_disabled()
    await expect(panel).to_contain_text("Event ini belum memiliki gate")
    await panel.get_by_role("button", name="Batalkan pilihan").click()
    await event_select.select_option("musik")
    await gate_select.select_option("Gate A")
    await expect(submit).to_be_disabled()
    count = len(mutations)
    await panel.locator("form").dispatch_event("submit")
    assert len(mutations) == count
    await capture("event-gate-selection", gate_select)
    await panel.get_by_role("button", name="Tambah", exact=True).click()
    await expect(submit).to_be_enabled()
    await event_select.select_option("musik")
    await gate_select.select_option("Gate A")
    await expect(panel).to_contain_text("Penugasan event/gate ini sudah ada di draft")
    await expect(panel.get_by_role("button", name="Tambah", exact=True)).to_be_disabled()
    await panel.get_by_role("button", name="Batalkan pilihan").click()
    await submit.click()
    await expect(panel).to_have_count(0)
    person = rows[-1]
    assert person["assignments"] == [dict(eventId="musik", gate="Gate A")]
    await reload_saved(person)
    await capture("manage", editor.locator("h2"))
    await editor.get_by_label("Nama", exact=True).fill("Nadia Petugas Utama")
    await editor.get_by_role("button", name="Simpan profil", exact=True).click()
    await page.get_by_text("Profil petugas berhasil diperbarui.", exact=True).wait_for()
    await reload_saved(person)
    assert person["name"] == "Nadia Petugas Utama"
    assignments_form = editor.locator("form").nth(1)
    await assignments_form.get_by_role("combobox", name="Event", exact=True).select_option("musik")
    await assignments_form.get_by_role("combobox", name="Gate", exact=True).select_option("Gate B")
    await expect(assignments_form.get_by_role("button", name="Simpan penugasan")).to_be_disabled()
    page.once("dialog", lambda dialog: dialog.dismiss())
    await editor.get_by_role("button", name="Kembali ke daftar", exact=True).click()
    await expect(editor).to_have_count(1)
    await assignments_form.get_by_role("button", name="Tambah", exact=True).click()
    await assignments_form.get_by_role("button", name="Hapus penugasan Gate A", exact=True).click()
    count = len(mutations)
    page.once("dialog", lambda dialog: dialog.dismiss())
    await assignments_form.get_by_role("button", name="Simpan penugasan").click()
    assert len(mutations) == count
    delay = True
    page.once("dialog", lambda dialog: dialog.accept())
    await assignments_form.get_by_role("button", name="Simpan penugasan").click()
    await asyncio.wait_for(started.wait(), 15)
    await assignments_form.dispatch_event("submit")
    assert len(mutations) == count + 1
    release.set(); delay = False
    await page.get_by_text("Penugasan berhasil diperbarui.", exact=True).wait_for()
    await reload_saved(person)
    assert person["assignments"] == [dict(eventId="musik", gate="Gate B")]
    await editor.get_by_role("button", name="Kembali ke daftar", exact=True).click()
    if width <= 600: await page.get_by_role("button", name="Filter lainnya", exact=True).click()
    await page.get_by_role("combobox", name="Status akun", exact=True).select_option("active")
    await page.locator(f"#staff-manage-{person['id']}").click()
    await editor.get_by_label("Akun aktif", exact=True).uncheck()
    count = len(mutations)
    page.once("dialog", lambda dialog: dialog.dismiss())
    await editor.get_by_role("button", name="Simpan profil", exact=True).click()
    assert len(mutations) == count
    page.once("dialog", lambda dialog: dialog.accept())
    await editor.get_by_role("button", name="Simpan profil", exact=True).click()
    await page.get_by_text("Profil petugas berhasil diperbarui.", exact=True).wait_for()
    await expect(editor).to_contain_text("tidak termasuk hasil saat ini")
    await editor.get_by_role("button", name="Kembali ke daftar", exact=True).click()
    await expect(page.get_by_role("combobox", name="Status akun", exact=True)).to_have_value("active")
    await expect(page.locator(f"#staff-account-{person['id']}")).to_have_count(0)
    await reset_filters()
    await reload_saved(person)
    await expect(editor.get_by_label("Akun aktif", exact=True)).not_to_be_checked()
    await editor.get_by_role("button", name="Kembali ke daftar", exact=True).click()
    await expect(page.locator(f"#staff-account-{person['id']} .staff-status")).to_have_text("Status: Nonaktif")
    await page.locator(f"#staff-manage-{person['id']}").click()
    await editor.get_by_label("Password baru", exact=True).fill("fixture-password-456")
    count = len(mutations)
    page.once("dialog", lambda dialog: dialog.dismiss())
    await editor.get_by_role("button", name="Reset password", exact=True).click()
    assert len(mutations) == count
    page.once("dialog", lambda dialog: dialog.accept())
    await editor.get_by_role("button", name="Reset password", exact=True).click()
    await page.get_by_text("Password direset dan sesi lama petugas dicabut.", exact=True).wait_for()
    await expect(editor.get_by_label("Password baru", exact=True)).to_have_value("")
    await reload_saved(person)
    assert [item["method"] for item in mutations] == ["POST", "PATCH", "PUT", "PATCH", "PUT"]

    # Unknown outcomes cannot be turned into another mutation by switching panels.
    mode = "network"
    await editor.get_by_label("Nama", exact=True).fill("Nadia Perubahan Terputus")
    await editor.get_by_role("button", name="Simpan profil", exact=True).click()
    await expect(editor.get_by_role("button", name="Simpan profil", exact=True)).to_be_disabled()
    count = len(mutations)
    await editor.locator("form").first.dispatch_event("submit")
    assert len(mutations) == count
    mode = "success"
    await page.get_by_role("button", name="Muat ulang", exact=True).click()
    await expect(page.get_by_role("button", name="Muat ulang", exact=True)).to_be_enabled()
    await expect(editor.get_by_role("button", name="Simpan profil", exact=True)).to_be_disabled()
    person["name"] = "Nadia Perubahan Terputus"
    await page.get_by_role("button", name="Muat ulang", exact=True).click()
    await expect(editor.get_by_role("button", name="Simpan profil", exact=True)).to_be_enabled()
    await assignments_form.get_by_role("button", name="Hapus penugasan Gate B").click()
    mode = "truncated"
    page.once("dialog", lambda dialog: dialog.accept())
    await assignments_form.get_by_role("button", name="Simpan penugasan").click()
    await expect(assignments_form.get_by_role("button", name="Simpan penugasan")).to_be_disabled()
    count = len(mutations)
    await assignments_form.dispatch_event("submit")
    assert len(mutations) == count
    page.once("dialog", lambda dialog: dialog.accept())
    await editor.get_by_role("button", name="Kembali ke daftar", exact=True).click()
    await page.locator(f"#staff-manage-{person['id']}").click()
    await expect(assignments_form.get_by_role("button", name="Simpan penugasan")).to_be_disabled()
    mode = "success"
    await page.get_by_role("button", name="Muat ulang", exact=True).click()
    await expect(assignments_form.get_by_role("button", name="Simpan penugasan")).to_be_enabled()
    await expect(assignments_form.locator(".staff-assignment-list li")).to_have_count(0)
    await expect(editor.locator(".staff-edit-unsaved")).to_have_count(0)
    mode = "network"
    await editor.get_by_label("Password baru", exact=True).fill("fixture-password-789")
    page.once("dialog", lambda dialog: dialog.accept())
    await editor.get_by_role("button", name="Reset password", exact=True).click()
    await expect(editor.get_by_role("button", name="Reset password", exact=True)).to_be_disabled()
    count = len(mutations)
    await editor.locator("form").last.dispatch_event("submit")
    assert len(mutations) == count
    mode = "success"
    await page.get_by_role("button", name="Muat ulang", exact=True).click()
    await expect(page.get_by_role("button", name="Muat ulang", exact=True)).to_be_enabled()
    await expect(editor.get_by_role("button", name="Reset password", exact=True)).to_be_disabled()
    page.once("dialog", lambda dialog: dialog.accept())
    await editor.get_by_role("button", name="Kembali ke daftar", exact=True).click()
    await page.locator(f"#staff-manage-{person['id']}").click()
    await expect(editor.get_by_role("button", name="Reset password", exact=True)).to_be_disabled()
    await capture("unknown-password", editor.locator("h2"))
    assert not errors, errors
    (output / f"management-{width}.json").write_text(json.dumps(dict(width=width, fixtureAccounts=20, automated=True, humanUAT="not performed", audits=audits, mutations=mutations, browserErrors=errors), indent=2))
    # Accept the native page-exit warning before the following scanner checks.
    page.once("dialog", lambda dialog: dialog.accept())
    await editor.get_by_role("button", name="Kembali ke daftar", exact=True).click()
    await page.unroute("**/api/v1/**", fixture)
    page.once("dialog", lambda dialog: dialog.accept())
    print(f"PASS {width}: 20-account filters, summaries, pending selections, full saved workflow/reloads, confirmations, unknown mutation locks")


async def main():
    base_url = sys.argv[1] if len(sys.argv) > 1 else "http://127.0.0.1:5173"
    width = int(sys.argv[2]) if len(sys.argv) > 2 else 1440
    async with async_playwright() as playwright:
        browser = await playwright.chromium.launch(channel="chrome", headless=True)
        context = await browser.new_context(viewport={"width": width, "height": 900}, is_mobile=width < 600, has_touch=width < 600)
        await context.add_init_script("sessionStorage.setItem('ticket-online:staff-session', JSON.stringify({accessToken:'s'.repeat(43),expiresAt:new Date(Date.now()+3600000).toISOString()}))")
        page = await context.new_page()
        admin = dict(id="c" * 32, name="Admin", email="admin@example.com", role="ADMIN", active=True, assignments=[])
        people = [dict(id=letter * 32, name=name, email=name.lower() + "@example.com", role="STAFF", active=True, assignments=[dict(eventId="event", gate=gate)]) for letter, name, gate in [("a", "Alice", "Gate A"), ("b", "Bob", "Gate B")]]
        if "--management-only" in sys.argv:
            await verify_management_flow(page, admin, width, base_url)
            await browser.close()
            return
        started, release = asyncio.Event(), asyncio.Event()
        mutations = []

        async def admin_api(route):
            path = route.request.url.split("/api/v1/")[1]
            if path == "staff/me":
                await route.fulfill(json=dict(staff=admin))
            elif path == "admin/events":
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
        await page.get_by_role("button", name="Kelola Alice", exact=True).click()
        editor = page.locator(".staff-edit-panel")
        await editor.get_by_label("Nama", exact=True).fill("Alice draft")
        await editor.get_by_label("Akun aktif", exact=True).uncheck()
        await editor.get_by_role("button", name="Simpan penugasan", exact=True).click()
        await asyncio.wait_for(started.wait(), 15)
        bob = page.get_by_role("button", name="Kelola Bob", exact=True)
        await expect(bob).to_have_count(0)
        await expect(page.get_by_role("button", name="Kembali ke daftar", exact=True)).to_be_disabled()
        await expect(page.get_by_role("button", name="Muat ulang", exact=True)).to_be_disabled()
        await expect(editor.locator("h2")).to_have_text("Edit petugas: Alice")
        release.set()
        await page.get_by_text("Penugasan berhasil diperbarui.", exact=True).wait_for()
        await expect(bob).to_have_count(0)
        await expect(editor.get_by_label("Nama", exact=True)).to_have_value("Alice draft")
        await expect(editor.get_by_label("Akun aktif", exact=True)).not_to_be_checked()
        page.once("dialog", lambda dialog: dialog.dismiss())
        await page.get_by_role("button", name="Kembali ke daftar", exact=True).click()
        await expect(editor.get_by_label("Nama", exact=True)).to_have_value("Alice draft")
        page.once("dialog", lambda dialog: dialog.accept())
        await page.get_by_role("button", name="Kembali ke daftar", exact=True).click()
        await bob.click()
        await expect(editor.get_by_label("Nama", exact=True)).to_have_value("Bob")
        await editor.get_by_role("button", name="Simpan penugasan", exact=True).click()
        await page.get_by_text("Penugasan berhasil diperbarui.", exact=True).wait_for()
        assert mutations == [dict(path=f"admin/staff/{person['id']}/assignments", body=dict(assignments=person["assignments"])) for person in people]
        print("PASS: delayed assignment save preserves selection and profile draft")
        await page.unroute("**/api/v1/**", admin_api)
        await verify_create_flow(page, people, admin, width, base_url)
        await verify_management_flow(page, admin, width, base_url)

        mode = "success"
        posts = []
        started.clear()
        release.clear()

        async def scanner_api(route):
            if route.request.url.endswith("/me"):
                await route.fulfill(json=dict(staff=people[0]))
                return
            if route.request.url.endswith("/events"):
                await route.fulfill(json=dict(events=[]))
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
        await asyncio.wait_for(started.wait(), 15)
        await expect(code_input).to_be_disabled()
        await page.locator(".scan-form").dispatch_event("submit")
        assert len(posts) == 1
        release.set()
        await page.get_by_text("Gate Masuk Terbuka", exact=True).wait_for()
        await expect(page.locator("#scan-result-title")).to_be_focused()
        await expect(code_input).to_be_disabled()
        next_scan = page.get_by_role("button", name="Scan berikutnya", exact=True)
        await next_scan.focus()
        await page.keyboard.press("Enter")
        await expect(code_input).to_be_focused()
        await expect(code_input).to_have_value("")
        await page.keyboard.type(second)
        await page.keyboard.press("Enter")
        await expect(page.locator(".ticket-detail-card")).to_contain_text(second)
        await expect(page.locator("#scan-result-title")).to_be_focused()
        assert [post["code"] for post in posts] == [first, second]
        print("PASS: two keyboard scans with explicit next scan, result focus, and concurrent submit blocked")

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
            await expect(page.get_by_role("button", name="Scan berikutnya", exact=True)).to_be_disabled()
            await page.wait_for_load_state("networkidle")
            assert len(posts) == count + 1
            print(f"PASS: {mode} keeps unknown result, recovery instructions, ticket code, and no retry")
        await browser.close()


if __name__ == "__main__":
    asyncio.run(main())
