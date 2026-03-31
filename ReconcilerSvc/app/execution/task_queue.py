from __future__ import annotations

import asyncio
import logging
from collections.abc import Awaitable, Callable
from typing import Any

from app.execution.models import ReconcileJob


class QueueSaturatedError(RuntimeError):
    """Raised when reconcile queue cannot accept a new task in configured timeout."""


class QueueNotRunningError(RuntimeError):
    """Raised when enqueue is attempted before queue startup or after shutdown."""


_SENTINEL = object()


class ReconcileTaskQueue:
    def __init__(
        self,
        *,
        max_workers: int,
        capacity: int,
        enqueue_timeout_seconds: float,
        handler: Callable[[ReconcileJob], Awaitable[None]],
        logger: logging.Logger,
    ) -> None:
        self._queue: asyncio.Queue[ReconcileJob | object] = asyncio.Queue(maxsize=capacity)
        self._max_workers = max_workers
        self._enqueue_timeout_seconds = enqueue_timeout_seconds
        self._handler = handler
        self._logger = logger

        self._workers: list[asyncio.Task[Any]] = []
        self._running = False
        self._accepting = False

    @property
    def is_running(self) -> bool:
        return self._running

    @property
    def qsize(self) -> int:
        return self._queue.qsize()

    async def start(self) -> None:
        if self._running:
            return

        self._accepting = True
        self._running = True
        self._workers = [
            asyncio.create_task(self._worker_loop(index + 1), name=f"reconcile-worker-{index + 1}")
            for index in range(self._max_workers)
        ]
        self._logger.info(
            "reconcile worker pool started",
            extra={"workers": self._max_workers, "queue_capacity": self._queue.maxsize},
        )

    async def stop(self) -> None:
        if not self._running:
            return

        self._accepting = False
        await self._queue.join()

        for _ in self._workers:
            await self._queue.put(_SENTINEL)

        await asyncio.gather(*self._workers, return_exceptions=True)
        self._workers = []
        self._running = False

        self._logger.info("reconcile worker pool stopped")

    async def enqueue(self, job: ReconcileJob) -> None:
        if not self._running or not self._accepting:
            raise QueueNotRunningError("reconcile queue is not running")

        try:
            await asyncio.wait_for(self._queue.put(job), timeout=self._enqueue_timeout_seconds)
        except asyncio.TimeoutError as exc:
            raise QueueSaturatedError("reconcile queue capacity reached") from exc

    async def drain(self) -> None:
        await self._queue.join()

    async def _worker_loop(self, worker_id: int) -> None:
        self._logger.info("reconcile worker started", extra={"worker_id": worker_id})

        while True:
            item = await self._queue.get()
            try:
                if item is _SENTINEL:
                    self._logger.info("reconcile worker stopping", extra={"worker_id": worker_id})
                    return

                job = item
                await self._handler(job)
            except Exception:
                self._logger.exception("unexpected worker error", extra={"worker_id": worker_id})
            finally:
                self._queue.task_done()
