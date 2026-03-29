from __future__ import annotations

import asyncio
import logging
from pathlib import Path

from app.execution.models import ExecutionPlan
from app.operators.base import ReconcileOperator
from app.operators.registry import OperatorRegistry
from app.schemas.reconcile import ComponentType, ReconcileRequest
from app.services.reconcile_service import ReconcileService


class FakeOperator(ReconcileOperator):
    component = ComponentType.NODE_EXPORTER
    operator_name = "fake_node_operator"

    def build_execution_plan(self, request: ReconcileRequest) -> ExecutionPlan:
        return ExecutionPlan(
            component=request.component,
            operator_name=self.operator_name,
            playbook_path=Path("ansible/node_exporter/playbook.yml"),
            target_fqdn=request.fqdn,
            extra_vars={"request_param_count": len(request.parameters)},
        )


class RecordingQueue:
    def __init__(self) -> None:
        self.jobs = []

    @property
    def qsize(self) -> int:
        return len(self.jobs)

    async def enqueue(self, job) -> None:
        self.jobs.append(job)


def test_reconcile_service_enqueues_job_and_returns_acceptance() -> None:
    async def scenario() -> None:
        queue = RecordingQueue()
        service = ReconcileService(
            operator_registry=OperatorRegistry([FakeOperator()]),
            execution_queue=queue,
            logger=logging.getLogger("test.reconcile.service"),
        )

        request = ReconcileRequest(
            host_id="srv-01",
            fqdn="srv-01.example.local",
            component=ComponentType.NODE_EXPORTER,
            correlation_id="drift-01",
            parameters={"a": 1, "b": 2},
        )

        response = await service.accept_reconcile(request)

        assert response.status == "accepted"
        assert response.component == ComponentType.NODE_EXPORTER
        assert response.fqdn == "srv-01.example.local"
        assert response.correlation_id == "drift-01"

        assert len(queue.jobs) == 1
        job = queue.jobs[0]
        assert job.fqdn == "srv-01.example.local"
        assert job.plan.operator_name == "fake_node_operator"
        assert job.plan.extra_vars["request_param_count"] == 2
        assert job.request_id == response.request_id

    asyncio.run(scenario())
