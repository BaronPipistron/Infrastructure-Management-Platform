# InventorySvc (MVP)

## 1. Назначение сервиса

`InventorySvc` — canonical read-model для актуального состояния хостов команды.

MVP-цель:
- хранить в памяти текущее состояние inventory;
- брать минимальный bootstrap из `selfProvisioning`;
- собирать workloads из cAdvisor;
- отдавать read-only API;
- явно показывать partial result.

Сервис **не** проксирует cAdvisor на каждый HTTP-запрос. Он периодически обновляет in-memory состояние и API читает только это состояние.

## 2. Ответственность и границы

### В зоне ответственности
- загрузка и валидация `appsettings`;
- загрузка и валидация `selfProvisioning`;
- initial sync + periodic sync;
- нормализация source-данных в доменную модель;
- безопасное хранение snapshot в памяти;
- выдача read-only API;
- сигнализация partial result.

### Вне MVP
- персистентное хранилище;
- snapshots/versioning/history;
- auth к cAdvisor;
- write API;
- полноценные интеграции NetBox/OtherSources.

## 3. High-Level Flow

1. Старт приложения.
2. Загрузка `appsettings`.
3. Загрузка `selfProvisioning`.
4. Initial sync по всем bootstrap-host.
5. Сохранение snapshot в memory store.
6. Выставление readiness.
7. Поднятие HTTP API.
8. Периодический sync по таймеру.

## 4. Структура проекта

```text
InventorySvc/
  cmd/inventory-svc/main.go
  appsettings.Develop.yml
  selfProvisioning.Develop.yml
  configs/
    appsettings.Prod.yml
    selfProvisioning.Prod.yml
  docs/swagger/
  internal/
    app/app.go
    config/config.go
    logger/logger.go
    domain/model.go
    bootstrap/selfprovisioning/
      loader.go
      types.go
    sources/
      source.go
      cadvisor/
        client.go
        types.go
        mapper.go
        source.go
    store/memory/store.go
    service/inventory/service.go
    scheduler/scheduler.go
    transport/http/
      dto.go
      handlers.go
      router.go
```

## 5. Конфигурация

Поддерживаемые ключи:
- `server.port`
- `logging.level`
- `sync.interval`
- `bootstrap.selfProvisioningPath`
- `sources.cadvisor.isEnabled`
- `sources.cadvisor.includeSystemWorkloads`
- `sources.cadvisor.scheme`
- `sources.cadvisor.port`
- `sources.cadvisor.basePath`
- `sources.cadvisor.containersPath`
- `sources.cadvisor.timeout`
- `sources.cadvisor.urlTemplate`
- `sources.netbox.isEnabled`
- `sources.otherSources.isEnabled`

### Develop: `InventorySvc/appsettings.Develop.yml`

```yaml
server:
  port: 8080
logging:
  level: info
sync:
  interval: 30s
sources:
  cadvisor:
    isEnabled: true
    includeSystemWorkloads: false
    scheme: http
    port: 8080
    basePath: /api/v1.3
    containersPath: /subcontainers/
    timeout: 5s
    urlTemplate: "{{scheme}}://{{fqdn}}:{{port}}{{basePath}}{{containersPath}}"
  netbox:
    isEnabled: false
  otherSources:
    isEnabled: false
bootstrap:
  selfProvisioningPath: ./selfProvisioning.Develop.yml
```

### Prod: `InventorySvc/configs/appsettings.Prod.yml`

```yaml
server:
  port: 8080
logging:
  level: info
sync:
  interval: 1m
sources:
  cadvisor:
    isEnabled: true
    includeSystemWorkloads: false
    scheme: http
    port: 8080
    basePath: /api/v1.3
    containersPath: /subcontainers/
    timeout: 5s
    urlTemplate: "{{scheme}}://{{fqdn}}:{{port}}{{basePath}}{{containersPath}}"
  netbox:
    isEnabled: false
  otherSources:
    isEnabled: false
bootstrap:
  selfProvisioningPath: ./selfProvisioning.Prod.yml
```

## 6. selfProvisioning формат

Файлы:
- `InventorySvc/selfProvisioning.Develop.yml`
- `InventorySvc/configs/selfProvisioning.Prod.yml`

Схема:

```yaml
hosts:
  - id: host-001
    fqdn: db01.dev.example.internal
    labels:
      managed_by: team-a
      purpose: database
      env: dev
      role: primary
      region: ru-msk-1
      owner: platform
```

Валидация:
- `id` обязателен и уникален;
- `fqdn` обязателен и уникален;
- `labels` обязательны;
- обязательные labels: `managed_by`, `purpose`, `env`.

## 7. cAdvisor адресация

Для MVP выбран endpoint:
- `GET /api/v1.3/subcontainers/`

URL строится из шаблона:

```text
{{scheme}}://{{fqdn}}:{{port}}{{basePath}}{{containersPath}}
```

По умолчанию:

```text
http://<fqdn>:8080/api/v1.3/subcontainers/
```

Фильтрация системных cgroup-узлов:
- `includeSystemWorkloads: false` (по умолчанию) — возвращаются только “контейнерные” workload;
- `includeSystemWorkloads: true` — возвращается весь набор, который отдает cAdvisor (включая системные cgroup entries).

