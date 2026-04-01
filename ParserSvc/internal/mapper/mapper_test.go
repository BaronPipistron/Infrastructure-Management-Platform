package mapper

import (
	"testing"

	"parser-svc/internal/parser"

	"github.com/stretchr/testify/require"
)

func TestMapManifestSuccess(t *testing.T) {
	t.Parallel()

	manifest := parser.StructurizrManifest{
		Model: parser.StructurizrModel{
			SoftwareSystems: []parser.SoftwareSystem{
				{Containers: []parser.Container{{ID: "2", Name: "node_exporter"}}},
			},
			DeploymentNodes: []parser.DeploymentNode{
				{
					ID: "node-1",
					Properties: map[string]interface{}{
						"host_id":    "srv-001",
						"fqdn":       "srv-001.example.local",
						"ip":         "10.10.10.11",
						"env":        "prod",
						"managed_by": "platform-team",
						"purpose":    "compute",
						"region":     "ru-msk-1",
					},
					ContainerInstances: []parser.ContainerInstance{
						{
							ID:          "ci-1",
							ContainerID: "2",
							Properties: map[string]interface{}{
								"enabled":         "true",
								"deployment_mode": "container",
								"image":           "quay.io/prometheus/node-exporter:v1.8.2",
								"port":            "9100",
							},
						},
					},
				},
			},
		},
	}

	result := MapManifest(manifest, "prod.json")
	require.Len(t, result.Hosts, 1)
	require.Empty(t, result.Warnings)
	require.Equal(t, 1, result.Workloads)

	host := result.Hosts[0]
	require.Equal(t, "srv-001", host.HostID)
	require.Equal(t, "srv-001.example.local", host.FQDN)
	require.Equal(t, "prod", host.Env)
	require.Equal(t, "platform-team", host.ManagedBy)
	require.Equal(t, "compute", host.Purpose)
	require.Equal(t, "ru-msk-1", host.Labels["region"])
	require.Equal(t, "prod", host.Labels["env"])

	require.Len(t, host.Workloads, 1)
	workload := host.Workloads[0]
	require.Equal(t, "node_exporter", workload.Name)
	require.True(t, workload.Enabled)
	require.Equal(t, "container", workload.DeploymentMode)
	require.NotNil(t, workload.Port)
	require.Equal(t, 9100, *workload.Port)
}

func TestMapManifestSkipsInvalidWorkload(t *testing.T) {
	t.Parallel()

	manifest := parser.StructurizrManifest{
		Model: parser.StructurizrModel{
			SoftwareSystems: []parser.SoftwareSystem{
				{Containers: []parser.Container{{ID: "2", Name: "node_exporter"}}},
			},
			DeploymentNodes: []parser.DeploymentNode{
				{
					ID: "node-1",
					Properties: map[string]interface{}{
						"host_id":    "srv-001",
						"fqdn":       "srv-001.example.local",
						"env":        "prod",
						"managed_by": "platform-team",
						"purpose":    "compute",
					},
					ContainerInstances: []parser.ContainerInstance{
						{
							ID:          "ci-1",
							ContainerID: "2",
							Properties: map[string]interface{}{
								"enabled":         "true",
								"deployment_mode": "container",
							},
						},
					},
				},
			},
		},
	}

	result := MapManifest(manifest, "prod.json")
	require.Len(t, result.Hosts, 1)
	require.Len(t, result.Warnings, 1)
	require.Empty(t, result.Hosts[0].Workloads)
}

func TestNormalizeBoolAndInt(t *testing.T) {
	t.Parallel()

	parsedBool, err := normalizeBool("true")
	require.NoError(t, err)
	require.True(t, parsedBool)

	parsedPort, err := normalizeInt("9100")
	require.NoError(t, err)
	require.Equal(t, 9100, parsedPort)
}
