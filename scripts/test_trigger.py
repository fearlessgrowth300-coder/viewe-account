import httpx
import json
import sys

# Ensure UTF-8 output encoding on Windows console
if sys.stdout.encoding != 'utf-8':
    try:
        sys.stdout.reconfigure(encoding='utf-8')
    except Exception:
        pass

API_BASE = "http://127.0.0.1:8000"

def trigger_test_swarm():
    payload = {
        "channel_name": "zaybosays",
        "platform": "Twitch",
        "viewer_count": 1,
        "use_chat": True,
        "proxy_tier": "Residential"
    }

    print("="*60)
    print("[*] Triggering Visible Single-Instance Stream Test...")
    print(f"Target Channel: https://www.twitch.tv/{payload['channel_name']}")
    print(f"Viewer Count: {payload['viewer_count']} | Chat Swarm: {payload['use_chat']}")
    print("="*60)

    try:
        res = httpx.post(f"{API_BASE}/api/tasks/start", json=payload, timeout=10.0)
        if res.status_code == 200:
            data = res.json()
            print(f"[SUCCESS] Task scheduled with ID: {data.get('task_id')}")
            print(f"Status Message: {data.get('message')}")
            print("\n[INFO] Since headless=False, a Chrome browser window is launching on your screen.")
        else:
            print(f"[ERROR] ({res.status_code}): {res.text}")
    except Exception as e:
        print(f"[ERROR] Connection failed: {e}")
        print("Ensure FastAPI server (uvicorn app.main:app --port 8000) and Redis are running.")

if __name__ == "__main__":
    trigger_test_swarm()
