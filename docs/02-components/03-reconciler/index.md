# ReconcilerSvc (MVP)

## 1. Назначение сервиса

`ReconcilerSvc` принимает события drift от `Drift Detector` и инициирует приведение workload на целевом хосте к желаемому состоянию.

В MVP сервис:
- принимает reconcile-запрос по HTTP;
- валидирует payload;
- выбирает operator по `component`;
- строит execution plan;
- ставит задачу в локальную очередь;
- сразу возвращает `202 Accepted` без ожидания завершения ansible;
- в фоне запускает `ansible-runner`;
- подробно логирует все этапы.

Поддерживаемые компоненты в MVP:
- `node_exporter`
- `cadvisor`

## 2. Ответственность и границы

### В зоне ответственности

- API-контракт для приема reconcile-команд.
- Роутинг запроса на нужный operator.
- Построение execution plan.
- Локальная in-process очередь задач.
- Ограничение параллелизма выполнения.
- Фоновый запуск ansible-playbook через `ansible-runner`.
- Структурированное логирование жизненного цикла reconcile.

### Вне MVP

- Хранение истории задач (DB, job-store, event-store).
- API получения статусов/результатов reconcile.
- Retry policy на уровне очереди/оркестратора.
- Внешняя очередь (Redis/Celery/RabbitMQ/Kafka).
- Авторизация/аутентификация API.

## 3. High-Level Flow

1. Drift Detector обнаруживает drift.
2. Drift Detector вызывает `POST /api/v1/reconcile`.
3. Reconciler валидирует запрос.
4. `ReconcileService` выбирает operator по `component`.
5. Operator строит execution plan (какой playbook + какие vars).
6. Задача ставится в локальную очередь.
7. API сразу отвечает `202 Accepted`.
8. Worker в фоне запускает ansible через adapter.
9. В логах фиксируются queue/start/finish/error.
10. При неуспехе задача не ретраится в сервисе; повтор инициирует следующая итерация Drift Detector.

## 4. Архитектура и структура проекта

```text
ReconcilerSvc/
  app/
    main.py
    api/routes.py
    schemas/reconcile.py
    config/{models.py,loader.py}
    logging/setup.py
    operators/{base.py,registry.py,node_exporter.py,cadvisor.py}
    services/{reconcile_service.py,reconcile_executor.py}
    execution/{models.py,task_queue.py}
    ansible/runner_adapter.py
  ansible/
    node_exporter/playbook.yml
    node_exporter/roles/node_exporter/tasks/main.yml
    cadvisor/playbook.yml
    cadvisor/roles/cadvisor/tasks/main.yml
  configs/
    appsettings.Develop.yml
    appsettings.Prod.yml
  tests/
  Dockerfile
  docker-compose.Develop.yml
  .env.example
```

Принципы разделения ответственности:
- FastAPI handler не знает, какой playbook запускать.
- Operator layer отвечает за plan building.
- Ansible adapter изолирует `ansible-runner` детали.
- Queue layer изолирует параллелизм и фоновых worker.

## 5. API Endpoints

### `GET /healthz`

Liveness probe.

Ответ `200`:

```json
{
  "status": "ok"
}
```

### `GET /readyz`

Readiness probe.

- `200` после старта worker pool.
- `503`, если runtime не готов.

Успешный ответ:

```json
{
  "status": "ready"
}
```

### `POST /api/v1/reconcile`

Принимает reconcile-команду и возвращает `202 Accepted` сразу после успешной постановки в очередь.

## 6. Request/Response Contract

### Request model

```json
{
  "host_id": "srv-001",
  "fqdn": "srv-001.example.local",
  "component": "node_exporter",
  "correlation_id": "drift-123",
  "parameters": {}
}
```

Поля:
- `fqdn` (required) — целевой хост.
- `component` (required) — enum: `node_exporter` | `cadvisor`.
- `host_id` (optional) — для логов/трассировки.
- `correlation_id` (optional) — корреляция с Drift Detector.
- `parameters` (optional) — дополнительные operator-specific параметры.

