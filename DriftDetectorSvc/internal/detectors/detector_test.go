package detectors

import (
	"testing"

	"drift-detector-svc/internal/domain"

	"github.com/stretchr/testify/require"
)

func TestNodeExporterDetectorDetectsMissingComponent(t *testing.T) {
	t.Parallel()

	detector := NewNodeExporterDetector()
	result := detector.Detect("cycle-1", domain.DesiredHost{
		HostID: "srv-001",
		FQDN:   "srv-001.example.local",
		Workloads: []domain.DesiredWorkload{
			{
				Name:           "node_exporter",
				Enabled:        true,
				DeploymentMode: "container",
				Image:          "quay.io/prometheus/node-exporter:v1.8.2",
			},
		},
	}, domain.ActualHost{
		HostID: "srv-001",
		FQDN:   "srv-001.example.local",
		SourceStatus: map[string]domain.ActualSourceStatus{
			"cadvisor": {Source: "cadvisor", Enabled: true, Status: "ok"},
		},
		Workloads: []domain.ActualWorkload{},
	})

	require.True(t, result.Applicable)
	require.True(t, result.Drift)
	require.NotNil(t, result.Finding)
	require.NotNil(t, result.ReconcileCommand)
	require.Equal(t, domain.ComponentNodeExporter, result.ReconcileCommand.Component)
}

func TestNodeExporterDetectorNoDriftWhenPresent(t *testing.T) {
	t.Parallel()

	detector := NewNodeExporterDetector()
	result := detector.Detect("cycle-1", domain.DesiredHost{
		HostID: "srv-001",
		FQDN:   "srv-001.example.local",
		Workloads: []domain.DesiredWorkload{
			{Name: "node_exporter", Enabled: true, DeploymentMode: "container"},
		},
	}, domain.ActualHost{
		HostID: "srv-001",
		FQDN:   "srv-001.example.local",
		SourceStatus: map[string]domain.ActualSourceStatus{
			"cadvisor": {Source: "cadvisor", Enabled: true, Status: "ok"},
		},
		Workloads: []domain.ActualWorkload{
			{Name: "node_exporter"},
		},
	})

	require.True(t, result.Applicable)
	require.False(t, result.Drift)
	require.Nil(t, result.ReconcileCommand)
}

func TestCadvisorDetectorBootstrapReconcileWhenActualDataIsUnavailable(t *testing.T) {
	t.Parallel()

	detector := NewCadvisorDetector()
	result := detector.Detect("cycle-1", domain.DesiredHost{
		HostID: "srv-001",
		FQDN:   "srv-001.example.local",
		Workloads: []domain.DesiredWorkload{
			{Name: "cadvisor", Enabled: true, DeploymentMode: "container"},
		},
	}, domain.ActualHost{
		HostID: "srv-001",
		FQDN:   "srv-001.example.local",
		SourceStatus: map[string]domain.ActualSourceStatus{
			"cadvisor": {Source: "cadvisor", Enabled: true, Status: "error", Error: "timeout"},
		},
	})

	require.True(t, result.Applicable)
	require.True(t, result.Drift)
	require.False(t, result.MissingActualData)
	require.NotNil(t, result.ReconcileCommand)
	require.Equal(t, domain.ComponentCadvisor, result.ReconcileCommand.Component)
}
