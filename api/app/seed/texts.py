from uuid import UUID

from sqlalchemy.dialects.postgresql import insert
from sqlalchemy.ext.asyncio import AsyncSession

from app.models.texts import TextModel

RAW_TEXTS: list[dict] = [
    {
        "id": "0f1d6c2e-6c5b-4f7a-9a1e-2b3c4d5e6f70",
        "content": "привет мир",
        "lang": "ru",
        "source": "seed",
        "title": None,
    },
    {
        "id": "1a2b3c4d-0001-4000-8000-000000000001",
        "lang": "ru",
        "source": "seed",
        "title": "О скорости",
        "content": (
            "Скорость печати растёт не от попыток печатать быстрее, а от точности. "
            "Ошибка стоит дороже, чем кажется: её нужно заметить, вернуться и "
            "исправить, и на это уходит больше времени, чем на три верных нажатия."
        ),
    },
]

async def seed_texts(session: AsyncSession) -> tuple[int, int]:
    """Возвращает (вставлено, пропущено)"""
    rows = [{**raw, "id": UUID(raw["id"])} for raw in RAW_TEXTS]

    query = (
        insert(TextModel)
        .values(rows)
        .on_conflict_do_nothing(index_elements=["id"])
        .returning(TextModel.id)
    )
    inserted_ids = (await session.execute(query)).scalars().all()
    await session.commit()

    return len(inserted_ids), len(rows) - len(inserted_ids)