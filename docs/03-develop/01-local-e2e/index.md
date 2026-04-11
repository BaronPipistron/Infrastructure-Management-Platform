# Local E2E Compose Stand

## Назначение

Этот стенд поднимает всю платформу локально и изолированно в одной Docker-сети и демонстрирует полный цикл:

1. Parser отдает desired state.
2. Inventory отдает actual state.
3. Drift Detector находит drift.
4. Reconciler по SSH запускает ansible на managed-host.
5. Workloads приводятся к desired state.
6. Drift исчезает на следующем цикле.

## Что важно в новой модели

`bootstrap-cadvisor` больше не нужен.

Локальный cold-start теперь работает так:

1. Inventory сначала не может получить cAdvisor-данные (`sourceStatus.cadvisor=error`).
2. `CadvisorDetector` в bootstrap-режиме отправляет reconcile-команду на установку `cadvisor`, даже когда source unhealthy.
3. Reconciler ставит `cadvisor` на managed-host.
4. На следующем sync Inventory начинает читать `managed-host:8080`.
5. После появления healthy source обычный drift-пайплайн продолжает converge (включая `node_exporter`).

## Состав стенда

`docker-compose.LocalE2E.yml` поднимает:

- `managed-host` (эмуляция целевого сервера: `sshd` + `dockerd`)
- `inventory-svc`
- `parser-svc`
- `drift-detector-svc`
- `reconciler-svc`

Все сервисы в одной bridge-сети `imp-local-e2e` и доступны по DNS-именам контейнеров.

## Структура local-e2e

`.local-e2e/`

- `managed-host/` — Dockerfile и entrypoint эмулируемого managed host
- `configs/` — dev-only конфиги сервисов для полного стенда
- `manifests/desired-state.local.json` — desired state для Parser
- `scripts/windows/up.ps1` — генерация новых SSH ключей + запуск стенда (Windows)
- `scripts/windows/down.ps1` — остановка стенда (+ очистка runtime артефактов) (Windows)
- `scripts/unix/up.sh` — генерация новых SSH ключей + запуск стенда (Linux/macOS)
- `scripts/unix/down.sh` — остановка стенда (+ очистка runtime артефактов) (Linux/macOS)
- `scripts/up.ps1` и `scripts/down.ps1` — совместимые wrappers на `scripts/windows/*`
- `.runtime/` — runtime-артефакты, включая SSH ключи (не коммитятся)

## SSH модель (ключи на каждый запуск)

При каждом запуске `./.local-e2e/scripts/windows/up.ps1` или `./.local-e2e/scripts/unix/up.sh`:

1. Генерируется новая пара `ed25519` в `.local-e2e/.runtime/ssh/`.
2. `.pub` монтируется в `managed-host` и копируется в `root/.ssh/authorized_keys`.
3. Приватный ключ монтируется в `reconciler-svc`.
4. Перед стартом Reconciler ключ копируется в `/tmp/reconciler-ssh/id_ed25519` и получает `chmod 600`.

Скрипты `up` по умолчанию используют `--force-recreate`, чтобы `managed-host` и `reconciler-svc` гарантированно подхватили новую пару ключей.

## Desired state (Parser)

Parser читает `.local-e2e/manifests/desired-state.local.json` и отдает:

- `host_id: managed-host-01`
- `fqdn: managed-host`
- required workloads:
  - `node_exporter`
  - `cadvisor`

## Actual state (Inventory)

- Inventory берет bootstrap host из `.local-e2e/configs/inventory/selfProvisioning.LocalE2E.yml`.
- В bootstrap указан `fqdn: managed-host`.
- cAdvisor source настроен на `managed-host:8080`.

До установки `cadvisor` source будет unhealthy (что ожидаемо). После reconcile `cadvisor` source становится healthy.

## E2E сценарий drift -> reconcile -> convergence

1. Initial state: `cadvisor` source unhealthy, нужных workloads нет.
2. Drift Detector отправляет reconcile для `cadvisor` (bootstrap behavior).
3. Reconciler ставит `cadvisor`.
4. Inventory начинает получать healthy actual data.
5. Drift Detector при следующем цикле отправляет reconcile для `node_exporter` (если отсутствует).
6. После следующих sync/циклов drift исчезает.

## Запуск

### Windows (PowerShell)

```powershell
./.local-e2e/scripts/windows/up.ps1
```

Остановка:

```powershell
./.local-e2e/scripts/windows/down.ps1
```

Остановка с сохранением runtime ключей:

```powershell
./.local-e2e/scripts/windows/down.ps1 -KeepRuntimeKeys
```

### Linux/macOS (bash)

Сделайте скрипты исполняемыми один раз:

```bash
chmod +x ./.local-e2e/scripts/unix/up.sh ./.local-e2e/scripts/unix/down.sh
```

Запуск:

```bash
./.local-e2e/scripts/unix/up.sh
```

Остановка:

```bash
./.local-e2e/scripts/unix/down.sh
```

Остановка с сохранением runtime ключей:

```bash
./.local-e2e/scripts/unix/down.sh --keep-runtime-keys
```

## Проверка e2e вручную

1. Проверить readiness:

```bash
curl http://localhost:18080/readyz
curl http://localhost:18082/readyz
curl http://localhost:18083/readyz
curl http://localhost:18084/readyz
```

2. Проверить desired/actual:

```bash
curl http://localhost:18082/api/v1/desired-state
curl http://localhost:18080/api/v1/hosts
```

3. Форсировать detection cycle (опционально):

```bash
curl -X POST http://localhost:18083/api/v1/detection/run
```

4. Смотреть ключевые логи:

```bash
docker logs -f drift-detector-svc
docker logs -f reconciler-svc
docker logs -f inventory-svc
```

## Ограничения dev-only

1. `managed-host` запускается как `privileged` (Docker-in-Docker).
2. Для первого запуска нужен доступ к registry и apt-репозиториям.
3. Это локальный демонстрационный/тестовый стенд, не production orchestration.
