"""Phase 59–63 buyer checks with eight explicit API fixtures, never live transactions.

Run from frontend/: python3 scripts/discovery-ui-check.py [base_url]
Uses the already installed Python Playwright and Chrome.
"""
import asyncio
import copy
import json
import runpy
import sys
from datetime import datetime, timezone
from pathlib import Path
from urllib.parse import quote, urlparse

from playwright.async_api import async_playwright, expect

ROOT = Path(__file__).resolve().parents[2]
OUTPUT = ROOT / 'docs' / 'phase59-63'
BASE_URL = (sys.argv[1] if len(sys.argv) > 1 else 'http://127.0.0.1:5173').rstrip('/')
PUBLIC = runpy.run_path(str(Path(__file__).with_name('public-ui-check.py')))


def fixtures():
    events = []
    for i in range(8):
        item = copy.deepcopy(PUBLIC['event']())
        item.update(id=f'fixture-{i}', artist=f'Konser Uji {i + 1}', city='Jakarta' if i % 2 == 0 else 'Bandung', startsAt=f'2027-08-{24 + i:02d}T19:00:00+07:00')
        width, height = [(400, 500), (1200, 300), (300, 1600)][i % 3]
        # Diagnostic images for layout checks only; these are not licensed concert posters.
        svg = f'<svg xmlns="http://www.w3.org/2000/svg" width="{width}" height="{height}"><rect width="100%" height="100%" fill="#1358d8"/><text x="10" y="24" fill="white" font-size="16">POSTER UJI {i + 1}</text></svg>'
        item['image'] = 'data:image/svg+xml,' + quote(svg)
        item['price'] = 100000 + 25000 * i
        for tier in item['ticketTiers']:
            tier['price'] = item['price']
        events.append(item)
    events[0]['artist'] = 'Konser dengan nama panjang untuk membandingkan harga dan jadwal'
    events[2]['status'] = 'Sold Out'
    for tier in events[2]['ticketTiers']:
        tier['availableQuantity'] = 0
    events[3]['image'] = '/missing-official-poster.jpg'
    events[4]['currentEvent'] = dict(PUBLIC['event']('RESCHEDULED')['currentEvent'], startsAt='2027-08-23T17:00:00Z')
    events[5]['currentEvent'] = dict(PUBLIC['event']('POSTPONED')['currentEvent'], startsAt=None)
    events[6]['currentEvent'] = PUBLIC['event']('CANCELLED')['currentEvent']
    events[7]['currentEvent'] = dict(id='fixture-7', status='SCHEDULED', version=1, salesPaused=True, startsAt=events[7]['startsAt'], announcement='', refundDeadline=None)
    return events


