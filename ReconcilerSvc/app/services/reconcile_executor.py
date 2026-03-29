from __future__ import annotations

import logging

from app.ansible.runner_adapter import AnsibleExecutionContext, AnsibleRunnerAdapter
from app.execution.models import ReconcileJob


class ReconcileExecutor:
    def __init__(self, adapter: AnsibleRunnerAdapter, logger: logging.Logger) -> None:
        self._adapter = adapter
        self._logger = logger

    async def execute(self, job: ReconcileJob) -> None:
        self._logger.info(
            "reconcile execution started",
            extra={
                "request_id": job.request_id,
                "host_id": job.host_id,
                "fqdn": job.fqdn,
                "component": job.component.value,
                "operator": job.plan.operator_name,
                "playbook_path": str(job.plan.playbook_path),
                "correlation_id": job.correlation_id,
            },
        )

        context = AnsibleExecutionContext(
            request_id=job.request_id,
            fqdn=job.fqdn,
            component=job.component.value,
            correlation_id=job.correlation_id,
        )

        try:
            result = await self._adapter.run(job.plan, context)
        except Exception:
            self._logger.exception(
                "reconcile execution failed",
                extra={
                    "request_id": job.request_id,
                    "host_id": job.host_id,
                    "fqdn": job.fqdn,
                    "component": job.component.value,
                    "operator": job.plan.operator_name,
                    "correlation_id": job.correlation_id,
                },
            )
            return

        level = logging.INFO if result.successful else logging.ERROR
        self._logger.log(
            level,
            "reconcile execution finished",
            extra={
                "request_id": job.request_id,
                "host_id": job.host_id,
                "fqdn": job.fqdn,
                "component": job.component.value,
                "operator": job.plan.operator_name,
                "correlation_id": job.correlation_id,
                "ansible_status": result.status,
                "ansible_rc": result.rc,
                "successful": result.successful,
                "duration_seconds": round(result.duration_seconds, 3),
            },
        )
