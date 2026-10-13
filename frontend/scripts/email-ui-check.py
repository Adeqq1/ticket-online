"""Email workflow fixtures; optional real HTTP backend relay from the Go SMTP integration.
Run from frontend/: python3 scripts/email-ui-check.py --base-url http://localhost:5173
"""
import argparse
import asyncio
import json
import os
from pathlib import Path
from datetime import datetime, timedelta, timezone
from runpy import run_path
from urllib.parse import urlparse

from playwright.async_api import async_playwright, expect

confirmation_helpers = run_path(str(Path(__file__).with_name('admin-confirm.py')))
answer_confirmation = confirmation_helpers['answer_confirmation']
auto_confirm = confirmation_helpers['auto_confirm']

common = run_path(str(Path(__file__).with_name('admin-ui-check.py')))
style_audit = run_path(str(Path(__file__).with_name('staff-review-check.py')))['STYLE_AUDIT']
JOB = 'e' * 32
REPLACEMENT = 'f' * 32


async def audit(page):
    result = await page.evaluate(style_audit)
    assert not result['failures'], result
    assert await page.evaluate('document.documentElement.scrollWidth <= innerWidth'), 'Page overflows'
    return result


async def enlarged(page, output, label):
    await page.evaluate("""() => {
      window.emailFontSizes = [...document.querySelectorAll('.admin-layout *')].filter(el => el.checkVisibility())
        .map(el => [el, parseFloat(getComputedStyle(el).fontSize), el.style.fontSize]);
      for (const [el, size] of window.emailFontSizes) el.style.fontSize = `${size * 2}px`;
    }""")
    try:
        await audit(page)
        await page.evaluate('window.scrollTo(0, 0)')
        await page.screenshot(path=str(output / (label + '-text200.png')), full_page=True)
    finally:
        await page.evaluate("for (const [el, , value] of window.emailFontSizes) el.style.fontSize = value")


