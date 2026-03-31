from __future__ import annotations

import logging
from uuid import uuid4

from app.execution.models import ReconcileJob
from app.execution.task_queue import ReconcileTaskQueue
from app.operators.registry import OperatorRegistry
from app.schemas.reconcile import ReconcileAcceptedResponse, ReconcileRequest


class ReconcileService:
    def __init__(
        self,
        *,
        operator_registry: OperatorRegistry,
        execution_queue: ReconcileTaskQueue,
        logger: logging.Logger,
    ) -> None:
        self._operator_registry = operator_registry
        self._execution_queue = execution_queue
        self._logger = logger

    async def accept_reconcile(self, request: ReconcileRequest) -> ReconcileAcceptedResponse:
        request_id = str(uuid4())

        self._logger.info(
            "reconcile request received",
            extra={
                "request_id": request_id,
                "host_id": request.host_id,
                "fqdn": request.fqdn,
                "component": request.component.value,
                "correlation_id": request.correlation_id,
            },
        )

        operator = self._operator_registry.get_operator(request.component)
        plan = operator.build_execution_plan(request)

        job = ReconcileJob(
            request_id=request_id,
            host_id=request.host_id,
            correlation_id=request.correlation_id,
            component=request.component,
            fqdn=request.fqdn,
            plan=plan,
        )

        await self._execution_queue.enqueue(job)

        self._logger.info(
            "reconcile request enqueued",
            extra={
                "request_id": request_id,
                "host_id": request.host_id,
                "fqdn": request.fqdn,
                "component": request.component.value,
                "correlation_id": request.correlation_id,
                "queue_size": self._execution_queue.qsize,
            },
        )

        return ReconcileAcceptedResponse(
            request_id=request_id,
            component=request.component,
            fqdn=request.fqdn,
            correlation_id=request.correlation_id,
            accepted_at=job.accepted_at,
        )
