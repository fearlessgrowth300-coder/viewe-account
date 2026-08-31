from celery import Celery
import asyncio
from app.config import settings
from app.swarm import launch_stealth_viewer_instance
from app.chat import ChatAutomationAgent

celery_app = Celery("darkboard_worker", broker=settings.REDIS_URL, backend=settings.REDIS_URL)

celery_app.conf.update(
    task_serializer="json",
    accept_content=["json"],
    result_serializer="json",
    timezone="UTC",
    worker_concurrency=16
)

@celery_app.task(bind=True, name="tasks.run_swarm_worker")
def run_swarm_worker(self, payload: dict):
    """
    Asynchronous Celery task that handles concurrent Headless Playwright instances
    with AI chat processing, proxy rotation, and fingerprinting.
    """
    channel_name = payload.get("channel_name")
    platform = payload.get("platform", "Twitch").lower()
    viewer_count = int(payload.get("viewer_count", 1))
    use_chat = payload.get("use_chat", False)
    use_ai_llm_chat = payload.get("use_ai_llm_chat", True)
    auto_follow = payload.get("auto_follow", True)
    auto_unlock_chat = payload.get("auto_unlock_chat", True)
    
    base_url = f"https://www.twitch.tv/{channel_name}" if platform == "twitch" else f"https://kick.com/{channel_name}"
    
    async def orchestrate():
        tasks = []
        for _ in range(viewer_count):
            tasks.append(launch_stealth_viewer_instance(
                channel_url=base_url,
                duration_seconds=3600,
                use_chat=use_chat,
                use_ai_llm=use_ai_llm_chat,
                auto_follow=auto_follow,
                auto_unlock_chat=auto_unlock_chat,
                headless=True # Runs silently in background without desktop windows
            ))
        
        if use_chat:
            tasks.append(ChatAutomationAgent.start_chat_loop(
                target_channel=channel_name, 
                frequency_seconds=12,
                use_ai=use_ai_llm_chat
            ))
            
        await asyncio.gather(*tasks, return_exceptions=True)

    loop = asyncio.new_event_loop()
    asyncio.set_event_loop(loop)
    try:
        loop.run_until_complete(orchestrate())
    finally:
        loop.close()
        
    return {"status": "completed", "channel": channel_name, "viewers": viewer_count}
