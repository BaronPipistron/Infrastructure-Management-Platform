from __future__ import annotations

from dataclasses import dataclass, field
from datetime import datetime, timezone
from pathlib import Path
from typing import Any

from app.schemas.reconcile import ComponentType


@dataclass(slots=True)
class ExecutionPlan:
    component: ComponentType
    operator_name: str
    playbook_path: Path
    target_fqdn: str
    extra_vars: dict[str, Any] = field(default_factory=dict)


@dataclass(slots=True)
class ReconcileJob:
    request_id: str
    host_id: str | None
    correlation_id: str | None
    component: ComponentType
    fqdn: str
    plan: ExecutionPlan
    accepted_at: datetime = field(default_factory=lambda: datetime.now(timezone.utc))
