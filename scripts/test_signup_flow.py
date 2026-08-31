import asyncio
import json
import os
import secrets
import string
from playwright.async_api import async_playwright

SIGNUP_URL = "http://127.0.0.1:8000/signup"
OUTPUT_ACCOUNT_FILE = os.path.join("data", "accounts", "test_user.json")

def generate_test_credentials():
    """Generates synthetic, unique test user credentials."""
    suffix = secrets.token_hex(4)
    username = f"test_user_{suffix}"
    email = f"test_{suffix}@example.com"
    
    alphabet = string.ascii_letters + string.digits + "!@#$%"
    password = "".join(secrets.choice(alphabet) for _ in range(16))
    
    return {
        "username": username,
        "email": email,
        "password": password
    }

async def run_signup_test():
    print("=" * 60)
    print("🧪 E2E PLAYWRIGHT REGISTRATION FLOW & SESSION EXPORT")
    print(f"Target: {SIGNUP_URL}")
    print("=" * 60)

    creds = generate_test_credentials()
    print(f"[*] Generated Synthetic Test Account: {creds['username']} ({creds['email']})")

    os.makedirs(os.path.dirname(OUTPUT_ACCOUNT_FILE), exist_ok=True)

    async with async_playwright() as p:
        browser = await p.chromium.launch(headless=False)
        context = await browser.new_context()
        page = await context.new_page()

        try:
            print(f"[*] Navigating to signup portal at {SIGNUP_URL}...")
            await page.goto(SIGNUP_URL, wait_until="domcontentloaded", timeout=15000)

            # Wait briefly so user can see form in visible window
            await asyncio.sleep(1)

            # Fill in the registration form fields
            print(f"[*] Auto-filling registration fields with synthetic credentials...")
            await page.locator('input[name="username"]').fill(creds["username"])
            await asyncio.sleep(0.5)
            await page.locator('input[name="email"]').fill(creds["email"])
            await asyncio.sleep(0.5)
            await page.locator('input[name="password"]').fill(creds["password"])
            await asyncio.sleep(1)

            # Submit the form
            print("[*] Clicking 'Register Account' button...")
            await page.locator('button[type="submit"]').click()

            # Wait for confirmation screen
            await page.wait_for_load_state("networkidle")
            print("✅ Form submitted successfully and response verified!")

            # Export storage state (cookies, session tokens)
            storage_state = await context.storage_state()
            
            saved_profile = {
                "account_id": creds["username"],
                "username": creds["username"],
                "email": creds["email"],
                "cookies": storage_state.get("cookies", []),
                "origins": storage_state.get("origins", [])
            }

            with open(OUTPUT_ACCOUNT_FILE, "w", encoding="utf-8") as f:
                json.dump(saved_profile, f, indent=2)

            print(f"✅ Session state and cookies exported to: {OUTPUT_ACCOUNT_FILE}")
            print("\n👀 Browser will stay open for 10 seconds so you can see the success screen...")
            await asyncio.sleep(10)

        except Exception as e:
            print(f"⚠️ Registration test error: {e}")
            print("   Make sure your FastAPI server is running: uvicorn app.main:app --port 8000")
        finally:
            await browser.close()

if __name__ == "__main__":
    asyncio.run(run_signup_test())
