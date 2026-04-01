package parser

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseStructurizrManifest(t *testing.T) {
	t.Parallel()

	payload := []byte(`
{
  "model": {
    "deploymentNodes": [
      {"id": "1", "properties": {"host_id": "srv-1"}, "containerInstances": []}
    ],
    "softwareSystems": [
      {"id": "sys-1", "containers": [{"id": "c1", "name": "node_exporter"}]}
    ]
  }
}`)

	manifest, err := ParseStructurizrManifest(payload)
	require.NoError(t, err)
	require.Len(t, manifest.Model.DeploymentNodes, 1)
	require.Len(t, manifest.Model.SoftwareSystems, 1)
	require.Equal(t, "c1", manifest.Model.SoftwareSystems[0].Containers[0].ID)
}

func TestParseStructurizrManifestInvalidJSON(t *testing.T) {
	t.Parallel()

	_, err := ParseStructurizrManifest([]byte(`{`))
	require.Error(t, err)
}

func TestParseStructurizrManifestNoDeploymentNodes(t *testing.T) {
	t.Parallel()

	_, err := ParseStructurizrManifest([]byte(`{"model": {"deploymentNodes": []}}`))
	require.Error(t, err)
	require.Contains(t, err.Error(), "no deployment nodes")
}
