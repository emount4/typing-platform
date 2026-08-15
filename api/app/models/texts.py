from uuid import UUID, uuid4
from datetime import datetime

from sqlalchemy import CheckConstraint, Index, String, Text, Computed, DateTime, func
from sqlalchemy.dialects.postgresql import UUID as PgUUID
from sqlalchemy.orm import Mapped, mapped_column

from app.database import Base
from app.enums import Lang


class TextModel(Base):
    __tablename__ = "texts"

    id: Mapped[UUID] = mapped_column(
        PgUUID(as_uuid=True), primary_key=True, default=uuid4
    )
    lang: Mapped[Lang] = mapped_column(String(2))
    source: Mapped[str] = mapped_column(String(32), server_default="seed")
    title: Mapped[str | None] = mapped_column(String(200))
    content: Mapped[str] = mapped_column(Text)
    length: Mapped[int] = mapped_column(
        Computed("char_length(content)", persisted=True)
    )
    created_at: Mapped[datetime] = mapped_column(
        DateTime(timezone=True), server_default=func.now()
    )

    __table_args__ = (
        CheckConstraint("lang in ('ru','en')", name="ck_texts_lang"),
        CheckConstraint("char_length(content) > 0", name="ck_texts_content_not_empty"),
        Index("ix_texts_lang_length", "lang", "length"),
    )