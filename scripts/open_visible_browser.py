import asyncio
import json
import os
import sys
import random
import time

# Ensure root workspace directory is in python path
ROOT_DIR = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
if ROOT_DIR not in sys.path:
    sys.path.insert(0, ROOT_DIR)

from seleniumbase import Driver
from playwright.async_api import async_playwright
from app.stealth import ProxyRotator

TARGET_CHANNEL_URL = "https://www.twitch.tv/vinco_vibeslive"
TEST_MESSAGE = "Let's gooo vinco! 🔥"
ACCOUNT_FILE = os.path.join(ROOT_DIR, "data", "accounts", "olaisaboy4.json")

# Route through SOAX residential proxy
USE_PROXY = True

def load_account_data():
    """Loads session cookies and auth token for olaisaboy4."""
    if os.path.exists(ACCOUNT_FILE):
        try:
            with open(ACCOUNT_FILE, "r", encoding="utf-8") as file:
                data = json.load(file)
                cookies = data.get("cookies", [])
                auth_token = data.get("auth_token", "bah2goqv6myrxqwov1q0xd81o0f4xv")
                username = data.get("username", "olaisaboy4")
                return cookies, auth_token, username
        except Exception as e:
            print(f"[AUTH ERROR] Failed reading {ACCOUNT_FILE}: {e}")
    return [], "bah2goqv6myrxqwov1q0xd81o0f4xv", "olaisaboy4"

async def simulate_human_viewer_behavior(page):
    """
    HUMAN EMULATION: Simulates active watching and page scrolling.
    """
    warmup_time = random.uniform(20.0, 30.0)
    print(f"\n[*] 🕒 Simulating real human viewer watching stream for {warmup_time:.1f} seconds...")
    
    await asyncio.sleep(warmup_time / 2)

    print("[*] 📜 Simulating active reading (scrolling down)...")
    await page.mouse.wheel(0, 400)
    await asyncio.sleep(random.uniform(2.5, 4.5))

    print("[*] 📜 Scrolling back up to video stream...")
    await page.mouse.wheel(0, -400)
    await asyncio.sleep(warmup_time / 2)

async def auto_follow_channel(page):
    """
    Clicks Follow once after genuine human emulation warm-up.
    """
    print("\n" + "="*50)
    print("💜 ATTEMPTING TO FOLLOW VIA UNDETECTED UC MODE...")
    print("="*50)

    try:
        already_following = page.locator('button[aria-label="Unfollow"], button:has-text("Unfollow"), button:has-text("Following")')
        if await already_following.count() > 0 and await already_following.first.is_visible():
            print("ℹ️ [AUTO-FOLLOW] Account is ALREADY following this channel.")
            return

        follow_btn = page.locator('button[aria-label="Follow"], button:has-text("Follow")').first
        await follow_btn.wait_for(state="visible", timeout=15000)
        
        pause = random.uniform(2.0, 3.5)
        print(f"[*] Simulating human pause ({pause:.1f}s) before clicking Follow...")
        await asyncio.sleep(pause)

        await follow_btn.scroll_into_view_if_needed()
        await follow_btn.click()
        print("💜 [AUTO-FOLLOW] Follow button clicked!")
        
        await asyncio.sleep(5.0)
        print("🎉 [AUTO-FOLLOW COMPLETE] Follow registered.")
    except Exception as e:
        print(f"⚠️ [AUTO-FOLLOW NOTICE]: {e}")

