# ParserSvc (MVP)

## 1. Назначение сервиса

`ParserSvc` — сервис, который читает Structurizr deployment manifest в формате JSON, строит host-centric `desired state` и отдает его через HTTP API.

Этот `desired state` является входом для `Drift Detector` (сравнение desired vs actual).

В MVP:
- источник данных только Structurizr JSON;
- данные читаются при старте сервиса;
- состояние хранится только в памяти;
- внешнее хранилище не используется.

## 2. Ответственность и границы

### В зоне ответственности
- загрузка и валидация `appconfig`;
- поиск manifest-файлов (`file` или `directory` режим);
- чтение только `.json` и игнорирование `.dsl`/прочих файлов;
- парсинг Structurizr JSON;
- маппинг deployment model -> host-centric desired state;
- валидация host/workload полей;
- хранение актуального desired state в in-memory store;
- read-only HTTP API + Swagger.

### Вне MVP
- Structurizr DSL parsing внутри сервиса;
- file watcher/reload endpoint;
- БД/snapshot versioning/history;
- authN/authZ;
- интеграции с внешними источниками.

## 3. High-Level Flow

1. Старт приложения.
2. Загрузка `appconfig`.
3. Резолв пути к manifest (относительно конфига или абсолютный).
4. Поиск файлов по режиму `file|directory`.
5. Игнорирование файлов с расширением, отличным от `.json`.
6. По каждому JSON: read -> parse -> map -> validate.
7. Битые файлы/узлы/workloads логируются как `WARN`, но не валят startup.
8. Валидные данные объединяются в in-memory snapshot.
9. Поднимается HTTP API + Swagger.
10. `readyz` зависит от успешной загрузки хотя бы одного валидного JSON.

## 4. Входной формат Structurizr JSON

MVP использует только следующие части JSON:
- `model.deploymentNodes`
- `model.softwareSystems[].containers`
- `deploymentNode.properties`
- `containerInstance.properties`

Игнорируется:
- `views`
- `documentation`
- `structurizr.dsl`
- `structurizr.dsl.identifier`
- inspection metadata и любые прочие служебные поля, не нужные для desired state.

## 5. Внутренняя модель desired state

```text
DesiredState
  hosts[]

DesiredHost
  host_id
  fqdn
  ip (optional)
  labels
  env
  managed_by
  purpose
  workloads[]

DesiredWorkload
  name
  enabled
  deployment_mode
  image
  version (optional)
  port (optional)
```

Нормализация типов:
- строки `"true"/"false"` -> `bool`;
- строковые числа (`"9100"`) -> `int`.

## 6. Validation Rules

### Хост
Обязательные поля:
- `host_id`
- `fqdn`
- `env`
- `managed_by`
- `purpose`

Опциональные:
- `ip`

### Workload
Обязательные поля:
- имя workload (через `containerId -> container definition`)
- `enabled`
- `deployment_mode`

Дополнительное правило:
- если `deployment_mode=container`, обязателен `image`.

`port` optional; при наличии нормализуется в `int`.

Поведение при ошибках:
- битый workload пропускается с `WARN`;
- битый deployment node (хост) пропускается с `WARN`;
- валидные элементы продолжают обрабатываться.

## 7. Поведение при битых manifest-файлах

Правила MVP:
- сервис читает manifests только при старте;
- `.dsl` и любые не-JSON файлы игнорируются;
- если часть JSON битые, сервис стартует и загружает валидные;
- по битым JSON логируется `WARN` с путем и причиной.

Инженерное решение для случая "все файлы битые":
- сервис стартует (чтобы оставаться наблюдаемым через `healthz` и логи);
- `readyz` возвращает `503` (`status=not_ready`),
- API возвращает пустой desired state + metadata с `filesLoaded=0`.

## 8. API

### Health
- `GET /healthz`
- `GET /readyz`

### Desired state
- `GET /api/v1/desired-state`
- `GET /api/v1/desired-state/hosts`
- `GET /api/v1/desired-state/hosts/:hostId`

### Swagger
- `GET /swagger/index.html`

## 9. Фильтрация (fqdn + labels)

`GET /api/v1/desired-state/hosts` поддерживает:
- `fqdn` как отдельный фильтр;
- остальные query params как label filters.

