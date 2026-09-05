import os
import psutil
from fastapi import FastAPI, HTTPException, Response, Form, WebSocket, WebSocketDisconnect
from fastapi.responses import HTMLResponse
from fastapi.middleware.cors import CORSMiddleware
from pydantic import BaseModel, Field
from typing import Optional, List, Set
import asyncio
import json

from app.tasks import run_swarm_worker, celery_app
from app.account_manager import AccountManager, AccountSession

app = FastAPI(title="Viewbotter Automation API", version="2.0.0")

app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

class StreamTaskRequest(BaseModel):
    channel_name: str = Field(..., example="zaybosays")
    platform: str = Field("Twitch", example="Twitch") # "Twitch" or "Kick"
    viewer_count: int = Field(..., ge=1, le=500000) # Unlocked limit (configurable by user)
    use_chat: bool = False
    use_ai_llm_chat: bool = True # Context-aware LLM stream response generation
    auto_follow: bool = True
    auto_unlock_chat: bool = True
    chat_list_id: Optional[str] = None
    proxy_tier: str = "Residential"

class CreateAccountRequest(BaseModel):
    account_id: str
    username: str
    platform: str
    auth_token: Optional[str] = None
    proxy_config: Optional[dict] = None

class AccountPublicResponse(BaseModel):
    account_id: str
    username: str
    platform: str
    status: str
    has_auth_token: bool
    cookies_count: int
    proxy_config: Optional[dict] = None
    created_at: str
    last_used: Optional[str] = None

def sanitize_account_response(acc: AccountSession) -> AccountPublicResponse:
    return AccountPublicResponse(
        account_id=acc.account_id,
        username=acc.username,
        platform=acc.platform,
        status=acc.status,
        has_auth_token=bool(acc.auth_token),
        cookies_count=len(acc.cookies),
        proxy_config=acc.proxy_config,
        created_at=acc.created_at,
        last_used=acc.last_used
    )

# Track actively running swarm tasks in memory
active_swarm_tasks: dict = {}

# ==============================================================================
# WEBSOCKET BROADCAST MANAGER (Live Telemetry & Live Chat Streaming)
# ==============================================================================
class ConnectionManager:
    def __init__(self):
        self.active_connections: Set[WebSocket] = set()

    async def connect(self, websocket: WebSocket):
        await websocket.accept()
        self.active_connections.add(websocket)

    def disconnect(self, websocket: WebSocket):
        self.active_connections.discard(websocket)

    async def broadcast(self, message: dict):
        for connection in list(self.active_connections):
            try:
                await connection.send_json(message)
            except Exception:
                self.active_connections.discard(connection)

ws_manager = ConnectionManager()

@app.websocket("/ws/live")
async def websocket_endpoint(websocket: WebSocket):
    """
    Persistent WebSocket connection streaming live chat events and telemetry without page reloads.
    """
    await ws_manager.connect(websocket)
    try:
        while True:
            data = await websocket.receive_text()
            # Handle incoming client commands or heartbeat
            await websocket.send_json({"type": "pong", "payload": data})
    except WebSocketDisconnect:
        ws_manager.disconnect(websocket)

@app.get("/")
async def root():
    return {"status": "online", "message": "Viewbotter Automation Core Gateway v2.0"}

# ==============================================================================
# 1. LIVE DASHBOARD ANALYTICS & TELEMETRY
# ==============================================================================
@app.get("/api/dashboard/stats")
async def get_dashboard_stats():
    cpu_usage = psutil.cpu_percent(interval=None)
    ram = psutil.virtual_memory()
    net = psutil.net_io_counters()

    total_viewers = sum(task.get("viewer_count", 0) for task in active_swarm_tasks.values())
    total_chatters = sum(1 for task in active_swarm_tasks.values() if task.get("use_chat"))
    active_plans = len(active_swarm_tasks)

    # If tasks are running via celery worker inspect
    try:
        inspector = celery_app.control.inspect()
        active_celery = inspector.active() or {}
        celery_instances = sum(len(tasks) for tasks in active_celery.values())
        if celery_instances > active_plans:
            active_plans = celery_instances
    except Exception:
        pass

    return {
        "active_viewers": total_viewers,
        "total_chatters": total_chatters,
        "plans_running": active_plans,
        "proxy_bandwidth_mb": round((net.bytes_sent + net.bytes_recv) / (1024 * 1024), 2),
        "chat_messages_per_min": total_chatters * 5 if total_chatters > 0 else 0,
        "system_telemetry": {
            "cpu_percent": cpu_usage,
            "ram_used_mb": round(ram.used / (1024 * 1024), 1),
            "ram_total_mb": round(ram.total / (1024 * 1024), 1),
            "ram_percent": ram.percent
        }
    }

# ==============================================================================
# 2. TASK ORCHESTRATION (UNLOCKED CONCURRENCY)
# ==============================================================================
@app.post("/api/tasks/start")
async def start_stream_swarm(payload: StreamTaskRequest):
    # Enforces custom user-selected instance count without arbitrary trial barriers
    task = run_swarm_worker.delay(payload.model_dump())
    
    active_swarm_tasks[task.id] = payload.model_dump()

    # Broadcast new task event to connected frontend WebSockets
    asyncio.create_task(ws_manager.broadcast({
        "type": "task_started",
        "task_id": task.id,
        "channel": payload.channel_name,
        "viewers": payload.viewer_count
    }))

    return {
        "status": "success",
        "task_id": task.id,
        "viewer_count": payload.viewer_count,
        "auto_follow": payload.auto_follow,
        "auto_unlock_chat": payload.auto_unlock_chat,
        "use_ai_llm_chat": payload.use_ai_llm_chat,
        "message": f"Swarm initialization scheduled for {payload.channel_name} ({payload.viewer_count} instances)"
    }

@app.post("/api/tasks/stop")
async def stop_stream_swarm(task_id: str):
    celery_app.control.revoke(task_id, terminate=True)
    active_swarm_tasks.pop(task_id, None)
    asyncio.create_task(ws_manager.broadcast({
        "type": "task_stopped",
        "task_id": task_id
    }))
    return {"status": "success", "message": f"Task {task_id} terminated."}

# ==============================================================================
# 3. ACCOUNT & SESSION MANAGEMENT (Masked Sensitive Credentials)
# ==============================================================================
@app.get("/api/accounts", response_model=List[AccountPublicResponse])
async def list_accounts(platform: Optional[str] = None):
    accounts = AccountManager.list_accounts(platform)
    return [sanitize_account_response(acc) for acc in accounts]

@app.post("/api/accounts", response_model=AccountPublicResponse)
async def create_account(payload: CreateAccountRequest):
    session = AccountManager.create_account(
        account_id=payload.account_id,
        username=payload.username,
        platform=payload.platform,
        auth_token=payload.auth_token,
        proxy_config=payload.proxy_config
    )
    return sanitize_account_response(session)

@app.get("/api/accounts/{account_id}", response_model=AccountPublicResponse)
async def get_account(account_id: str):
    acc = AccountManager.get_account(account_id)
    if not acc:
        raise HTTPException(status_code=404, detail="Account not found")
    return sanitize_account_response(acc)

@app.delete("/api/accounts/{account_id}")
async def delete_account(account_id: str):
    success = AccountManager.delete_account(account_id)
    if not success:
        raise HTTPException(status_code=404, detail="Account not found")
    return {"status": "success", "message": f"Account {account_id} deleted."}

@app.get("/api/health")
async def health_check():
    return {"status": "ok", "service": "viewbotter-core-v2"}
