from __future__ import annotations

import asyncio
import logging
from pathlib import Path

from app.execution.models import ExecutionPlan, ReconcileJob
from app.execution.task_queue import QueueSaturatedError, ReconcileTaskQueue
from app.schemas.reconcile import ComponentType


def _build_job(index: int) -> ReconcileJob:
    return ReconcileJob(
        request_id=f"req-{index}",
        host_id=f"host-{index}",
        correlation_id=f"corr-{index}",
        component=ComponentType.NODE_EXPORTER,
        fqdn=f"srv-{index}.example.local",
        plan=ExecutionPlan(
            component=ComponentType.NODE_EXPORTER,
            operator_name="node_exporter_operator",
            playbook_path=Path("ansible/node_exporter/playbook.yml"),
            target_fqdn=f"srv-{index}.example.local",
            extra_vars={},
        ),
    )


def test_queue_respects_parallel_limit() -> None:
    async def scenario() -> int:
        active = 0
        max_active = 0
        lock = asyncio.Lock()

        async def handler(job: ReconcileJob) -> None:
            nonlocal active, max_active

            async with lock:
                active += 1
                max_active = max(max_active, active)

            await asyncio.sleep(0.05)

            async with lock:
                active -= 1

        queue = ReconcileTaskQueue(
            max_workers=2,
            capacity=20,
            enqueue_timeout_seconds=0.2,
            handler=handler,
            logger=logging.getLogger("test.queue.parallel"),
        )

        await queue.start()
        for idx in range(8):
            await queue.enqueue(_build_job(idx))

        await queue.drain()
        await queue.stop()

        return max_active

    max_active = asyncio.run(scenario())
    assert max_active <= 2


def test_queue_raises_when_full() -> None:
    async def scenario() -> None:
        release = asyncio.Event()

        async def handler(job: ReconcileJob) -> None:
            await release.wait()

        queue = ReconcileTaskQueue(
            max_workers=1,
            capacity=1,
            enqueue_timeout_seconds=0.05,
            handler=handler,
            logger=logging.getLogger("test.queue.full"),
        )

        await queue.start()
        await queue.enqueue(_build_job(1))
        await asyncio.sleep(0.02)
        await queue.enqueue(_build_job(2))

        try:
            await queue.enqueue(_build_job(3))
        except QueueSaturatedError:
            pass
        else:
            raise AssertionError("Expected QueueSaturatedError when queue is full")

        release.set()
        await queue.drain()
        await queue.stop()

    asyncio.run(scenario())
