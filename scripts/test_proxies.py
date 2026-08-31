import os
import sys
import requests

ROOT_DIR = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
if ROOT_DIR not in sys.path:
    sys.path.insert(0, ROOT_DIR)

from app.stealth import ProxyRotator

proxies = ProxyRotator.get_all_proxies()
print(f"Loaded {len(proxies)} proxies from data/proxies.json\n")

for p in proxies:
    proxy_url = f"http://{p['username']}:{p['password']}@{p['server'].replace('http://', '')}"
    print(f"Testing {p['name']} ({p.get('city')}, {p.get('country')})...")
    try:
        r = requests.get('https://api.ipify.org?format=json', proxies={'http': proxy_url, 'https': proxy_url}, timeout=12)
        print(f"  [SUCCESS] Outbound IP: {r.json().get('ip')}")
    except Exception as e:
        print(f"  [FAILED] Error: {e}")