async def fixtures(browser, base, output, width):
    results = []
    for scenario in ('standard', 'same-id', 'replacement', 'unproven', 'rejected', 'rate-limit', 'expired', 'refresh-error', 'detail-error', 'success-refresh-error', 'recovery-new-request', 'queued-same-id', 'queued-replacement'):
        print(f'Checking email: {width}px {scenario}', flush=True)
        context = await browser.new_context(viewport={'width': width, 'height': 900}, timezone_id='Asia/Jakarta', has_touch=width < 768)
        await context.add_init_script(common['SESSION_INIT'])
        page = await context.new_page()
        page.on('dialog', lambda d: d.accept())
        errors = []
        page.on('pageerror', lambda e: errors.append(str(e)))
        state = {'posts': 0, 'lists': 0, 'details': 0, 'prove': False, 'sent': False, 'detailFail': False, 'hold': None}
        original = common['body']('/api/v1/admin/email-jobs/' + JOB, 'standard', 'ADMIN')[1]
        original.update(lastError='SMTP koneksi gagal', history=[], attempts=3)
        if scenario == 'recovery-new-request':
            original.update(kind='RECOVERY', orderStatus='', lastError='permintaan pemulihan kedaluwarsa')
        second = {**original, 'id': 'a' * 32, 'reference': 'TO-PENERIMA-KOSONG', 'recipient': '', 'kind': 'RECOVERY', 'orderStatus': '', 'canRetry': False, 'retryReason': 'Data permintaan tidak dapat diverifikasi atau pesanan sudah kedaluwarsa.', 'lastError': 'permintaan pemulihan kedaluwarsa'}

        async def fixture(route):
            endpoint = urlparse(route.request.url).path.removeprefix('/api/v1')
            if endpoint == '/admin/email-jobs':
                state['lists'] += 1
                if scenario in ('refresh-error', 'success-refresh-error') and state['lists'] > 1:
                    return await route.fulfill(status=503, json={'error': {'code': 'SERVICE_UNAVAILABLE', 'message': 'Daftar gagal dimuat.'}})
                rows = [{**original, 'recipient': 'n***@example.test'}, second] if not state['prove'] else [second]
                if urlparse(route.request.url).query:
                    rows = [second, {**second, 'id': 'b' * 32, 'reference': 'TO-HALAMAN-2', 'lastError': 'unrecognized reason'}]
                return await route.fulfill(json={'items': rows, 'nextCursor': None if urlparse(route.request.url).query else 'cursor-2'})
            if endpoint == '/admin/payment-cases':
                return await route.fulfill(json={'items': [], 'nextCursor': None})
            if endpoint == '/admin/operations':
                data = common['body']('/api/v1/admin/operations', 'empty', 'ADMIN')[1]
                return await route.fulfill(json=data)
            if endpoint == f'/admin/email-jobs/{JOB}/retry':
                state['posts'] += 1
                await asyncio.sleep(.25)
                if scenario in ('rejected', 'rate-limit', 'expired'):
                    status, code, message = {'rejected': (409, 'CONFLICT', 'Job tidak lagi tersedia.'), 'rate-limit': (429, 'RATE_LIMITED', 'Tunggu sebelum mencoba kembali.'), 'expired': (401, 'UNAUTHORIZED', 'Sesi sudah berakhir.')}[scenario]
                    return await route.fulfill(status=status, json={'error': {'code': code, 'message': message}})
                if scenario in ('same-id', 'replacement', 'unproven'):
                    return await route.abort('failed')
                state['prove'] = True
                return await route.fulfill(json={'jobId': JOB, 'retryJobId': REPLACEMENT if scenario in ('recovery-new-request', 'queued-replacement') else JOB, 'status': 'PENDING'})
            if endpoint == '/admin/email-jobs/' + JOB:
                state['details'] += 1
                if state['hold']: await state['hold'].wait()
                if scenario == 'detail-error' and state['details'] == 1 or state['detailFail'] and scenario != 'queued-replacement':
                    return await route.fulfill(status=503, json={'error': {'code': 'SERVICE_UNAVAILABLE', 'message': 'Detail gagal dimuat.'}})
                data = dict(original)
                if state['prove']:
                    data.update(status='PENDING', canRetry=False, attempts=0, lastError='', updatedAt='2026-10-11T03:01:00Z', history=[{'action': 'RETRY', 'actorName': 'Admin', 'createdAt': '2026-10-11T03:00:00.000001Z', 'data': {'retryJobId': JOB, 'status': 'PENDING'}}])
                    if scenario in ('replacement', 'recovery-new-request', 'queued-replacement'):
                        data.update(status='FAILED', supersededBy=REPLACEMENT, retryJobId=REPLACEMENT)
                        data['history'][0]['data'] = {'retryJobId': REPLACEMENT, 'status': 'SUPERSEDED'}
                    elif state['sent']: data.update(status='SENT', attempts=1)
                return await route.fulfill(json=data)
            if endpoint == '/admin/email-jobs/' + second['id']:
                return await route.fulfill(json=second)
            if endpoint == '/admin/email-jobs/' + REPLACEMENT:
                if state['detailFail']:
                    return await route.fulfill(status=503, json={'error': {'code': 'SERVICE_UNAVAILABLE', 'message': 'Detail gagal dimuat.'}})
                return await route.fulfill(json={**original, 'id': REPLACEMENT, 'status': 'PENDING' if scenario == 'queued-replacement' and not state['sent'] else 'SENT', 'canRetry': False})
            status, data = common['body'](urlparse(route.request.url).path, 'standard', 'ADMIN')
            await route.fulfill(status=status, json=data)

        await page.route('**/api/v1/**', fixture)
        await page.goto(base + '/admin/issues')
        opener = page.get_by_role('button', name='Lihat detail email ' + original['reference'], exact=True)
        await expect(opener).to_be_visible()
        await expect(page.locator('#issues-content')).not_to_contain_text('Menunggu verifikasi')
        await expect(page.get_by_text('Alamat penerima belum tersedia', exact=True)).to_be_visible()
        await audit(page)
        if scenario == 'standard':
            await common['check_help'](page, width)
            assert not state['posts'], 'Help must not schedule email'
            await page.locator('section[aria-labelledby="email-list-title"]').get_by_role('button', name='Berikutnya' if width < 768 else 'Muat berikutnya', exact=True).click()
            await expect(page.locator('section[aria-labelledby="email-list-title"] .admin-mobile-list > li, section[aria-labelledby="email-list-title"] tbody tr')).to_have_count(3)
            await expect(page.get_by_text('Terakhir berhasil dimuat:', exact=False)).to_have_count(0)
            await expect(page.locator('.email-scope')).to_have_count(0)
            await expect(page.get_by_text('email gagal sudah dimuat', exact=False)).to_have_count(0)
            await page.evaluate('window.scrollTo(0, 0)')
            await page.screenshot(path=str(output / f'list-{width}.png'), full_page=True)
            await enlarged(page, output, f'list-{width}')
        if scenario == 'refresh-error':
            await page.get_by_role('button', name='Muat ulang', exact=True).click()
            await expect(page.get_by_text('Pembaruan gagal.', exact=False)).to_be_visible()
            await expect(opener).to_be_visible()
            await context.close(); results.append(scenario); continue
        await opener.focus()
        await page.keyboard.press('Enter')
        heading = page.locator('#issue-detail-title')
        await expect(heading).to_be_focused()
        assert await page.locator('#email-list-title').count() == 0
        refresh = page.get_by_role('button', name='Periksa status pengiriman', exact=True)
        await expect(refresh).to_be_visible()
        if scenario == 'detail-error':
            await expect(page.get_by_role('alert')).to_contain_text('Detail gagal dimuat')
            await refresh.focus()
            await page.keyboard.press('Enter')
            await expect(page.get_by_role('heading', name='Rincian pengiriman', exact=True)).to_be_visible()
            await expect(page.get_by_role('alert')).to_have_count(0)
            await expect(refresh).to_be_focused()
            assert state['details'] == 2 and state['posts'] == 0, state
        else:
            retry = page.get_by_role('button', name='Buat permintaan pemulihan baru' if scenario == 'recovery-new-request' else 'Kirim ulang email', exact=True)
            await expect(retry).to_be_visible()
            await audit(page)
            if scenario == 'standard':
                await retry.click()
                await answer_confirmation(page, False)
                assert state['posts'] == 0
                await expect(retry).to_be_focused()
            if scenario != 'standard':
                await retry.dblclick()
                await answer_confirmation(page)
                if scenario == 'expired':
                    await page.wait_for_url('**/admin/login')
                elif scenario in ('same-id', 'replacement', 'unproven'):
                    verify = page.get_by_role('button', name='Periksa hasil tindakan', exact=True)
                    try: await expect(verify).to_be_visible()
                    except AssertionError:
                        await page.screenshot(path=str(output / f'failure-{width}-{scenario}.png'), full_page=True)
                        print('Failed scenario:', width, scenario, state, await page.locator('#issues-content').inner_text(), flush=True)
                        raise
                    if width < 768:
                        await expect(page.get_by_role('button', name='Kembali ke daftar', exact=True)).to_be_disabled()
                    else:
                        await expect(page.get_by_role('button', name='Kembali ke daftar', exact=True)).to_have_count(0)
                    await expect(retry).to_be_disabled()
                    await verify.click()
                    await expect(page.get_by_text('Detail terbaru belum membuktikan hasil tindakan.', exact=False)).to_be_visible()
                    if scenario != 'unproven':
                        state['prove'] = True
                        await verify.click()
                        await expect(page.get_by_text('Pengiriman ulang dijadwalkan.', exact=False).first).to_be_visible()
                        await expect(verify).to_have_count(0)
                        await expect(page.get_by_text('Terkirim', exact=True)).to_be_visible() if scenario == 'replacement' else await expect(page.get_by_text('Menunggu pengiriman', exact=True)).to_be_visible()
                elif scenario in ('rejected', 'rate-limit'):
                    await expect(page.get_by_role('alert')).to_be_visible()
                elif scenario in ('queued-same-id', 'queued-replacement'):
                    await expect(page.get_by_text('Pengiriman ulang dijadwalkan.', exact=False).first).to_be_visible()
                    await expect(page.get_by_text('Menunggu pengiriman', exact=True)).to_be_visible()
                    assert state['lists'] == 2, state  # Original job has left the failed list.
                    state['detailFail'] = True
                    await refresh.click()
                    await expect(page.get_by_role('alert')).to_be_visible()
                    await expect(page.get_by_text('Pengiriman ulang dijadwalkan.', exact=False).first).to_be_visible()
                    await expect(page.get_by_text('Menunggu pengiriman', exact=True)).to_be_visible()
                    state['detailFail'] = False
                    state['sent'] = True  # Worker completes after the initial post-retry GET.
                    state['hold'] = asyncio.Event()
                    count = state['details']
                    await refresh.focus()
                    await page.keyboard.press('Enter')
                    await expect(refresh).to_be_disabled()
                    await refresh.dispatch_event('click')
                    state['hold'].set()
                    await expect(page.get_by_text('Terkirim', exact=True)).to_be_visible()
                    await expect(page.get_by_role('alert')).to_have_count(0)
                    await expect(refresh).to_be_enabled()
                    await expect(refresh).to_be_focused()
                    assert state['details'] == count + 1, state
                    await audit(page)
                    await page.screenshot(path=str(output / f'{scenario}-sent-{width}.png'), full_page=True)
                else:
                    await expect(page.get_by_text('Pengiriman ulang dijadwalkan.', exact=False).first).to_be_visible()
                    if scenario == 'success-refresh-error':
                        await expect(page.get_by_role('alert')).to_be_visible()
                    else:
                        await expect(page.get_by_text('Terkirim', exact=True)).to_be_visible()
                        await expect(page.get_by_role('button', name='Buat permintaan pemulihan baru', exact=True)).to_have_count(0)
                assert state['posts'] == 1, state
        if scenario == 'standard':
            await page.screenshot(path=str(output / f'detail-{width}.png'), full_page=True)
            await enlarged(page, output, f'detail-{width}')
            if await page.get_by_role('button', name='Menu', exact=True).is_visible():
                await page.get_by_role('button', name='Menu', exact=True).click()
            await page.get_by_role('link', name='Masalah', exact=True).click()
            await page.get_by_role('button', name='Lihat detail email TO-PENERIMA-KOSONG', exact=True).click()
            await expect(page.get_by_role('heading', name='Rincian pengiriman', exact=True)).to_be_visible()
            await expect(page.locator('.email-problem-summary')).to_have_count(0)
            if width < 768:
                await page.get_by_role('button', name='Kembali ke daftar', exact=True).click()
                await expect(page.get_by_role('button', name='Lihat detail email TO-PENERIMA-KOSONG', exact=True)).to_be_focused()
                await page.get_by_role('button', name='Lihat detail email TO-PENERIMA-KOSONG', exact=True).click()
            else:
                await expect(page.get_by_role('button', name='Kembali ke daftar', exact=True)).to_have_count(0)
            await expect(page.get_by_role('button', name='Muat ulang detail', exact=True)).to_have_count(0)
            await expect(page.get_by_text('Referensi permintaan', exact=True)).to_be_visible()
            await expect(page.locator('.email-facts')).not_to_contain_text('Tidak terkait pesanan')
            await expect(page.get_by_role('button', name='Kirim ulang email', exact=True)).to_have_count(0)
            await expect(page.get_by_role('button', name='Muat ulang', exact=True)).to_have_count(0)
            meanings = page.get_by_text('Arti status pengiriman', exact=True)
            await expect(meanings.locator('..')).not_to_have_attribute('open', '')
            await audit(page)
            await page.evaluate('window.scrollTo(0, 0)')
            await page.screenshot(path=str(output / f'expired-recovery-{width}.png'), full_page=True)
            await enlarged(page, output, f'expired-recovery-{width}')
            await meanings.focus()
            await page.keyboard.press('Enter')
            await expect(meanings.locator('..')).to_have_attribute('open', '')
            await expect(page.get_by_text('Dijadwalkan ulang berarti masuk antrean', exact=False)).to_be_visible()
            await page.keyboard.press('Enter')
            await expect(meanings.locator('..')).not_to_have_attribute('open', '')
            await page.goto(base + '/admin/operations')
            await expect(page.get_by_role('heading', name='Ringkasan metrik', exact=True)).to_be_visible()
            await expect(page.get_by_role('link', name='Periksa daftar email gagal', exact=False)).to_have_count(0)
            await expect(page.locator('.ops-actions')).to_have_count(0)
            await expect(page.get_by_text('Data Operasional per:', exact=False)).to_contain_text('WIB')
            await page.screenshot(path=str(output / f'operations-zero-{width}.png'), full_page=True)
            await enlarged(page, output, f'operations-zero-{width}')
        assert not errors, errors
        results.append(scenario)
        await context.close()
    return {'width': width, 'scenarios': results, 'keyboardOpenAndMenuNavigation': True, 'disclosures': 'Email status meanings initially closed; Enter opens and closes', 'textEnlargement': '200% injected font sizes, email list/detail and Operations', 'contrast': 'rendered text/borders/focus audited', 'cursorDeduplication': True, 'selectedCopyAndButtonsRemoved': True}


