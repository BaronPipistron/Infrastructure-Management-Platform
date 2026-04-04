package detectors

import (
	"testing"

	"drift-detector-svc/internal/domain"

	"github.com/stretchr/testify/require"
)

func TestRegistrySelectsDetectorByComponent(t *testing.T) {
	t.Parallel()

	registry := NewRegistry(
		NewNodeExporterDetector(),
		NewCadvisorDetector(),
	)

	nodeDetector, found := registry.Get(domain.ComponentNodeExporter)
	require.True(t, found)
	require.Equal(t, domain.ComponentNodeExporter, nodeDetector.Component())

	cadvisorDetector, found := registry.Get(domain.ComponentCadvisor)
	require.True(t, found)
	require.Equal(t, domain.ComponentCadvisor, cadvisorDetector.Component())
}

func TestRegistryReturnsFalseForUnknownComponent(t *testing.T) {
	t.Parallel()

	registry := NewRegistry(NewNodeExporterDetector())
	_, found := registry.Get(domain.ComponentCadvisor)
	require.False(t, found)
}
