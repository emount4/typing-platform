from collections.abc import AsyncIterator
from typing import Annotated

from sqlalchemy.ext.asyncio import AsyncSession, async_sessionmaker, create_async_engine
from sqlalchemy.orm import DeclarativeBase
from fastapi import Depends

from app.config import get_settings


settings = get_settings()
engine = create_async_engine(settings.database_url, pool_pre_ping=True, echo=settings.db_echo)

new_session = async_sessionmaker(engine, expire_on_commit=False)

async def get_session() -> AsyncIterator[AsyncSession]:
    async with new_session() as session:
        yield session

class Base(DeclarativeBase):
    """Общий класс моделей. От него берётся metadata"""
    pass

SessionDep = Annotated[AsyncSession, Depends(get_session)]