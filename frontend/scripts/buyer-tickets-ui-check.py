"""Phase 64-68 browser fixtures, separate from real sandbox transactions and human UAT.
Run from frontend/: python3 scripts/buyer-tickets-ui-check.py [base_url]
"""
import asyncio
import copy
import json
import re
import sys
from datetime import datetime, timedelta, timezone
from pathlib import Path
from urllib.parse import urlparse
from playwright.async_api import async_playwright, expect

BASE = (sys.argv[1] if len(sys.argv) > 1 else 'http://127.0.0.1:5175').rstrip('/')
OUTPUT = Path(__file__).resolve().parents[2] / 'docs' / 'phase64-68'


def data():
    now = datetime.now(timezone.utc)
    date = lambda delta: (now + delta).isoformat()
    orders, tickets, access = {}, {}, []
    scenarios = [('single', 'PAID', [True, False]), ('multiple', 'PAID', [True, True]), ('pending', 'PENDING', []), ('confirming', 'PENDING', []), ('deadline', 'PENDING', []), ('expired', 'EXPIRED', []), ('cancelled', 'CANCELLED', []), ('refund', 'REFUND_PENDING', [False]), ('refunded', 'REFUNDED', [False]), ('postponed', 'PAID', [False]), ('partial', 'PAID', [True]), ('unknown', 'PAID', [None]), ('failed', 'PAID', []), ('access-ended', 'PAID', []), ('old-response', 'PENDING', [])]
    for index, (name, status, usable) in enumerate(scenarios):
        identity = f'{index + 1:032x}'
        current = dict(id='same-concert', status='SCHEDULED', version=0, salesPaused=False, startsAt=date(timedelta(days=10 + index)), announcement='', refundDeadline=None)
        item = dict(id=identity, reference='TO-' + f'{index + 1:020x}', reservationId='a' * 32, status=status, expiresAt=date(timedelta(minutes=20)), accessExpiresAt=date(timedelta(days=12)), subtotal=200000, adminFee=7500, discount=0, total=207500, items=[dict(tierId='festival', name='Festival', quantity=len(usable) or 2, unitPrice=100000, lineTotal=200000)], buyer=dict(name='Ayu Pertiwi', email='ayu@example.test', phone='081234567890', identity='123456789012'), attendees=[], payment=None, createdAt=date(timedelta(days=-index)), updatedAt=date(timedelta()), eventId='same-concert', eventArtist='Nusa Malam' if name in ('single', 'multiple') else 'Konser ' + name, eventVenue='Istora Senayan', eventCity='Jakarta', eventStartsAt=current['startsAt'], currentEvent=current, refund=None, refundRight=None)
        if name == 'single': current['startsAt'] = date(timedelta(days=-1))
        if name == 'confirming': item['payment'] = dict(id='pay', method='QRIS', status='SUCCEEDED', amount=item['total'])
        if name == 'deadline': item['expiresAt'] = date(timedelta(minutes=-1))
        if name in ('refund', 'refunded'): item['refund'] = dict(status='PROCESSING' if name == 'refund' else 'SUCCEEDED', amount=item['total'])
        if name == 'postponed': current.update(status='POSTPONED', startsAt=None, salesPaused=True)
        if name == 'expired': current.update(status='RESCHEDULED', version=1)
        if name == 'partial': item['items'][0]['quantity'] = 2
        if name == 'old-response':
            for field in ('eventId', 'eventArtist', 'eventVenue', 'eventCity', 'currentEvent'): item.pop(field)
        orders[name] = item
        tickets[name] = [dict(id=f'{index + 100:030x}{n:02x}', code='ET-' + f'{index + n + 1:032X}', attendeeName=['Ayu Pertiwi', 'Dimas Pratama'][n], tierName='Festival' if n == 0 else 'Tribune', gate='Gate A', issuedAt=item['createdAt'], orderReference=item['reference'], eventId=item['eventId'], eventArtist=item['eventArtist'], eventCity='Jakarta', eventVenue=item['eventVenue'], eventAddress='Jl. Pintu Satu', eventStartsAt=item['eventStartsAt'], currentEvent=current, **({'usable': value} if value is not None else {})) for n, value in enumerate(usable)]
        access.append(dict(orderId=identity, reference=item['reference'], reservationId=item['reservationId'], expiresAt=item['expiresAt'], accessExpiresAt=item['accessExpiresAt'], accessToken='fixture-access', ticketIds=[]))
    return orders, tickets, access


