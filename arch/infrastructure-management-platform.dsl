workspace "Infrastructure Management Platform" {
  model {
    user = person "Platform User" {
      description "SRE / DevOps / Architect"
    }

    netBox = softwareSystem "NetBox" {
      tags "External System"
    }

    otherSourceOfTruth = softwareSystem "Other Source of Truth about Hosts" {
      tags "External System"
    }

    gitlab = softwareSystem "GitLab" {
      description "Store Architecture as Code manifests in Parser repo"
      tags "External System"
    }

    managedHost = softwareSystem "Managed Host"{
      cAdvisor = container "cAdvisor" {
        tags "External System"
      }
      
      workloads = container "Workloads" {
        tags "External System"
      }

      tags "External System"
    }

    infrastructureManagementPlatform = softwareSystem "Infrastructure Management Platform" {
      parser = container "Parser" {
        technology "Golang"

        parserApi = component "Parser API" {
          description "Allow get hosts desired state"
        }
        parserConfigLoader = component "Configuration Loader"
        dslParser = component "DSL Parser"
        modelValidator = component "Model Validator"
        desiredStateBuilder = component "Desired State Builder"
        desiredStateStore = component "InMemory Desired State Store"
      }

      inventory = container "Inventory" {
        technology "Golang"

        inventoryApi = component "Inventory API"
        inventoryService = component "Inventory Service"
        netBoxClient = component "NetBox Client"
        cAdvisorClient = component "cAdvisor Client"
        externalSourcesClient = component "External Sources Client"
        inventoryConfigLoader = component "Configuration Loader"
      }

      driftDetector = container "Drift Detector" {
        technology "Golang"

        driftDetectionService = component "Drift Detection Service"
        inventoryClient = component "Inventory Client"
        reconcilerClient = component "Reconciler Client"
        parserClient = component "Parser Client"
        driftDetectorConfigLoader = component "Configuration Loader"
      }

      reconciler = container "Reconciler" {
        technology "Python"

        reconcilerApi = component "Reconciler API"
        reconcileService = component "Reconcile Service" {
          description "Ansible Runner"
        }
        reconcilerConfigLoader = component "Configuration Loader"
      }
    }

    user -> gitlab
    gitlab -> parser "DSL files deployed on Parser host by CI/CD"

    parserConfigLoader -> dslParser
    dslParser -> modelValidator
    dslParser -> desiredStateBuilder
    desiredStateBuilder -> desiredStateStore
    parserApi -> desiredStateStore

    inventoryApi -> inventoryService
    inventoryService -> netBoxClient
    inventoryService -> cAdvisorClient
    inventoryService -> externalSourcesClient
    inventoryService -> inventoryConfigLoader

    netBoxClient -> netBox
    cAdvisorClient -> cAdvisor
    externalSourcesClient -> otherSourceOfTruth

    driftDetectionService -> parserClient
    driftDetectionService -> inventoryClient
    driftDetectionService -> reconcilerClient
    driftDetectionService -> driftDetectorConfigLoader

    parserClient -> parserApi
    inventoryClient -> inventoryApi
    reconcilerClient -> reconcilerApi

    reconcilerApi -> reconcileService
    reconcileService -> workloads
    reconcileService -> reconcilerConfigLoader
  }

  views {
    systemContext infrastructureManagementPlatform {
      include user
      include *

      autoLayout lr
    }

    container managedHost {
      include gitlab

      include parser
      include inventory
      include driftDetector
      include reconciler

      include netBox
      include otherSourceOfTruth

      include managedHost
      include cAdvisor
      include workloads

      autoLayout lr
    }

    component inventory "inventory-components" {
      include *

      autoLayout lr
    }

    component driftDetector "drift-detector-components" {
      include *

      autoLayout lr
    }

    component reconciler "reconciler-components" {
      include *

      autoLayout lr
    }

    component parser "parser-components" {
      include *

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
      element "Component" {
        background "#85bbf0"
        color "#000000"
      }
    }
  }
}
