import os
from pydantic_settings import BaseSettings

class Settings(BaseSettings):
    APP_NAME: str = "Viewbotter Automation Engine"
    REDIS_URL: str = os.getenv("REDIS_URL", "redis://localhost:6379/0")
    CAPTCHA_API_KEY: str = os.getenv("CAPTCHA_API_KEY", "")
    PROXY_HOST: str = os.getenv("PROXY_HOST", "proxyprovider.com")
    PROXY_PORT: int = int(os.getenv("PROXY_PORT", "9000"))
    MAX_VIEWERS_PER_TASK: int = 50
    DEFAULT_TIMEOUT_SEC: int = 3600

    class Config:
        env_file = ".env"
        extra = "ignore"

settings = Settings()
