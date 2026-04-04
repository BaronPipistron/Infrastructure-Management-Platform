package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"drift-detector-svc/internal/domain"

	"github.com/spf13/viper"
)

type Config struct {
	Server    ServerConfig    `mapstructure:"server"`
	Logging   LoggingConfig   `mapstructure:"logging"`
	Detection DetectionConfig `mapstructure:"detection"`
	AntiSpam  AntiSpamConfig  `mapstructure:"antiSpam"`
	Clients   ClientsConfig   `mapstructure:"clients"`
}

type ServerConfig struct {
	Host string `mapstructure:"host"`
	Port int    `mapstructure:"port"`
}

type LoggingConfig struct {
	Level string `mapstructure:"level"`
}

type DetectionConfig struct {
	Interval          time.Duration `mapstructure:"interval"`
	EnabledComponents []string      `mapstructure:"enabledComponents"`
}

type AntiSpamConfig struct {
	ReconcileCooldown time.Duration `mapstructure:"reconcileCooldown"`
}

type ClientsConfig struct {
	Inventory  InventoryClientConfig  `mapstructure:"inventory"`
	Parser     ParserClientConfig     `mapstructure:"parser"`
	Reconciler ReconcilerClientConfig `mapstructure:"reconciler"`
}

type InventoryClientConfig struct {
	BaseURL   string        `mapstructure:"baseURL"`
	HostsPath string        `mapstructure:"hostsPath"`
	Timeout   time.Duration `mapstructure:"timeout"`
	Retry     RetryConfig   `mapstructure:"retry"`
}

type ParserClientConfig struct {
	BaseURL          string        `mapstructure:"baseURL"`
	DesiredStatePath string        `mapstructure:"desiredStatePath"`
	Timeout          time.Duration `mapstructure:"timeout"`
	Retry            RetryConfig   `mapstructure:"retry"`
}

type ReconcilerClientConfig struct {
	BaseURL       string        `mapstructure:"baseURL"`
	ReconcilePath string        `mapstructure:"reconcilePath"`
	Timeout       time.Duration `mapstructure:"timeout"`
	Retry         RetryConfig   `mapstructure:"retry"`
}

type RetryConfig struct {
	MaxAttempts int           `mapstructure:"maxAttempts"`
	Backoff     time.Duration `mapstructure:"backoff"`
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

	cfg.normalize()
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
	v.SetDefault("server.host", "0.0.0.0")
	v.SetDefault("server.port", 8080)
	v.SetDefault("logging.level", "info")

	v.SetDefault("detection.interval", "30s")
	v.SetDefault("detection.enabledComponents", []string{"node_exporter", "cadvisor"})

	v.SetDefault("antiSpam.reconcileCooldown", "2m")

	v.SetDefault("clients.inventory.baseURL", "http://localhost:8080")
	v.SetDefault("clients.inventory.hostsPath", "/api/v1/hosts")
	v.SetDefault("clients.inventory.timeout", "5s")
	v.SetDefault("clients.inventory.retry.maxAttempts", 2)
	v.SetDefault("clients.inventory.retry.backoff", "300ms")

	v.SetDefault("clients.parser.baseURL", "http://localhost:8082")
	v.SetDefault("clients.parser.desiredStatePath", "/api/v1/desired-state")
	v.SetDefault("clients.parser.timeout", "5s")
	v.SetDefault("clients.parser.retry.maxAttempts", 2)
	v.SetDefault("clients.parser.retry.backoff", "300ms")

	v.SetDefault("clients.reconciler.baseURL", "http://localhost:8083")
	v.SetDefault("clients.reconciler.reconcilePath", "/api/v1/reconcile")
	v.SetDefault("clients.reconciler.timeout", "5s")
	v.SetDefault("clients.reconciler.retry.maxAttempts", 2)
	v.SetDefault("clients.reconciler.retry.backoff", "300ms")
}

