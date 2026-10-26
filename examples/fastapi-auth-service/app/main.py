from contextlib import asynccontextmanager

from app.db import Base, engine
from app.routes import router
from fastapi import FastAPI


@asynccontextmanager
async def lifespan(_: FastAPI):
    Base.metadata.create_all(engine)  # tự tạo bảng khi khởi động
    yield


app = FastAPI(title="Auth API", lifespan=lifespan)
app.include_router(router)


@app.get("/health")
def health():
    return {"status": "ok"}
