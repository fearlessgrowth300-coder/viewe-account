# Viewbotter Automation Engine (Darkboard UI & FastAPI / Playwright Core)

A complete FastAPI, Celery, and Playwright automation stack paired with a dark Tailwind CSS dashboard replicating the modern anti-detect UI design.

---

## ⚡ Execution Order & Terminal Setup

To run the full stack, open **3 separate terminal windows** and execute these commands in sequence:

### Terminal 1: Message Queue (Redis)
Make sure Docker Desktop is started:
```bash
docker run -d -p 6379:6379 redis:alpine
```
*(If you are running Redis natively on Windows/WSL, start `redis-server`)*

---

### Terminal 2: API Backend (FastAPI + Uvicorn)
Navigate to the project root:
```bash
cd C:\Users\UPCOMING\viewe-account
pip install -r requirements.txt
playwright install chromium
uvicorn app.main:app --reload --port 8000
```
Interactive Swagger docs: `http://localhost:8000/docs`

---

### Terminal 3: Automation Task Worker (Celery)
Navigate to the project root:
- **Windows**:
  ```bash
  cd C:\Users\UPCOMING\viewe-account
  celery -A app.tasks.celery_app worker --loglevel=info -P solo
  ```
- **Linux / macOS**:
  ```bash
  celery -A app.tasks.celery_app worker --loglevel=info -P threads
  ```

---

## 🏛️ Architecture Breakdown

1. **`app/main.py`**: API Gateway with CORS, validation, rate limiting, and background queue dispatching.
2. **`app/tasks.py`**: Celery worker orchestrator that receives tasks and manages multi-instance concurrency.
3. **`app/stealth.py`**: Hardware signature spoofer (WebGL, Canvas, AudioContext, screen geometry, user-agents) and sticky residential proxy rotator.
4. **`app/swarm.py`**: Playwright browser lifecycle manager with aggressive network route filtering (`image`, `font`, `media`, `stylesheet` aborting) for low CPU/RAM footprint.
5. **`app/captcha.py`**: Async Cloudflare Turnstile & hCAPTCHA resolution client with polling.
6. **`app/chat.py`**: Natural chat pacing engine with jittered intervals.
7. **`frontend/`**: Complete Tailwind CSS UI kit with buttons, badges, modals, sliders, and dark theme layout.
