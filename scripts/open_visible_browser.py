import asyncio
import json
import os
import sys
import random
import argparse

# Force UTF-8 on Windows console to prevent charmap errors
if sys.platform == "win32":
    try:
        sys.stdout.reconfigure(encoding='utf-8')
        sys.stderr.reconfigure(encoding='utf-8')
    except Exception:
        pass

# Ensure root workspace directory is in python path
ROOT_DIR = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
if ROOT_DIR not in sys.path:
    sys.path.insert(0, ROOT_DIR)

from playwright.async_api import async_playwright
from playwright_stealth import Stealth
from app.stealth import ProxyRotator

CHAT_MESSAGES_POOL = [
    "W stream let's gooo 🔥",
    "insane play right there haha",
    "stream looking super clean today",
    "LET'S GOOOO!",
    "vibes are so good in here 🙌",
    "ggwp",
    "that was actually crazy lol",
    "facts 100%"
]

def load_account_data(account_name: str):
    """Loads session cookies and auth token from data/accounts/<account_name>.json."""
    if not account_name.endswith(".json"):
        account_name += ".json"
    
    account_path = os.path.join(ROOT_DIR, "data", "accounts", account_name)
    if not os.path.exists(account_path):
        # Fallback search
        files = [f for f in os.listdir(os.path.join(ROOT_DIR, "data", "accounts")) if f.endswith(".json")]
        if files:
            account_path = os.path.join(ROOT_DIR, "data", "accounts", files[0])
            print(f"[ACCOUNT] Specified account not found, falling back to: {files[0]}")
    
    if os.path.exists(account_path):
        try:
            with open(account_path, "r", encoding="utf-8") as f:
                data = json.load(f)
                cookies = data.get("cookies", [])
                auth_token = data.get("auth_token", "")
                username = data.get("username", os.path.splitext(os.path.basename(account_path))[0])
                print(f"[ACCOUNT] Loaded credentials for: @{username}")
                return cookies, auth_token, username
        except Exception as e:
            print(f"[ACCOUNT ERROR] Failed reading {account_path}: {e}")

    return [], "", "anonymous"

def sanitize_cookies_for_playwright(raw_cookies):
    """Formats raw cookie objects for Playwright compatibility."""
    valid_cookies = []
    for c in raw_cookies:
        if not c.get("name") or not c.get("value"):
            continue
        cookie = {
            "name": c["name"],
            "value": c["value"],
            "domain": c.get("domain", ".twitch.tv"),
            "path": c.get("path", "/"),
        }
        if "secure" in c:
            cookie["secure"] = bool(c["secure"])
        if "httpOnly" in c:
            cookie["httpOnly"] = bool(c["httpOnly"])
        
        # Playwright strictly allows 'Strict', 'Lax', or 'None'
        same_site = str(c.get("sameSite", "")).capitalize()
        if same_site in ["Strict", "Lax", "None"]:
            cookie["sameSite"] = same_site
        
        if c.get("expires") and isinstance(c["expires"], (int, float)) and c["expires"] > 0:
            cookie["expires"] = int(c["expires"])

        valid_cookies.append(cookie)
    return valid_cookies

async def dismiss_popups_and_mature_warning(page):
    """Dismisses mature stream warnings and consent banners if present."""
    try:
        # Mature warning accept
        mature_selectors = [
            'button[data-a-target="player-overlay-mature-accept"]',
            'button:has-text("Start Watching")',
            'button:has-text("I understand")'
        ]
        for sel in mature_selectors:
            btn = page.locator(sel)
            if await btn.count() > 0 and await btn.first.is_visible():
                print("⚠️ [MODAL] Dismissing mature stream warning...")
                await btn.first.click()
                await asyncio.sleep(1.0)
                break

        # Consent cookie banner
        consent_selectors = [
            'button[data-a-target="consent-banner-accept"]',
            'button:has-text("Accept Cookies")',
            'button:has-text("Accept")'
        ]
        for sel in consent_selectors:
            btn = page.locator(sel)
            if await btn.count() > 0 and await btn.first.is_visible():
                print("🍪 [MODAL] Accepting consent cookies...")
                await btn.first.click()
                await asyncio.sleep(1.0)
                break
    except Exception as e:
        pass

