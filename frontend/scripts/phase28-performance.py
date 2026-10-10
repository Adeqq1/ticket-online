"""Compare public route loading with a throttled production browser session."""
import argparse
import asyncio
import json
import statistics
import time
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
from urllib.parse import urlparse

from playwright.async_api import async_playwright

ROOT = Path(__file__).resolve().parents[2]
FIXTURES = ROOT / "docs" / "phase20" / "fixtures"

ROUTES = {
    "home": ("/", ".event-section"),
    "catalog": ("/konser", ".concert-grid"),
    "detail": ("/konser/nusa-malam", ".detail-heading"),
    "checkout": ("/checkout/nusa-malam?festival=1", ".checkout-heading"),
}
IMAGES = {"nusa-malam": "nusa-malam.jpg", "ticket-online-concert-crowd": "concert-crowd.jpg", "ticket-online-stage-lights": "stage-lights.jpg"}

def fixture_event():
    return {"id": "nusa-malam", "artist": "Nusa Malam", "city": "Jakarta", "venue": "Ruang Selatan", "address": "Jl. Musik Raya, Jakarta", "startsAt": "2027-08-24T19:30:00+07:00", "genre": "Indie", "status": "Presale", "publicationStatus": "PUBLISHED", "image": "https://picsum.photos/seed/nusa-malam/900/1100", "description": "Konser Nusa Malam.", "lineup": ["Nusa Malam"], "price": 275000, "zones": [{"id": "festival", "name": "Festival", "description": "Area berdiri"}], "ticketTiers": [{"id": "festival", "name": "Festival", "zoneId": "festival", "price": 275000, "availableQuantity": 42, "maxPerOrder": 6, "benefit": "Area berdiri", "gate": "Gate B", "seating": "free-standing"}], "scheduleLocked": False}


async def measure(browser, base_url, route_name, route, selector, runs):
    samples = []
    for _ in range(runs):
        context = await browser.new_context(viewport={"width": 390, "height": 844}, device_scale_factor=1)
        page = await context.new_page()
        session = await context.new_cdp_session(page)
        await session.send("Network.enable")
        await session.send("Network.setCacheDisabled", {"cacheDisabled": True})
        await session.send("Network.emulateNetworkConditions", {"offline": False, "downloadThroughput": 200_000, "uploadThroughput": 93_750, "latency": 150, "connectionType": "cellular3g"})
        await session.send("Emulation.setCPUThrottlingRate", {"rate": 4})
        await page.add_init_script("""window.__lcp=0;window.__cls=0;window.__shifts=[];new PerformanceObserver(list=>{for(const entry of list.getEntries())window.__lcp=entry.startTime}).observe({type:'largest-contentful-paint',buffered:true});new PerformanceObserver(list=>{window.__shifts.push(...list.getEntries().filter(entry=>!entry.hadRecentInput).map(entry=>({time:entry.startTime,value:entry.value,sources:entry.sources.map(source=>source.node?.tagName+'.'+source.node?.className)})));let start=-1,last=-1,total=0,max=0;for(const entry of window.__shifts){if(start<0||entry.time-last>1000||entry.time-start>5000){max=Math.max(max,total);start=entry.time;total=0}total+=entry.value;last=entry.time}window.__cls=Math.max(max,total)}).observe({type:'layout-shift',buffered:true});""")

        async def api(request):
            url = urlparse(request.request.url)
            if url.path == "/api/v1/reservations" and request.request.method == "POST":
                body = {"id": "fixture-reservation", "status": "ACTIVE", "expiresAt": "2099-01-01T00:00:00.000Z", "event": {"id": "nusa-malam", "artist": "Nusa Malam"}, "items": [{"tierId": "festival", "name": "Festival", "quantity": 1, "unitPrice": 275000, "lineTotal": 275000}], "subtotal": 275000}
            else:
                body = {"events": [fixture_event()]} if url.path == "/api/v1/events" else fixture_event()
            serialized = json.dumps(body).replace("https://picsum.photos/seed/nusa-malam/900/1100", f"{base_url}/fixtures/nusa-malam.jpg")
            await asyncio.sleep(0.3)
            await request.fulfill(status=200, content_type="application/json", body=serialized)

        await context.route("**/api/v1/**", api)
        await page.add_init_script("""localStorage.setItem('ticket-online:reservation:nusa-malam:festival=1', JSON.stringify({reservationId:'fixture-reservation',idempotencyKey:'fixture-idempotency-key',eventId:'nusa-malam',basketKey:'festival=1'}));""")
        await page.goto(base_url + route, wait_until="domcontentloaded")
        await page.locator(selector).wait_for(state="visible", timeout=30000)
        await page.wait_for_function("document.fonts.status === 'loaded'")
        result = await page.evaluate("""() => {
          const navigation = performance.getEntriesByType('navigation')[0];
          const paint = name => performance.getEntriesByName(name)[0]?.startTime ?? null;
          const resources = performance.getEntriesByType('resource').filter(item => item.initiatorType === 'script' || item.name.endsWith('.js'));
          return {fcp:paint('first-contentful-paint'), lcp:window.__lcp ?? null, cls:window.__cls ?? 0, ready:performance.now(), jsBytes:resources.reduce((sum,item)=>sum+item.transferSize,0), jsFiles:resources.length, domContentLoaded:navigation.domContentLoadedEventEnd};
        }""")
        await page.wait_for_timeout(1000)
        result.update(await page.evaluate("""() => ({lcp:window.__lcp,cls:window.__cls,shifts:window.__shifts})"""))
        samples.append(result)
        await context.close()
    metrics = ("fcp", "lcp", "cls", "ready", "jsBytes", "jsFiles", "domContentLoaded")
    return {"route": route_name, "median": {key: statistics.median(sample[key] for sample in samples if sample[key] is not None) if any(sample[key] is not None for sample in samples) else None for key in metrics}, "runs": samples}