async def main():
    OUTPUT.mkdir(parents=True, exist_ok=True)
    events = fixtures()
    results = []
    errors = []
    scenario = 'standard'
    async with async_playwright() as p:
        browser = await p.chromium.launch(channel='chrome', headless=True)
        context = await browser.new_context(viewport={'width':1440, 'height':900}, reduced_motion='reduce')
        async def api(route):
            path = urlparse(route.request.url)
            if path.path == '/api/v1/events':
                if scenario == 'loading':
                    await asyncio.sleep(.6)
                if scenario == 'error':
                    await route.fulfill(status=503, json={'error':{'message':'Katalog gagal dimuat.'}})
                else:
                    await route.fulfill(json={'events':[] if scenario == 'empty' else events})
            elif path.path.startswith('/api/v1/events/fixture-'):
                await route.fulfill(json=next(e for e in events if e['id'] == path.path.split('/')[-1]))
            else:
                status, body = PUBLIC['reply'](path, 'standard')
                await route.fulfill(status=status, json=body)
        await context.route('**/api/v1/**', api)
        await context.route('https://picsum.photos/**', lambda route: route.abort())
        page = await context.new_page()
        page.on('pageerror', lambda error: errors.append(str(error)))
        await page.goto(BASE_URL)
        await expect(page.locator('.concert-card')).to_have_count(4)
        await expect(page.locator('.results-count')).to_have_text('Menampilkan 4 dari 8 konser')
        for selector in ['#event-query', '.concert-card h3', '.concert-price']:
            box = await page.locator(selector).first.bounding_box()
            assert box and box['y'] + box['height'] <= 900, (selector, box)
        await page.get_by_role('button', name='Muat lebih banyak').click()
        await expect(page.locator('.concert-card')).to_have_count(8)
        await expect(page.locator('.results-count')).to_have_text('Menampilkan 8 dari 8 konser')
        assert await page.evaluate("document.activeElement.closest('#home-fixture-4') !== null")
        for word in ['Habis', 'Acara ditunda', 'Acara dibatalkan', 'Penjualan dihentikan', 'Jadwal belum diumumkan']:
            assert await page.get_by_text(word, exact=True).count() >= 1, word
        await expect(page.locator('.poster-fallback')).to_have_text('Poster belum tersedia')
        await page.locator('#event-query').fill('Ruang')
        await expect(page.locator('.concert-card')).to_have_count(8)
        await expect(page.locator('.reset-filters')).to_have_count(0)
        await page.get_by_role('button', name='Cari', exact=True).click()
        await expect(page.locator('.concert-card')).to_have_count(4)
        await page.locator('#event-city').select_option('Jakarta')
        await page.locator('[name=dateFrom]').fill('2027-08-24')
        await page.locator('[name=dateTo]').fill('2027-08-24')
        await expect(page.locator('.concert-card')).to_have_count(4)
        await page.get_by_role('button', name='Cari', exact=True).click()
        await expect(page.locator('.concert-card')).to_have_count(2)
        assert await page.locator('.concert-card-link').evaluate_all("items => items.map(e=>e.pathname)") == ['/konser/fixture-0', '/konser/fixture-4']
        await page.locator('[name=dateTo]').fill('2027-08-23')
        await expect(page.get_by_text('Tanggal akhir tidak boleh sebelum tanggal mulai.', exact=True)).to_be_visible()
        await expect(page.locator('[name=dateTo]')).to_have_attribute('aria-invalid', 'true')
        await page.get_by_role('button', name='Cari', exact=True).click()
        await expect(page.locator('.concert-card')).to_have_count(2)
        await page.get_by_role('button', name='Reset filter', exact=True).first.click()
        await expect(page.locator('.concert-card')).to_have_count(4)
        results.append('Home: all eight results, focus on newly loaded result, reset to four, AND filters, latest WIB date, reversed range, unavailable states and poster fallback.')

        for route, query in [('/', '#event-query'), ('/konser', '#concert-query')]:
            for scenario, message in [('loading', 'Memuat'), ('empty', 'Belum ada konser di katalog.'), ('error', 'Katalog gagal dimuat.')]:
                await page.goto(BASE_URL + route)
                await expect(page.get_by_text(message, exact=scenario != 'loading').first).to_be_visible()
            scenario = 'standard'
            await page.goto(BASE_URL + route)
            await page.locator(query).fill('Tidak cocok')
            await page.locator(query).press('Enter')
            await expect(page.get_by_text('Tidak ada konser yang cocok dengan filtermu.', exact=True)).to_be_visible()
        results.append('Home and catalog: loading, API failure, empty catalog and no matching results are distinct.')

        await page.goto(BASE_URL + '/konser')
        await page.get_by_role('button', name='Cari', exact=True).click()
        await page.get_by_role('button', name='Reset filter', exact=True).first.click()
        await page.locator('#concert-city').select_option('Jakarta')
        await page.locator('[name=dateFrom]').fill('2027-08-24')
        await page.locator('[name=dateTo]').fill('2027-08-24')
        await expect(page.locator('.concert-card')).to_have_count(8)
        await page.get_by_role('button', name='Cari', exact=True).click()
        await expect(page.locator('.concert-card')).to_have_count(2)
        await page.evaluate('window.scrollTo(0, 500)')
        await page.wait_for_function('scrollY >= 400')
        # Disable BFCache to exercise session restoration after a fresh document load.
        await page.evaluate("addEventListener('unload',()=>{})")
        detail_path = await page.locator('.concert-card-link').first.get_attribute('href')
        await page.locator('.concert-card-link').first.click()
        await page.wait_for_url('**' + detail_path)
        saved = await page.evaluate("JSON.parse(sessionStorage.getItem('ticket-online:catalog:v1'))")
        await page.go_back()
        await expect(page.locator('.concert-card')).to_have_count(2)
        await expect(page.locator('[name=dateFrom]')).to_have_value('2027-08-24')
        await expect(page.locator('[name=dateTo]')).to_have_value('2027-08-24')
        await expect(page.locator('#concert-city')).to_have_value('Jakarta')
        await page.wait_for_function('y => Math.abs(scrollY-y)<5', arg=saved['scrollY'])
        await expect(page.locator('.search-actions .reset-filters')).to_be_visible()
        # Unsaved edits must not replace the applied session when navigating away.
        await page.locator('#concert-query').fill('Belum diterapkan')
        await page.locator('[name=dateTo]').fill('2027-08-23')
        await page.get_by_role('button', name='Cari', exact=True).click()
        await expect(page.locator('.concert-card')).to_have_count(2)
        await page.locator('.concert-card-link').first.click()
        await page.go_back()
        await expect(page.locator('#concert-query')).to_have_value('')
        await expect(page.locator('[name=dateTo]')).to_have_value('2027-08-24')
        await page.get_by_role('button', name='Reset filter', exact=True).first.click()
        await expect(page.locator('.concert-card')).to_have_count(8)
        results.append('Catalog: date/city filters and list scroll survive returning from detail without BFCache.')

        await page.locator('#concert-sort').select_option('price')
        await expect(page.locator('#concert-price')).to_be_disabled()
        budget_before = await page.locator('#concert-price').bounding_box()
        await page.get_by_label('Batasi harga').check()
        budget_after = await page.locator('#concert-price').bounding_box()
        assert budget_before == budget_after, (budget_before, budget_after)
        aligned = await page.locator('#concert-sort, #concert-city, .filter-dates input, #concert-price').evaluate_all('es=>es.map(e=>({x:e.getBoundingClientRect().x,y:e.getBoundingClientRect().y,width:e.getBoundingClientRect().width}))')
        assert abs(aligned[0]['y']-aligned[1]['y']) < 1, aligned
        assert abs(aligned[2]['y']-aligned[3]['y']) < 1 and abs(aligned[3]['y']-aligned[4]['y']) < 1, aligned
        assert abs(aligned[0]['x']-aligned[2]['x']) < 1 and abs(aligned[1]['x']-aligned[4]['x']) < 1, aligned
        await page.emulate_media(color_scheme='dark', reduced_motion='reduce')
        await page.locator('.catalog-form').screenshot(path='/tmp/catalog-filters-layout.png')
        slider = page.get_by_role('slider', name='Anggaran maksimum')
        await expect(slider).to_have_attribute('max', '300000')
        await slider.click(position={'x':budget_after['width']/2, 'y':budget_after['height']/2})
        assert 0 < int(await slider.input_value()) < 300000
        await slider.press('Home')
        await expect(slider).to_have_value('0')
        await expect(page.locator('#concert-price-value')).to_have_text('Sampai Rp 0')
        await slider.press('End')
        await expect(slider).to_have_value('300000')
        await slider.press('ArrowLeft')
        await expect(slider).to_have_value('250000')
        await slider.press('Home')
        await slider.press('ArrowRight')
        await expect(slider).to_have_value('50000')
        await slider.press('ArrowRight')
        await expect(slider).to_have_value('100000')
        await expect(slider).to_have_attribute('step', '50000')
        await expect(page.locator('.concert-card')).to_have_count(8)
        await page.get_by_role('button', name='Cari', exact=True).click()
        await expect(page.locator('.concert-card')).to_have_count(1)
        await page.reload()
        await expect(slider).to_have_value('100000')
        await expect(page.locator('.concert-card')).to_have_count(1)
        await page.locator('.reset-filters').click()
        await expect(page.locator('#concert-sort')).to_have_value('date')
        await expect(page.get_by_label('Batasi harga')).not_to_be_checked()
        await expect(page.locator('.reset-filters')).to_have_count(0)
        await expect(slider).to_be_disabled()
        results.append('Explicit submit; invalid dates preserve results; native price slider supports pointer, keyboard, exact inclusive limits and session restoration; reset clears filters and disables slider.')

        for width in [320, 390, 768, 1024, 1366, 1440]:
            for color in ['light', 'dark']:
                for text_scale in [1, 2]:
                    await page.set_viewport_size({'width':width, 'height':900})
                    await page.emulate_media(color_scheme=color, reduced_motion='reduce')
                    for path in ['/', '/konser']:
                        await page.evaluate("sessionStorage.removeItem('ticket-online:catalog:v1')")
                        await page.goto(BASE_URL + path)
                        await page.locator('.concert-card').first.wait_for()
                        if text_scale == 2:
                            await page.evaluate("""() => {const items=[...document.querySelectorAll('.public-site *')].map(e=>[e,parseFloat(getComputedStyle(e).fontSize),parseFloat(getComputedStyle(e).lineHeight)]);for(const[e,size,line]of items){e.style.fontSize=`${size*2}px`;if(Number.isFinite(line))e.style.lineHeight=`${line*2}px`}}""")
                            await page.add_style_tag(content='.public-site .search-actions .button, .public-site .reset-filters { font-size: 28px !important; line-height: 2 !important; }')
                        if width < 768:
                            assert not await page.locator('.filters').evaluate('(e)=>e.open')
                            await page.locator('.filters summary').focus()
                            await page.keyboard.press('Enter')
                        assert await page.locator('.filters').evaluate('(e)=>e.open')
                        if not await page.evaluate('document.documentElement.scrollWidth <= innerWidth'):
                            await page.screenshot(path='/tmp/discovery-overflow.png', full_page=True)
                            overflow = await page.evaluate("""() => [...document.querySelectorAll('body *')].filter(e=>e.getBoundingClientRect().right>innerWidth+1).map(e=>({tag:e.tagName,cls:e.className,text:e.textContent.slice(0,80),width:e.getBoundingClientRect().width})).slice(-20)""")
                            parents = await page.locator('.filter-tools, .filters, .filter-fields, fieldset, .date-inputs').evaluate_all("es=>es.map(e=>({tag:e.tagName,cls:e.className,width:e.getBoundingClientRect().width,min:getComputedStyle(e).minWidth,display:getComputedStyle(e).display}))")
                            raise AssertionError((path, width, color, text_scale, overflow, parents))
                        cards = await page.locator('.concert-card-link').evaluate_all("""items=>items.map(e=>{const r=e.querySelector('.concert-poster-frame').getBoundingClientRect(),img=e.querySelector('img');return{ratio:r.width/r.height,nested:e.querySelectorAll('a,button').length,fit:img?getComputedStyle(img).objectFit:'fallback'}})""")
                        assert all(abs(c['ratio']-.8)<.01 and c['nested']==0 and c['fit'] in ('contain','fallback') for c in cards), cards
                        await page.locator('.concert-card-link').first.focus()
                        assert await page.locator('.concert-card-link').first.evaluate("e=>document.activeElement===e && getComputedStyle(e).outlineStyle !== 'none'")
                        columns = await page.locator('.event-list' if path == '/' else '.concert-grid').evaluate("e=>getComputedStyle(e).gridTemplateColumns.split(' ').length")
                        assert columns == (1 if width < 768 else 2 if width < 1024 else 4)
                        # Both routes use a compact icon and one stable action area.
                        actions = page.locator('.search-actions')
                        await expect(actions.locator('button')).to_have_count(1)
                        initial_width = (await actions.bounding_box())['width']
                        await expect(actions.locator('button')).to_have_css('min-height', '48px')
                        await expect(actions.locator('.search-submit')).to_have_accessible_name('Cari')
                        await expect(actions.locator('.search-submit')).to_have_css('width', '48px')
                        await expect(actions.locator('.search-submit')).to_have_css('height', '48px')
                        await expect(actions.locator('.search-submit svg')).to_be_visible()
                        if path == '/konser' and width >= 768:
                            search_box = await page.locator('#concert-query').bounding_box()
                            button_box = await actions.locator('.search-submit').bounding_box()
                            assert abs(search_box['y']-button_box['y']) < 1 and button_box['x'] >= search_box['x']+search_box['width'], (search_box, button_box)
                        await page.get_by_role('button', name='Cari', exact=True).click()
                        await expect(actions.locator('button')).to_have_count(2)
                        sizes = await actions.locator('button').evaluate_all('es=>es.map(e=>e.getBoundingClientRect().width)')
                        assert abs(sizes[0]-48) < 1 and abs((await actions.bounding_box())['width']-initial_width) < 1, sizes
                        assert await page.evaluate('document.documentElement.scrollWidth <= innerWidth')
                        query = '#event-query' if path == '/' else '#concert-query'
                        await page.locator(query).fill('Tidak cocok')
                        await page.locator(query).press('Enter')
                        await expect(page.locator('.reset-filters')).to_have_count(2)
                        for button in await page.locator('.reset-filters').all():
                            await page.mouse.move(0,0)
                            await expect(button).to_have_css('background-color', 'rgba(0, 0, 0, 0)')
                            accent = await button.evaluate('e=>getComputedStyle(e).color')
                            await button.hover()
                            await expect(button).to_have_css('background-color', accent)
                            await button.focus()
                            assert await button.evaluate("e=>document.activeElement===e && getComputedStyle(e).outlineStyle==='solid'")
                        await page.locator('.reset-filters').last.click()
                        await expect(page.locator('.reset-filters')).to_have_count(0)
                        await expect(actions.locator('button')).to_have_count(1)
                        await expect(page.locator(query)).to_have_value('')
                        assert await page.locator(query).evaluate('e=>document.activeElement===e')
                        if path == '/' and (color == 'light' and text_scale == 1 or width == 320 and text_scale == 2):
                            if width < 768:
                                await page.locator('.filters summary').focus()
                                await page.keyboard.press('Enter')
                            await page.evaluate("window.scrollTo({top:0,behavior:'instant'})")
                            await page.screenshot(path=str(OUTPUT/f'home-{width}-{color}-text{text_scale}.png'), full_page=True, animations='disabled')
                        results.append(f'{path} {width}px {color} text {text_scale*100}%: no page overflow, keyboard focus/disclosure, poster ratio and columns.')
        await page.set_viewport_size({'width':390,'height':844})
        await page.goto(BASE_URL + '/panduan#ketentuan')
        await page.locator('#ketentuan .accordion-trigger').first.click()
        await expect(page.locator('#ketentuan .accordion-panel')).to_be_visible()
        assert await page.locator('.site-footer a[href="/panduan#refund"]').count() == 1
        assert await page.locator('a[href*=".example"]').count() == 0
        await page.goto(BASE_URL + '/checkout/nusa-malam?festival=1')
        for selector, value in [('buyer-name','Nadia Pembeli'),('buyer-email','nadia@example.test'),('buyer-phone','08123456789'),('buyer-identity','1234567890123456'),('attendee-festival-0','Nadia Pembeli')]:
            await page.locator('#'+selector).fill(value)
        await page.get_by_role('button', name='Lanjut ke pembayaran').click()
        await expect(page.get_by_role('heading', name='Pilih metode pembayaran', exact=True)).to_be_visible()
        await page.locator('input[name=payment][value=QRIS]').check()
        await page.get_by_role('button', name='Tinjau pesanan').click()
        assert await page.evaluate("document.activeElement.id === 'confirmation-title'")
        for href in ['/panduan', '/panduan#ketentuan', '/panduan#refund']:
            await expect(page.locator(f'.purchase-help a[href="{href}"]')).to_be_visible()
        await page.get_by_role('button', name='Ketentuan pembelian', exact=True).click()
        await expect(page.locator('.terms-dialog')).to_be_visible()
        await page.get_by_role('button', name='Tutup', exact=True).click()
        results.append('Guide and checkout: shared terms, refund/help before confirmation, dialog keyboard focus retained. API fixtures only; no payment sent.')
        assert not errors, errors
        await browser.close()
    (OUTPUT/'browser-checks.json').write_text(json.dumps({'capturedAt':datetime.now(timezone.utc).isoformat(),'evidence':'Automated Chrome checks with intercepted API fixtures, not staging or human UAT. Text 200% is computed font/line-height enlargement, not browser zoom.','fixtures':events,'checks':results,'pageErrors':errors}, indent=2))
    print(f'Passed {len(results)} browser checks; evidence in {OUTPUT}')


asyncio.run(main())
