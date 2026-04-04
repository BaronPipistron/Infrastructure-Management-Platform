# DriftDetectorSvc (MVP)

## 1. Назначение сервиса

`DriftDetectorSvc` сравнивает:
- `desired state` (источник истины: `ParserSvc`);
- `actual state` (источник истины: `InventorySvc`);

и при обнаружении drift отправляет асинхронную reconcile-команду в `ReconcilerSvc`.

MVP-поддержка компонентов:
- `node_exporter`;
- `cadvisor`.

## 2. Ответственность и границы

### В зоне ответственности
- загрузка и валидация `appsettings`;
- периодический detection loop по расписанию;
- ручной запуск одного detection cycle через API;
- сравнение desired/actual по хостам и компонентам;
- формирование reconcile-команд;
- anti-spam/cooldown подавление повторной отправки;
- структурированное логирование всего цикла.

### Вне MVP
- хранение истории drift/reconcile в БД;
- отслеживание completion статуса reconcile jobs;
- сложные типы drift (version/config/health degradation);
- персистентный dedup state (cooldown state только in-memory).

## 3. Реальные внешние контракты (из исходников)

Ниже перечислены контракты, по которым реализован клиентский слой DriftDetectorSvc.

### 3.1 InventorySvc (actual state)

Источник: `InventorySvc/internal/transport/http/handlers.go`, `InventorySvc/docs/swagger/swagger.yaml`.

- Endpoint: `GET /api/v1/hosts`
- Ответ:
  - `metadata.isPartial`, `metadata.totalHosts`, `metadata.failedHosts`, `metadata.lastSyncAt`
  - `hosts[]`:
    - `id`, `fqdn`, `labels`
    - `status`
    - `workloads[]` (используется `name`)
    - `sourceStatus` map (используется `cadvisor.status`, `cadvisor.enabled`, `cadvisor.error`)

Использование в DriftDetectorSvc:
- `id/fqdn` для матчинга хостов;
- `workloads[].name` для проверки наличия компонента;
- `sourceStatus["cadvisor"]` для оценки полноты actual-данных.

### 3.2 ParserSvc (desired state)

Источник: `ParserSvc/internal/transport/http/handlers.go`, `ParserSvc/docs/swagger/swagger.yaml`.

- Endpoint: `GET /api/v1/desired-state`
- Ответ:
  - `metadata.ready`, `metadata.readyReason`, `metadata.filesLoaded`, `metadata.filesBroken`
  - `desiredState.hosts[]`:
    - `host_id`, `fqdn`, `labels`
    - `workloads[]`: `name`, `enabled`, `deployment_mode`, `image`, `version`, `port`

Использование в DriftDetectorSvc:
- `host_id/fqdn` для матчинга;
- `workloads` как источник истины по required компонентам.

### 3.3 ReconcilerSvc (асинхронный reconcile)

Источник: `ReconcilerSvc/app/api/routes.py`, `ReconcilerSvc/app/schemas/reconcile.py`.

- Endpoint: `POST /api/v1/reconcile`
- Request:
  - required: `fqdn`, `component`
  - optional: `host_id`, `correlation_id`, `parameters`
- Success: `202 Accepted` (означает принятие в очередь, не завершение reconcile).

Использование в DriftDetectorSvc:
- при drift отправляется reconcile-команда;
- `202` считается успешной отправкой;
- completion reconcile не ожидается.

## 4. Структура DriftDetectorSvc

Реализована в стиле `InventorySvc`/`ParserSvc`:

```text
DriftDetectorSvc/
  cmd/drift-detector-svc/main.go
  configs/appsettings.{Develop,Prod}.yml
  docs/swagger/
  internal/
    app/
    config/
    logger/
    domain/
    clients/{inventory,parser,reconciler}/
    detectors/
    cooldown/
    service/detection/
    scheduler/
    transport/http/
```

## 5. High-Level Flow

1. Старт приложения.
2. Загрузка и валидация config.
3. Инициализация клиентов (`InventorySvc`, `ParserSvc`, `ReconcilerSvc`).
4. Инициализация detector registry (`node_exporter`, `cadvisor`).
5. Initial detection cycle (`trigger=startup`).
6. Поднятие HTTP API.
7. Запуск scheduler по `detection.interval`.
8. Дополнительно возможен ручной цикл через `POST /api/v1/detection/run`.

## 6. Detection Cycle Semantics

Один cycle:

1. Fetch `actual state` из `InventorySvc`.
2. Fetch `desired state` из `ParserSvc`.
3. Нормализация во внутренние доменные модели.
4. Матчинг хостов `desired -> actual` по `host_id`, fallback по `fqdn`.
5. Для каждого enabled detector:
   - извлечение component desired/actual;
   - определение drift;
   - формирование reconcile command.
6. Anti-spam фильтр (cooldown по `component|fqdn`).
7. Отправка разрешенных команд в `ReconcilerSvc`.
8. Формирование итоговой статистики цикла.

Собираем статистику:
- `desiredHosts`, `comparedHosts`;
- `skippedHostsNoActualHost`, `skippedHostsNoActualData`;
- `driftsFound`;
- `reconcileSent`, `reconcileSuppressed`;
- `errors`;
- `partial`.

