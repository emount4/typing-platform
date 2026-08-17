from uuid import UUID

from sqlalchemy import select, func
from sqlalchemy.ext.asyncio import AsyncSession

from app.models.texts import TextModel
from app.enums import Lang


async def get_by_id(session: AsyncSession, text_id: UUID) -> TextModel | None:
    return await session.get(TextModel, text_id)


async def pick_random(
        session: AsyncSession,
        lang: Lang,
        min_length: int,
        max_length: int,
        exclude: set[UUID] | None = None) -> TextModel | None:
    query = (select(TextModel)
            .where(TextModel.lang == lang,
                   TextModel.length.between(min_length, max_length))
            .order_by(func.random()).limit(1)
    )

    if exclude:
        exclude_query = query.where(TextModel.id.notin_(exclude))
        found = (await session.execute(exclude_query)).scalar_one_or_none()
        if found is not None:
            return found

    return (await session.execute(query)).scalar_one_or_none()