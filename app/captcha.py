import httpx
import asyncio
from app.config import settings

class CaptchaSolver:
    """Handles external solver API handshakes asynchronously."""
    
    @classmethod
    async def resolve_turnstile(cls, page_url: str, site_key: str) -> str:
        if not settings.CAPTCHA_API_KEY:
            raise ValueError("CAPTCHA_API_KEY is not configured in settings.")
            
        async with httpx.AsyncClient(timeout=30.0) as client:
            submit_url = "https://2captcha.com/in.php"
            payload = {
                "key": settings.CAPTCHA_API_KEY,
                "method": "turnstile",
                "sitekey": site_key,
                "pageurl": page_url,
                "json": 1
            }
            res = await client.post(submit_url, data=payload)
            request_id = res.json().get("request")
            
            poll_url = f"https://2captcha.com/res.php?key={settings.CAPTCHA_API_KEY}&action=get&id={request_id}&json=1"
            for _ in range(30):
                await asyncio.sleep(2)
                check_res = await client.get(poll_url)
                body = check_res.json()
                if body.get("status") == 1:
                    return body.get("request")
            
            raise TimeoutError("CAPTCHA solving matrix timed out after 60 seconds.")