## 7. Partial Processing и неполные данные

Реализованы правила MVP:

- если `InventorySvc` вернул `metadata.isPartial=true`, cycle помечается partial;
- если actual-host для desired-host не найден:
  - WARN лог;
  - host пропускается;
  - не считается drift;
- если `sourceStatus["cadvisor"]` не `ok`:
  - WARN лог;
  - host/components пропускаются;
  - не считается drift;
- отсутствие данных никогда не трактуется автоматически как drift.

Это согласовано с требованием безопасной частичной обработки.

## 8. Detector/Operator Architecture

Реализован интерфейс detector и registry:

- `NodeExporterDetector`
- `CadvisorDetector`
- `Registry` (выбор detector по `component`)

Каждый detector сам знает:
- как извлечь desired workload для своего компонента;
- как проверить доступность actual данных;
- как определить drift (в MVP: required+enabled в desired и отсутствует в actual);
- как собрать reconcile request payload.

### 8.1 Node Exporter detector

Drift:
- в desired есть `node_exporter` с `enabled=true`, `deployment_mode=container`;
- в actual нет workload `node_exporter`.

Команда в Reconciler:
- `component=node_exporter`;
- optional parameters:
  - `node_exporter_image` (из desired.image);
  - `node_exporter_port` (из desired.port).

### 8.2 cAdvisor detector

Drift:
- в desired есть `cadvisor` с `enabled=true`, `deployment_mode=container`;
- в actual нет workload `cadvisor`.

Команда в Reconciler:
- `component=cadvisor`;
- optional parameters:
  - `cadvisor_image` (из desired.image);
  - `cadvisor_port` (из desired.port).

## 9. Anti-spam / Cooldown

Реализация:
- in-memory map: `component|fqdn -> lastSentAt`;
- конфиг: `antiSpam.reconcileCooldown`;
- если с момента последней отправки прошло меньше cooldown:
  - повторная отправка suppress;
  - логируется факт suppress.

Ограничение MVP:
- state не персистится и теряется при рестарте сервиса.

## 10. API DriftDetectorSvc

### `GET /healthz`
- liveness probe.

### `GET /readyz`
- `200 ready` после хотя бы одного успешного detection cycle;
- `503 not_ready`, если успешного цикла еще не было.

### `POST /api/v1/detection/run`
- ручной запуск одного detection cycle;
- scheduler не останавливается и продолжает работать;
- параллельный запуск запрещен:
  - при уже выполняющемся цикле возвращается `409`.

Swagger UI:
- `GET /swagger/index.html`

## 11. Scheduler Model

- `ticker` с интервалом `detection.interval`;
- каждый тик инициирует `RunCycle(trigger=scheduler)`;
- если цикл уже идет (например, manual run), scheduler тик пропускается без падения;
- graceful stop при shutdown context.

## 12. Конфигурация

Поддерживаемые ключи:

- `server.host`
- `server.port`
- `logging.level`
- `detection.interval`
- `detection.enabledComponents`
- `antiSpam.reconcileCooldown`
- `clients.inventory.*` (`baseURL`, `hostsPath`, `timeout`, `retry.*`)
- `clients.parser.*` (`baseURL`, `desiredStatePath`, `timeout`, `retry.*`)
- `clients.reconciler.*` (`baseURL`, `reconcilePath`, `timeout`, `retry.*`)

Файлы:
- `DriftDetectorSvc/configs/appsettings.Develop.yml`
- `DriftDetectorSvc/configs/appsettings.Prod.yml`

## 13. Логирование и observability

Используется `zap` sugared logger.

Логируются:
- startup/config;
- старт/финиш detection cycle;
- partial причины и skip;
- каждый drift;
- reconcile send/suppress;
- ошибки внешних вызовов и оркестрации.

## 14. Error Handling

- ошибки fetch из `InventorySvc`/`ParserSvc` завершают текущий cycle как failed;
- неполные/частично недоступные actual-данные ведут к partial cycle, но не к падению сервиса;
- ошибки отправки в `ReconcilerSvc` учитываются в stats/errors и логах, но не валят процесс целиком.

## 15. Допущения, сделанные в MVP

1. Для проверки фактического наличия workload используется `InventorySvc` workload `name`.
2. Полнота actual по workload оценивается через `sourceStatus["cadvisor"]`.
3. Матчинг хостов делается по `host_id`, fallback `fqdn`.
4. Workload в desired учитывается только при `enabled=true` и `deployment_mode=container`.
5. `202` от `ReconcilerSvc` трактуется как успешное принятие reconcile-команды.

## 16. Ограничения MVP

1. Поддержаны только `node_exporter` и `cadvisor`.
2. Нет version/config/health drift-логики.
3. Нет persisted состояния cooldown.
4. Нет API истории detection/reconcile.
5. Нет retry-оркестрации reconcile на уровне job-state (повтор обеспечивает следующий detection cycle).

## 17. Направления развития

1. Добавить новые detector-ы (blackbox-exporter, alertmanager, и т.д.) без изменения API-слоя.
2. Расширить drift-модель: version/health/config drift.
3. Добавить persistence для cooldown/history.
4. Добавить метрики Prometheus по cycle/drift/reconcile.
5. Добавить policy/priority на reconcile-команды.
