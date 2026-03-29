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

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "appsettings.yml")

	content := `server:
  port: 9090
logging:
  level: debug
sync:
  interval: 45s
sources:
  cadvisor:
    isEnabled: true
    includeSystemWorkloads: true
    scheme: http
    port: 8085
    basePath: /api/v1.3
    containersPath: /subcontainers/
    timeout: 3s
    urlTemplate: "{{scheme}}://{{fqdn}}:{{port}}{{basePath}}{{containersPath}}"
  netbox:
    isEnabled: false
  otherSources:
    isEnabled: false
bootstrap:
  selfProvisioningPath: ./selfProvisioning.yml
`
	require.NoError(t, os.WriteFile(configPath, []byte(content), 0o600))

	cfg, absPath, err := Load(configPath)
	require.NoError(t, err)
	require.Equal(t, 9090, cfg.Server.Port)
	require.Equal(t, "debug", cfg.Logging.Level)
	require.Equal(t, 45*time.Second, cfg.Sync.Interval)
	require.True(t, cfg.Sources.CAdvisor.IsEnabled)
	require.True(t, cfg.Sources.CAdvisor.IncludeSystemWorkloads)
	require.Equal(t, 8085, cfg.Sources.CAdvisor.Port)
	require.Equal(t, 3*time.Second, cfg.Sources.CAdvisor.Timeout)
	require.NotEmpty(t, absPath)
}

func TestLoadInvalidConfig(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "invalid.yml")

	content := `server:
  port: 70000
sync:
  interval: 0s
logging:
  level: info
sources:
  cadvisor:
    isEnabled: true
    port: 0
    timeout: 0s
bootstrap:
  selfProvisioningPath: ""
`
	require.NoError(t, os.WriteFile(configPath, []byte(content), 0o600))

	_, _, err := Load(configPath)
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid config")
}

func TestResolvePath(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "configs", "appsettings.yml")
	require.NoError(t, os.MkdirAll(filepath.Dir(configPath), 0o755))
	require.NoError(t, os.WriteFile(configPath, []byte("server:\n  port: 8080\n"), 0o600))

	target := filepath.Join(tmpDir, "configs", "selfProvisioning.yml")
	require.NoError(t, os.WriteFile(target, []byte("hosts: []\n"), 0o600))

	resolved, err := ResolvePath(configPath, "selfProvisioning.yml")
	require.NoError(t, err)
	require.Equal(t, target, resolved)
}

func TestLoadConfig_DefaultIncludeSystemWorkloadsIsFalse(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "appsettings-default.yml")

	content := `server:
  port: 8080
logging:
  level: info
sync:
  interval: 30s
sources:
  cadvisor:
    isEnabled: true
    port: 8080
    timeout: 5s
bootstrap:
  selfProvisioningPath: ./selfProvisioning.yml
`
	require.NoError(t, os.WriteFile(configPath, []byte(content), 0o600))

	cfg, _, err := Load(configPath)
	require.NoError(t, err)
	require.False(t, cfg.Sources.CAdvisor.IncludeSystemWorkloads)
}
