from __future__ import annotations

import pytest
from pydantic import ValidationError

from app.schemas.reconcile import ComponentType, ReconcileAcceptedResponse, ReconcileRequest


def test_reconcile_request_requires_fqdn_and_component() -> None:
    with pytest.raises(ValidationError):
        ReconcileRequest()


def test_reconcile_request_rejects_invalid_fqdn() -> None:
    with pytest.raises(ValidationError):
        ReconcileRequest(fqdn="bad host", component=ComponentType.CADVISOR)


def test_reconcile_response_defaults_status_and_message() -> None:
    response = ReconcileAcceptedResponse(
        request_id="req-1",
        component=ComponentType.CADVISOR,
        fqdn="srv-01.example.local",
        accepted_at="2026-01-01T00:00:00Z",
    )

    assert response.status == "accepted"
    assert response.message == "reconcile request accepted"
