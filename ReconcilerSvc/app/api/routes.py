from __future__ import annotations

import logging
from fastapi import APIRouter, HTTPException, Request, status

from app.execution.task_queue import QueueNotRunningError, QueueSaturatedError
from app.operators.base import OperatorPlanError
from app.operators.registry import OperatorNotFoundError
from app.schemas.reconcile import (
    HealthResponse,
    ReadinessResponse,
    ReconcileAcceptedResponse,
    ReconcileRequest,
)

router = APIRouter()
logger = logging.getLogger("reconciler.api")


@router.get("/healthz", response_model=HealthResponse, tags=["health"])
async def healthz() -> HealthResponse:
    return HealthResponse(status="ok")


@router.get("/readyz", response_model=ReadinessResponse, tags=["health"])
async def readyz(request: Request) -> ReadinessResponse:
    runtime = request.app.state.runtime
    if runtime.is_ready:
        return ReadinessResponse(status="ready")

    raise HTTPException(
        status_code=status.HTTP_503_SERVICE_UNAVAILABLE,
        detail="service is not ready",
    )


@router.post(
    "/api/v1/reconcile",
    response_model=ReconcileAcceptedResponse,
    status_code=status.HTTP_202_ACCEPTED,
    tags=["reconcile"],
)
async def reconcile(
    payload: ReconcileRequest,
    request: Request,
) -> ReconcileAcceptedResponse:
    runtime = request.app.state.runtime

    try:
        return await runtime.reconcile_service.accept_reconcile(payload)
    except QueueSaturatedError:
        raise HTTPException(
            status_code=status.HTTP_503_SERVICE_UNAVAILABLE,
            detail="reconcile queue is full",
        )
    except QueueNotRunningError:
        raise HTTPException(
            status_code=status.HTTP_503_SERVICE_UNAVAILABLE,
            detail="reconcile queue is not running",
        )
    except OperatorNotFoundError as exc:
        raise HTTPException(
            status_code=status.HTTP_400_BAD_REQUEST,
            detail=str(exc),
        ) from exc
    except OperatorPlanError as exc:
        logger.exception("failed to build execution plan")
        raise HTTPException(
            status_code=status.HTTP_500_INTERNAL_SERVER_ERROR,
            detail="failed to build execution plan",
        ) from exc
