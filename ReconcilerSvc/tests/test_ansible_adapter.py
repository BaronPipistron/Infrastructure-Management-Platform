from __future__ import annotations

import asyncio
import logging
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
        status_handler = kwargs.get("status_handler")
        if callable(status_handler):
            status_handler("running", None)

        event_handler = kwargs.get("event_handler")
        if callable(event_handler):
            event_handler(
                {
                    "event": "runner_on_ok",
                    "stdout": "ok: [srv-01.example.local]",
                    "event_data": {
                        "play": "Reconcile cadvisor",
                        "task": "Ensure docker service is running",
                        "host": "srv-01.example.local",
                        "res": {"changed": False, "failed": False},
                    },
                }
            )

        return _FakeRunnerResult()


def test_ansible_runner_adapter_builds_inventory_and_extravars(monkeypatch, caplog) -> None:
    async def scenario() -> None:
        caplog.set_level(logging.INFO, logger="reconciler.ansible")

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
        assert callable(call["status_handler"])
        assert callable(call["event_handler"])

        assert any(record.getMessage() == "ansible status update" for record in caplog.records)
        ansible_output_records = [record for record in caplog.records if record.getMessage() == "ansible output"]
        assert ansible_output_records
        output_record = ansible_output_records[0]
        assert output_record.__dict__["ansible_stdout"] == "ok: [srv-01.example.local]"
        assert output_record.__dict__["task"] == "Ensure docker service is running"

    asyncio.run(scenario())
