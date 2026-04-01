package loader

import (
	"os"
	"path/filepath"
	"testing"

	"parser-svc/internal/config"

	"github.com/stretchr/testify/require"
)

func TestDiscoverFilesDirectoryModeIgnoresDSL(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	jsonPath := filepath.Join(dir, "prod.json")
	dslPath := filepath.Join(dir, "prod.dsl")
	txtPath := filepath.Join(dir, "README.txt")

	require.NoError(t, os.WriteFile(jsonPath, []byte(`{"model":{"deploymentNodes":[]}}`), 0o600))
	require.NoError(t, os.WriteFile(dslPath, []byte("workspace {}"), 0o600))
	require.NoError(t, os.WriteFile(txtPath, []byte("ignored"), 0o600))

	set, err := DiscoverFiles(config.ManifestModeDirectory, dir)
	require.NoError(t, err)
	require.Equal(t, []string{jsonPath}, set.JSONFiles)
	require.ElementsMatch(t, []string{dslPath, txtPath}, set.IgnoredFiles)
}

func TestDiscoverFilesFileMode(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	jsonPath := filepath.Join(dir, "prod.json")
	require.NoError(t, os.WriteFile(jsonPath, []byte(`{"ok":true}`), 0o600))

	set, err := DiscoverFiles(config.ManifestModeFile, jsonPath)
	require.NoError(t, err)
	require.Equal(t, []string{jsonPath}, set.JSONFiles)
	require.Empty(t, set.IgnoredFiles)
}

func TestLoadFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	jsonPath := filepath.Join(dir, "prod.json")
	require.NoError(t, os.WriteFile(jsonPath, []byte("payload"), 0o600))

	data, err := LoadFile(jsonPath)
	require.NoError(t, err)
	require.Equal(t, "payload", string(data))
}
