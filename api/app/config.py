from functools import lru_cache

from pydantic_settings import BaseSettings, SettingsConfigDict
from pydantic import SecretStr, field_validator


class Settings(BaseSettings):
    model_config = SettingsConfigDict(
        env_file=".env",
        env_file_encoding="utf-8",
        case_sensitive=False,
        extra="ignore",
    )

    database_url: str = "postgresql+asyncpg://typing:typing@localhost:5432/typing"
    db_echo: bool = False

    internal_token: SecretStr

    @field_validator("database_url")
    @classmethod
    def _force_asyncpg(cls, v: str) -> str:
        if v.startswith(("postgresql+asyncpg://", "postgresql+psycopg://")):
            return v

        for prefix in ("postgres://", "postgresql://"):
            if v.startswith(prefix):
                return "postgresql+asyncpg://" + v[len(prefix):]

        raise ValueError(f"DATABASE_URL должен быть postgres-строкой: {v[:24]}...")

@lru_cache
def get_settings() -> Settings:
    return Settings() # type: ignore [call-arg]