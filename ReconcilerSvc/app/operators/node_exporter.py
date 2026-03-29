from __future__ import annotations

from pathlib import Path

from app.config.models import OperatorRuntimeConfig
from app.execution.models import ExecutionPlan
from app.operators.base import OperatorPlanError, ReconcileOperator
from app.schemas.reconcile import ComponentType, ReconcileRequest


class NodeExporterOperator(ReconcileOperator):
    component = ComponentType.NODE_EXPORTER
    operator_name = "node_exporter_operator"

    def __init__(self, playbooks_base_dir: Path, config: OperatorRuntimeConfig) -> None:
        self._playbook_path = (playbooks_base_dir / config.playbook).resolve()
        self._default_parameters = dict(config.default_parameters)

    def build_execution_plan(self, request: ReconcileRequest) -> ExecutionPlan:
        if not self._playbook_path.exists():
            raise OperatorPlanError(f"Playbook not found: {self._playbook_path}")

        extra_vars = dict(self._default_parameters)
        extra_vars.update(request.parameters)

        return ExecutionPlan(
            component=request.component,
            operator_name=self.operator_name,
            playbook_path=self._playbook_path,
            target_fqdn=request.fqdn,
            extra_vars=extra_vars,
        )