async def type_and_send_chat(page, message: str):
    """Types and sends chat message with natural human keystroke jitter."""
    print("\n" + "="*50)
    print("💬 SENDING CHAT MESSAGE...")
    print("="*50)

    try:
        chat_box = page.get_by_placeholder("Send a message")
        if await chat_box.count() == 0:
            chat_box = page.get_by_role("textbox", name="Chat input")
        if await chat_box.count() == 0:
            chat_box = page.locator('div[data-slate-editor="true"]').first

        await chat_box.wait_for(state="visible", timeout=15000)
        
        await asyncio.sleep(random.uniform(1.5, 3.0))
        await chat_box.scroll_into_view_if_needed()
        await chat_box.click()
        await asyncio.sleep(0.5)

        print(f"[*] Typing message: '{message}'")
        await page.keyboard.type(message, delay=random.randint(50, 120))
        
        await asyncio.sleep(random.uniform(1.0, 2.0))

        send_btn = page.get_by_role("button", name="Chat", exact=True)
        if await send_btn.count() > 0 and await send_btn.first.is_visible():
            print("[*] Clicking 'Chat' button...")
            await send_btn.click()
        else:
            print("[*] Pressing ENTER key...")
            await page.keyboard.press("Enter")

        await asyncio.sleep(1.0)
        print(f"🎉 [CHAT SENT] Message dispatched to live stream!")
    except Exception as e:
        print(f"❌ [CHAT ERROR]: {e}")

async def main():
    print("="*60)
    print("🛡️ Launching HYBRID SELENIUMBASE UC MODE + PLAYWRIGHT CDP")
    print(f"Target: {TARGET_CHANNEL_URL}")
    print("="*60)

    cookies, auth_token, username = load_account_data()
    soax_proxy = None

    # 1. Launch real undetected browser with SeleniumBase UC Mode
    proxy_string = None
    if USE_PROXY:
        soax_proxy = ProxyRotator.get_authenticated_proxy()
        if soax_proxy:
            print(f"🌐 [PROXY ACTIVE] SOAX Residential ({soax_proxy.get('city')}, {soax_proxy.get('country')})")
            user = soax_proxy["username"]
            pwd = soax_proxy["password"]
            server = soax_proxy["server"].replace("http://", "")
            proxy_string = f"{user}:{pwd}@{server}"

    print("[*] Starting SeleniumBase Undetected Chrome Driver...")
    driver_kwargs = {"uc": True, "headless": False}
    if proxy_string:
        driver_kwargs["proxy"] = proxy_string

    driver = Driver(**driver_kwargs)
    cdp_address = driver.capabilities["goog:chromeOptions"]["debuggerAddress"]
    print(f"✅ [UC MODE READY] Chrome DevTools Protocol bound at: {cdp_address}")

    # 2. Connect Playwright to the undetected SeleniumBase Chrome instance
    async with async_playwright() as p:
        browser = await p.chromium.connect_over_cdp(f"http://{cdp_address}")
        context = browser.contexts[0]
        page = context.pages[0]


        # Inject session cookies
        if cookies:
            print(f"[AUTH] Injected {len(cookies)} cookies for '{username}'...")
            await context.add_cookies(cookies)

        # Inject LocalStorage token
        await context.add_init_script(f"""
            try {{
                localStorage.setItem('auth-token', '{auth_token}');
                localStorage.setItem('twilight.oauth.token', '{auth_token}');
                localStorage.setItem('login', '{username}');
            }} catch(e) {{}}
        """)

        print(f"[*] Navigating to {TARGET_CHANNEL_URL}...")
        await page.goto(TARGET_CHANNEL_URL, wait_until="domcontentloaded", timeout=60000)

        # 1. HUMAN EMULATION: Watch stream for 20-30s and scroll
        await simulate_human_viewer_behavior(page)

        # 2. Click Follow
        await auto_follow_channel(page)
        await asyncio.sleep(3)

        # 3. Send Chat Message
        await type_and_send_chat(page, TEST_MESSAGE)

        print("\n" + "="*60)
        print(f"🟢 Browser running permanently on your screen as '{username}'.")
        print("   (Close window or press Ctrl+C in terminal to exit)")
        print("="*60 + "\n")
        
        await asyncio.Event().wait()
        await browser.close()
        driver.quit()

if __name__ == "__main__":
    asyncio.run(main())
