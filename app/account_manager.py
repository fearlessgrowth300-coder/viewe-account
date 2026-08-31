import json
import os
from typing import Dict, List, Optional, Any
from pydantic import BaseModel, Field
from datetime import datetime
from app.stealth import FingerprintGenerator

ACCOUNTS_DIR = os.path.join(os.path.dirname(os.path.dirname(os.path.abspath(__file__))), "data", "accounts")
os.makedirs(ACCOUNTS_DIR, exist_ok=True)

class AccountSession(BaseModel):
    account_id: str
    username: str
    platform: str  # "Twitch" | "Kick"
    status: str = "idle"  # "idle" | "running" | "expired" | "error"
    auth_token: Optional[str] = None
    cookies: List[Dict[str, Any]] = Field(default_factory=list)
    proxy_config: Optional[Dict[str, str]] = None
    hardware_profile: Dict[str, Any] = Field(default_factory=dict)
    created_at: str = Field(default_factory=lambda: datetime.utcnow().isoformat())
    last_used: Optional[str] = None

class AccountManager:
    """Manages persistent account sessions, auth tokens, and fixed hardware profiles."""
    
    @staticmethod
    def _get_account_file(account_id: str) -> str:
        return os.path.join(ACCOUNTS_DIR, f"{account_id}.json")

    @classmethod
    def create_account(
        cls, 
        account_id: str, 
        username: str, 
        platform: str, 
        auth_token: Optional[str] = None,
        cookies: Optional[List[Dict[str, Any]]] = None,
        proxy_config: Optional[Dict[str, str]] = None
    ) -> AccountSession:
        """Creates a new account session with a permanently bound hardware profile."""
        profile = FingerprintGenerator.create_stealth_profile()
        session = AccountSession(
            account_id=account_id,
            username=username,
            platform=platform,
            auth_token=auth_token,
            cookies=cookies or [],
            proxy_config=proxy_config,
            hardware_profile=profile
        )
        cls.save_account(session)
        return session

    @classmethod
    def save_account(cls, session: AccountSession) -> None:
        file_path = cls._get_account_file(session.account_id)
        with open(file_path, "w", encoding="utf-8") as f:
            json.dump(session.model_dump(), f, indent=2)

    @classmethod
    def get_account(cls, account_id: str) -> Optional[AccountSession]:
        file_path = cls._get_account_file(account_id)
        if not os.path.exists(file_path):
            return None
        with open(file_path, "r", encoding="utf-8") as f:
            data = json.load(f)
            return AccountSession(**data)

    @classmethod
    def list_accounts(cls, platform: Optional[str] = None) -> List[AccountSession]:
        accounts = []
        for file_name in os.listdir(ACCOUNTS_DIR):
            if file_name.endswith(".json"):
                with open(os.path.join(ACCOUNTS_DIR, file_name), "r", encoding="utf-8") as f:
                    data = json.load(f)
                    acc = AccountSession(**data)
                    if platform is None or acc.platform.lower() == platform.lower():
                        accounts.append(acc)
        return accounts

    @classmethod
    def delete_account(cls, account_id: str) -> bool:
        file_path = cls._get_account_file(account_id)
        if os.path.exists(file_path):
            os.remove(file_path)
            return True
        return False