async def simulate_human_viewer_behavior(page, warmup_seconds: float):
    """Simulates active human watching and natural scrolling."""
    print(f"\n[*] 🕒 Human viewer warmup: watching live stream for {warmup_seconds:.1f}s...")
    
    half_time = max(3.0, warmup_seconds / 2)
    await asyncio.sleep(half_time)

    await dismiss_popups_and_mature_warning(page)

    print("[*] 📜 Simulating active reading (gentle scroll down)...")
    try:
        await page.mouse.wheel(0, 350)
        await asyncio.sleep(random.uniform(2.0, 3.5))
        print("[*] 📜 Scrolling back to stream player...")
        await page.mouse.wheel(0, -350)
    except Exception:
        pass

    await asyncio.sleep(half_time)

async def auto_follow_channel(page):
    """Checks and clicks the Follow button as an authenticated user."""
    print("\n" + "="*50)
    print("💜 CHECKING CHANNEL FOLLOW STATUS...")
    print("="*50)

    try:
        await dismiss_popups_and_mature_warning(page)

        # 1. Check if already following
        unfollow_selectors = [
            'button[data-a-target="unfollow-button"]',
            'button[aria-label="Unfollow"]',
            'button:has-text("Following")',
            'button:has-text("Unfollow")'
        ]
        for sel in unfollow_selectors:
            loc = page.locator(sel)
            if await loc.count() > 0 and await loc.first.is_visible():
                print("ℹ️ [AUTO-FOLLOW] Account is ALREADY following this channel! ✅")
                return True

        # 2. Look for Follow button
        follow_selectors = [
            'button[data-a-target="follow-button"]',
            'button[aria-label="Follow"]',
            'button:has-text("Follow")'
        ]
        
        target_btn = None
        for sel in follow_selectors:
            loc = page.locator(sel)
            if await loc.count() > 0 and await loc.first.is_visible():
                target_btn = loc.first
                break

        if not target_btn:
            print("⚠️ [AUTO-FOLLOW] Follow button not immediately found. Searching with wait...")
            for sel in follow_selectors:
                try:
                    target_btn = page.locator(sel).first
                    await target_btn.wait_for(state="visible", timeout=6000)
                    if await target_btn.is_visible():
                        break
                except Exception:
                    target_btn = None

        if target_btn and await target_btn.is_visible():
            pause = random.uniform(1.5, 3.0)
            print(f"[*] Simulating human hesitation ({pause:.1f}s) before clicking Follow...")
            await asyncio.sleep(pause)

            await target_btn.scroll_into_view_if_needed()
            await target_btn.click()
            print("💜 [AUTO-FOLLOW] Clicked 'Follow' button successfully!")
            await asyncio.sleep(3.0)
            print("🎉 [AUTO-FOLLOW COMPLETE] Follow status registered.")
            return True
        else:
            print("⚠️ [AUTO-FOLLOW] Could not locate Follow button (might be offline or already followed).")
            return False

    except Exception as e:
        print(f"⚠️ [AUTO-FOLLOW NOTICE]: {e}")
        return False

