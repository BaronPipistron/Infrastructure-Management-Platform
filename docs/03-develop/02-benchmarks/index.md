# E2E Benchmarks

## Назначение

Этот раздел описывает benchmark harness, который запускает серию e2e benchmark/resilience сценариев поверх существующего локального Docker-стенда (`docker-compose.LocalE2E.yml`).

Harness:

- поднимает/перезапускает e2e окружение;
- генерирует fixture-конфиги для разных масштабов;
- прогоняет benchmark сценарии;
- собирает raw результаты;
- строит итоговый summary report.

## Сценарии

- Benchmark 1: Detection scalability (`1 / 10 / 100 / 500` hosts)
- Benchmark 2: Convergence time
- Benchmark 3: Reconcile throughput
- Benchmark 4: Partial data robustness
- Benchmark 5: Cooldown / anti-spam efficiency

## Запуск

Из корня репозитория:

```bash
python tests/benchmarks/e2e/harness.py all
```

Отдельные сценарии:

```bash
python tests/benchmarks/e2e/harness.py detection-scalability
python tests/benchmarks/e2e/harness.py convergence
python tests/benchmarks/e2e/harness.py reconcile-throughput
python tests/benchmarks/e2e/harness.py partial-data
python tests/benchmarks/e2e/harness.py cooldown
```

PowerShell wrapper:

```powershell
./scripts/benchmarks/run.ps1 all
```

## Артефакты

По умолчанию артефакты пишутся в:

`tests/benchmarks/e2e/results/<UTC timestamp>/`

Структура:

- `raw/benchmark1_detection_scalability.json`
- `raw/benchmark2_convergence_time.json`
- `raw/benchmark3_reconcile_throughput.json`
- `raw/benchmark4_partial_data_robustness.json`
- `raw/benchmark5_cooldown_efficiency.json`
- `summary/summary.json`
- `summary/summary.md`

## Ограничения локального стенда

Для больших N harness использует логическое масштабирование хостов через DNS aliases на `managed-host` в сети e2e стенда.

Это позволяет воспроизводимо прогонять сценарии `100/500` на локальной машине, но не эквивалентно 100/500 независимым физическим узлам.

