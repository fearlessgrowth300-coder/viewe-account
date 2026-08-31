import asyncio
import json
import os
import random
from typing import Optional, List, Dict, Any
from playwright.async_api import async_playwright, Page
from playwright_stealth import Stealth
from app.stealth import FingerprintGenerator, ProxyRotator
from app.chat import AIChatEngine

def load_account_cookies(account_id: Optional[str] = None) -> tuple:
    """Loads session cookies from data/accounts/ directory."""
    accounts_dir = os.path.join(os.path.dirname(os.path.dirname(os.path.abspath(__file__))), "data", "accounts")
    if os.path.exists(accounts_dir):
        if account_id:
            specific_path = os.path.join(accounts_dir, f"{account_id}.json")
            if os.path.exists(specific_path):
                try:
                    with open(specific_path, "r", encoding="utf-8") as file:
                        d = json.load(file)
                        return d.get("cookies", []), d.get("auth_token", ""), d.get("username", account_id)
                except Exception:
                    pass

        for f in os.listdir(accounts_dir):
            if f.endswith(".json"):
                f_path = os.path.join(accounts_dir, f)
                try:
                    with open(f_path, "r", encoding="utf-8") as file:
                        data = json.load(file)
                        cookies = data.get("cookies", [])
                        if cookies:
                            return cookies, data.get("auth_token", ""), data.get("username", f.replace(".json", ""))
                except Exception:
                    pass
    return [], "", "olaisaboy4"

async def auto_follow_channel(page: Page):
    """Resilient Follow Button Handler with Unfollow Guard."""
    try:
        already_following = page.locator('button[aria-label="Unfollow"], button:has-text("Unfollow"), button:has-text("Following")')
        if await already_following.count() > 0 and await already_following.first.is_visible():
            return

        follow_btn = page.locator('button[data-a-target="follow-button"], button[aria-label="Follow"]').first
        if await follow_btn.count() == 0:
            follow_btn = page.get_by_role("button", name="Follow", exact=True)

        await follow_btn.wait_for(state="visible", timeout=12000)
        await asyncio.sleep(random.uniform(1.2, 2.5))
        await follow_btn.scroll_into_view_if_needed()
        await follow_btn.click()
        print("💜 [AUTO-FOLLOW] Clicked Follow button ONCE!")
    except Exception:
        pass

async def type_and_send_chat(
    page: Page, 
    channel_name: str,
    platform: str = "twitch",
    use_ai: bool = True,
    auto_follow: bool = True
):
    """Resilient Chat Handler using semantic role & placeholder targeting."""
    try:
        if auto_follow:
            await auto_follow_channel(page)

        chat_box = page.get_by_placeholder("Send a message")
        if await chat_box.count() == 0:
            chat_box = page.get_by_role("textbox", name="Chat input")
        if await chat_box.count() == 0:
            chat_box = page.locator('div[data-slate-editor="true"]').first

        await chat_box.wait_for(state="visible", timeout=12000)
        await asyncio.sleep(random.uniform(1.0, 2.0))
        await chat_box.click()
        await asyncio.sleep(0.4)

        message = await AIChatEngine.generate_contextual_message(channel_name) if use_ai else "W stream! 🔥"
        await page.keyboard.type(message, delay=random.randint(50, 120))
        await asyncio.sleep(random.uniform(0.8, 1.5))

        send_btn = page.get_by_role("button", name="Chat", exact=True)
        if await send_btn.count() > 0 and await send_btn.first.is_visible():
            await send_btn.click()
        else:
            await page.keyboard.press("Enter")

        print(f"✅ [CHAT SENT] Dispatched: '{message}'")
    except Exception as e:
        print(f"[CHAT NOTICE] {e}")

async def launch_stealth_viewer_instance(
    channel_url: str, 
    duration_seconds: int = 3600, 
    account_id: Optional[str] = None,
    use_chat: bool = False,
    use_ai_llm: bool = True,
    auto_follow: bool = True,
    auto_unlock_chat: bool = True,
    headless: bool = True,
    custom_proxy: Optional[Dict[str, str]] = None,
    stop_event: Optional[asyncio.Event] = None
):
    proxy_info = custom_proxy or ProxyRotator.get_authenticated_proxy()
    profile = FingerprintGenerator.create_stealth_profile(country=proxy_info.get("country", "US") if proxy_info else "US")
    cookies, auth_token, username = load_account_cookies(account_id)
    channel_name = channel_url.rstrip("/").split("/")[-1]

    async with async_playwright() as p:
        launch_kwargs = {
            "headless": headless,
            "args": [
                "--disable-blink-features=AutomationControlled",
                "--no-sandbox",
                "--disable-dev-shm-usage",
                f"--window-size={profile['screen_width']},{profile['screen_height']}"
            ]
        }
        
        # Attach SOAX residential proxy
        if proxy_info:
            launch_kwargs["proxy"] = {
                "server": proxy_info["server"],
                "username": proxy_info["username"],
                "password": proxy_info["password"]
            }

        browser = await p.chromium.launch(**launch_kwargs)
        
        context = await browser.new_context(
            user_agent=profile["user_agent"],
            viewport={"width": profile["screen_width"], "height": profile["screen_height"]},
            locale=profile["locale"],
            timezone_id=profile["timezone"],
            geolocation={"latitude": profile["latitude"], "longitude": profile["longitude"]},
            permissions=["geolocation"]
        )

        if cookies:
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

        # Activate Playwright Stealth on worker page
        stealth = Stealth()
        await stealth.apply_stealth_async(page)

        # Performance Block: Abort heavy media
        await page.route(
            "**/*",
            lambda route: route.abort() if route.request.resource_type in ["image", "font", "media"]
            else route.continue_()
        )

        try:
            print(f"[BACKGROUND] Instance connected to: {channel_url} via SOAX ({proxy_info.get('city') if proxy_info else 'Direct'})")
            await page.goto(channel_url, wait_until="domcontentloaded", timeout=45000)
            await asyncio.sleep(6)

            if auto_follow:
                await auto_follow_channel(page)

            elapsed = 0
            while elapsed < duration_seconds:
                if stop_event and stop_event.is_set():
                    break

                if use_chat and (elapsed > 0 and elapsed % 25 == 0):
                    platform = "twitch" if "twitch.tv" in channel_url else "kick"
                    await type_and_send_chat(
                        page,
                        channel_name=channel_name,
                        platform=platform,
                        use_ai=use_ai_llm,
                        auto_follow=auto_follow
                    )

                await asyncio.sleep(5)
                elapsed += 5
        finally:
            await context.close()
            await browser.close()