async def type_and_send_chat(page, message: str):
    """Types and sends chat message with natural human keystroke jitter."""
    print("\n" + "="*50)
    print("💬 ATTEMPTING TO SEND LIVE CHAT...")
    print("="*50)

    try:
        await dismiss_popups_and_mature_warning(page)

        # 1. Dismiss any chat rules / guidelines modal
        chat_rule_selectors = [
            'button[data-a-target="chat-rules-ok-button"]',
            'button:has-text("Agree")',
            'button:has-text("Got it")',
            'button:has-text("I understand")',
            'button:has-text("OK")'
        ]
        for sel in chat_rule_selectors:
            rule_btn = page.locator(sel)
            if await rule_btn.count() > 0 and await rule_btn.first.is_visible():
                print("[*] Dismissing chat rules dialog...")
                await rule_btn.first.click()
                await asyncio.sleep(1.0)
                break

        # 2. Locate chat box
        chat_box_selectors = [
            'div[data-a-target="chat-input"]',
            'div[data-slate-editor="true"]',
            'textarea[data-a-target="chat-input"]',
            '[placeholder="Send a message"]',
            'div[role="textbox"][aria-label*="Chat"]',
            'div[role="textbox"]'
        ]

        chat_input = None
        for sel in chat_box_selectors:
            loc = page.locator(sel)
            if await loc.count() > 0 and await loc.first.is_visible():
                chat_input = loc.first
                break

        if not chat_input:
            print("[*] Waiting for chat input box to mount...")
            for sel in chat_box_selectors:
                try:
                    chat_input = page.locator(sel).first
                    await chat_input.wait_for(state="visible", timeout=8000)
                    if await chat_input.is_visible():
                        break
                except Exception:
                    chat_input = None

        if not chat_input or not await chat_input.is_visible():
            print("⚠️ [CHAT NOTICE] Chat input box not accessible (chat might be in subscriber-only mode or offline).")
            return False

        # 3. Focus and human type
        await asyncio.sleep(random.uniform(1.2, 2.5))
        await chat_input.scroll_into_view_if_needed()
        await chat_input.click()
        await asyncio.sleep(0.4)

        print(f"[*] Human-typing chat message: '{message}'")
        await page.keyboard.type(message, delay=random.randint(60, 110))
        await asyncio.sleep(random.uniform(0.8, 1.8))

        # 4. Dispatch via Send Button or Enter
        send_btn_selectors = [
            'button[data-a-target="chat-send-button"]',
            'button:has-text("Chat")',
            'button[aria-label="Send message"]'
        ]

        clicked_button = False
        for sel in send_btn_selectors:
            btn = page.locator(sel)
            if await btn.count() > 0 and await btn.first.is_visible():
                print("[*] Clicking 'Chat' send button...")
                await btn.first.click()
                clicked_button = True
                break

        if not clicked_button:
            print("[*] Pressing 'Enter' key to dispatch...")
            await page.keyboard.press("Enter")

        await asyncio.sleep(2.0)
        print("🎉 [CHAT SENT] Message dispatched to stream live chat!")
        return True

    except Exception as e:
        print(f"❌ [CHAT ERROR]: {e}")
        return False