async def audit(page):
    assert await page.evaluate('document.documentElement.scrollWidth <= innerWidth'), 'Horizontal overflow'
    text = await page.locator('main').inner_text()
    assert not re.search(r'\b(snapshot|backend|browser|reference)\b', text, re.I), text
    assert not re.search('[—–]', text), text
    assert not await page.locator('main button:visible, main a:visible, main summary:visible').evaluate_all('els => els.some(el => el.getBoundingClientRect().right > innerWidth + 1)'), 'Action outside viewport'


async def main():
    OUTPUT.mkdir(parents=True, exist_ok=True)
    results, errors = [], []
    (OUTPUT / "automated.json").write_text(json.dumps(dict(status="running", baseUrl=BASE)) + "\n")
    async with async_playwright() as p:
        browser = await p.chromium.launch(channel='chrome', headless=True)
        for width in (320, 390, 768, 1440):
            orders, tickets, accesses = data()
            state = dict(fail=True, posts=[], uncertain=False, offline=False)
            context = await browser.new_context(viewport=dict(width=width, height=900), timezone_id='Asia/Jakarta', reduced_motion='reduce')
            await context.add_init_script('''(records => {
              if (!sessionStorage.getItem('fixture-seeded')) {
                for (const record of records) localStorage.setItem('ticket-online:order:' + record.orderId, JSON.stringify(record));
                localStorage.setItem('ticket-online:ticket:demo', JSON.stringify({ version: 1 }));
                sessionStorage.setItem('fixture-seeded', 'yes');
              }
            })(''' + json.dumps(accesses) + ')')
            async def api(route):
                path = urlparse(route.request.url).path
                if route.request.method == 'POST':
                    state['posts'].append(dict(path=path, body=route.request.post_data_json, authorization=route.request.headers.get('authorization')))
                    assert path.endswith('/payments') or path in ('/api/v1/ticket-recovery', '/api/v1/ticket-recovery/verify'), path
                    if path.endswith('/payments'):
                        await asyncio.sleep(.2)
                        if state['uncertain']: return await route.abort('failed')
                        return await route.fulfill(json=dict(redirectUrl='https://app.sandbox.midtrans.com/snap/v2/fixture'))
                    if path.endswith('/verify'):
                        item = orders['expired' if route.request.post_data_json['token'] == 'e' * 43 else 'pending']
                        return await route.fulfill(json=dict(orderId=item['id'], reference=item['reference'], reservationId=item['reservationId'], expiresAt=item['expiresAt'], accessExpiresAt=item['accessExpiresAt'], accessToken='a' * 43, ticketIds=[] if item['status'] == 'EXPIRED' else [item['id']], changedEvent=item['currentEvent']['version'] > 0))
                    return await route.fulfill(status=202, json=dict(message='Jika data cocok, tautan pemulihan dikirim ke email pembeli.'))
                if path.startswith('/api/v1/orders/'):
                    if state['offline']: return await route.abort('internetdisconnected')
                    identity = path.split('/')[4]
                    name = next(name for name, item in orders.items() if item['id'] == identity)
                    if name == 'access-ended': return await route.fulfill(status=410, json=dict(error=dict(code='ACCESS_TOKEN_EXPIRED', message='technical token diagnostic')))
                    if name == 'failed' and state['fail']: return await route.fulfill(status=503, json=dict(error=dict(code='INTERNAL_ERROR', message='technical service diagnostic')))
                    if path.endswith('/tickets'): return await route.fulfill(json=dict(tickets=tickets[name]))
                    return await route.fulfill(json=orders[name])
                if path.startswith('/api/v1/tickets/'):
                    item = next(ticket for group in tickets.values() for ticket in group if ticket['id'] == path.split('/')[-1])
                    return await route.fulfill(json=item)
                raise AssertionError('Unexpected request: ' + path)
            await context.route('**/api/v1/**', api)
            await context.route('https://app.sandbox.midtrans.com/**', lambda route: route.fulfill(body='Sandbox redirect fixture'))
            page = await context.new_page()
            page.on('pageerror', lambda error: errors.append(str(error)))
            await page.goto(BASE + '/tiket-saya')
            active = page.get_by_role('region', name='Aktif', exact=True)
            review = page.get_by_role('region', name='Perlu diperiksa', exact=True)
            await expect(active.locator('article')).to_have_count(2)
            await expect(active.locator('.my-ticket-mark')).to_have_text(['E-ticket', 'E-ticket'])
            await expect(review.locator('article')).to_have_count(4)
            await expect(page.get_by_role('button', name='Aktif 2', exact=True)).to_have_attribute('aria-pressed', 'true')
            await expect(active.get_by_text('2 tiket dipesan · 1 dapat digunakan', exact=True)).to_be_visible()
            await audit(page)
            await page.screenshot(path=str(OUTPUT / f'active-{width}.png'), full_page=True)
            await page.emulate_media(color_scheme='dark')
            await audit(page)
            await page.screenshot(path=str(OUTPUT / f'active-dark-{width}.png'), full_page=True)
            await page.emulate_media(color_scheme='light')
            await page.get_by_role('button', name='Belum dibayar 4', exact=True).focus()
            await page.keyboard.press('Space')
            await expect(page.get_by_role('button', name='Belum dibayar 4', exact=True)).to_have_attribute('aria-pressed', 'true')
            await page.get_by_role('button', name='Aktif 2', exact=True).click()

            await expect(active.locator('a.button').first).to_have_attribute('href', '/tiket/' + tickets['single'][0]['id'])
            await active.locator('summary').click()
            await expect(active.get_by_role('link', name='Dimas Pratama · Tribune')).to_have_attribute('href', '/tiket/' + tickets['multiple'][1]['id'])
            await active.get_by_role('link', name='Dimas Pratama · Tribune').click()
            await expect(page.locator('.pass-grid')).to_contain_text('Dimas Pratama')
            await page.goto(BASE + '/tiket-saya')
            await page.get_by_role('button', name='Belum dibayar 4', exact=True).click()
            pending = page.get_by_role('region', name='Belum dibayar', exact=True)
            await expect(pending.locator('article')).to_have_count(4)
            await expect(pending.locator('.my-ticket-mark')).to_have_text(['Menunggu pembayaran'] * 4)
            await expect(pending.get_by_role('button', name='Lanjutkan pembayaran', exact=True)).to_have_count(2)
            await expect(pending.get_by_role('button', name='Periksa status pembayaran', exact=True)).to_have_count(2)
            await expect(pending.get_by_text('Informasi konser belum tersedia', exact=True)).to_be_visible()
            await audit(page)
            await page.screenshot(path=str(OUTPUT / f'pending-{width}.png'), full_page=True)
            await page.get_by_role('button', name='Riwayat 5', exact=True).click()
            history = page.get_by_role('region', name='Riwayat', exact=True)
            await expect(history.locator('.my-ticket-status').filter(has_text='Pesanan kedaluwarsa')).to_be_visible()
            await expect(history.locator('article').filter(has_text='Konser expired').locator('.my-ticket-mark')).to_have_text('Pesanan kedaluwarsa')
            await expect(history.locator('article').filter(has_text='Konser cancelled').locator('.my-ticket-mark')).to_have_text('Pesanan dibatalkan')
            await expect(review.locator('article').filter(has_text=orders['failed']['reference']).locator('.my-ticket-mark')).to_have_text('Pesanan')
            await expect(history.get_by_text('Batas waktu pembayaran telah lewat.', exact=True)).to_be_visible()
            await expect(history.get_by_text('Jadwal belum diumumkan', exact=True)).to_be_visible()
            await audit(page)
            await page.screenshot(path=str(OUTPUT / f'history-{width}.png'), full_page=True)
            state['fail'] = False
            await review.locator('article').filter(has_text='0000000000000000000d').get_by_role('button', name='Coba lagi').click()
            await expect(review.locator('article')).to_have_count(4)
            await page.get_by_role('button', name='Belum dibayar 4', exact=True).click()
            # A late payment confirmation during the click moves categories without starting a payment.
            orders['pending']['status'] = 'PAID'
            tickets['pending'] = copy.deepcopy(tickets['multiple'])
            await pending.locator('article').filter(has_text='Konser pending').get_by_role('button', name='Lanjutkan pembayaran').click()
            await expect(page.get_by_role('button', name='Aktif 3', exact=True)).to_be_visible()
            assert not state['posts']
            orders['pending']['status'] = 'PENDING'
            tickets['pending'] = []
            await page.goto(BASE + '/tiket-saya')
            await page.get_by_role('button', name='Belum dibayar 4', exact=True).click()
            await pending.locator('article').filter(has_text='Konser pending').get_by_role('button', name='Lanjutkan pembayaran').click()
            await expect(page).to_have_url(BASE + '/pesanan/' + orders['pending']['id'] + '#pembayaran')
            await page.locator('#order-payment-method').select_option('GOPAY')
            await page.locator('#pembayaran form').evaluate('(form) => { form.requestSubmit(); form.requestSubmit(); }')
            await expect(page).to_have_url('https://app.sandbox.midtrans.com/snap/v2/fixture')
            assert len(state['posts']) == 1, state
            post = state['posts'][0]
            assert post['path'] == '/api/v1/orders/' + orders['pending']['id'] + '/payments'
            assert post['body'] == dict(method='GOPAY', returnUrl=BASE + '/pesanan/' + orders['pending']['id'])
            assert post['authorization'] == 'Bearer fixture-access'
            orders['pending']['payment'] = dict(id='existing-payment', method='GOPAY', status='PENDING', amount=orders['pending']['total'])
            await page.goto(BASE + '/pesanan/' + orders['pending']['id'] + '#pembayaran')
            await expect(page.locator('#order-payment-method')).to_have_value('GOPAY')
            await expect(page.locator('#order-payment-method')).to_be_disabled()
            await expect(page.locator('#pembayaran').get_by_role('button', name='Lanjutkan pembayaran')).to_be_enabled()
            orders['pending']['payment'] = None
            state['uncertain'] = True
            await page.reload()
            await page.locator('#order-payment-method').select_option('QRIS')
            await page.locator('#pembayaran').get_by_role('button', name='Lanjutkan pembayaran').click()
            await expect(page.get_by_text('Hasil pembayaran belum dapat dipastikan. Periksa status pembayaran sebelum mencoba lagi.', exact=True)).to_be_visible()
            assert len(state['posts']) == 2
            await audit(page)
            await page.screenshot(path=str(OUTPUT / f'payment-uncertain-{width}.png'), full_page=True)
            orders['pending']['status'] = 'PAID'
            tickets['pending'] = copy.deepcopy(tickets['multiple'])
            await page.goto(BASE + '/tiket-saya')
            await expect(page.get_by_role('button', name='Aktif 3', exact=True)).to_be_visible()
            for item in tickets['pending']: item['usable'] = False
            await active.get_by_role('button', name='Periksa lagi').click()
            await expect(page.get_by_role('button', name='Aktif 2', exact=True)).to_be_visible()
            await expect(page.get_by_role('button', name='Riwayat 6', exact=True)).to_be_visible()
            await page.goto(BASE + '/pesanan/' + orders['deadline']['id'] + '#pembayaran')
            await expect(page.locator('#pembayaran').get_by_role('button', name='Periksa status pembayaran')).to_be_visible()
            await expect(page.locator('#order-payment-method')).to_have_count(0)
            await page.evaluate('localStorage.clear(); sessionStorage.clear(); sessionStorage.setItem("fixture-seeded", "yes")')
            await page.goto(BASE + '/tiket-saya')
            await expect(page.get_by_text('Belum ada tiket aktif.', exact=True)).to_be_visible()
            await expect(page.get_by_role('link', name='Cari konser', exact=True)).to_be_visible()
            await expect(page.get_by_role('link', name='Sudah membeli, tetapi tiket tidak muncul?')).to_be_visible()
            await page.screenshot(path=str(OUTPUT / f'empty-{width}.png'), full_page=True)
            await page.goto(BASE + '/pulihkan-tiket#token=' + 'a' * 43)
            await page.get_by_role('button', name='Pulihkan tiket', exact=True).click()
            await expect(page).to_have_url(BASE + '/pesanan/' + orders['pending']['id'])
            restored = await page.evaluate('(id) => JSON.parse(localStorage.getItem("ticket-online:order:" + id))', orders['pending']['id'])
            assert 'basketKey' not in restored and 'idempotencyKey' not in restored
            if width == 320:
                other = await browser.new_context(viewport=dict(width=390, height=900))
                await other.route('**/api/v1/**', api)
                other_page = await other.new_page()
                await other_page.goto(BASE + '/tiket-saya')
                await expect(other_page.get_by_text('Belum ada tiket aktif.', exact=True)).to_be_visible()
                await other_page.goto(BASE + '/pulihkan-tiket#token=' + 'c' * 43)
                await other_page.get_by_role('button', name='Pulihkan tiket', exact=True).click()
                await expect(other_page).to_have_url(BASE + '/pesanan/' + orders['pending']['id'])
                await other_page.goto(BASE + '/pulihkan-tiket#token=' + 'e' * 43)
                await expect(other_page.locator('.recovery-access-note')).to_contain_text('Pesanan kedaluwarsa tetap kedaluwarsa')
                await other_page.get_by_role('button', name='Pulihkan tiket', exact=True).click()
                await expect(other_page).to_have_url(BASE + '/pesanan/' + orders['expired']['id'])
                await expect(other_page.locator('.order-status')).to_have_text('Kedaluwarsa')
                await expect(other_page.get_by_role('link', name='Buka e-ticket')).to_have_count(0)
                await expect(other_page.locator('#pembayaran')).to_have_count(0)
                assert orders['expired']['status'] == 'EXPIRED'
                assert sum(post['path'].endswith('/payments') for post in state['posts']) == 2
                await other.close()
            await page.add_init_script('Storage.prototype.setItem = () => { throw new DOMException("denied", "SecurityError"); }')
            await page.goto(BASE + '/tiket-saya')
            await expect(page.get_by_text('Akses pesanan belum tersimpan di perangkat ini.', exact=False)).to_be_visible()
            await expect(active.get_by_role('link', name='Buka e-ticket', exact=True)).to_have_count(0)
            await audit(page)
            await page.screenshot(path=str(OUTPUT / f'storage-denied-{width}.png'), full_page=True)
            state['offline'] = True
            await page.reload()
            await expect(page.get_by_text('Sebagian status tiket belum dapat dipastikan. Periksa pesanan di bagian Perlu diperiksa.', exact=True)).to_be_visible()
            await audit(page)
            await page.screenshot(path=str(OUTPUT / f'offline-{width}.png'), full_page=True)
            await page.goto(BASE + '/pulihkan-tiket')
            await expect(page.get_by_text('Nomor pesanan tertera pada judul email dan ringkasan di awal email.', exact=False)).to_be_visible()
            await expect(page.locator('.recovery-access-note')).to_contain_text('Pemulihan hanya mengembalikan akses pesanan.')
            boxes = [await page.locator('#recovery-' + field).bounding_box() for field in ('email', 'reference')]
            assert all(abs(box['height'] - 48) < 1 for box in boxes), boxes
            if width >= 768: assert abs(boxes[0]['y'] - boxes[1]['y']) < 1, boxes
            for field, box in zip(('email', 'reference'), boxes):
                hint = await page.locator('#recovery-' + field + '-hint').bounding_box()
                assert hint['y'] >= box['y'] + box['height'], hint
            labels = await page.locator('form label').evaluate_all('els => els.map(el => el.getBoundingClientRect().bottom)')
            button = await page.get_by_role('button', name='Kirim tautan pemulihan').bounding_box()
            assert abs(button['y'] - max(labels) - 12) < 1, (button, labels)
            await page.emulate_media(color_scheme='dark')
            await audit(page)
            await page.screenshot(path=str(OUTPUT / f'recovery-dark-{width}.png'), full_page=True)
            await page.emulate_media(color_scheme='light')
            await page.get_by_role('button', name='Kirim tautan pemulihan').click()
            await expect(page.locator('#recovery-email')).to_be_focused()
            await audit(page)
            await page.screenshot(path=str(OUTPUT / f'recovery-{width}.png'), full_page=True)
            await page.locator('#recovery-email').fill('ayu@example.test')
            await page.locator('#recovery-reference').fill(orders['expired']['reference'])
            await page.get_by_role('button', name='Kirim tautan pemulihan').click()
            await expect(page.locator('#recovery-requested')).to_be_focused()
            await expect(page.locator('.recovery-access-note')).to_contain_text('Pesanan kedaluwarsa tetap kedaluwarsa')
            state['offline'] = False
            await page.goto(BASE + '/tiket-saya')
            await page.goto(BASE + '/pulihkan-tiket#token=' + 'b' * 43)
            await page.get_by_role('button', name='Pulihkan tiket', exact=True).click()
            await expect(page.get_by_role('heading', name='Akses belum tersimpan di perangkat ini.')).to_be_visible()
            await audit(page)
            await page.screenshot(path=str(OUTPUT / f'recovery-unsaved-{width}.png'), full_page=True)
            results.append(dict(width=width, passed=True, paymentPosts=2, screenshots='fixture only'))
            await context.close()
        await browser.close()
    assert not errors, errors
    (OUTPUT / 'automated.json').write_text(json.dumps(dict(status='passed', generatedAt=datetime.now(timezone.utc).isoformat(), baseUrl=BASE, results=results, pageErrors=errors, realSandbox='not tested', humanUAT='pending'), indent=2) + '\n')
    print(json.dumps(results, indent=2))


asyncio.run(main())
