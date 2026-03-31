from __future__ import annotations

from abc import ABC, abstractmethod

from app.execution.models import ExecutionPlan
from app.schemas.reconcile import ComponentType, ReconcileRequest


class OperatorPlanError(RuntimeError):
    """Raised when operator cannot build a reconcile execution plan."""


class ReconcileOperator(ABC):
    component: ComponentType
    operator_name: str

    def supports(self, component: ComponentType) -> bool:
        return self.component == component

    @abstractmethod
    def build_execution_plan(self, request: ReconcileRequest) -> ExecutionPlan:
        """Builds execution plan for a single reconcile request."""