async def main():
    parser = argparse.ArgumentParser(description="Twitch Visible Browser - Session Follow & Chat Runner")
    parser.add_argument("-c", "--channel", default="reallweston", help="Target Twitch channel username or URL")
    parser.add_argument("-a", "--account", default="olaisaboy4", help="Account file name in data/accounts (e.g. olaisaboy4, boy_mular3)")
    parser.add_argument("-m", "--message", default="", help="Custom message to send to live chat")
    parser.add_argument("--warmup", type=float, default=12.0, help="Human emulation warmup watch time in seconds")
    parser.add_argument("--no-follow", action="store_true", help="Skip the follow action")
    parser.add_argument("--no-chat", action="store_true", help="Skip the chat action")
    parser.add_argument("--no-proxy", action="store_true", help="Do not use proxy (direct connection)")
    parser.add_argument("--proxy-index", type=int, default=0, help="Index of SOAX proxy to use")
    parser.add_argument("--headless", action="store_true", help="Run headless instead of visible window")

    args = parser.parse_args()

    # Determine channel URL
    channel = args.channel.strip()
    if not channel.startswith("http://") and not channel.startswith("https://"):
        channel = channel.lstrip("/").replace("twitch.tv/", "")
        target_url = f"https://www.twitch.tv/{channel}"
    else:
        target_url = channel

    chat_message = args.message if args.message else random.choice(CHAT_MESSAGES_POOL)

    print("==================================================================")
    print("🛡️ TWITCH AUTHENTICATED SESSION RUNNER (FOLLOW & CHAT TEST)")
    print(f"   Target Channel:    {target_url}")
    print(f"   Account Profile:   {args.account}")
    print(f"   Chat Message:      '{chat_message}'")
    print(f"   Proxy Mode:        {'Direct' if args.no_proxy else 'SOAX Residential'}")
    print(f"   Display Mode:      {'HEADLESS' if args.headless else 'VISIBLE on desktop'}")
    print("==================================================================")

    # 1. Load Account
    raw_cookies, auth_token, username = load_account_data(args.account)
    cookies = sanitize_cookies_for_playwright(raw_cookies)

    # 2. Configure Proxy
    proxy_config = None
    if not args.no_proxy:
        soax_proxy = ProxyRotator.get_authenticated_proxy(index=args.proxy_index)
        if soax_proxy:
            print(f"🌐 [PROXY TUNNEL] {soax_proxy.get('city')}, {soax_proxy.get('country')} ({soax_proxy['server']})")
            proxy_config = {
                "server": f"http://{soax_proxy['server']}",
                "username": soax_proxy["username"],
                "password": soax_proxy["password"]
            }

    async with async_playwright() as p:
        launch_args = {
            "headless": args.headless,
            "args": [
                "--disable-blink-features=AutomationControlled",
                "--start-maximized",
                "--disable-infobars",
                "--no-sandbox",
                "--mute-audio"
            ]
        }
        if proxy_config:
            launch_args["proxy"] = proxy_config

        print("\n🚀 Spawning Chrome instance...")
        browser = await p.chromium.launch(**launch_args)

        context = await browser.new_context(
            no_viewport=True,
            user_agent="Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36",
            locale="en-US",
            timezone_id="America/New_York"
        )

        # 3. Inject Cookies & LocalStorage
        if cookies:
            print(f"🔑 Injecting {len(cookies)} session cookies for @{username}...")
            await context.add_cookies(cookies)

        if auth_token:
            await context.add_init_script(f"""
                try {{
                    localStorage.setItem('auth-token', '{auth_token}');
                    localStorage.setItem('twilight.oauth.token', '{auth_token}');
                    localStorage.setItem('login', '{username}');
                }} catch(e) {{}}
            """)

        page = await context.new_page()

        # Stealth evasion
        stealth = Stealth()
        await stealth.apply_stealth_async(page)

        print(f"[*] Navigating to {target_url}...")
        try:
            await page.goto(target_url, wait_until="domcontentloaded", timeout=45000)
        except Exception as e:
            print(f"⚠️ [PAGE LOAD] Notice: {e}")

        # 4. Human Emulation Warmup
        await simulate_human_viewer_behavior(page, args.warmup)

        # 5. Execute Follow
        if not args.no_follow:
            await auto_follow_channel(page)
            await asyncio.sleep(2.5)

        # 6. Execute Chat
        if not args.no_chat:
            await type_and_send_chat(page, chat_message)

        print("\n" + "="*65)
        print(f"💚 CHROME WINDOW PERMANENTLY LOCKED & ACTIVE AS @{username}")
        print("   Stream is watching, follow & chat executed.")
        print("   Press Ctrl+C in this terminal when you are ready to stop.")
        print("="*65 + "\n")

        # Keep browser open permanently until user presses Ctrl+C or closes window
        try:
            while True:
                await asyncio.sleep(10)
        except (KeyboardInterrupt, asyncio.CancelledError):
            print("\n🛑 Closing browser instance...")
        finally:
            await browser.close()

if __name__ == "__main__":
    try:
        asyncio.run(main())
    except KeyboardInterrupt:
        print("\nProcess terminated by user.")
