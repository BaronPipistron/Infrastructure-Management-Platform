# E2E Benchmark Report

- Generated at: `2026-04-24T17:20:47.836957+00:00`
- Run mode: `all`
- Scaling mode: `single managed-host + DNS aliases (logical hosts)`

## Benchmark 1: Detection Scalability

| Hosts | Cycle p50 (ms) | Cycle p95 (ms) | Inventory p50 (ms) | Parser p50 (ms) | Compare p50 (ms) | Dispatch p50 (ms) |
|---:|---:|---:|---:|---:|---:|---:|
| 1 | 0.0 | 0.0 | 0.0 | 0.0 | 0.0 | 0.0 |
| 10 | 1.0 | 1.0 | 0.0 | 0.0 | 0.0 | 0.0 |
| 100 | 6.0 | 8.4 | 3.0 | 2.0 | 0.0 | 0.0 |
| 500 | 20.0 | 21.8 | 12.0 | 6.0 | 0.0 | 0.0 |

## Benchmark 2: Convergence Time

| Metric | Value (ms) |
|---|---:|
| Inject -> first detection | 4075 |
| Detection -> reconcile send | 4 |
| Reconcile send -> workload restored | 10219 |
| Total convergence | 14300 |

## Benchmark 3: Reconcile Throughput

| Case | Parallelism | Requests | Accepted | Rejected | Success completions | Completion throughput (ops/s) |
|---|---:|---:|---:|---:|---:|---:|
| steady_parallel_1 | 1 | 90 | 90 | 0 | 90 | 0.2392 |
| steady_parallel_2 | 2 | 90 | 90 | 0 | 81 | 0.4636 |
| steady_parallel_4 | 4 | 90 | 90 | 0 | 83 | 0.7822 |
| saturation_probe_parallel_2 | 2 | 180 | 102 | 78 | 99 | 0.4579 |

## Benchmark 4: Partial Data Robustness

| Metric | Value |
|---|---:|
| Cycle partial flag | True |
| Inventory isPartial | True |
| Inventory failedHosts | 6 |
| Warnings: missing actual data | 6 |
| Node exporter reconciles for missing hosts | 0 |
| Cadvisor reconciles for missing hosts | 0 |

## Benchmark 5: Cooldown / Anti-spam Efficiency

| Metric | Value |
|---|---:|
| Reconcile sent without cooldown | 20 |
| Reconcile sent with cooldown | 0 |
| Prevented reconcile commands | 20 |
| Suppression ratio | 1.0 |

## Caveats

- Benchmarks run on local Docker stand with logical host scaling via DNS aliases.
- `500` host case represents control-plane scaling, not 500 independent physical nodes.
- Throughput includes queueing and ansible execution behavior of the local machine.
