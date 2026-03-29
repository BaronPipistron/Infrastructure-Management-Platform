from __future__ import annotations

import asyncio
import os
import time
from dataclasses import dataclass
from pathlib import Path
from typing import Any

from app.config.models import AnsibleConfig
from app.execution.models import ExecutionPlan

try:
    import ansible_runner
except ImportError:  # pragma: no cover - guarded at runtime
    ansible_runner = None


@dataclass(slots=True)
class AnsibleExecutionContext:
    request_id: str
    fqdn: str
    component: str
    correlation_id: str | None


@dataclass(slots=True)
class AnsibleExecutionResult:
    status: str
    rc: int | None
    successful: bool
    duration_seconds: float


@dataclass(slots=True)
class SSHConnectionDetails:
    user: str
    private_key_path: str
    port: int
    host_key_checking: str


class AnsibleRunnerAdapter:
    def __init__(
        self,
        config: AnsibleConfig,
        runner_module: Any | None = None,
        base_dir: Path | None = None,
    ) -> None:
        self._config = config
        self._runner = runner_module if runner_module is not None else ansible_runner
        self._base_dir = (base_dir or Path.cwd()).resolve()

        private_data_dir = Path(config.private_data_dir)
        if not private_data_dir.is_absolute():
            private_data_dir = self._base_dir / private_data_dir
        self._private_data_dir = private_data_dir.resolve()
        self._private_data_dir.mkdir(parents=True, exist_ok=True)

    async def run(self, plan: ExecutionPlan, context: AnsibleExecutionContext) -> AnsibleExecutionResult:
        return await asyncio.to_thread(self._run_blocking, plan, context)

    def _run_blocking(
        self,
        plan: ExecutionPlan,
        context: AnsibleExecutionContext,
    ) -> AnsibleExecutionResult:
        if self._runner is None:
            raise RuntimeError("ansible-runner is not installed")

        ssh = self._read_ssh_connection_details()
        inventory = self._build_inventory(context.fqdn, ssh)
        extra_vars = self._build_extra_vars(plan, context)
        env_vars = {
            self._config.ssh.host_key_checking_env_var: ssh.host_key_checking,
        }

        started = time.monotonic()
        result = self._runner.run(
            private_data_dir=str(self._private_data_dir),
            playbook=str(plan.playbook_path),
            inventory=inventory,
            extravars=extra_vars,
            envvars=env_vars,
            ident=context.request_id,
            quiet=True,
            timeout=self._config.run_timeout_seconds,
        )
        duration = time.monotonic() - started

        status = str(getattr(result, "status", "unknown"))
        rc = getattr(result, "rc", None)
        success = bool(rc == 0 or status.lower() == "successful")

        return AnsibleExecutionResult(
            status=status,
            rc=rc,
            successful=success,
            duration_seconds=duration,
        )

    def _read_ssh_connection_details(self) -> SSHConnectionDetails:
        user = os.getenv(self._config.ssh.user_env_var, "").strip()
        private_key_path = os.getenv(self._config.ssh.private_key_path_env_var, "").strip()
        port_raw = os.getenv(self._config.ssh.port_env_var, str(self._config.ssh.default_port)).strip()
        host_key_checking = os.getenv(self._config.ssh.host_key_checking_env_var, "False").strip() or "False"

        if not user:
            raise RuntimeError(f"Missing required env var: {self._config.ssh.user_env_var}")
        if not private_key_path:
            raise RuntimeError(f"Missing required env var: {self._config.ssh.private_key_path_env_var}")

        try:
            port = int(port_raw)
        except ValueError as exc:
            raise RuntimeError(f"Invalid SSH port in env var {self._config.ssh.port_env_var}: {port_raw}") from exc

        return SSHConnectionDetails(
            user=user,
            private_key_path=private_key_path,
            port=port,
            host_key_checking=host_key_checking,
        )

    @staticmethod
    def _build_inventory(fqdn: str, ssh: SSHConnectionDetails) -> dict[str, Any]:
        return {
            "all": {
                "hosts": {
                    fqdn: {
                        "ansible_host": fqdn,
                        "ansible_user": ssh.user,
                        "ansible_port": ssh.port,
                        "ansible_ssh_private_key_file": ssh.private_key_path,
                    }
                }
            }
        }

    @staticmethod
    def _build_extra_vars(
        plan: ExecutionPlan,
        context: AnsibleExecutionContext,
    ) -> dict[str, Any]:
        extra_vars = dict(plan.extra_vars)
        extra_vars.update(
            {
                "reconciler_request_id": context.request_id,
                "reconciler_component": context.component,
                "reconciler_correlation_id": context.correlation_id,
            }
        )
        return extra_vars
