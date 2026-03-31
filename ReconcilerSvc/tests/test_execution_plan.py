from __future__ import annotations

from pathlib import Path

from app.config.models import OperatorRuntimeConfig
from app.operators.node_exporter import NodeExporterOperator
from app.schemas.reconcile import ComponentType, ReconcileRequest


def test_node_exporter_execution_plan_builds_expected_values() -> None:
    playbooks_dir = Path(__file__).resolve().parents[1] / "ansible"
    operator = NodeExporterOperator(
        playbooks_dir,
        OperatorRuntimeConfig(
            playbook="node_exporter/playbook.yml",
            default_parameters={"install_method": "package", "version": "1.0"},
        ),
    )

    request = ReconcileRequest(
        host_id="srv-01",
        fqdn="srv-01.example.local",
        component=ComponentType.NODE_EXPORTER,
        parameters={"version": "1.7.0", "custom_flag": True},
    )

    plan = operator.build_execution_plan(request)

    assert plan.component == ComponentType.NODE_EXPORTER
    assert plan.operator_name == "node_exporter_operator"
    assert plan.target_fqdn == "srv-01.example.local"
    assert str(plan.playbook_path).endswith("ansible\\node_exporter\\playbook.yml") or str(plan.playbook_path).endswith("ansible/node_exporter/playbook.yml")
    assert plan.extra_vars["install_method"] == "package"
    assert plan.extra_vars["version"] == "1.7.0"
    assert plan.extra_vars["custom_flag"] is True
