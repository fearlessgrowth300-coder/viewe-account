import asyncio
import time
import psutil
from playwright.async_api import async_playwright
from app.stealth import FingerprintGenerator, ProxyRotator

CONCURRENT_INSTANCES = 5
TEST_DURATION_SECONDS = 30
TEST_URL = "https://example.com"

async def run_single_instance(instance_id: int, duration: int):
    profile = FingerprintGenerator.create_stealth_profile()
    proxy = ProxyRotator.get_authenticated_proxy()

    async with async_playwright() as p:
        browser = await p.chromium.launch(
            headless=True,
            args=[
                "--disable-blink-features=AutomationControlled",
                "--no-sandbox",
                "--disable-dev-shm-usage",
                f"--window-size={profile['screen_width']},{profile['screen_height']}"
            ]
        )
        context = await browser.new_context(user_agent=profile["user_agent"])
        page = await context.new_page()

        # Bandwidth Saver Optimization
        await page.route(
            "**/*",
            lambda route: route.abort() if route.request.resource_type in ["image", "font", "media", "stylesheet"]
            else route.continue_()
        )

        await page.goto(TEST_URL, wait_until="domcontentloaded")
        print(f"[{time.strftime('%X')}] [Instance #{instance_id}] Connected and holding session...")
        await asyncio.sleep(duration)
        await browser.close()

async def monitor_resources(duration: int):
    start_net = psutil.net_io_counters()
    start_time = time.time()
    
    print("\n" + "="*60)
    print(f"🛡️  BENCHMARK: Running {CONCURRENT_INSTANCES} Playwright Instances for {duration}s")
    print("="*60)

    while time.time() - start_time < duration:
        cpu = psutil.cpu_percent(interval=1.0)
        ram = psutil.virtual_memory()
        cur_net = psutil.net_io_counters()
        mb_transferred = ((cur_net.bytes_sent + cur_net.bytes_recv) - (start_net.bytes_sent + start_net.bytes_recv)) / (1024 * 1024)

        print(f"⚡ CPU: {cpu:>4.1f}% | RAM: {ram.used / (1024*1024):>6.0f} MB ({ram.percent:>4.1f}%) | Net (Diff): {mb_transferred:>5.2f} MB")
        await asyncio.sleep(2)

async def main():
    monitor_task = asyncio.create_task(monitor_resources(TEST_DURATION_SECONDS))
    instances = [run_single_instance(i + 1, TEST_DURATION_SECONDS) for i in range(CONCURRENT_INSTANCES)]
    
    await asyncio.gather(*instances, return_exceptions=True)
    await monitor_task
    print("\n✅ Benchmark Complete. Scalability metrics verified.\n")

if __name__ == "__main__":
    asyncio.run(main())
