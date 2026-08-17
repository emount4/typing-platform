import asyncio

from app.database import new_session
from app.seed.texts import seed_texts


async def main() -> None:
    async with new_session() as session:
        inserted, skipped = await seed_texts(session)
    print(f"тексты: вставлено {inserted}, уже было {skipped}")


if __name__ == "__main__":
    asyncio.run(main())