async def real_backend(browser, args, output):
    assert urlparse(args.real_api).hostname in ('127.0.0.1', 'localhost')
    session = {'accessToken': os.environ['PHASE58_TOKEN'], 'expiresAt': (datetime.now(timezone.utc) + timedelta(hours=1)).isoformat()}
    jobs = json.loads(os.environ['PHASE58_JOBS'])
    context = await browser.new_context(viewport={'width': 1440, 'height': 900}, timezone_id='Asia/Jakarta')
    await context.add_init_script('sessionStorage.setItem("ticket-online:staff-session", JSON.stringify(' + json.dumps(session) + '))')
    page = await context.new_page()
    page.on('dialog', lambda d: d.accept())
    await auto_confirm(page)
    responses = []

    async def relay(route):
        parsed = urlparse(route.request.url)
        try:
            response = await route.fetch(url=args.real_api + parsed.path + ('?' + parsed.query if parsed.query else ''), max_retries=0, timeout=3000)
        except Exception:
            if os.environ.get('PHASE58_DROP_RESPONSE') != 'true' or route.request.method != 'POST' or not parsed.path.endswith('/retry'): raise
            responses.append({'path': parsed.path, 'method': 'POST', 'status': 0, 'connectionInterrupted': True})
            return await route.abort('failed')
        responses.append({'path': parsed.path, 'method': route.request.method, 'status': response.status})
        if response.status >= 400: print('Real API rejected:', responses[-1], await response.text(), flush=True)
        await route.fulfill(response=response)

    await page.route('**/api/v1/**', relay)
    await page.goto(args.base_url + '/admin/operations')
    await expect(page.get_by_role('heading', name='Ringkasan metrik', exact=True)).to_be_visible()
    await page.screenshot(path=str(output / 'real-operations-before.png'), full_page=True)
    await page.goto(args.base_url + '/admin/issues')
    await expect(page.get_by_text('Alamat penerima belum tersedia', exact=True).first).to_be_visible()
    await page.screenshot(path=str(output / 'real-list-before.png'), full_page=True)
    for i, job in enumerate(jobs):
        await page.locator('#email-' + job).click()
        await page.get_by_role('button', name='Kirim ulang email', exact=True).click()
        if os.environ.get('PHASE58_DROP_RESPONSE') == 'true':
            verify = page.get_by_role('button', name='Periksa hasil tindakan', exact=True)
            try: await expect(verify).to_be_visible(timeout=10000)
            except AssertionError:
                print('Real retry detail:', responses, await page.locator('#issues-content').inner_text(), flush=True)
                raise
            await expect(page.get_by_role('button', name='Kembali ke daftar', exact=True)).to_have_count(0)
            await verify.click()
        await expect(page.get_by_text('Pengiriman ulang dijadwalkan.', exact=False).first).to_be_visible()
        await expect(page.get_by_text('Menunggu pengiriman', exact=True)).to_be_visible()
        await page.screenshot(path=str(output / f'real-queued-{i}.png'), full_page=True)
        await page.get_by_role('link', name='Masalah', exact=True).click()
    Path(os.environ['PHASE58_WORKER_SIGNAL']).touch()
    # Backend test starts the real worker and captures SMTP only after both queue states are recorded.
    for job in jobs:
        for _ in range(150):
            response = await context.request.get(args.real_api + '/api/v1/admin/email-jobs/' + job, headers={'Authorization': 'Bearer ' + session['accessToken']})
            detail = await response.json()
            if detail.get('retryJobId') and detail['retryJobId'] != job:
                response = await context.request.get(args.real_api + '/api/v1/admin/email-jobs/' + detail['retryJobId'], headers={'Authorization': 'Bearer ' + session['accessToken']})
                detail = await response.json()
            if detail['status'] == 'SENT': break
            await asyncio.sleep(.1)
        assert detail['status'] == 'SENT', detail
    await page.reload()
    for job in jobs: await expect(page.locator('#email-' + job)).to_have_count(0)
    await page.screenshot(path=str(output / 'real-list-after.png'), full_page=True)
    for _ in range(100):
        response = await context.request.get(args.real_api + '/api/v1/admin/operations', headers={'Authorization': 'Bearer ' + session['accessToken']})
        if (await response.json())['failedEmailJobs'] == 0: break
        await asyncio.sleep(.1)
    await page.goto(args.base_url + '/admin/operations')
    await expect(page.locator('.ops-metrics dt:has-text("Email gagal dalam cakupan pemantauan") + dd')).to_have_text('0')
    await page.screenshot(path=str(output / 'real-operations-after.png'), full_page=True)
    assert all(r['status'] < 400 for r in responses), responses
    assert len([r for r in responses if r['method'] == 'POST']) == 2, responses
    if os.environ.get('PHASE58_DROP_RESPONSE') == 'true': assert len([r for r in responses if r['status'] == 0]) == 2, responses
    (output / 'real-browser.json').write_text(json.dumps({'realBackend': True, 'transport': 'Playwright relay to isolated Go HTTP server; no API fixtures', 'responses': responses, 'humanUAT': 'not performed'}, indent=2))
    await context.close()


async def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--base-url', default='http://localhost:5173')
    parser.add_argument('--real-api')
    parser.add_argument('--output-dir', type=Path, default=Path('/tmp/phase56-58/email'))
    args = parser.parse_args()
    args.output_dir.mkdir(parents=True, exist_ok=True)
    async with async_playwright() as p:
        browser = await p.chromium.launch(channel='chrome', headless=True)
        if args.real_api:
            await real_backend(browser, args, args.output_dir)
        else:
            results = [await fixtures(browser, args.base_url, args.output_dir, width) for width in (320, 390, 768, 1440)]
            (args.output_dir / 'fixtures.json').write_text(json.dumps(results, indent=2))
        await browser.close()
    print('PASS: email workflow browser checks')


if __name__ == '__main__':
    asyncio.run(main())
