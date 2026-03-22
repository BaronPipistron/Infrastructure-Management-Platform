package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/viper"
)

const defaultCAdvisorURLTemplate = "{{scheme}}://{{fqdn}}:{{port}}{{basePath}}{{containersPath}}"

type Config struct {
	Server    ServerConfig    `mapstructure:"server"`
	Logging   LoggingConfig   `mapstructure:"logging"`
	Sync      SyncConfig      `mapstructure:"sync"`
	Sources   SourcesConfig   `mapstructure:"sources"`
	Bootstrap BootstrapConfig `mapstructure:"bootstrap"`
}

type ServerConfig struct {
	Port int `mapstructure:"port"`
}

type LoggingConfig struct {
	Level string `mapstructure:"level"`
}

type SyncConfig struct {
	Interval time.Duration `mapstructure:"interval"`
}

type BootstrapConfig struct {
	SelfProvisioningPath string `mapstructure:"selfProvisioningPath"`
}

type SourcesConfig struct {
	CAdvisor     CAdvisorConfig   `mapstructure:"cadvisor"`
	NetBox       ToggleOnlySource `mapstructure:"netbox"`
	OtherSources ToggleOnlySource `mapstructure:"otherSources"`
}

type ToggleOnlySource struct {
	IsEnabled bool `mapstructure:"isEnabled"`
}

type CAdvisorConfig struct {
	IsEnabled              bool          `mapstructure:"isEnabled"`
	IncludeSystemWorkloads bool          `mapstructure:"includeSystemWorkloads"`
	Scheme                 string        `mapstructure:"scheme"`
	Port                   int           `mapstructure:"port"`
	BasePath               string        `mapstructure:"basePath"`
	ContainersPath         string        `mapstructure:"containersPath"`
	Timeout                time.Duration `mapstructure:"timeout"`
	URLTemplate            string        `mapstructure:"urlTemplate"`
}

func Load(path string) (Config, string, error) {
	if strings.TrimSpace(path) == "" {
		return Config{}, "", errors.New("config path is required")
	}

	v := viper.New()
	v.SetConfigFile(path)
	v.SetConfigType("yaml")
	setDefaults(v)

	if err := v.ReadInConfig(); err != nil {
		return Config{}, "", fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return Config{}, "", fmt.Errorf("unmarshal config: %w", err)
	}

	normalizeCAdvisorConfig(&cfg.Sources.CAdvisor)
	if err := cfg.Validate(); err != nil {
		return Config{}, "", err
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return Config{}, "", fmt.Errorf("resolve absolute config path: %w", err)
	}

	return cfg, absPath, nil
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("server.port", 8080)
	v.SetDefault("logging.level", "info")
	v.SetDefault("sync.interval", "30s")
	v.SetDefault("sources.cadvisor.scheme", "http")
	v.SetDefault("sources.cadvisor.includeSystemWorkloads", false)
	v.SetDefault("sources.cadvisor.port", 8080)
	v.SetDefault("sources.cadvisor.basePath", "/api/v1.3")
	v.SetDefault("sources.cadvisor.containersPath", "/subcontainers/")
	v.SetDefault("sources.cadvisor.timeout", "5s")
	v.SetDefault("sources.cadvisor.urlTemplate", defaultCAdvisorURLTemplate)
}

func normalizeCAdvisorConfig(cfg *CAdvisorConfig) {
	cfg.Scheme = strings.TrimSpace(cfg.Scheme)
	if cfg.Scheme == "" {
		cfg.Scheme = "http"
	}

	if cfg.BasePath == "" {
		cfg.BasePath = "/"
	}
	if !strings.HasPrefix(cfg.BasePath, "/") {
		cfg.BasePath = "/" + cfg.BasePath
	}
	cfg.BasePath = strings.TrimSuffix(cfg.BasePath, "/")

	if cfg.ContainersPath == "" {
		cfg.ContainersPath = "/subcontainers/"
	}
	if !strings.HasPrefix(cfg.ContainersPath, "/") {
		cfg.ContainersPath = "/" + cfg.ContainersPath
	}

	if strings.TrimSpace(cfg.URLTemplate) == "" {
		cfg.URLTemplate = defaultCAdvisorURLTemplate
	}
}

func (c Config) Validate() error {
	var errs []string

	if c.Server.Port <= 0 || c.Server.Port > 65535 {
		errs = append(errs, "server.port must be in range [1..65535]")
	}

	if c.Sync.Interval <= 0 {
		errs = append(errs, "sync.interval must be > 0")
	}

	if strings.TrimSpace(c.Logging.Level) == "" {
		errs = append(errs, "logging.level is required")
	}

	if strings.TrimSpace(c.Bootstrap.SelfProvisioningPath) == "" {
		errs = append(errs, "bootstrap.selfProvisioningPath is required")
	}

	if c.Sources.CAdvisor.IsEnabled {
		if c.Sources.CAdvisor.Port <= 0 || c.Sources.CAdvisor.Port > 65535 {
			errs = append(errs, "sources.cadvisor.port must be in range [1..65535]")
		}
		if c.Sources.CAdvisor.Timeout <= 0 {
			errs = append(errs, "sources.cadvisor.timeout must be > 0")
		}
		if strings.TrimSpace(c.Sources.CAdvisor.URLTemplate) == "" {
			errs = append(errs, "sources.cadvisor.urlTemplate is required when cAdvisor is enabled")
		}
	}

	if len(errs) > 0 {
		return errors.New("invalid config: " + strings.Join(errs, "; "))
	}

	return nil
}

func ResolvePath(configFilePath, configuredPath string) (string, error) {
	configuredPath = strings.TrimSpace(configuredPath)
	if configuredPath == "" {
		return "", errors.New("configured path is empty")
	}

	if filepath.IsAbs(configuredPath) {
		return configuredPath, nil
	}

	if pathExists(configuredPath) {
		absPath, err := filepath.Abs(configuredPath)
		if err != nil {
			return "", fmt.Errorf("resolve absolute configured path: %w", err)
		}
		return absPath, nil
	}

	baseDir := filepath.Dir(configFilePath)
	joined := filepath.Join(baseDir, configuredPath)
	absPath, err := filepath.Abs(joined)
	if err != nil {
		return "", fmt.Errorf("resolve config-relative path: %w", err)
	}

	return absPath, nil
}

func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
