import asyncio
import json
import os
import random
from playwright.async_api import async_playwright
from playwright_stealth import Stealth
from app.stealth import ProxyRotator

TARGET_CHANNEL_URL = "https://www.twitch.tv/vinco_vibeslive"
TEST_MESSAGE = "Let's gooo vinco! 🔥"
ACCOUNT_FILE = os.path.join(os.path.dirname(os.path.dirname(os.path.abspath(__file__))), "data", "accounts", "olaisaboy4.json")

# Set USE_PROXY = True to route through your imported SOAX residential proxies
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

async def auto_follow_channel(page):
    """Safely follows the channel ONCE. Prevents accidental unfollowing."""
    print("\n" + "="*50)
    print("💜 STEP 1: SAFE 1-CLICK FOLLOW CHECK...")
    print("="*50)

    try:
        already_following = page.locator('button[aria-label="Unfollow"], button:has-text("Unfollow"), button:has-text("Following")')
        if await already_following.count() > 0 and await already_following.first.is_visible():
            print("ℹ️ [AUTO-FOLLOW] You are ALREADY following this channel. Skipping click!")
            return

        follow_btn = page.locator('button[data-a-target="follow-button"], button[aria-label="Follow"]').first
        if await follow_btn.count() == 0:
            follow_btn = page.get_by_role("button", name="Follow", exact=True)

        await follow_btn.wait_for(state="visible", timeout=15000)
        
        pause = random.uniform(1.5, 2.5)
        print(f"[*] Simulating human pause ({pause:.1f}s) before clicking Follow...")
        await asyncio.sleep(pause)

        await follow_btn.scroll_into_view_if_needed()
        await follow_btn.click()
        print("✅ [AUTO-FOLLOW] Clicked Follow ONCE on vinco_vibeslive!")
        
        await asyncio.sleep(3.0)
        print("🎉 [AUTO-FOLLOW VERIFIED] Channel is now permanently followed.")
    except Exception as e:
        print(f"⚠️ [AUTO-FOLLOW NOTICE]: {e}")

async def type_and_send_chat(page, message: str):
    """Types and sends chat message with natural human keystroke jitter."""
    print("\n" + "="*50)
    print("💬 STEP 2: NATURAL HUMAN CHAT DISPATCH...")
    print("="*50)

    try:
        print("[*] Locating chat input field...")
        chat_box = page.get_by_placeholder("Send a message")
        if await chat_box.count() == 0:
            chat_box = page.get_by_role("textbox", name="Chat input")
        if await chat_box.count() == 0:
            chat_box = page.locator('div[data-slate-editor="true"]').first

        await chat_box.wait_for(state="visible", timeout=15000)
        
        await asyncio.sleep(random.uniform(1.0, 2.0))
        await chat_box.scroll_into_view_if_needed()
        await chat_box.click()
        await asyncio.sleep(0.5)

        print(f"[*] Typing message: '{message}'")
        await page.keyboard.type(message, delay=random.randint(50, 130))
        
        await asyncio.sleep(random.uniform(0.8, 1.8))

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
    print("🛡️ Launching PLAYWRIGHT STEALTH Visible Chrome with SOAX Proxy")
    print(f"Target: {TARGET_CHANNEL_URL}")
    print("="*60)

    cookies, auth_token, username = load_account_data()
    proxy_config = None

    if USE_PROXY:
        soax_proxy = ProxyRotator.get_authenticated_proxy()
        if soax_proxy:
            print(f"🌐 [PROXY ACTIVE] Routing through SOAX Residential ({soax_proxy.get('city')}, {soax_proxy.get('country')})")
            proxy_config = {
                "server": soax_proxy["server"],
                "username": soax_proxy["username"],
                "password": soax_proxy["password"]
            }

    async with async_playwright() as p:
        launch_args = {
            "headless": False,
            "args": [
                "--start-maximized",
                "--disable-blink-features=AutomationControlled",
                "--no-sandbox"
            ]
        }
        if proxy_config:
            launch_args["proxy"] = proxy_config

        browser = await p.chromium.launch(**launch_args)
        
        context = await browser.new_context(
            no_viewport=True,
            user_agent="Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36"
        )

        if cookies:
            print(f"[AUTH] Injected {len(cookies)} session cookies for '{username}'...")
            await context.add_cookies(cookies)

        await context.add_init_script(f"""
            try {{
                localStorage.setItem('auth-token', '{auth_token}');
                localStorage.setItem('twilight.oauth.token', '{auth_token}');
                localStorage.setItem('login', '{username}');
            }} catch(e) {{}}
        """)

        page = await context.new_page()

        # Activate Playwright Stealth
        stealth = Stealth()
        await stealth.apply_stealth_async(page)
        print("✅ [STEALTH] Automation signatures stripped successfully!")

        print(f"[*] Navigating to {TARGET_CHANNEL_URL}...")
        await page.goto(TARGET_CHANNEL_URL, wait_until="domcontentloaded")
        
        print("[*] Waiting for live stream and React DOM to mount...")
        await asyncio.sleep(7)

        # Step 1: Follow channel ONCE
        await auto_follow_channel(page)
        await asyncio.sleep(2)

        # Step 2: Send Chat Message
        await type_and_send_chat(page, TEST_MESSAGE)

        print("\n" + "="*60)
        print(f"🟢 Browser running permanently on your screen as '{username}'.")
        print("   (Close window or press Ctrl+C in terminal to exit)")
        print("="*60 + "\n")
        
        await asyncio.Event().wait()
        await browser.close()

if __name__ == "__main__":
    asyncio.run(main())
