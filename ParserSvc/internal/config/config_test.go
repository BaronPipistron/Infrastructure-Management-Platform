package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	configPath := filepath.Join(dir, "appconfig.yml")
	err := os.WriteFile(configPath, []byte(`
server:
  host: 127.0.0.1
  port: 19090
logging:
  level: debug
manifests:
  mode: file
  path: ./prod.json
`), 0o600)
	require.NoError(t, err)

	cfg, absPath, err := Load(configPath)
	require.NoError(t, err)
	require.Equal(t, "127.0.0.1", cfg.Server.Host)
	require.Equal(t, 19090, cfg.Server.Port)
	require.Equal(t, "debug", cfg.Logging.Level)
	require.Equal(t, ManifestModeFile, cfg.Manifests.Mode)
	require.Equal(t, "./prod.json", cfg.Manifests.Path)
	require.Equal(t, configPath, absPath)
}

func TestLoadConfigInvalidMode(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	configPath := filepath.Join(dir, "appconfig.yml")
	err := os.WriteFile(configPath, []byte(`
manifests:
  mode: unknown
  path: ./prod.json
`), 0o600)
	require.NoError(t, err)

	_, _, err = Load(configPath)
	require.Error(t, err)
	require.Contains(t, err.Error(), "manifests.mode")
}

func TestResolvePath(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "configs", "appconfig.yml")
	require.NoError(t, os.MkdirAll(filepath.Dir(cfgPath), 0o755))
	require.NoError(t, os.WriteFile(cfgPath, []byte("manifests: {}"), 0o600))

	resolved, err := ResolvePath(cfgPath, "../manifests")
	require.NoError(t, err)
	require.Equal(t, filepath.Join(dir, "manifests"), resolved)
}
