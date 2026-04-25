#!/usr/bin/env python3
from __future__ import annotations

import argparse
import concurrent.futures
import datetime as dt
import json
import math
import statistics
import subprocess
import sys
import time
import uuid
from dataclasses import dataclass
from pathlib import Path
from typing import Any, Callable
from urllib import error as urlerror
from urllib import request as urlrequest


UTC = dt.timezone.utc


@dataclass(frozen=True)
class HostRecord:
    host_id: str
    fqdn: str
    reachable: bool


class BenchmarkHarness:
    def __init__(self, repo_root: Path, output_root: Path, scenario: str) -> None:
        self.repo_root = repo_root
        self.output_root = output_root
        self.scenario = scenario
        self.run_started_at = dt.datetime.now(UTC)
        self.override_compose_path = self.repo_root / ".local-e2e" / ".runtime" / "benchmarks" / "docker-compose.Benchmark.override.yml"
        self.mutable_files = [
            self.repo_root / ".local-e2e" / "configs" / "inventory" / "appsettings.LocalE2E.yml",
            self.repo_root / ".local-e2e" / "configs" / "inventory" / "selfProvisioning.LocalE2E.yml",
            self.repo_root / ".local-e2e" / "configs" / "drift-detector" / "appsettings.LocalE2E.yml",
            self.repo_root / ".local-e2e" / "configs" / "reconciler" / "appsettings.LocalE2E.yml",
            self.repo_root / ".local-e2e" / "manifests" / "desired-state.local.json",
        ]
        self.backup_contents: dict[Path, bytes] = {}
        self.images_built = False

    @property
    def compose_files(self) -> list[Path]:
        return [
            self.repo_root / "docker-compose.LocalE2E.yml",
            self.override_compose_path,
        ]

    def run(self) -> dict[str, Any]:
        self.output_root.mkdir(parents=True, exist_ok=True)
        (self.output_root / "raw").mkdir(parents=True, exist_ok=True)
        (self.output_root / "summary").mkdir(parents=True, exist_ok=True)
        (self.override_compose_path.parent).mkdir(parents=True, exist_ok=True)

        self._backup_mutable_files()
        results: dict[str, Any] = {}
        try:
            if self.scenario in ("all", "detection-scalability"):
                result = self.benchmark_detection_scalability()
                results["detection_scalability"] = result
                self._write_json(self.output_root / "raw" / "benchmark1_detection_scalability.json", result)

            if self.scenario in ("all", "convergence"):
                result = self.benchmark_convergence_time()
                results["convergence_time"] = result
                self._write_json(self.output_root / "raw" / "benchmark2_convergence_time.json", result)

            if self.scenario in ("all", "reconcile-throughput"):
                result = self.benchmark_reconcile_throughput()
                results["reconcile_throughput"] = result
                self._write_json(self.output_root / "raw" / "benchmark3_reconcile_throughput.json", result)

            if self.scenario in ("all", "partial-data"):
                result = self.benchmark_partial_data_robustness()
                results["partial_data_robustness"] = result
                self._write_json(self.output_root / "raw" / "benchmark4_partial_data_robustness.json", result)

            if self.scenario in ("all", "cooldown"):
                result = self.benchmark_cooldown_efficiency()
                results["cooldown_efficiency"] = result
                self._write_json(self.output_root / "raw" / "benchmark5_cooldown_efficiency.json", result)
        finally:
            self._compose_down()
            self._restore_mutable_files()

        summary = self._build_summary(results)
        self._write_json(self.output_root / "summary" / "summary.json", summary)
        report = self._render_markdown_report(results)
        (self.output_root / "summary" / "summary.md").write_text(report, encoding="utf-8")
        return summary

    # ------------------------------
    # Benchmarks
    # ------------------------------

    def benchmark_detection_scalability(self) -> dict[str, Any]:
        host_counts = [1, 10, 100, 500]
        cases: list[dict[str, Any]] = []
        notes: list[str] = []

        for host_count in host_counts:
            hosts = self._build_hosts(host_count, missing_hosts=0)
            scenario_started_at = self._prepare_environment(
                hosts=hosts,
                detection_interval="10m",
                reconcile_cooldown="30s",
                max_parallel_reconciliations=2,
                workloads_enabled=True,
            )
            convergence = self._ensure_converged(max_attempts=8, pause_seconds=2.0)

            cycles: list[dict[str, Any]] = []
            for _ in range(5):
                cycles.append(self._run_detection_cycle())
                time.sleep(0.8)

            total_ms = [int(c["durationMs"]) for c in cycles]
            inventory_fetch_ms = [self._cycle_stage_ms(c, "InventoryFetchMs") for c in cycles]
            parser_fetch_ms = [self._cycle_stage_ms(c, "ParserFetchMs") for c in cycles]
            compare_ms = [self._cycle_stage_ms(c, "DriftComparisonMs") for c in cycles]
            dispatch_ms = [self._cycle_stage_ms(c, "ReconcileDispatchMs") for c in cycles]
            drifts = [self._cycle_stat(c, "DriftsFound") for c in cycles]
            reconcile_sent = [self._cycle_stat(c, "ReconcileSent") for c in cycles]

            if any(value > 0 for value in reconcile_sent):
                notes.append(
                    f"host_count={host_count}: observed reconcile dispatch in measured cycles "
                    f"(max reconcileSent={max(reconcile_sent)})."
                )
            else:
                notes.append(
                    f"host_count={host_count}: measured cycles were steady-state (no reconcile dispatch)."
                )

            cases.append(
                {
                    "host_count": host_count,
                    "scenario_started_at": scenario_started_at.isoformat(),
                    "logical_scaling_mode": "single-managed-host-with-dns-aliases",
                    "convergence_before_measurement": convergence,
                    "samples": cycles,
                    "aggregates": {
                        "cycle_total_ms": self._series_stats(total_ms),
                        "inventory_fetch_ms": self._series_stats(inventory_fetch_ms),
                        "parser_fetch_ms": self._series_stats(parser_fetch_ms),
                        "drift_compare_ms": self._series_stats(compare_ms),
                        "reconcile_dispatch_ms": self._series_stats(dispatch_ms),
                        "drifts_found_per_cycle": self._series_stats(drifts),
                        "reconcile_sent_per_cycle": self._series_stats(reconcile_sent),
                    },
                }
            )

        return {
            "benchmark": "Benchmark 1: Detection scalability",
            "host_counts": host_counts,
            "cases": cases,
            "notes": notes,
        }

    def benchmark_convergence_time(self) -> dict[str, Any]:
        hosts = self._build_hosts(1, missing_hosts=0)
        self._prepare_environment(
            hosts=hosts,
            detection_interval="10m",
            reconcile_cooldown="30s",
            max_parallel_reconciliations=2,
            workloads_enabled=True,
        )
        convergence_before_break = self._ensure_converged(max_attempts=10, pause_seconds=2.0)
        target_host = hosts[0]

        self._run_cmd(
            [
                "docker",
                "exec",
                "managed-host",
                "docker",
                "rm",
                "-f",
                "node_exporter",
            ],
            check=False,
        )
        injected_at = dt.datetime.now(UTC)

        first_detection_cycle: dict[str, Any] | None = None
        for _ in range(6):
            cycle = self._run_detection_cycle()
            if self._cycle_stat(cycle, "DriftsFound") > 0:
                first_detection_cycle = cycle
                break
            time.sleep(1.0)
        if first_detection_cycle is None:
            raise RuntimeError("Failed to observe drift after host break injection.")

        cycle_id = str(first_detection_cycle["cycleId"])
        first_detection_started_at = self._parse_iso8601(str(first_detection_cycle["startedAt"]))

        drift_send_event = self._wait_for_log_event(
            container_name="drift-detector-svc",
            since=injected_at - dt.timedelta(seconds=5),
            timeout_seconds=180,
            predicate=lambda event: event.get("msg") == "reconcile request accepted"
            and event.get("cycle_id") == cycle_id
            and event.get("component") == "node_exporter",
        )
        reconcile_request_id = None
        reconcile_sent_at = None
        if drift_send_event is not None:
            reconcile_request_id = str(drift_send_event.get("request_id"))
            reconcile_sent_at = self._event_time(drift_send_event)

        reconcile_finished_event = None
        reconcile_finished_at = None
        if reconcile_request_id:
            reconcile_finished_event = self._wait_for_log_event(
                container_name="reconciler-svc",
                since=injected_at - dt.timedelta(seconds=5),
                timeout_seconds=900,
                predicate=lambda event: event.get("message") == "reconcile execution finished"
                and event.get("request_id") == reconcile_request_id,
            )
            if reconcile_finished_event is not None:
                reconcile_finished_at = self._event_time(reconcile_finished_event)

        restored_at = self._wait_for_inventory_workload(
            host_id=target_host.host_id,
            workload_name="node_exporter",
            timeout_seconds=240,
        )

        metrics = {
            "inject_to_first_detection_ms": self._delta_ms(injected_at, first_detection_started_at),
            "detection_to_reconcile_send_ms": self._delta_ms(first_detection_started_at, reconcile_sent_at),
            "reconcile_send_to_workload_restored_ms": self._delta_ms(reconcile_sent_at, restored_at),
            "reconcile_send_to_reconcile_finished_ms": self._delta_ms(reconcile_sent_at, reconcile_finished_at),
            "total_convergence_ms": self._delta_ms(injected_at, restored_at),
        }

        return {
            "benchmark": "Benchmark 2: Convergence time",
            "target_host": {
                "host_id": target_host.host_id,
                "fqdn": target_host.fqdn,
            },
            "convergence_before_break": convergence_before_break,
            "injected_at": injected_at.isoformat(),
            "first_detection_cycle": first_detection_cycle,
            "drift_reconcile_event": drift_send_event,
            "reconcile_request_id": reconcile_request_id,
            "reconcile_finished_event": reconcile_finished_event,
            "workload_restored_at": restored_at.isoformat(),
            "metrics_ms": metrics,
        }

    def benchmark_reconcile_throughput(self) -> dict[str, Any]:
        steady_cases: list[dict[str, Any]] = []
        request_count_per_case = 90

        for parallelism in (1, 2, 4):
            hosts = self._build_hosts(max(request_count_per_case, 100), missing_hosts=0)
            started_at = self._prepare_environment(
                hosts=hosts,
                detection_interval="10m",
                reconcile_cooldown="30s",
                max_parallel_reconciliations=parallelism,
                workloads_enabled=False,
            )
            case = self._run_reconcile_load_case(
                case_name=f"steady_parallel_{parallelism}",
                hosts=hosts,
                request_count=request_count_per_case,
                parallelism=parallelism,
                scenario_started_at=started_at,
            )
            steady_cases.append(case)

        saturation_hosts = self._build_hosts(200, missing_hosts=0)
        saturation_started_at = self._prepare_environment(
            hosts=saturation_hosts,
            detection_interval="10m",
            reconcile_cooldown="30s",
            max_parallel_reconciliations=2,
            workloads_enabled=False,
        )
        saturation_case = self._run_reconcile_load_case(
            case_name="saturation_probe_parallel_2",
            hosts=saturation_hosts,
            request_count=180,
            parallelism=2,
            scenario_started_at=saturation_started_at,
        )

        saturation_point = None
        if int(saturation_case["requests"]["rejected_total"]) > 0:
            saturation_point = 180

        return {
            "benchmark": "Benchmark 3: Reconcile throughput",
            "steady_cases": steady_cases,
            "saturation_case": saturation_case,
            "observed_saturation_request_burst": saturation_point,
            "notes": [
                "Throughput is measured by completed reconcile executions per second from Reconciler logs.",
                "Saturation probe is based on HTTP 503 queue-full responses during a burst load.",
            ],
        }

    def benchmark_partial_data_robustness(self) -> dict[str, Any]:
        total_hosts = 20
        missing_hosts = 6
        hosts = self._build_hosts(total_hosts, missing_hosts=missing_hosts)
        scenario_started_at = self._prepare_environment(
            hosts=hosts,
            detection_interval="10m",
            reconcile_cooldown="30s",
            max_parallel_reconciliations=2,
            workloads_enabled=True,
        )

        for _ in range(3):
            self._run_detection_cycle()
            time.sleep(1.0)

        cycle = self._run_detection_cycle()
        cycle_id = str(cycle["cycleId"])
        inventory_snapshot = self._fetch_inventory_hosts()

        events = self._docker_logs_json("drift-detector-svc", since=scenario_started_at - dt.timedelta(seconds=5))
        cycle_events = [event for event in events if str(event.get("cycle_id")) == cycle_id]

        warnings_missing_data = [
            event for event in cycle_events if event.get("msg") == "skip detection due to missing actual data"
        ]
        warnings_missing_host = [
            event for event in cycle_events if event.get("msg") == "actual host missing, skip host"
        ]
        reconcile_events = [event for event in cycle_events if event.get("msg") == "reconcile request accepted"]

        missing_prefix = "missing-host-"
        node_exporter_missing_reconciles = [
            event
            for event in reconcile_events
            if event.get("component") == "node_exporter"
            and str(event.get("fqdn", "")).startswith(missing_prefix)
        ]
        cadvisor_missing_reconciles = [
            event
            for event in reconcile_events
            if event.get("component") == "cadvisor"
            and str(event.get("fqdn", "")).startswith(missing_prefix)
        ]

        return {
            "benchmark": "Benchmark 4: Partial data robustness",
            "total_hosts": total_hosts,
            "missing_hosts": missing_hosts,
            "cycle": cycle,
            "inventory_metadata": inventory_snapshot.get("metadata", {}),
            "warnings_missing_actual_data_count": len(warnings_missing_data),
            "warnings_missing_actual_host_count": len(warnings_missing_host),
            "reconcile_events_total": len(reconcile_events),
            "node_exporter_reconcile_for_missing_hosts": len(node_exporter_missing_reconciles),
            "cadvisor_reconcile_for_missing_hosts": len(cadvisor_missing_reconciles),
            "sample_warnings_missing_actual_data": warnings_missing_data[:8],
            "sample_reconcile_events": reconcile_events[:8],
        }

    def benchmark_cooldown_efficiency(self) -> dict[str, Any]:
        no_cooldown = self._run_cooldown_subscenario("0s")
        with_cooldown = self._run_cooldown_subscenario("60s")

        baseline_sent = int(no_cooldown["totals"]["reconcile_sent"])
        with_cooldown_sent = int(with_cooldown["totals"]["reconcile_sent"])
        prevented = max(baseline_sent - with_cooldown_sent, 0)
        suppression_ratio = round((prevented / baseline_sent), 4) if baseline_sent > 0 else None

        return {
            "benchmark": "Benchmark 5: Cooldown / anti-spam efficiency",
            "without_cooldown": no_cooldown,
            "with_cooldown": with_cooldown,
            "derived": {
                "would_send_without_cooldown": baseline_sent,
                "sent_with_cooldown": with_cooldown_sent,
                "prevented_reconcile_commands": prevented,
                "suppression_ratio": suppression_ratio,
            },
        }

    # ------------------------------
    # Scenario helpers
    # ------------------------------

    def _run_cooldown_subscenario(self, cooldown: str) -> dict[str, Any]:
        hosts = self._build_hosts(12, missing_hosts=4)
        started_at = self._prepare_environment(
            hosts=hosts,
            detection_interval="10m",
            reconcile_cooldown=cooldown,
            max_parallel_reconciliations=2,
            workloads_enabled=True,
        )
        cycles: list[dict[str, Any]] = []
        for _ in range(5):
            cycles.append(self._run_detection_cycle())
            time.sleep(1.5)

        totals = {
            "drifts_found": sum(self._cycle_stat(c, "DriftsFound") for c in cycles),
            "reconcile_sent": sum(self._cycle_stat(c, "ReconcileSent") for c in cycles),
            "reconcile_suppressed": sum(self._cycle_stat(c, "ReconcileSuppressed") for c in cycles),
            "errors": sum(self._cycle_stat(c, "Errors") for c in cycles),
        }

        return {
            "cooldown": cooldown,
            "scenario_started_at": started_at.isoformat(),
            "cycles": cycles,
            "totals": totals,
        }

    def _run_reconcile_load_case(
        self,
        case_name: str,
        hosts: list[HostRecord],
        request_count: int,
        parallelism: int,
        scenario_started_at: dt.datetime,
    ) -> dict[str, Any]:
        burst = self._send_reconcile_burst(hosts=hosts, request_count=request_count, component="node_exporter")
        accepted_ids = [item["request_id"] for item in burst["accepted"]]
        completions = self._wait_for_reconcile_completions(
            request_ids=accepted_ids,
            since=scenario_started_at - dt.timedelta(seconds=5),
            timeout_seconds=900,
        )

        completed_events = list(completions.values())
        successful_events = [event for event in completed_events if bool(event.get("successful"))]
        completion_times = [self._event_time(event) for event in completed_events if self._event_time(event) is not None]
        completion_throughput = None
        if len(completion_times) >= 2:
            span_seconds = (max(completion_times) - min(completion_times)).total_seconds()
            if span_seconds > 0:
                completion_throughput = round(len(successful_events) / span_seconds, 4)

        durations = [float(event.get("duration_seconds", 0.0)) for event in completed_events]
        avg_duration = round(statistics.fmean(durations), 4) if durations else None

        return {
            "case_name": case_name,
            "parallelism": parallelism,
            "request_count": request_count,
            "requests": {
                "accepted_total": len(burst["accepted"]),
                "rejected_total": len(burst["rejected"]),
                "accepted_per_second": burst["accepted_per_second"],
                "accepted_window_seconds": burst["accepted_window_seconds"],
                "rejected_samples": burst["rejected"][:10],
            },
            "completions": {
                "observed_total": len(completed_events),
                "successful_total": len(successful_events),
                "completion_throughput_success_per_second": completion_throughput,
                "avg_execution_duration_seconds": avg_duration,
            },
        }

    def _send_reconcile_burst(self, hosts: list[HostRecord], request_count: int, component: str) -> dict[str, Any]:
        endpoint = "http://localhost:18084/api/v1/reconcile"
        selected_hosts = hosts[:request_count]
        if len(selected_hosts) < request_count:
            raise RuntimeError("Not enough hosts for requested reconcile burst.")

        accepted: list[dict[str, Any]] = []
        rejected: list[dict[str, Any]] = []
        call_started = dt.datetime.now(UTC)

        def one_call(host: HostRecord, index: int) -> dict[str, Any]:
            payload = {
                "host_id": host.host_id,
                "fqdn": host.fqdn,
                "component": component,
                "correlation_id": f"bench-throughput-{uuid.uuid4()}",
                "parameters": {},
            }
            started_at = dt.datetime.now(UTC)
            status, body = self._request_json("POST", endpoint, payload=payload, timeout_seconds=20.0)
            finished_at = dt.datetime.now(UTC)
            return {
                "index": index,
                "host_id": host.host_id,
                "fqdn": host.fqdn,
                "status": status,
                "body": body,
                "started_at": started_at,
                "finished_at": finished_at,
            }

        worker_count = min(32, max(4, request_count))
        with concurrent.futures.ThreadPoolExecutor(max_workers=worker_count) as executor:
            futures = [executor.submit(one_call, host, idx) for idx, host in enumerate(selected_hosts)]
            for future in concurrent.futures.as_completed(futures):
                result = future.result()
                status = int(result["status"])
                body = result["body"]
                if status == 202 and isinstance(body, dict):
                    accepted.append(
                        {
                            "host_id": result["host_id"],
                            "fqdn": result["fqdn"],
                            "request_id": body.get("request_id"),
                            "accepted_at": body.get("accepted_at"),
                        }
                    )
                else:
                    rejected.append(
                        {
                            "host_id": result["host_id"],
                            "fqdn": result["fqdn"],
                            "status": status,
                            "body": body,
                        }
                    )

        call_finished = dt.datetime.now(UTC)
        accepted_window_seconds = (call_finished - call_started).total_seconds()
        accepted_per_second = round((len(accepted) / accepted_window_seconds), 4) if accepted_window_seconds > 0 else None
        return {
            "accepted": accepted,
            "rejected": rejected,
            "accepted_window_seconds": round(accepted_window_seconds, 4),
            "accepted_per_second": accepted_per_second,
        }

    def _wait_for_reconcile_completions(
        self,
        request_ids: list[str],
        since: dt.datetime,
        timeout_seconds: int,
    ) -> dict[str, dict[str, Any]]:
        expected = {request_id for request_id in request_ids if request_id}
        observed: dict[str, dict[str, Any]] = {}
        if not expected:
            return observed

        deadline = time.time() + timeout_seconds
        while time.time() < deadline:
            events = self._docker_logs_json("reconciler-svc", since=since)
            for event in events:
                if event.get("message") != "reconcile execution finished":
                    continue
                request_id = str(event.get("request_id", ""))
                if request_id in expected and request_id not in observed:
                    observed[request_id] = event
            if len(observed) == len(expected):
                return observed
            time.sleep(2.0)
        return observed

    def _prepare_environment(
        self,
        hosts: list[HostRecord],
        detection_interval: str,
        reconcile_cooldown: str,
        max_parallel_reconciliations: int,
        workloads_enabled: bool,
    ) -> dt.datetime:
        self._write_inventory_appsettings(sync_interval="5s")
        self._write_self_provisioning(hosts)
        self._write_desired_manifest(hosts, workloads_enabled=workloads_enabled)
        self._write_drift_detector_appsettings(
            detection_interval=detection_interval,
            reconcile_cooldown=reconcile_cooldown,
        )
        self._write_reconciler_appsettings(max_parallel_reconciliations=max_parallel_reconciliations)
        self._write_compose_override(hosts)
        self._ensure_ssh_keypair()

        self._compose_down()
        self._compose_up(build=not self.images_built)
        self.images_built = True
        self._wait_for_readiness(timeout_seconds=360)
        return dt.datetime.now(UTC)

    def _build_hosts(self, total_hosts: int, missing_hosts: int) -> list[HostRecord]:
        if total_hosts <= 0:
            raise ValueError("total_hosts must be > 0")
        missing_hosts = max(0, min(missing_hosts, total_hosts))
        reachable_hosts = total_hosts - missing_hosts
        hosts: list[HostRecord] = []
        for index in range(1, total_hosts + 1):
            host_id = f"managed-host-{index:04d}"
            if index <= reachable_hosts:
                fqdn = f"bench-host-{index:04d}"
                reachable = True
            else:
                fqdn = f"missing-host-{index:04d}"
                reachable = False
            hosts.append(HostRecord(host_id=host_id, fqdn=fqdn, reachable=reachable))
        return hosts

    def _wait_for_inventory_workload(
        self,
        host_id: str,
        workload_name: str,
        timeout_seconds: int,
    ) -> dt.datetime:
        deadline = time.time() + timeout_seconds
        while time.time() < deadline:
            payload = self._fetch_inventory_hosts()
            hosts = payload.get("hosts", [])
            for host in hosts:
                if str(host.get("id")) != host_id:
                    continue
                workloads = host.get("workloads") or []
                names = {str(item.get("name", "")) for item in workloads}
                if workload_name in names:
                    return dt.datetime.now(UTC)
            time.sleep(2.0)
        raise RuntimeError(f"Timed out waiting for workload {workload_name} on host {host_id}.")

    def _ensure_converged(self, max_attempts: int, pause_seconds: float) -> dict[str, Any]:
        attempts: list[dict[str, Any]] = []
        for _ in range(max_attempts):
            cycle = self._run_detection_cycle()
            attempts.append(cycle)
            drifts_found = self._cycle_stat(cycle, "DriftsFound")
            errors = self._cycle_stat(cycle, "Errors")
            if drifts_found == 0 and errors == 0:
                return {"converged": True, "attempts": attempts}
            time.sleep(pause_seconds)
        return {"converged": False, "attempts": attempts}

    def _run_detection_cycle(self) -> dict[str, Any]:
        endpoint = "http://localhost:18083/api/v1/detection/run"
        for _ in range(12):
            status, body = self._request_json("POST", endpoint, payload=None, timeout_seconds=45.0)
            if status == 200 and isinstance(body, dict):
                return body["result"]
            if status == 409:
                time.sleep(1.0)
                continue
            raise RuntimeError(f"Detection cycle request failed: status={status} body={body}")
        raise RuntimeError("Detection cycle request kept returning 409 conflict.")

    def _fetch_inventory_hosts(self) -> dict[str, Any]:
        status, payload = self._request_json("GET", "http://localhost:18080/api/v1/hosts", timeout_seconds=30.0)
        if status != 200 or not isinstance(payload, dict):
            raise RuntimeError(f"Failed to fetch inventory hosts: status={status} payload={payload}")
        return payload

    # ------------------------------
    # Compose + system helpers
    # ------------------------------

    def _compose_up(self, build: bool) -> None:
        command = ["docker", "compose"]
        for file_path in self.compose_files:
            command.extend(["-f", str(file_path)])
        command.extend(["up", "-d", "--force-recreate"])
        if build:
            command.append("--build")
        self._run_cmd(command, check=True)

    def _compose_down(self) -> None:
        command = ["docker", "compose"]
        for file_path in self.compose_files:
            command.extend(["-f", str(file_path)])
        command.extend(["down", "--remove-orphans"])
        self._run_cmd(command, check=False)

    def _wait_for_readiness(self, timeout_seconds: int) -> None:
        endpoints = {
            "inventory": "http://localhost:18080/readyz",
            "parser": "http://localhost:18082/readyz",
            "drift_detector": "http://localhost:18083/readyz",
            "reconciler": "http://localhost:18084/readyz",
        }
        deadline = time.time() + timeout_seconds
        last_statuses: dict[str, int | str] = {}
        while time.time() < deadline:
            ready = True
            for name, url in endpoints.items():
                try:
                    status, _ = self._request_json("GET", url, timeout_seconds=10.0)
                except Exception as exc:  # noqa: BLE001
                    last_statuses[name] = f"error: {exc}"
                    ready = False
                    continue
                last_statuses[name] = status
                if status != 200:
                    ready = False
            if ready:
                return
            time.sleep(2.0)
        raise RuntimeError(f"Readiness timeout. Last statuses: {last_statuses}")

    def _ensure_ssh_keypair(self) -> None:
        runtime_ssh_dir = self.repo_root / ".local-e2e" / ".runtime" / "ssh"
        runtime_ssh_dir.mkdir(parents=True, exist_ok=True)
        private_key = runtime_ssh_dir / "reconciler_local_e2e_key"
        public_key = runtime_ssh_dir / "reconciler_local_e2e_key.pub"
        if private_key.exists():
            private_key.unlink()
        if public_key.exists():
            public_key.unlink()

        self._run_cmd(
            [
                "ssh-keygen",
                "-q",
                "-t",
                "ed25519",
                "-N",
                "",
                "-C",
                "imp-local-e2e-runtime",
                "-f",
                str(private_key),
            ],
            check=False,
        )
        if not private_key.exists():
            raise RuntimeError("ssh-keygen did not create private key.")
        if not public_key.exists():
            result = self._run_cmd(["ssh-keygen", "-y", "-f", str(private_key)], check=True)
            public_key.write_text(result.stdout.strip(), encoding="ascii")

    # ------------------------------
    # File rendering
    # ------------------------------

    def _write_inventory_appsettings(self, sync_interval: str) -> None:
        content = (
            "server:\n"
            "  port: 8080\n"
            "\n"
            "logging:\n"
            "  level: info\n"
            "\n"
            "sync:\n"
            f"  interval: {sync_interval}\n"
            "\n"
            "sources:\n"
            "  cadvisor:\n"
            "    isEnabled: true\n"
            "    includeSystemWorkloads: false\n"
            "    scheme: http\n"
            "    port: 8080\n"
            "    basePath: /api/v1.3\n"
            "    containersPath: /subcontainers/\n"
            "    timeout: 5s\n"
            '    urlTemplate: "{{scheme}}://{{fqdn}}:{{port}}{{basePath}}{{containersPath}}"\n'
            "  netbox:\n"
            "    isEnabled: false\n"
            "  otherSources:\n"
            "    isEnabled: false\n"
            "\n"
            "bootstrap:\n"
            "  selfProvisioningPath: ./selfProvisioning.LocalE2E.yml\n"
        )
        target = self.repo_root / ".local-e2e" / "configs" / "inventory" / "appsettings.LocalE2E.yml"
        target.write_text(content, encoding="utf-8")

    def _write_self_provisioning(self, hosts: list[HostRecord]) -> None:
        lines = ["hosts:"]
        for host in hosts:
            lines.extend(
                [
                    f"  - id: {host.host_id}",
                    f"    fqdn: {host.fqdn}",
                    "    labels:",
                    "      managed_by: platform-team",
                    "      purpose: compute",
                    "      env: dev",
                    "      role: benchmark",
                    "      region: local",
                    "      owner: platform",
                ]
            )
        content = "\n".join(lines) + "\n"
        target = self.repo_root / ".local-e2e" / "configs" / "inventory" / "selfProvisioning.LocalE2E.yml"
        target.write_text(content, encoding="utf-8")

    def _write_desired_manifest(self, hosts: list[HostRecord], workloads_enabled: bool) -> None:
        enabled_value = "true" if workloads_enabled else "false"
        deployment_nodes: list[dict[str, Any]] = []
        for host in hosts:
            deployment_nodes.append(
                {
                    "id": f"node-{host.host_id}",
                    "name": host.fqdn,
                    "environment": "LocalE2E-Benchmark",
                    "properties": {
                        "host_id": host.host_id,
                        "fqdn": host.fqdn,
                        "env": "dev",
                        "managed_by": "platform-team",
                        "purpose": "compute",
                        "region": "local",
                        "owner": "platform",
                    },
                    "containerInstances": [
                        {
                            "id": f"{host.host_id}-node-exporter",
                            "containerId": "2",
                            "properties": {
                                "enabled": enabled_value,
                                "deployment_mode": "container",
                                "image": "quay.io/prometheus/node-exporter:v1.8.2",
                                "port": "9100",
                            },
                        },
                        {
                            "id": f"{host.host_id}-cadvisor",
                            "containerId": "3",
                            "properties": {
                                "enabled": enabled_value,
                                "deployment_mode": "container",
                                "image": "gcr.io/cadvisor/cadvisor:v0.49.1",
                                "port": "8080",
                            },
                        },
                    ],
                }
            )

        manifest = {
            "name": "Infrastructure Management Local E2E Benchmark Desired State",
            "model": {
                "softwareSystems": [
                    {
                        "id": "1",
                        "name": "Managed Workloads",
                        "containers": [
                            {"id": "2", "name": "node_exporter", "technology": "container", "properties": {}},
                            {"id": "3", "name": "cadvisor", "technology": "container", "properties": {}},
                        ],
                    }
                ],
                "deploymentNodes": deployment_nodes,
            },
        }
        target = self.repo_root / ".local-e2e" / "manifests" / "desired-state.local.json"
        target.write_text(json.dumps(manifest, indent=2, ensure_ascii=False) + "\n", encoding="utf-8")

    def _write_drift_detector_appsettings(self, detection_interval: str, reconcile_cooldown: str) -> None:
        content = (
            "server:\n"
            "  host: 0.0.0.0\n"
            "  port: 8080\n"
            "\n"
            "logging:\n"
            "  level: info\n"
            "\n"
            "detection:\n"
            f"  interval: {detection_interval}\n"
            "  enabledComponents:\n"
            "    - node_exporter\n"
            "    - cadvisor\n"
            "\n"
            "antiSpam:\n"
            f"  reconcileCooldown: {reconcile_cooldown}\n"
            "\n"
            "clients:\n"
            "  inventory:\n"
            "    baseURL: http://inventory-svc:8080\n"
            "    hostsPath: /api/v1/hosts\n"
            "    timeout: 5s\n"
            "    retry:\n"
            "      maxAttempts: 2\n"
            "      backoff: 300ms\n"
            "  parser:\n"
            "    baseURL: http://parser-svc:8080\n"
            "    desiredStatePath: /api/v1/desired-state\n"
            "    timeout: 5s\n"
            "    retry:\n"
            "      maxAttempts: 2\n"
            "      backoff: 300ms\n"
            "  reconciler:\n"
            "    baseURL: http://reconciler-svc:8082\n"
            "    reconcilePath: /api/v1/reconcile\n"
            "    timeout: 10s\n"
            "    retry:\n"
            "      maxAttempts: 2\n"
            "      backoff: 300ms\n"
        )
        target = self.repo_root / ".local-e2e" / "configs" / "drift-detector" / "appsettings.LocalE2E.yml"
        target.write_text(content, encoding="utf-8")

    def _write_reconciler_appsettings(self, max_parallel_reconciliations: int) -> None:
        content = (
            "server:\n"
            "  host: 0.0.0.0\n"
            "  port: 8082\n"
            "\n"
            "logging:\n"
            "  level: INFO\n"
            "\n"
            "reconciler:\n"
            f"  max_parallel_reconciliations: {max_parallel_reconciliations}\n"
            "  queue_capacity: 100\n"
            "  enqueue_timeout_seconds: 1.0\n"
            "\n"
            "ansible:\n"
            "  playbooks_base_dir: /etc/reconciler-svc/ansible\n"
            "  private_data_dir: /tmp/.ansible-runner\n"
            "  run_timeout_seconds: 900\n"
            "  ssh:\n"
            "    user_env_var: ANSIBLE_SSH_USER\n"
            "    private_key_path_env_var: ANSIBLE_SSH_PRIVATE_KEY_PATH\n"
            "    port_env_var: ANSIBLE_SSH_PORT\n"
            "    host_key_checking_env_var: ANSIBLE_HOST_KEY_CHECKING\n"
            "    default_port: 22\n"
            "\n"
            "operators:\n"
            "  node_exporter:\n"
            "    enabled: true\n"
            "    playbook: node_exporter/playbook.yml\n"
            "    default_parameters:\n"
            "      node_exporter_image: quay.io/prometheus/node-exporter:v1.8.2\n"
            "      node_exporter_port: 9100\n"
            "  cadvisor:\n"
            "    enabled: true\n"
            "    playbook: cadvisor/playbook.yml\n"
            "    default_parameters:\n"
            "      cadvisor_image: gcr.io/cadvisor/cadvisor:v0.49.1\n"
            "      cadvisor_port: 8080\n"
        )
        target = self.repo_root / ".local-e2e" / "configs" / "reconciler" / "appsettings.LocalE2E.yml"
        target.write_text(content, encoding="utf-8")

    def _write_compose_override(self, hosts: list[HostRecord]) -> None:
        aliases = sorted({host.fqdn for host in hosts if host.reachable})
        if "managed-host" not in aliases:
            aliases.insert(0, "managed-host")

        lines = [
            "services:",
            "  managed-host:",
            "    networks:",
            "      imp-local-e2e:",
            "        aliases:",
        ]
        for alias in aliases:
            lines.append(f"          - {alias}")
        self.override_compose_path.write_text("\n".join(lines) + "\n", encoding="utf-8")

    # ------------------------------
    # Logs, HTTP, cmd utilities
    # ------------------------------

    def _wait_for_log_event(
        self,
        container_name: str,
        since: dt.datetime,
        timeout_seconds: int,
        predicate: Callable[[dict[str, Any]], bool],
    ) -> dict[str, Any] | None:
        deadline = time.time() + timeout_seconds
        while time.time() < deadline:
            events = self._docker_logs_json(container_name, since=since)
            for event in events:
                if predicate(event):
                    return event
            time.sleep(2.0)
        return None

    def _docker_logs_json(self, container_name: str, since: dt.datetime | None) -> list[dict[str, Any]]:
        command = ["docker", "logs"]
        if since is not None:
            command.extend(["--since", since.isoformat().replace("+00:00", "Z")])
        command.append(container_name)
        result = self._run_cmd(command, check=False)
        raw_output = (result.stdout or "") + "\n" + (result.stderr or "")
        events: list[dict[str, Any]] = []
        for line in raw_output.splitlines():
            stripped = line.strip()
            if not stripped.startswith("{"):
                continue
            try:
                event = json.loads(stripped)
            except json.JSONDecodeError:
                continue
            if isinstance(event, dict):
                events.append(event)
        events.sort(key=lambda item: self._event_time(item) or dt.datetime.min.replace(tzinfo=UTC))
        return events

    def _event_time(self, event: dict[str, Any]) -> dt.datetime | None:
        if "timestamp" in event:
            value = str(event["timestamp"])
            try:
                return self._parse_iso8601(value)
            except ValueError:
                return None
        if "ts" in event:
            try:
                return dt.datetime.fromtimestamp(float(event["ts"]), tz=UTC)
            except (TypeError, ValueError):
                return None
        return None

    def _request_json(
        self,
        method: str,
        url: str,
        payload: dict[str, Any] | None = None,
        timeout_seconds: float = 20.0,
    ) -> tuple[int, Any]:
        body = None
        headers = {}
        if payload is not None:
            body = json.dumps(payload).encode("utf-8")
            headers["Content-Type"] = "application/json"

        request = urlrequest.Request(url=url, method=method.upper(), data=body, headers=headers)
        try:
            with urlrequest.urlopen(request, timeout=timeout_seconds) as response:
                raw_body = response.read().decode("utf-8")
                if raw_body.strip() == "":
                    return int(response.status), {}
                try:
                    return int(response.status), json.loads(raw_body)
                except json.JSONDecodeError:
                    return int(response.status), {"raw": raw_body}
        except urlerror.HTTPError as exc:
            raw_body = exc.read().decode("utf-8", errors="replace")
            if raw_body.strip() == "":
                return int(exc.code), {}
            try:
                return int(exc.code), json.loads(raw_body)
            except json.JSONDecodeError:
                return int(exc.code), {"raw": raw_body}
        except urlerror.URLError as exc:
            raise RuntimeError(f"Request to {url} failed: {exc}") from exc

    def _run_cmd(self, command: list[str], check: bool) -> subprocess.CompletedProcess[str]:
        result = subprocess.run(
            command,
            cwd=self.repo_root,
            text=True,
            capture_output=True,
            encoding="utf-8",
            errors="replace",
        )
        if check and result.returncode != 0:
            raise RuntimeError(
                f"Command failed ({' '.join(command)}), exit={result.returncode}\n"
                f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
            )
        return result

    def _parse_iso8601(self, value: str) -> dt.datetime:
        normalized = value.strip()
        if normalized.endswith("Z"):
            normalized = normalized[:-1] + "+00:00"
        parsed = dt.datetime.fromisoformat(normalized)
        if parsed.tzinfo is None:
            parsed = parsed.replace(tzinfo=UTC)
        return parsed.astimezone(UTC)

    # ------------------------------
    # Backup/restore and report
    # ------------------------------

    def _backup_mutable_files(self) -> None:
        self.backup_contents.clear()
        for path in self.mutable_files:
            self.backup_contents[path] = path.read_bytes()

    def _restore_mutable_files(self) -> None:
        for path, content in self.backup_contents.items():
            path.write_bytes(content)

    def _build_summary(self, results: dict[str, Any]) -> dict[str, Any]:
        return {
            "run_started_at": self.run_started_at.isoformat(),
            "run_finished_at": dt.datetime.now(UTC).isoformat(),
            "scenario": self.scenario,
            "results_available": list(results.keys()),
        }

    def _render_markdown_report(self, results: dict[str, Any]) -> str:
        lines: list[str] = []
        lines.append("# E2E Benchmark Report")
        lines.append("")
        lines.append(f"- Generated at: `{dt.datetime.now(UTC).isoformat()}`")
        lines.append(f"- Run mode: `{self.scenario}`")
        lines.append("- Scaling mode: `single managed-host + DNS aliases (logical hosts)`")
        lines.append("")

        benchmark1 = results.get("detection_scalability")
        if benchmark1:
            lines.append("## Benchmark 1: Detection Scalability")
            lines.append("")
            lines.append("| Hosts | Cycle p50 (ms) | Cycle p95 (ms) | Inventory p50 (ms) | Parser p50 (ms) | Compare p50 (ms) | Dispatch p50 (ms) |")
            lines.append("|---:|---:|---:|---:|---:|---:|---:|")
            for case in benchmark1["cases"]:
                aggregate = case["aggregates"]
                lines.append(
                    "| {hosts} | {cycle_p50} | {cycle_p95} | {inv_p50} | {parser_p50} | {cmp_p50} | {disp_p50} |".format(
                        hosts=case["host_count"],
                        cycle_p50=aggregate["cycle_total_ms"]["p50"],
                        cycle_p95=aggregate["cycle_total_ms"]["p95"],
                        inv_p50=aggregate["inventory_fetch_ms"]["p50"],
                        parser_p50=aggregate["parser_fetch_ms"]["p50"],
                        cmp_p50=aggregate["drift_compare_ms"]["p50"],
                        disp_p50=aggregate["reconcile_dispatch_ms"]["p50"],
                    )
                )
            lines.append("")

        benchmark2 = results.get("convergence_time")
        if benchmark2:
            metrics = benchmark2["metrics_ms"]
            lines.append("## Benchmark 2: Convergence Time")
            lines.append("")
            lines.append("| Metric | Value (ms) |")
            lines.append("|---|---:|")
            lines.append(f"| Inject -> first detection | {metrics.get('inject_to_first_detection_ms')} |")
            lines.append(f"| Detection -> reconcile send | {metrics.get('detection_to_reconcile_send_ms')} |")
            lines.append(f"| Reconcile send -> workload restored | {metrics.get('reconcile_send_to_workload_restored_ms')} |")
            lines.append(f"| Total convergence | {metrics.get('total_convergence_ms')} |")
            lines.append("")

        benchmark3 = results.get("reconcile_throughput")
        if benchmark3:
            lines.append("## Benchmark 3: Reconcile Throughput")
            lines.append("")
            lines.append("| Case | Parallelism | Requests | Accepted | Rejected | Success completions | Completion throughput (ops/s) |")
            lines.append("|---|---:|---:|---:|---:|---:|---:|")
            for case in benchmark3["steady_cases"]:
                lines.append(
                    "| {name} | {par} | {req} | {acc} | {rej} | {succ} | {thr} |".format(
                        name=case["case_name"],
                        par=case["parallelism"],
                        req=case["request_count"],
                        acc=case["requests"]["accepted_total"],
                        rej=case["requests"]["rejected_total"],
                        succ=case["completions"]["successful_total"],
                        thr=case["completions"]["completion_throughput_success_per_second"],
                    )
                )
            sat = benchmark3["saturation_case"]
            lines.append(
                "| {name} | {par} | {req} | {acc} | {rej} | {succ} | {thr} |".format(
                    name=sat["case_name"],
                    par=sat["parallelism"],
                    req=sat["request_count"],
                    acc=sat["requests"]["accepted_total"],
                    rej=sat["requests"]["rejected_total"],
                    succ=sat["completions"]["successful_total"],
                    thr=sat["completions"]["completion_throughput_success_per_second"],
                )
            )
            lines.append("")

        benchmark4 = results.get("partial_data_robustness")
        if benchmark4:
            lines.append("## Benchmark 4: Partial Data Robustness")
            lines.append("")
            lines.append("| Metric | Value |")
            lines.append("|---|---:|")
            lines.append(f"| Cycle partial flag | {benchmark4['cycle']['partial']} |")
            lines.append(f"| Inventory isPartial | {benchmark4['inventory_metadata'].get('isPartial')} |")
            lines.append(f"| Inventory failedHosts | {benchmark4['inventory_metadata'].get('failedHosts')} |")
            lines.append(f"| Warnings: missing actual data | {benchmark4['warnings_missing_actual_data_count']} |")
            lines.append(f"| Node exporter reconciles for missing hosts | {benchmark4['node_exporter_reconcile_for_missing_hosts']} |")
            lines.append(f"| Cadvisor reconciles for missing hosts | {benchmark4['cadvisor_reconcile_for_missing_hosts']} |")
            lines.append("")

        benchmark5 = results.get("cooldown_efficiency")
        if benchmark5:
            lines.append("## Benchmark 5: Cooldown / Anti-spam Efficiency")
            lines.append("")
            lines.append("| Metric | Value |")
            lines.append("|---|---:|")
            lines.append(
                f"| Reconcile sent without cooldown | {benchmark5['derived']['would_send_without_cooldown']} |"
            )
            lines.append(f"| Reconcile sent with cooldown | {benchmark5['derived']['sent_with_cooldown']} |")
            lines.append(
                f"| Prevented reconcile commands | {benchmark5['derived']['prevented_reconcile_commands']} |"
            )
            lines.append(f"| Suppression ratio | {benchmark5['derived']['suppression_ratio']} |")
            lines.append("")

        lines.append("## Caveats")
        lines.append("")
        lines.append("- Benchmarks run on local Docker stand with logical host scaling via DNS aliases.")
        lines.append("- `500` host case represents control-plane scaling, not 500 independent physical nodes.")
        lines.append("- Throughput includes queueing and ansible execution behavior of the local machine.")
        lines.append("")
        return "\n".join(lines)

    def _write_json(self, path: Path, payload: Any) -> None:
        path.write_text(json.dumps(payload, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")

    def _series_stats(self, values: list[int]) -> dict[str, float | int]:
        if not values:
            return {"count": 0, "min": 0, "p50": 0, "p95": 0, "avg": 0, "max": 0}
        return {
            "count": len(values),
            "min": min(values),
            "p50": self._percentile(values, 50.0),
            "p95": self._percentile(values, 95.0),
            "avg": round(statistics.fmean(values), 2),
            "max": max(values),
        }

    def _percentile(self, values: list[int], percentile: float) -> float:
        if not values:
            return 0.0
        sorted_values = sorted(values)
        if len(sorted_values) == 1:
            return float(sorted_values[0])
        rank = (len(sorted_values) - 1) * (percentile / 100.0)
        lower = math.floor(rank)
        upper = math.ceil(rank)
        if lower == upper:
            return float(sorted_values[lower])
        weight = rank - lower
        interpolated = sorted_values[lower] + (sorted_values[upper] - sorted_values[lower]) * weight
        return round(float(interpolated), 2)

    def _delta_ms(self, started_at: dt.datetime | None, finished_at: dt.datetime | None) -> int | None:
        if started_at is None or finished_at is None:
            return None
        return int((finished_at - started_at).total_seconds() * 1000)

    def _cycle_stat(self, cycle: dict[str, Any], stat_name_pascal: str) -> int:
        stats = cycle.get("stats")
        if not isinstance(stats, dict):
            return 0
        lower_first = stat_name_pascal[:1].lower() + stat_name_pascal[1:]
        value = stats.get(stat_name_pascal, stats.get(lower_first, 0))
        try:
            return int(value)
        except (TypeError, ValueError):
            return 0

    def _cycle_stage_ms(self, cycle: dict[str, Any], stage_name_pascal: str) -> int:
        timings = cycle.get("stageTimings")
        if not isinstance(timings, dict):
            return 0
        lower_first = stage_name_pascal[:1].lower() + stage_name_pascal[1:]
        value = timings.get(stage_name_pascal, timings.get(lower_first, 0))
        try:
            return int(value)
        except (TypeError, ValueError):
            return 0


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Run local e2e benchmark scenarios.")
    parser.add_argument(
        "scenario",
        choices=[
            "all",
            "detection-scalability",
            "convergence",
            "reconcile-throughput",
            "partial-data",
            "cooldown",
        ],
        help="Benchmark scenario to run.",
    )
    parser.add_argument(
        "--output-root",
        default=None,
        help="Output directory root for raw and summary artifacts.",
    )
    return parser.parse_args()


def resolve_output_root(repo_root: Path, override: str | None) -> Path:
    if override:
        return Path(override).resolve()
    timestamp = dt.datetime.now(UTC).strftime("%Y%m%dT%H%M%SZ")
    return repo_root / "tests" / "benchmarks" / "e2e" / "results" / timestamp


def main() -> int:
    args = parse_args()
    repo_root = Path(__file__).resolve().parents[3]
    output_root = resolve_output_root(repo_root, args.output_root)
    harness = BenchmarkHarness(repo_root=repo_root, output_root=output_root, scenario=args.scenario)
    summary = harness.run()

    print(json.dumps(summary, ensure_ascii=False, indent=2))
    print(f"Artifacts saved to: {output_root}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
