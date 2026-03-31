from __future__ import annotations

from pathlib import Path

import pytest

from app.config.models import OperatorRuntimeConfig
from app.operators.cadvisor import CadvisorOperator
from app.operators.node_exporter import NodeExporterOperator
from app.operators.registry import OperatorNotFoundError, OperatorRegistry
from app.schemas.reconcile import ComponentType


@pytest.fixture
def playbooks_dir() -> Path:
    return Path(__file__).resolve().parents[1] / "ansible"


def test_operator_registry_selects_operator_by_component(playbooks_dir: Path) -> None:
    registry = OperatorRegistry(
        [
            NodeExporterOperator(playbooks_dir, OperatorRuntimeConfig(playbook="node_exporter/playbook.yml")),
            CadvisorOperator(playbooks_dir, OperatorRuntimeConfig(playbook="cadvisor/playbook.yml")),
        ]
    )

    node_op = registry.get_operator(ComponentType.NODE_EXPORTER)
    cadvisor_op = registry.get_operator(ComponentType.CADVISOR)

    assert node_op.operator_name == "node_exporter_operator"
    assert cadvisor_op.operator_name == "cadvisor_operator"


def test_operator_registry_raises_for_missing_operator(playbooks_dir: Path) -> None:
    registry = OperatorRegistry(
        [NodeExporterOperator(playbooks_dir, OperatorRuntimeConfig(playbook="node_exporter/playbook.yml"))]
    )

    with pytest.raises(OperatorNotFoundError):
        registry.get_operator(ComponentType.CADVISOR)
