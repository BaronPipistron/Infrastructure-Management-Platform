from __future__ import annotations

from datetime import datetime, timezone

from fastapi.testclient import TestClient

from app.execution.task_queue import QueueSaturatedError
from app.main import create_app
from app.schemas.reconcile import ComponentType, ReconcileAcceptedResponse


class FakeReconcileService:
    def __init__(self) -> None:
        self.requests = []

    async def accept_reconcile(self, request):
        self.requests.append(request)
        return ReconcileAcceptedResponse(
            request_id="req-001",
            component=request.component,
            fqdn=request.fqdn,
            correlation_id=request.correlation_id,
            accepted_at=datetime.now(timezone.utc),
        )


class QueueFullReconcileService:
    async def accept_reconcile(self, request):
        raise QueueSaturatedError("queue is full")


class FakeRuntime:
    def __init__(self, service, ready: bool = True) -> None:
        self.reconcile_service = service
        self.is_ready = ready

    async def start(self) -> None:
        self.is_ready = True

    async def stop(self) -> None:
        self.is_ready = False


def test_healthz_and_readyz() -> None:
    app = create_app(runtime=FakeRuntime(FakeReconcileService()))
    with TestClient(app) as client:
        health_resp = client.get("/healthz")
        ready_resp = client.get("/readyz")

    assert health_resp.status_code == 200
    assert health_resp.json() == {"status": "ok"}
    assert ready_resp.status_code == 200
    assert ready_resp.json() == {"status": "ready"}


def test_reconcile_accepts_and_returns_202() -> None:
    service = FakeReconcileService()
    app = create_app(runtime=FakeRuntime(service))

    payload = {
        "host_id": "srv-001",
        "fqdn": "srv-001.example.local",
        "component": "node_exporter",
        "correlation_id": "drift-123",
        "parameters": {"force": True},
    }

    with TestClient(app) as client:
        response = client.post("/api/v1/reconcile", json=payload)

    assert response.status_code == 202
    body = response.json()
    assert body["status"] == "accepted"
    assert body["request_id"] == "req-001"
    assert body["component"] == ComponentType.NODE_EXPORTER.value
    assert body["fqdn"] == "srv-001.example.local"
    assert len(service.requests) == 1


def test_reconcile_validation_error_for_invalid_component() -> None:
    app = create_app(runtime=FakeRuntime(FakeReconcileService()))

    payload = {
        "fqdn": "srv-001.example.local",
        "component": "unknown_component",
    }

    with TestClient(app) as client:
        response = client.post("/api/v1/reconcile", json=payload)

    assert response.status_code == 422


def test_reconcile_returns_503_when_queue_is_full() -> None:
    app = create_app(runtime=FakeRuntime(QueueFullReconcileService()))

    payload = {
        "fqdn": "srv-001.example.local",
        "component": "cadvisor",
    }

    with TestClient(app) as client:
        response = client.post("/api/v1/reconcile", json=payload)

    assert response.status_code == 503
    assert response.json()["detail"] == "reconcile queue is full"
