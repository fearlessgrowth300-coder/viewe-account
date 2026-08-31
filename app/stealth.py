import random
import secrets
import json
import os
from typing import Dict, Any, Optional

PROXIES_FILE = os.path.join(os.path.dirname(os.path.dirname(os.path.abspath(__file__))), "data", "proxies.json")

class FingerprintGenerator:
    """
    Generates realistic hardware profiles (GPU, Canvas, Audio, Concurrency, Screen).
    """
    
    GPU_POOL = [
        {"vendor": "Google Inc. (NVIDIA)", "renderer": "ANGLE (NVIDIA, NVIDIA GeForce RTX 4090 Direct3D11 vs_5_0 ps_5_0, D3D11)"},
        {"vendor": "Google Inc. (NVIDIA)", "renderer": "ANGLE (NVIDIA, NVIDIA GeForce RTX 4070/PCIe/SSE2, D3D11)"},
        {"vendor": "Google Inc. (NVIDIA)", "renderer": "ANGLE (NVIDIA, NVIDIA GeForce RTX 3080 Direct3D11 vs_5_0 ps_5_0, D3D11)"},
        {"vendor": "Google Inc. (Apple)", "renderer": "ANGLE (Apple, Apple M3 Max, OpenGL 4.1)"},
        {"vendor": "Google Inc. (Intel)", "renderer": "ANGLE (Intel, Intel(R) Iris(R) Xe Graphics Direct3D11 vs_5_0 ps_5_0, D3D11)"}
    ]
    
    USER_AGENTS = [
        "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36",
        "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/123.0.0.0 Safari/537.36",
        "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36"
    ]

    GEOLOCATIONS = {
        "US": {"timezone": "America/New_York", "locale": "en-US", "lat": 25.7617, "lon": -80.1918},
        "GB": {"timezone": "Europe/London", "locale": "en-GB", "lat": 51.5074, "lon": -0.1278}
    }

    @classmethod
    def create_stealth_profile(cls, country: str = "US") -> Dict[str, Any]:
        gpu = random.choice(cls.GPU_POOL)
        geo = cls.GEOLOCATIONS.get(country, cls.GEOLOCATIONS["US"])
        screen_w = random.choice([1440, 1920])
        screen_h = 900 if screen_w == 1440 else 1080
        
        return {
            "user_agent": random.choice(cls.USER_AGENTS),
            "hardware_concurrency": random.choice([8, 12, 16]),
            "device_memory": random.choice([8, 16, 32]),
            "webgl_vendor": gpu["vendor"],
            "webgl_renderer": gpu["renderer"],
            "screen_width": screen_w,
            "screen_height": screen_h,
            "timezone": geo["timezone"],
            "locale": geo["locale"],
            "latitude": geo["lat"],
            "longitude": geo["lon"]
        }

class ProxyRotator:
    """
    Loads and rotates real SOAX residential proxies from data/proxies.json.
    """
    
    @classmethod
    def get_all_proxies(cls):
        if os.path.exists(PROXIES_FILE):
            try:
                with open(PROXIES_FILE, "r", encoding="utf-8") as f:
                    return json.load(f)
            except Exception:
                pass
        return []

    @classmethod
    def get_authenticated_proxy(cls, index: Optional[int] = None) -> Optional[Dict[str, str]]:
        proxies = cls.get_all_proxies()
        if not proxies:
            return None
        
        selected = proxies[index % len(proxies)] if index is not None else random.choice(proxies)
        return {
            "server": selected["server"],
            "username": selected["username"],
            "password": selected["password"],
            "country": selected.get("country", "US"),
            "city": selected.get("city", "Miami")
        }