async def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--base-url", default="http://127.0.0.1:4173")
    parser.add_argument("--runs", type=int, default=5)
    parser.add_argument("--output", type=Path)
    parser.add_argument("--serve", action="store_true")
    parser.add_argument("--dist", type=Path)
    parser.add_argument("--check-chunk-retry", action="store_true")
    args = parser.parse_args()
    if args.serve:
        serve(args.dist or Path(__file__).resolve().parents[1] / "dist", args.base_url)
        return
    async with async_playwright() as playwright:
        browser = await playwright.chromium.launch(channel="chrome", headless=True)
        if args.check_chunk_retry:
            context = await browser.new_context(viewport={"width": 390, "height": 844})
            page = await context.new_page()
            attempts = 0

            async def fail_first_home_chunk(route):
                nonlocal attempts
                attempts += 1
                if attempts == 1:
                    await route.abort()
                else:
                    await route.continue_()

            await context.route("**/assets/HomePage-*.js", fail_first_home_chunk)
            await page.goto(args.base_url, wait_until="domcontentloaded")
            await page.get_by_role("alert").wait_for(state="visible")
            await page.get_by_role("button", name="Coba lagi").click()
            await page.locator(".hero h1").wait_for(state="visible")
            assert attempts == 2, f"Expected one failed chunk request and one successful retry, got {attempts}."
            await context.close()

            context = await browser.new_context(viewport={"width": 390, "height": 844})
            page = await context.new_page()
            event_attempts = 0

            async def fail_first_catalog_request(route):
                nonlocal event_attempts
                event_attempts += 1
                if event_attempts == 1:
                    await route.fulfill(status=503, content_type="application/json", body=json.dumps({"error": {"code": "SERVICE_UNAVAILABLE", "message": "Koneksi belum tersedia."}}))
                else:
                    await route.fulfill(status=200, content_type="application/json", body=json.dumps({"events": [fixture_event()]}))

            await context.route("**/api/v1/events", fail_first_catalog_request)
            await page.goto(args.base_url, wait_until="domcontentloaded")
            await page.get_by_role("alert").wait_for(state="visible")
            await page.get_by_role("button", name="Coba lagi").click()
            await page.locator(".concert-card").wait_for(state="visible")
            assert event_attempts == 2, f"Expected one failed API request and one successful retry, got {event_attempts}."
            await context.close()
            await browser.close()
            print("Lazy chunk failure, API failure, and both retry paths passed.")
            return
        results = [await measure(browser, args.base_url, name, path, selector, args.runs) for name, (path, selector) in ROUTES.items()]
        await browser.close()
    args.output.write_text(json.dumps({"profile": {"viewport": "390x844", "downloadKbps": 1600, "uploadKbps": 750, "latencyMs": 150, "cpuSlowdown": 4, "cache": "cold", "runs": args.runs}, "results": results}, indent=2) + "\n")
    print(args.output)


def serve(dist: Path, base_url: str):
    address = urlparse(base_url)
    root = dist.resolve()

    class Handler(BaseHTTPRequestHandler):
        def do_GET(self):
            path = urlparse(self.path).path
            if path.startswith("/fixtures/"):
                image = FIXTURES / Path(path).name
                if image.name not in IMAGES.values():
                    return self.send_error(404)
                body, content_type = image.read_bytes(), "image/jpeg"
            else:
                file = (root / path.lstrip("/")).resolve()
                if root not in file.parents and file != root:
                    return self.send_error(403)
                if not file.is_file():
                    file = root / "index.html"
                content_type = {".html": "text/html", ".js": "text/javascript", ".css": "text/css", ".woff2": "font/woff2", ".svg": "image/svg+xml"}.get(file.suffix, "application/octet-stream")
                body = file.read_bytes()
            self.send_response(200)
            self.send_header("Content-Type", content_type)
            self.send_header("Content-Length", str(len(body)))
            self.end_headers()
            self.wfile.write(body)

        def log_message(self, *_):
            pass

    ThreadingHTTPServer((address.hostname, address.port or 4173), Handler).serve_forever()


if __name__ == "__main__":
    asyncio.run(main())
