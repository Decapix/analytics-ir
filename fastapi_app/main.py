import asyncio
import logging
import os
from typing import Any

import httpx
from fastapi import FastAPI, Request
from pydantic import BaseModel, Field

logger = logging.getLogger("analytics-gateway")

COLLECTOR_URL = os.getenv("COLLECTOR_URL", "http://event-collector.analytics.svc.cluster.local:8080")
TIMEOUT_SECONDS = float(os.getenv("ANALYTICS_TIMEOUT_SECONDS", "0.35"))

app = FastAPI(title="Analytics Gateway")


class AnalyticsEvent(BaseModel):
    event_name: str
    entity_type: str
    entity_id: str
    actor_user_id: str = ""
    actor_entity_id: str = ""
    actor_entity_type: str = ""
    session_id: str
    source: str
    platform: str
    app_version: str = ""
    device_type: str = ""
    properties: dict[str, Any] | None = Field(default_factory=dict)


async def _forward_analytics(event: dict[str, Any], headers: dict[str, str]) -> None:
    try:
        async with httpx.AsyncClient(timeout=TIMEOUT_SECONDS) as client:
            await client.post(f"{COLLECTOR_URL}/events", json=event, headers=headers)
    except Exception as exc:  # noqa: BLE001
        logger.warning("analytics forward failed: %s", exc)


@app.post("/api/analytics/events", status_code=202)
async def ingest_analytics(event: AnalyticsEvent, request: Request) -> dict[str, str]:
    headers = {
        "User-Agent": request.headers.get("user-agent", ""),
        "X-Forwarded-For": request.headers.get("x-forwarded-for", request.client.host if request.client else ""),
    }
    asyncio.create_task(_forward_analytics(event.model_dump(), headers))
    return {"status": "accepted"}


@app.get("/api/analytics/percentages")
async def get_analytics_percentages(hours: int = 24) -> dict[str, Any]:
    params = {"hours": max(1, hours)}
    try:
        async with httpx.AsyncClient(timeout=1.0) as client:
            response = await client.get(f"{COLLECTOR_URL}/internal/percentages", params=params)
            response.raise_for_status()
            return response.json()
    except Exception as exc:  # noqa: BLE001
        logger.warning("collector percentages unavailable: %s", exc)
        return {"interval_hours": params["hours"], "results": []}
