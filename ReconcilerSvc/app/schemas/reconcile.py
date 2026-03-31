from __future__ import annotations

import re
from datetime import datetime
from enum import Enum
from typing import Any, Literal

from pydantic import BaseModel, Field, field_validator


_FQDN_RE = re.compile(r"^[A-Za-z0-9.-]+$")


class ComponentType(str, Enum):
    NODE_EXPORTER = "node_exporter"
    CADVISOR = "cadvisor"


class ReconcileRequest(BaseModel):
    host_id: str | None = Field(default=None, max_length=128)
    fqdn: str = Field(min_length=1, max_length=253)
    component: ComponentType
    correlation_id: str | None = Field(default=None, max_length=128)
    parameters: dict[str, Any] = Field(default_factory=dict)

    @field_validator("fqdn")
    @classmethod
    def validate_fqdn(cls, value: str) -> str:
        normalized = value.strip()
        if not normalized:
            raise ValueError("fqdn must not be empty")
        if not _FQDN_RE.match(normalized):
            raise ValueError("fqdn must contain only letters, digits, dots and dashes")
        return normalized


class ReconcileAcceptedResponse(BaseModel):
    status: Literal["accepted"] = "accepted"
    message: str = "reconcile request accepted"
    request_id: str
    component: ComponentType
    fqdn: str
    correlation_id: str | None = None
    accepted_at: datetime


class HealthResponse(BaseModel):
    status: str


class ReadinessResponse(BaseModel):
    status: str
