workspace "Infrastructure Management Platform - диаграммы для ВКР" "Диаграммы Architecture-as-Code для закрытия замечаний 4, 5 и 6." {
  model {
    platformUser = person "Пользователь платформы" "SRE-инженер, DevOps-инженер или архитектор, который готовит архитектурные манифесты и работает с платформой."

    architectureManifests = softwareSystem "Архитектурные манифесты" "Артефакты Structurizr DSL/JSON, описывающие управляемые хосты и требуемые инфраструктурные workload." {
      tags "External System"
    }

    inventoryBootstrapConfig = softwareSystem "Bootstrap-конфигурация inventory" "YAML-конфигурация selfProvisioning с начальным списком управляемых хостов." {
      tags "External System"
    }

    managedHost = softwareSystem "Управляемый хост" "Хост, находящийся под управлением платформы. В текущей реализации управляемые workload запускаются как Docker-контейнеры." {
      tags "External System"

      cadvisorAgent = container "cAdvisor" "Собирает и предоставляет сведения о состоянии контейнеров для InventorySvc." "cAdvisor HTTP API" {
        tags "External Container"
      }

      dockerWorkloads = container "Контейнерные workload" "Инфраструктурные workload, управляемые платформой, включая cadvisor и node_exporter." "Docker-контейнеры" {
        tags "External Container"
      }
    }

    infrastructureManagementPlatform = softwareSystem "Платформа управления инфраструктурой" "Автоматизированная платформа управления инфраструктурой с поддержкой подхода Architecture-as-Code." {
      parserSvc = container "ParserSvc" "Загружает Structurizr JSON-манифесты, формирует host-centric desired state и предоставляет read-only API." "Go, Gin, HTTP REST API"
      inventorySvc = container "InventorySvc" "Загружает bootstrap-данные хостов, периодически получает actual state из cAdvisor и предоставляет in-memory snapshot inventory." "Go, Gin, HTTP REST API"
      driftDetectorSvc = container "DriftDetectorSvc" "Запускает detection cycle, сравнивает desired state и actual state и отправляет reconcile-команды для поддерживаемых workload." "Go, Gin, Scheduler, HTTP clients"
      reconcilerSvc = container "ReconcilerSvc" "Принимает reconcile-команды, выбирает operator, ставит выполнение в очередь и запускает Ansible playbook на управляемых хостах." "Python, FastAPI, Ansible Runner"
    }

    platformUser -> architectureManifests "Ведет Architecture-as-Code-описание и экспортирует Structurizr JSON-манифесты" "Git + Structurizr DSL/JSON"
    platformUser -> infrastructureManagementPlatform "Проверяет состояние платформы, читает API и может запускать ручную проверку drift" "HTTP REST API"
    infrastructureManagementPlatform -> architectureManifests "Читает смонтированные Structurizr JSON-манифесты как источник desired state" "Локальный файловый том"
    infrastructureManagementPlatform -> managedHost "Получает actual state и выполняет поддерживаемые reconcile-действия" "HTTP REST API к cAdvisor; SSH + Ansible"

    platformUser -> parserSvc "Читает API desired state и проверяет готовность сервиса" "HTTP REST API"
    platformUser -> inventorySvc "Читает API actual state и проверяет готовность сервиса" "HTTP REST API"
    platformUser -> driftDetectorSvc "Запускает ручной detection cycle и проверяет готовность сервиса" "HTTP REST API: POST /api/v1/detection/run"
    platformUser -> reconcilerSvc "При необходимости отправляет ручные reconcile-запросы для поддерживаемых workload" "HTTP REST API: POST /api/v1/reconcile"

    parserSvc -> architectureManifests "Загружает Structurizr JSON-манифесты при старте" "Локальный файловый том"

    inventorySvc -> inventoryBootstrapConfig "Загружает начальный список управляемых хостов" "YAML-файл конфигурации"
    inventorySvc -> cadvisorAgent "Получает инвентаризацию контейнеров и статус источника" "HTTP REST API: GET /api/v1.3/subcontainers/"

    driftDetectorSvc -> parserSvc "Получает desired state для detection cycle" "HTTP REST API: GET /api/v1/desired-state"
    driftDetectorSvc -> inventorySvc "Получает snapshot actual state для detection cycle" "HTTP REST API: GET /api/v1/hosts"
    driftDetectorSvc -> reconcilerSvc "Отправляет reconcile-команду при обнаружении поддерживаемого drift" "HTTP REST API: POST /api/v1/reconcile"

    reconcilerSvc -> managedHost "Выполняет playbook, устанавливающий или запускающий Docker-based workload" "SSH + Ansible Runner"

    cadvisorAgent -> dockerWorkloads "Наблюдает запущенные контейнеры и предоставляет метаданные workload" "Docker/cgroup filesystem + cAdvisor HTTP API"
  }

  views {
    systemContext infrastructureManagementPlatform "vkr-system-context" {
      include platformUser
      include architectureManifests
      include inventoryBootstrapConfig
      include managedHost
      include infrastructureManagementPlatform
      autoLayout lr
    }

    container infrastructureManagementPlatform "vkr-container-diagram" {
      include platformUser
      include architectureManifests
      include managedHost
      include cadvisorAgent
      include dockerWorkloads
      include parserSvc
      include inventorySvc
      include driftDetectorSvc
      include reconcilerSvc
      autoLayout lr
    }

    dynamic infrastructureManagementPlatform "vkr-dynamic-desired-state" "Сценарий 1. Получение желаемого состояния" {
      platformUser -> architectureManifests "Ведет Architecture-as-Code-описание и экспортирует Structurizr JSON-манифесты / Git + Structurizr DSL/JSON"
      parserSvc -> architectureManifests "Загружает Structurizr JSON-манифесты и преобразует deployment nodes в host-centric desired state / Локальный файловый том + in-process mapping"
      driftDetectorSvc -> parserSvc "Запрашивает desired state для detection cycle / HTTP REST API: GET /api/v1/desired-state"
      autoLayout lr
    }

    dynamic infrastructureManagementPlatform "vkr-dynamic-actual-state" "Сценарий 2. Получение фактического состояния" {
      inventorySvc -> inventoryBootstrapConfig "Загружает bootstrap-список хостов из selfProvisioning-конфигурации / YAML-файл конфигурации"
      inventorySvc -> cadvisorAgent "Запрашивает runtime-данные контейнеров для каждого управляемого хоста / HTTP REST API: GET /api/v1.3/subcontainers/"
      cadvisorAgent -> dockerWorkloads "Считывает метаданные и runtime-состояние контейнеров / Docker/cgroup filesystem"
      driftDetectorSvc -> inventorySvc "Запрашивает snapshot actual state с metadata о partial result / HTTP REST API: GET /api/v1/hosts"
      autoLayout lr
    }

    dynamic infrastructureManagementPlatform "vkr-dynamic-drift-detection" "Сценарий 3. Обнаружение drift" {
      driftDetectorSvc -> inventorySvc "Получает snapshot actual state / HTTP REST API: GET /api/v1/hosts"
      driftDetectorSvc -> parserSvc "Получает snapshot desired state / HTTP REST API: GET /api/v1/desired-state"
      driftDetectorSvc -> reconcilerSvc "Отправляет reconcile-команду, если сравнение выявило поддерживаемый drift и cooldown допускает отправку / HTTP REST API: POST /api/v1/reconcile"
      autoLayout lr
    }

    dynamic infrastructureManagementPlatform "vkr-dynamic-drift-reconcile" "Сценарий 4. Устранение drift" {
      driftDetectorSvc -> reconcilerSvc "Отправляет reconcile-команду для обнаруженного поддерживаемого drift / HTTP REST API: POST /api/v1/reconcile"
      reconcilerSvc -> managedHost "Запускает Ansible playbook, который создает или стартует требуемый Docker workload-контейнер / SSH + Ansible Runner + Docker runtime"
      autoLayout lr
    }

    styles {
      element "Person" {
        shape Person
        background "#08427b"
        color "#ffffff"
      }
      element "Software System" {
        background "#1168bd"
        color "#ffffff"
      }
      element "External System" {
        background "#999999"
        color "#ffffff"
      }
      element "Container" {
        background "#438dd5"
        color "#ffffff"
      }
      element "External Container" {
        background "#999999"
        color "#ffffff"
      }
      relationship "Relationship" {
        color "#707070"
      }
    }
  }
}
