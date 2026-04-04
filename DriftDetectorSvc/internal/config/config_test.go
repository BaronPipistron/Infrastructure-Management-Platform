package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestLoadValidConfig(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	configPath := filepath.Join(dir, "appsettings.yml")

	content := `server:
  host: 127.0.0.1
  port: 18080
logging:
  level: debug
detection:
  interval: 45s
  enabledComponents:
    - node_exporter
    - cadvisor
antiSpam:
  reconcileCooldown: 1m
clients:
  inventory:
    baseURL: http://inventory-svc:8080
    hostsPath: /api/v1/hosts
    timeout: 3s
    retry:
      maxAttempts: 3
      backoff: 200ms
  parser:
    baseURL: http://parser-svc:8080
    desiredStatePath: /api/v1/desired-state
    timeout: 4s
    retry:
      maxAttempts: 2
      backoff: 150ms
  reconciler:
    baseURL: http://reconciler-svc:8082
    reconcilePath: /api/v1/reconcile
    timeout: 5s
    retry:
      maxAttempts: 4
      backoff: 250ms
`
	require.NoError(t, os.WriteFile(configPath, []byte(content), 0o600))

	cfg, absPath, err := Load(configPath)
	require.NoError(t, err)
	require.Equal(t, "127.0.0.1", cfg.Server.Host)
	require.Equal(t, 18080, cfg.Server.Port)
	require.Equal(t, "debug", cfg.Logging.Level)
	require.Equal(t, 45*time.Second, cfg.Detection.Interval)
	require.Equal(t, []string{"node_exporter", "cadvisor"}, cfg.Detection.EnabledComponents)
	require.Equal(t, time.Minute, cfg.AntiSpam.ReconcileCooldown)
	require.Equal(t, configPath, absPath)
}

func TestLoadInvalidComponent(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	configPath := filepath.Join(dir, "appsettings.yml")

	content := `detection:
  enabledComponents:
    - unknown_component
`
	require.NoError(t, os.WriteFile(configPath, []byte(content), 0o600))

	_, _, err := Load(configPath)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unsupported component")
}

func TestLoadInvalidClientURL(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	configPath := filepath.Join(dir, "appsettings.yml")

	content := `clients:
  inventory:
    baseURL: "://bad"
    hostsPath: /api/v1/hosts
    timeout: 3s
    retry:
      maxAttempts: 2
      backoff: 100ms
`
	require.NoError(t, os.WriteFile(configPath, []byte(content), 0o600))

	_, _, err := Load(configPath)
	require.Error(t, err)
	require.Contains(t, err.Error(), "clients.inventory.baseURL")
}

func TestResolvePath(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "configs", "appsettings.yml")
	require.NoError(t, os.MkdirAll(filepath.Dir(cfgPath), 0o755))
	require.NoError(t, os.WriteFile(cfgPath, []byte("server: {}"), 0o600))

	resolved, err := ResolvePath(cfgPath, "../manifests")
	require.NoError(t, err)
	require.Equal(t, filepath.Join(dir, "manifests"), resolved)
}
