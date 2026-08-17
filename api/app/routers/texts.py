from fastapi import APIRouter, HTTPException, status, Query
from uuid import UUID

from app.schemas.texts import STextOut
from app.database import SessionDep
from app.services import texts as text_services
from app.enums import Lang


router = APIRouter(prefix="/api/v1/texts", tags=["texts"])

@router.get(
    path="/random",
    response_model=STextOut,
    summary="Случайный текст")
async def get_random_text(
        session: SessionDep,
        lang: Lang,
        min_length: int = 120,
        max_length: int = 400,
        exclude: set[UUID] = Query(default_factory=set)):
    text = await text_services.pick_random(session, lang, min_length, max_length, exclude)
    if text is None:
        raise HTTPException(
            status_code=status.HTTP_404_NOT_FOUND,
            detail="Text not found."
        )

    return text


@router.get(
    path="/{text_id:uuid}",
    response_model=STextOut,
    summary="Текст по идентификатору")
async def get_text(text_id: UUID, session: SessionDep):
    text = await text_services.get_by_id(session, text_id)
    if text is None:
        raise HTTPException(
            status_code=status.HTTP_404_NOT_FOUND,
            detail="Text not found."
        )

    return text