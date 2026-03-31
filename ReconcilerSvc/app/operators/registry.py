from __future__ import annotations

from app.operators.base import ReconcileOperator
from app.schemas.reconcile import ComponentType


class OperatorNotFoundError(LookupError):
    """Raised when no operator supports requested component."""


class OperatorRegistry:
    def __init__(self, operators: list[ReconcileOperator]) -> None:
        self._operators_by_component = {
            operator.component: operator
            for operator in operators
        }

    def get_operator(self, component: ComponentType) -> ReconcileOperator:
        try:
            return self._operators_by_component[component]
        except KeyError as exc:
            raise OperatorNotFoundError(f"No operator registered for component: {component}") from exc
