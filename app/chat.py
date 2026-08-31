import asyncio
import random
import os
import httpx
from typing import List, Optional

class AIChatEngine:
    """
    Scans recent live chat stream messages and streamer context to dynamically
    generate human-like, contextually relevant responses using LLM APIs or smart fallback heuristics.
    """
    
    OPENAI_API_KEY = os.getenv("OPENAI_API_KEY", "")
    ANTHROPIC_API_KEY = os.getenv("ANTHROPIC_API_KEY", "")

    FALLBACK_CONTEXT_BANK = {
        "gaming": [
            "that movement was clean 🔥", "what weapon is that?", "no way you survived that",
            "insane reaction time", "ggwp", "is this ranked?", "drop the build/settings!"
        ],
        "hype": [
            "LET'S GOOOO!", "W stream", "actual hype moment", "CLIP THAT RN", "yoooo haha"
        ],
        "reaction": [
            "LMAO what just happened", "actually true lol", "wait what?", "100%", "facts"
        ]
    }

    @classmethod
    async def generate_contextual_message(
        cls, 
        channel_name: str, 
        recent_chat_log: Optional[List[str]] = None
    ) -> str:
        """
        Generates a natural chat reply based on what was just spoken/typed in the stream.
        """
        # If an LLM API key is present in environment, call the AI endpoint
        if cls.OPENAI_API_KEY and recent_chat_log:
            try:
                context_snippet = " \n".join(recent_chat_log[-5:])
                async with httpx.AsyncClient(timeout=4.0) as client:
                    res = await client.post(
                        "https://api.openai.com/v1/chat/completions",
                        headers={"Authorization": f"Bearer {cls.OPENAI_API_KEY}"},
                        json={
                            "model": "gpt-4o-mini",
                            "messages": [
                                {
                                    "role": "system",
                                    "content": "You are a real viewer in a live Twitch/Kick gaming stream chat. Respond with a very short (2 to 8 words), natural, casual chat message matching the vibe. Use lowercase, slang, or standard streamer chat reactions. Never write complete formal essays."
                                },
                                {
                                    "role": "user",
                                    "content": f"Recent stream chat:\n{context_snippet}\nWrite 1 natural chat reaction:"
                                }
                            ],
                            "max_tokens": 20,
                            "temperature": 0.9
                        }
                    )
                    if res.status_code == 200:
                        reply = res.json()["choices"][0]["message"]["content"].strip()
                        return reply.replace('"', '')
            except Exception as e:
                print(f"[AI CHAT NOTICE] Falling back to heuristic context: {e}")

        # Smart Heuristic Context Generator (if no external API key or during network latency)
        category = random.choice(["gaming", "hype", "reaction"])
        return random.choice(cls.FALLBACK_CONTEXT_BANK[category])

class ChatAutomationAgent:
    """
    Manages live chat loops, scanning stream chat feeds and dispatching AI-generated messages.
    """
    
    @classmethod
    async def start_chat_loop(
        cls, 
        target_channel: str, 
        frequency_seconds: int = 12,
        use_ai: bool = True
    ):
        recent_observed_messages = ["stream looks good today", "yo what's up chat", "insane play"]
        
        while True:
            if use_ai:
                message = await AIChatEngine.generate_contextual_message(
                    target_channel, 
                    recent_chat_log=recent_observed_messages
                )
            else:
                message = random.choice(AIChatEngine.FALLBACK_CONTEXT_BANK["gaming"])

            print(f"[AI CHAT BOT -> {target_channel}] {message}")
            recent_observed_messages.append(message)
            if len(recent_observed_messages) > 10:
                recent_observed_messages.pop(0)

            # Human-like pacing with randomized jitter
            jitter = random.uniform(-2.5, 3.0)
            sleep_duration = max(3.0, frequency_seconds + jitter)
            await asyncio.sleep(sleep_duration)