Реализация изолирована в `internal/sources/cadvisor`.

## 8. Доменная модель

### Host
- `id`
- `fqdn`
- `labels`
- `workloads`
- `lastObservedAt`
- `status`: `ok` | `partial` | `error` | `bootstrap_only`
- `sourceStatus` (статус по каждому источнику)
- `errors`

### Workload
- `id`
- `name`
- `image`
- `runtime`
- `source`
- `lastSeenAt`

### Snapshot metadata
- `isPartial`
- `totalHosts`
- `failedHosts`
- `lastSyncAt`
- `lastSuccessfulSyncAt`
- `lastFullSyncAt`
- `lastPartialSyncAt`
- `syncDurationMs`

## 9. Partial result semantics

Partial result отражается на двух уровнях:
1. глобально в `metadata.isPartial` (ответ списка);
2. по хосту в `status`, `sourceStatus`, `errors`.

Если часть хостов недоступна в cAdvisor:
- сервис не падает;
- данные по доступным хостам возвращаются;
- ошибки логируются;
- ответ явно помечается как partial.

## 10. Sync lifecycle

### Initial sync
- выполняется до готовности сервиса;
- после завершения (включая partial) сервис становится ready.

### Periodic sync
- выполняется scheduler-ом по `sync.interval`;
- обновление snapshot потокобезопасно (`RWMutex`);
- API читает согласованный snapshot во время апдейта.

## 11. Ошибки и observability

Логируются:
- старт приложения;
- загрузка конфигов;
- загрузка `selfProvisioning`;
- initial/periodic sync;
- ошибки по host/source;
- агрегированные метрики sync в логах: total/success/failed/duration/isPartial.

Логгер: `zap` sugared logger.

## 12. API

### `GET /healthz`
Проверка liveness.

### `GET /readyz`
Проверка readiness.
- `200` после initial sync;
- `503` до завершения initial sync.

### `GET /api/v1/hosts`
Список хостов + metadata.

Поддержка фильтрации labels query-параметрами (AND):
- `/api/v1/hosts?managed_by=team-a`
- `/api/v1/hosts?env=prod&purpose=database`

### `GET /api/v1/hosts/:id`
Полная карточка хоста.

`404`, если id не найден.

## 13. Swagger

Интеграция: `swaggo/gin-swagger`.

- UI: `/swagger/index.html`
- generated package: `InventorySvc/docs/swagger`

Генерация:

```bash
make swagger
```

## 14. Makefile

Цели:
- `make build`
- `make run`
- `make test`
- `make swagger`
- `make tidy`

Локальный запуск:

```bash
make run CONFIG=./appsettings.Develop.yml
```

## 15. Docker (Production Style)

Образ сделан в production-style формате:
- в image только бинарник и runtime-зависимости;
- конфиги не вшиваются;
- `appsettings` и `selfProvisioning` подключаются как внешний volume.

По умолчанию контейнер читает:
- `APP_CONFIG_PATH=/etc/inventory-svc/configs/appsettings.Prod.yml`

Пример запуска с внешними файлами:

```bash
docker build -t inventory-svc:latest ./InventorySvc
docker run --rm -p 8080:8080 \
  -v "$(pwd)/InventorySvc:/etc/inventory-svc:ro" \
  inventory-svc:latest
```

Запуск с prod-конфигом:

```bash
docker run --rm -p 8080:8080 \
  -v "$(pwd)/InventorySvc/configs:/etc/inventory-svc/configs:ro" \
  inventory-svc:latest
```


### Локальный интеграционный прогон через docker-compose

Для локального теста cAdvisor + InventorySvc добавлен файл:
- `InventorySvc/docker-compose.Develop.yml`

Команды:

```bash
cd InventorySvc
docker compose -f docker-compose.Develop.yml up --build
```

Проверка:

```bash
curl http://localhost:8080/healthz
curl http://localhost:8080/readyz
curl http://localhost:8080/api/v1/hosts
```

Используемые docker-local конфиги:
- `InventorySvc/configs/appsettings.DockerLocal.yml`
- `InventorySvc/configs/selfProvisioning.DockerLocal.yml`
## 16. Тесты MVP

Реализованы тесты:
- config load/validation
- selfProvisioning load/validation
- labels filtering
- cAdvisor mapping
- in-memory store behavior
- basic API handler

Запуск:

```bash
make test
```

## 17. Ограничения и допущения MVP

1. Реально интегрирован только cAdvisor.
2. NetBox/OtherSources сейчас только как расширяемые точки (`isEnabled`).
3. Хранение состояния только in-memory.
4. Нет persistence/snapshot/versioning.
5. Нет auth для cAdvisor.
6. endpoint cAdvisor для MVP: `/api/v1.3/subcontainers/`.

## 18. Дальнейшее развитие

1. Полноценная интеграция NetBox.
2. Приоритеты и merge-policy между источниками.
3. Метрики (Prometheus) для sync/source errors.
4. Retry/backoff/circuit breaker.
5. Versioned snapshot store (вне MVP).



