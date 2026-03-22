package inventory

import (
	"testing"

	"inventory-svc/internal/domain"

	"github.com/stretchr/testify/require"
)

func TestFilterHostsByLabels_ANDSemantics(t *testing.T) {
	t.Parallel()

	hosts := []domain.Host{
		{
			ID: "host-1",
			Labels: map[string]string{
				"env":     "prod",
				"purpose": "database",
			},
		},
		{
			ID: "host-2",
			Labels: map[string]string{
				"env":     "prod",
				"purpose": "compute",
			},
		},
		{
			ID: "host-3",
			Labels: map[string]string{
				"env":     "dev",
				"purpose": "database",
			},
		},
	}

	filters := map[string]string{
		"env":     "prod",
		"purpose": "database",
	}

	filtered := FilterHostsByLabels(hosts, filters)
	require.Len(t, filtered, 1)
	require.Equal(t, "host-1", filtered[0].ID)
}

func TestFilterHostsByLabels_ReturnsClonedItems(t *testing.T) {
	t.Parallel()

	hosts := []domain.Host{
		{
			ID: "host-1",
			Labels: map[string]string{
				"env": "prod",
			},
		},
	}

	result := FilterHostsByLabels(hosts, nil)
	require.Len(t, result, 1)
	result[0].Labels["env"] = "changed"
	require.Equal(t, "prod", hosts[0].Labels["env"])
}
