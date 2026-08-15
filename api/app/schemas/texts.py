from uuid import UUID
from pydantic import BaseModel, ConfigDict
from app.enums import Lang


class STextOut(BaseModel):
    id: UUID
    content: str
    lang: Lang
    length: int
    source: str
    title: str | None

    model_config = ConfigDict(from_attributes=True)