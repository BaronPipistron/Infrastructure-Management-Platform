from __future__ import annotations

import asyncio
from pathlib import Path

from app.ansible.runner_adapter import (
    AnsibleExecutionContext,
    AnsibleRunnerAdapter,
)
from app.config.models import AnsibleConfig
from app.execution.models import ExecutionPlan
from app.schemas.reconcile import ComponentType


class _FakeRunnerResult:
    status = "successful"
    rc = 0


class _FakeRunnerModule:
    def __init__(self) -> None:
        self.calls = []

    def run(self, **kwargs):
        self.calls.append(kwargs)
        return _FakeRunnerResult()


def test_ansible_runner_adapter_builds_inventory_and_extravars(monkeypatch) -> None:
    async def scenario() -> None:
        monkeypatch.setenv("ANSIBLE_SSH_USER", "ubuntu")
        monkeypatch.setenv("ANSIBLE_SSH_PRIVATE_KEY_PATH", "/tmp/id_rsa")
        monkeypatch.setenv("ANSIBLE_SSH_PORT", "2222")
        monkeypatch.setenv("ANSIBLE_HOST_KEY_CHECKING", "False")

        fake_runner = _FakeRunnerModule()

        adapter = AnsibleRunnerAdapter(
            AnsibleConfig(
                playbooks_base_dir="./ansible",
                private_data_dir="./.ansible-runner-tests",
                run_timeout_seconds=30,
            ),
            runner_module=fake_runner,
        )

        plan = ExecutionPlan(
            component=ComponentType.CADVISOR,
            operator_name="cadvisor_operator",
            playbook_path=Path("/opt/reconciler/ansible/cadvisor/playbook.yml"),
            target_fqdn="srv-01.example.local",
            extra_vars={"cadvisor_image": "gcr.io/cadvisor/cadvisor:v0.49.1"},
        )

        context = AnsibleExecutionContext(
            request_id="req-001",
            fqdn="srv-01.example.local",
            component="cadvisor",
            correlation_id="drift-001",
        )

        result = await adapter.run(plan, context)

        assert result.successful is True
        assert result.rc == 0

        assert len(fake_runner.calls) == 1
        call = fake_runner.calls[0]
        host_vars = call["inventory"]["all"]["hosts"]["srv-01.example.local"]

        assert host_vars["ansible_user"] == "ubuntu"
        assert host_vars["ansible_port"] == 2222
        assert host_vars["ansible_ssh_private_key_file"] == "/tmp/id_rsa"

        assert call["extravars"]["cadvisor_image"] == "gcr.io/cadvisor/cadvisor:v0.49.1"
        assert call["extravars"]["reconciler_request_id"] == "req-001"
        assert call["extravars"]["reconciler_component"] == "cadvisor"
        assert call["extravars"]["reconciler_correlation_id"] == "drift-001"
        assert call["timeout"] == 30

    asyncio.run(scenario())
