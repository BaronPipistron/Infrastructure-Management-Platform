workspace "Infrastructure Management Desired State" {
  !identifiers hierarchical

  model {
    managedWorkloads = softwareSystem "Managed Workloads" {
      nodeExporter = container "node_exporter" "Host metrics exporter" "container"
      cadvisor = container "cadvisor" "Container and workload metrics collector" "container"
    }

    deploymentEnvironment "Production" {
      deploymentNode "srv-001" {
        properties {
          host_id "srv-001"
          fqdn "srv-001.example.local"
          ip "10.10.10.11"
          env "prod"
          managed_by "platform-team"
          purpose "compute"
        }

        containerInstance managedWorkloads.nodeExporter {
          properties {
            enabled "true"
            deployment_mode "container"
            image "quay.io/prometheus/node-exporter:v1.8.2"
            port "9100"
          }
        }

        containerInstance managedWorkloads.cadvisor {
          properties {
            enabled "true"
            deployment_mode "container"
            image "gcr.io/cadvisor/cadvisor:v0.49.1"
            port "8080"
          }
        }
      }
    }
  }

  views {
    deployment managedWorkloads "Production" "prod-deployment" {
      include *
      autoLayout lr
    }
  }
}