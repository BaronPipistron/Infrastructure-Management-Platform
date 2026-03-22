package selfprovisioning

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadValidSelfProvisioning(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "selfProvisioning.yml")

	content := `hosts:
  - id: host-1
    fqdn: db01.dev.example.internal
    labels:
      managed_by: team-a
      purpose: database
      env: dev
`
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	hosts, err := Load(path)
	require.NoError(t, err)
	require.Len(t, hosts, 1)
	require.Equal(t, "host-1", hosts[0].ID)
	require.Equal(t, "db01.dev.example.internal", hosts[0].FQDN)
}

func TestLoadInvalidSelfProvisioning(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "selfProvisioning-invalid.yml")

	content := `hosts:
  - id: host-1
    fqdn: db01.dev.example.internal
    labels:
      managed_by: team-a
      env: dev
`
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	_, err := Load(path)
	require.Error(t, err)
	require.Contains(t, err.Error(), "labels.purpose")
}