### Accepted response

```json
{
  "status": "accepted",
  "message": "reconcile request accepted",
  "request_id": "0b3bcd3e-b4f0-4d5b-90d8-6c2f2a53f72f",
  "component": "node_exporter",
  "fqdn": "srv-001.example.local",
  "correlation_id": "drift-123",
  "accepted_at": "2026-03-29T08:00:00.000000+00:00"
}
```

### Ошибки

- `422` — невалидный request payload.
- `503` — очередь переполнена или не готова принимать задачи.
- `500` — ошибка построения execution plan.

## 7. Semantics Async Accepted Execution

`POST /api/v1/reconcile` реализует async-контракт по семантике принятия:

- `202` означает только то, что запрос принят и поставлен в локальную очередь.
- `202` не означает успех reconcile на хосте.
- Фактический результат выполнения фиксируется только в логах.
- API не хранит и не отдает completion-status в MVP.

Это упрощает сервис и соответствует модели "best effort + следующая итерация Drift Detector повторит попытку".

## 8. Operator Architecture

### Базовый контракт

`ReconcileOperator`:
- `supports(component)`
- `build_execution_plan(request)`

`OperatorRegistry`:
- хранит список операторов;
- выбирает operator по `component`;
- позволяет расширять набор supported workloads без изменения API слоя.

### Текущее расширение

- `NodeExporterOperator`
- `CadvisorOperator`

Оба оператора:
- получают конфиг с `playbook` и `default_parameters`;
- собирают `ExecutionPlan`;
- не запускают ansible напрямую.

## 9. Node Exporter Operator

Файл:
- `ansible/node_exporter/playbook.yml`
- `ansible/node_exporter/roles/node_exporter/tasks/main.yml`

Поведение роли MVP:
- установка Docker runtime;
- запуск и enable docker service;
- проверка наличия/статуса контейнера `node_exporter`;
- создание контейнера, если отсутствует;
- старт контейнера, если есть, но остановлен.

Поддерживаемые параметры через `parameters/default_parameters`:
- `node_exporter_image`
- `node_exporter_port`

Цель MVP: обеспечить наличие и запущенность `node_exporter` на хосте через Docker контейнер.

## 10. cAdvisor Operator

Файл:
- `ansible/cadvisor/playbook.yml`
- `ansible/cadvisor/roles/cadvisor/tasks/main.yml`

Поведение роли MVP:
- установка Docker runtime;
- запуск и enable docker service;
- проверка наличия/статуса контейнера `cadvisor`;
- создание контейнера, если отсутствует;
- старт контейнера, если есть, но остановлен.

Поддерживаемые параметры через `parameters/default_parameters`:
- `cadvisor_image`
- `cadvisor_port`

## 11. Queue и Concurrency Model

`ReconcileTaskQueue`:
- `asyncio.Queue` с ограничением `queue_capacity`.
- Worker pool фиксированного размера `max_parallel_reconciliations`.
- Прием новых запросов возможен быстрее, чем выполнение.
- Одновременно выполняется не более `N` ansible jobs.

Поведение при перегрузке:
- если очередь не может принять задачу за `enqueue_timeout_seconds`, возвращается `503`.

Graceful lifecycle:
- startup: поднимаются worker.
- shutdown: сервис прекращает прием новых задач и дожидается завершения queued jobs.

## 12. Интеграция ansible-runner

Слой: `app/ansible/runner_adapter.py`.

Adapter делает:
- читает SSH-параметры из env;
- строит in-memory inventory для target `fqdn`;
- объединяет operator `extra_vars` и служебные vars (`reconciler_request_id`, `reconciler_component`, `reconciler_correlation_id`);
- запускает `ansible_runner.run(...)`;
- нормализует результат (`status`, `rc`, `successful`, `duration_seconds`).

Почему adapter выделен отдельно:
- исключает ansible-знание из API/service/operator слоев;
- упрощает тестирование через fake runner;
- оставляет точку расширения для не-ansible исполнителей в будущем.

## 13. Конфигурация

