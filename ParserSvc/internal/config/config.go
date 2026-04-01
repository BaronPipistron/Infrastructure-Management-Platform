package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	Server    ServerConfig    `mapstructure:"server"`
	Logging   LoggingConfig   `mapstructure:"logging"`
	Manifests ManifestsConfig `mapstructure:"manifests"`
}

type ServerConfig struct {
	Host string `mapstructure:"host"`
	Port int    `mapstructure:"port"`
}

type LoggingConfig struct {
	Level string `mapstructure:"level"`
}

type ManifestsConfig struct {
	Mode string `mapstructure:"mode"`
	Path string `mapstructure:"path"`
}

const (
	ManifestModeFile      = "file"
	ManifestModeDirectory = "directory"
)

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
	v.SetDefault("manifests.mode", ManifestModeDirectory)
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

	c.Manifests.Mode = strings.ToLower(strings.TrimSpace(c.Manifests.Mode))
	c.Manifests.Path = strings.TrimSpace(c.Manifests.Path)
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

	switch c.Manifests.Mode {
	case ManifestModeFile, ManifestModeDirectory:
	default:
		errs = append(errs, "manifests.mode must be one of: file, directory")
	}

	if strings.TrimSpace(c.Manifests.Path) == "" {
		errs = append(errs, "manifests.path is required")
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