Несколько query-параметров работают как `AND`.

Примеры:
- `/api/v1/desired-state/hosts?fqdn=srv-001.example.local`
- `/api/v1/desired-state/hosts?env=prod`
- `/api/v1/desired-state/hosts?managed_by=platform-team&purpose=compute`
- `/api/v1/desired-state/hosts?fqdn=srv-001.example.local&env=prod`

## 10. Примеры ответов API

### `GET /healthz`

```json
{
  "status": "ok"
}
```

### `GET /readyz` (ready)

```json
{
  "status": "ready",
  "loadedFiles": 1,
  "brokenFiles": 0,
  "hostsTotal": 1
}
```

### `GET /readyz` (not ready)

```json
{
  "status": "not_ready",
  "readyReason": "no valid manifest files were loaded",
  "loadedFiles": 0,
  "brokenFiles": 2,
  "hostsTotal": 0
}
```

### `GET /api/v1/desired-state/hosts`

```json
{
  "metadata": {
    "loadedAt": "2026-03-31T12:00:00Z",
    "totalHosts": 1,
    "returnedHosts": 1,
    "workloadsTotal": 2,
    "filesLoaded": 1,
    "filesBroken": 0
  },
  "hosts": [
    {
      "host_id": "srv-001",
      "fqdn": "srv-001.example.local",
      "ip": "10.10.10.11",
      "labels": {
        "env": "prod",
        "managed_by": "platform-team",
        "purpose": "compute"
      },
      "env": "prod",
      "managed_by": "platform-team",
      "purpose": "compute",
      "workloads": [
        {
          "name": "node_exporter",
          "enabled": true,
          "deployment_mode": "container",
          "image": "quay.io/prometheus/node-exporter:v1.8.2",
          "port": 9100
        }
      ]
    }
  ]
}
```

## 11. Конфигурация

Файлы:
- `ParserSvc/configs/appconfig.Develop.yml`
- `ParserSvc/configs/appconfig.Prod.yml`

Поддерживаемые ключи:
- `server.host`
- `server.port`
- `logging.level`
- `manifests.mode` (`file` | `directory`)
- `manifests.path`

Пример (`Develop`):

```yaml
server:
  host: 0.0.0.0
  port: 8080

logging:
  level: info

manifests:
  mode: directory
  path: ../manifests
```

## 12. Docker / Development Run

### Make targets
- `make build`
- `make run`
- `make test`
- `make lint`
- `make swagger`
- `make tidy`

### Локальный запуск

```bash
cd ParserSvc
make run CONFIG=./configs/appconfig.Develop.yml
```

### Docker Compose (develop)

```bash
cd ParserSvc
docker compose -f docker-compose.Develop.yml up --build
```

После старта:
- `http://localhost:8082/healthz`
- `http://localhost:8082/readyz`
- `http://localhost:8082/api/v1/desired-state`
- `http://localhost:8082/swagger/index.html`

## 13. Структура ParserSvc

```text
ParserSvc/
  cmd/parser-svc/main.go
  configs/
    appconfig.Develop.yml
    appconfig.Prod.yml
  docs/swagger/
  internal/
    app/
    config/
    domain/
    loader/
    parser/
    mapper/
    validator/
    store/memory/
    service/desiredstate/
    transport/http/
    logger/
  manifests/
  Dockerfile
  docker-compose.Develop.yml
  Makefile
  .golangci.yml
```

## 14. Ограничения MVP

1. Только Structurizr JSON как вход.
2. Нет runtime reload (только загрузка на старте).
3. Хранилище только in-memory.
4. Нет истории/snapshot versioning.
5. Нет авторизации.
6. При duplicate `host_id` между файлами используется правило: `последний загруженный host заменяет предыдущий` (с WARN в лог).
7. Для `directory` режима читается верхний уровень директории (без рекурсии).

## 15. Направления развития

1. Добавить безопасный `reload` endpoint или periodic re-load.
2. Добавить схему/контракт валидации входных manifests (JSON Schema).
3. Ввести merge-политику по duplicate `host_id` (merge workloads вместо replace).
4. Экспортировать технические метрики (Prometheus): counts/errors/latency.
5. Добавить API versioning и расширенные фильтры.
6. Поддержать snapshot persistence как отдельный этап после MVP.
