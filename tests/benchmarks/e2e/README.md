# E2E Benchmark Harness

Эта директория содержит воспроизводимый benchmark harness, который запускается поверх существующего локального Docker-стенда E2E (`docker-compose.LocalE2E.yml`)

## Какие сценарии запускаются

- Benchmark 1: масштабируемость detection (`1 / 10 / 100 / 500` хостов)
- Benchmark 2: время сходимости после дрифта хоста
- Benchmark 3: throughput reconcile и проверка saturation
- Benchmark 4: устойчивость к частичным данным
- Benchmark 5: эффективность cooldown / anti-spam

## Команды запуска

Из корня репозитория:

```bash
python tests/benchmarks/e2e/harness.py all
```

Запуск отдельных benchmark-сценариев:

```bash
python tests/benchmarks/e2e/harness.py detection-scalability
python tests/benchmarks/e2e/harness.py convergence
python tests/benchmarks/e2e/harness.py reconcile-throughput
python tests/benchmarks/e2e/harness.py partial-data
python tests/benchmarks/e2e/harness.py cooldown
```

С кастомной директорией для результатов:

```bash
python tests/benchmarks/e2e/harness.py all --output-root tests/benchmarks/e2e/results/manual-run
```

## Артефакты

Путь вывода по умолчанию:

`tests/benchmarks/e2e/results/<UTC timestamp>/`

После каждого запуска сохраняются:

- `raw/benchmark*.json` (сырые результаты benchmark)
- `summary/summary.json` (метаданные сводки запуска)
- `summary/summary.md` (сводный отчёт в читаемом виде)

## Модель масштабирования

Для больших значений количества хостов harness использует логическое масштабирование хостов через DNS aliases, которые резолвятся в `managed-host` в существующей e2e Docker-сети

Это сохраняет взаимодействия сервисов (контракты Inventory/Parser/DriftDetector/Reconciler) и позволяет избежать непрактичного локального запуска сотен привилегированных DIND-контейнеров
