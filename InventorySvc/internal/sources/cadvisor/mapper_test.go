package cadvisor

import (
	"testing"
	"time"

	"inventory-svc/internal/domain"

	"github.com/stretchr/testify/require"
)

func TestNormalizeContainers(t *testing.T) {
	t.Parallel()

	firstTs := time.Date(2026, 3, 22, 10, 0, 0, 0, time.UTC)
	secondTs := time.Date(2026, 3, 22, 10, 1, 0, 0, time.UTC)

	response := containersResponse{
		{
			Name:    "/",
			Aliases: []string{"root"},
		},
		{
			Name:      "/docker/abc123",
			Aliases:   []string{"abc123", "nginx"},
			Namespace: "docker",
			Spec: containerSpec{
				Image: "nginx:1.27",
			},
			Stats: []containerStat{
				{Timestamp: firstTs},
				{Timestamp: secondTs},
			},
		},
	}

	workloads, latest := NormalizeContainers(response)

	require.Len(t, workloads, 1)
	require.Equal(t, "abc123", workloads[0].ID)
	require.Equal(t, "abc123", workloads[0].Name)
	require.Equal(t, "nginx:1.27", workloads[0].Image)
	require.Equal(t, "docker", workloads[0].Runtime)
	require.NotNil(t, workloads[0].LastSeenAt)
	require.True(t, secondTs.Equal(*workloads[0].LastSeenAt))
	require.True(t, secondTs.Equal(latest))
}

func TestFilterWorkloads_DefaultExcludesSystemEntries(t *testing.T) {
	t.Parallel()

	workloads := []domain.Workload{
		{
			ID:      "/docker",
			Name:    "docker",
			Runtime: "unknown",
			Source:  sourceName,
		},
		{
			ID:      "inventory-svc",
			Name:    "inventory-svc",
			Image:   "inventorysvc-inventory-svc",
			Runtime: "docker",
			Source:  sourceName,
		},
	}

	filtered := FilterWorkloads(workloads, false)
	require.Len(t, filtered, 1)
	require.Equal(t, "inventory-svc", filtered[0].ID)
}

func TestFilterWorkloads_IncludeSystemKeepsAll(t *testing.T) {
	t.Parallel()

	workloads := []domain.Workload{
		{
			ID:      "/docker",
			Name:    "docker",
			Runtime: "unknown",
			Source:  sourceName,
		},
		{
			ID:      "inventory-svc",
			Name:    "inventory-svc",
			Image:   "inventorysvc-inventory-svc",
			Runtime: "docker",
			Source:  sourceName,
		},
	}

	filtered := FilterWorkloads(workloads, true)
	require.Len(t, filtered, 2)
}
