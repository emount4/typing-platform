from fastapi import FastAPI
from app.routers.texts import router as text_router

app = FastAPI(
    title="Typing Platform API",
    docs_url="/api/docs",
    redoc_url="/api/redoc",
    openapi_url="/api/openapi.json",
)

@app.get("/health")
def health():
    return {"status": "ok"}

@app.get("/api/v1/ping")
def ping():
    return {"message": "pong"}

app.include_router(text_router)