func (c *Config) normalize() {
	c.Server.Host = strings.TrimSpace(c.Server.Host)
	if c.Server.Host == "" {
		c.Server.Host = "0.0.0.0"
	}

	c.Logging.Level = strings.ToLower(strings.TrimSpace(c.Logging.Level))
	if c.Logging.Level == "" {
		c.Logging.Level = "info"
	}

	normalized := make([]string, 0, len(c.Detection.EnabledComponents))
	seen := make(map[string]struct{})
	for _, item := range c.Detection.EnabledComponents {
		value := strings.ToLower(strings.TrimSpace(item))
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		normalized = append(normalized, value)
	}
	c.Detection.EnabledComponents = normalized

	c.Clients.Inventory.BaseURL = strings.TrimRight(strings.TrimSpace(c.Clients.Inventory.BaseURL), "/")
	c.Clients.Parser.BaseURL = strings.TrimRight(strings.TrimSpace(c.Clients.Parser.BaseURL), "/")
	c.Clients.Reconciler.BaseURL = strings.TrimRight(strings.TrimSpace(c.Clients.Reconciler.BaseURL), "/")

	c.Clients.Inventory.HostsPath = normalizePath(c.Clients.Inventory.HostsPath)
	c.Clients.Parser.DesiredStatePath = normalizePath(c.Clients.Parser.DesiredStatePath)
	c.Clients.Reconciler.ReconcilePath = normalizePath(c.Clients.Reconciler.ReconcilePath)
}

func normalizePath(path string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return ""
	}
	if !strings.HasPrefix(trimmed, "/") {
		return "/" + trimmed
	}
	return trimmed
}

func (c Config) Validate() error {
	var errs []string

	if strings.TrimSpace(c.Server.Host) == "" {
		errs = append(errs, "server.host is required")
	}
	if c.Server.Port <= 0 || c.Server.Port > 65535 {
		errs = append(errs, "server.port must be in range [1..65535]")
	}
	if strings.TrimSpace(c.Logging.Level) == "" {
		errs = append(errs, "logging.level is required")
	}
	if c.Detection.Interval <= 0 {
		errs = append(errs, "detection.interval must be > 0")
	}
	if c.AntiSpam.ReconcileCooldown < 0 {
		errs = append(errs, "antiSpam.reconcileCooldown must be >= 0")
	}
	if len(c.Detection.EnabledComponents) == 0 {
		errs = append(errs, "detection.enabledComponents must contain at least one component")
	}

	for _, raw := range c.Detection.EnabledComponents {
		if _, err := domain.ParseComponent(raw); err != nil {
			errs = append(errs, "detection.enabledComponents contains unsupported component: "+raw)
		}
	}

	validateHTTPClientConfig("clients.inventory", c.Clients.Inventory.BaseURL, c.Clients.Inventory.HostsPath, c.Clients.Inventory.Timeout, c.Clients.Inventory.Retry, &errs)
	validateHTTPClientConfig("clients.parser", c.Clients.Parser.BaseURL, c.Clients.Parser.DesiredStatePath, c.Clients.Parser.Timeout, c.Clients.Parser.Retry, &errs)
	validateHTTPClientConfig("clients.reconciler", c.Clients.Reconciler.BaseURL, c.Clients.Reconciler.ReconcilePath, c.Clients.Reconciler.Timeout, c.Clients.Reconciler.Retry, &errs)

	if len(errs) > 0 {
		return errors.New("invalid config: " + strings.Join(errs, "; "))
	}

	return nil
}

func validateHTTPClientConfig(prefix, baseURL, path string, timeout time.Duration, retry RetryConfig, errs *[]string) {
	if strings.TrimSpace(baseURL) == "" {
		*errs = append(*errs, prefix+".baseURL is required")
	} else {
		parsed, err := url.Parse(baseURL)
		if err != nil || parsed.Scheme == "" || parsed.Host == "" {
			*errs = append(*errs, prefix+".baseURL must be a valid absolute URL")
		}
	}

	if strings.TrimSpace(path) == "" {
		*errs = append(*errs, prefix+" endpoint path is required")
	} else if !strings.HasPrefix(path, "/") {
		*errs = append(*errs, prefix+" endpoint path must start with '/'")
	}

	if timeout <= 0 {
		*errs = append(*errs, prefix+".timeout must be > 0")
	}
	if retry.MaxAttempts <= 0 {
		*errs = append(*errs, prefix+".retry.maxAttempts must be > 0")
	}
	if retry.Backoff < 0 {
		*errs = append(*errs, prefix+".retry.backoff must be >= 0")
	}
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