Файлы:
- `ReconcilerSvc/configs/appsettings.Develop.yml`
- `ReconcilerSvc/configs/appsettings.Prod.yml`

Ключи:
- `server.host`
- `server.port`
- `logging.level`
- `reconciler.max_parallel_reconciliations`
- `reconciler.queue_capacity`
- `reconciler.enqueue_timeout_seconds`
- `ansible.playbooks_base_dir`
- `ansible.private_data_dir`
- `ansible.run_timeout_seconds`
- `ansible.ssh.*`
- `operators.node_exporter.*`
- `operators.cadvisor.*`

Выбор конфига:
- env `APP_CONFIG_PATH`
- default: `configs/appsettings.Develop.yml`

## 14. Env Vars

Минимальные runtime env vars:
- `ANSIBLE_SSH_USER`
- `ANSIBLE_SSH_PRIVATE_KEY_PATH`
- `ANSIBLE_SSH_PORT`
- `ANSIBLE_HOST_KEY_CHECKING`
- `APP_CONFIG_PATH`

Пример см. в `ReconcilerSvc/.env.example`.

## 15. Логирование

Сервис использует structured JSON logging.

Минимально логируются события:
- загрузка конфигурации;
- старт/остановка приложения;
- принятие reconcile request;
- постановка задачи в очередь;
- старт reconcile execution;
- потоковый вывод ansible (stdout + task/play/host metadata);
- завершение reconcile execution;
- ошибки worker/ansible/operator.

Ключевые поля:
- `request_id`
- `correlation_id`
- `host_id`
- `fqdn`
- `component`
- `operator`
- `ansible_status`
- `ansible_rc`
- `duration_seconds`

## 16. Docker и локальный запуск

### Docker image

Файл: `ReconcilerSvc/Dockerfile`.

Содержит:
- Python runtime;
- `ansible-core` + `ansible-runner`;
- entrypoint `reconciler-svc`.

### Development compose

Файл: `ReconcilerSvc/docker-compose.Develop.yml`.

Сценарий:
- поднимает сервис;
- подхватывает `.env`;
- монтирует `configs` и `ansible` директории;
- публикует порт `8082`.

### Пример запуска

```bash
cd ReconcilerSvc
cp .env.example .env
docker compose -f docker-compose.Develop.yml up --build
```

Проверка:

```bash
curl http://localhost:8082/healthz
curl http://localhost:8082/readyz
curl -X POST http://localhost:8082/api/v1/reconcile \
  -H "Content-Type: application/json" \
  -d '{
    "host_id": "srv-001",
    "fqdn": "srv-001.example.local",
    "component": "node_exporter",
    "correlation_id": "drift-123",
    "parameters": {}
  }'
```

## 17. Тесты

Покрытие MVP тестами (`14 passed`):
- API endpoints + 202/422/503 семантика.
- Pydantic request/response validation.
- Operator registry selection.
- Execution plan building.
- Queue параллелизм и queue full behavior.
- Reconcile service enqueue flow.
- Ansible adapter через fake runner (без реального ansible).

## 18. Допущения и ограничения MVP

1. Результаты reconcile не сохраняются и не отдаются через API.
2. Сервис не имеет встроенного retry/backoff.
3. Сервис не имеет внешней очереди и persistence.
4. Operator `node_exporter` ориентирован на Docker-based запуск контейнера node_exporter.
5. Operator `cadvisor` ориентирован на Docker-based запуск cAdvisor.
6. Для SSH требуется заранее подготовленный ключ/доступ.
7. Нет authn/authz на API в MVP.

## 19. Дальнейшее развитие

1. Добавить persisted job state и `GET /api/v1/reconcile/{request_id}`.
2. Добавить retry policy (exponential backoff + jitter).
3. Добавить приоритизацию задач и dead-letter queue.
4. Добавить metrics (Prometheus): queue depth, reconcile duration, success/error rates.
5. Добавить pluggable executors (например, SSH-native, agent-based), сохранив operator contract.
6. Добавить authn/authz и audit trail.
7. Добавить больше operators и versioned execution plans.

