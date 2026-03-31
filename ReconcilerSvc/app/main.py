from __future__ import annotations

import logging
from contextlib import asynccontextmanager
from pathlib import Path

import uvicorn
from fastapi import FastAPI

from app.ansible.runner_adapter import AnsibleRunnerAdapter
from app.api.routes import router
from app.config.loader import APP_CONFIG_ENV_VAR, load_app_config
from app.config.models import AppConfig
from app.execution.task_queue import ReconcileTaskQueue
from app.logging.setup import configure_logging
from app.operators.cadvisor import CadvisorOperator
from app.operators.node_exporter import NodeExporterOperator
from app.operators.registry import OperatorRegistry
from app.services.reconcile_executor import ReconcileExecutor
from app.services.reconcile_service import ReconcileService


class ApplicationRuntime:
    def __init__(
        self,
        *,
        config: AppConfig,
        config_path: Path,
        reconcile_service: ReconcileService,
        execution_queue: ReconcileTaskQueue,
    ) -> None:
        self.config = config
        self.config_path = config_path
        self.reconcile_service = reconcile_service
        self._execution_queue = execution_queue
        self._ready = False
        self._logger = logging.getLogger("reconciler.runtime")

    @property
    def is_ready(self) -> bool:
        return self._ready

    async def start(self) -> None:
        self._logger.info(
            "starting reconciler service",
            extra={
                "config_path": str(self.config_path),
                "max_parallel_reconciliations": self.config.reconciler.max_parallel_reconciliations,
                "queue_capacity": self.config.reconciler.queue_capacity,
            },
        )
        await self._execution_queue.start()
        self._ready = True
        self._logger.info("reconciler service is ready")

    async def stop(self) -> None:
        self._ready = False
        await self._execution_queue.stop()
        self._logger.info("reconciler service stopped")


def _resolve_path(path_value: str, base_dir: Path) -> Path:
    path = Path(path_value)
    if not path.is_absolute():
        path = base_dir / path
    return path.resolve()


def build_runtime(config: AppConfig, config_path: Path) -> ApplicationRuntime:
    logger = logging.getLogger("reconciler.bootstrap")
    service_root = config_path.parent.parent.resolve()

    playbooks_base_dir = _resolve_path(config.ansible.playbooks_base_dir, service_root)
    logger.info("resolved playbooks directory", extra={"playbooks_base_dir": str(playbooks_base_dir)})

    operators = []
    if config.operators.node_exporter.enabled:
        operators.append(NodeExporterOperator(playbooks_base_dir, config.operators.node_exporter))
    if config.operators.cadvisor.enabled:
        operators.append(CadvisorOperator(playbooks_base_dir, config.operators.cadvisor))
    if not operators:
        raise RuntimeError("No operators enabled in configuration")

    registry = OperatorRegistry(operators)
    adapter = AnsibleRunnerAdapter(config.ansible, base_dir=service_root)
    executor = ReconcileExecutor(adapter, logging.getLogger("reconciler.executor"))

    execution_queue = ReconcileTaskQueue(
        max_workers=config.reconciler.max_parallel_reconciliations,
        capacity=config.reconciler.queue_capacity,
        enqueue_timeout_seconds=config.reconciler.enqueue_timeout_seconds,
        handler=executor.execute,
        logger=logging.getLogger("reconciler.queue"),
    )

    reconcile_service = ReconcileService(
        operator_registry=registry,
        execution_queue=execution_queue,
        logger=logging.getLogger("reconciler.service"),
    )

    return ApplicationRuntime(
        config=config,
        config_path=config_path,
        reconcile_service=reconcile_service,
        execution_queue=execution_queue,
    )


def build_default_runtime() -> ApplicationRuntime:
    config, config_path = load_app_config()
    configure_logging(config.logging.level)

    logger = logging.getLogger("reconciler.bootstrap")
    logger.info(
        "configuration loaded",
        extra={
            "config_path": str(config_path),
            "config_env_var": APP_CONFIG_ENV_VAR,
            "log_level": config.logging.level,
            "api_host": config.server.host,
            "api_port": config.server.port,
        },
    )

    return build_runtime(config, config_path)


def create_app(runtime: ApplicationRuntime | None = None) -> FastAPI:
    app_runtime = runtime or build_default_runtime()

    @asynccontextmanager
    async def lifespan(app: FastAPI):
        app.state.runtime = app_runtime
        await app_runtime.start()
        try:
            yield
        finally:
            await app_runtime.stop()

    app = FastAPI(
        title="ReconcilerSvc",
        description="Infrastructure Management Platform Reconciler",
        version="0.1.0",
        lifespan=lifespan,
    )
    # Expose runtime immediately so CLI run() can read bind host/port before lifespan starts.
    app.state.runtime = app_runtime
    app.include_router(router)
    return app


def run() -> None:
    runtime: ApplicationRuntime = app.state.runtime
    uvicorn.run(
        app,
        host=runtime.config.server.host,
        port=runtime.config.server.port,
    )


app = create_app()
