package logger

import (
	"fmt"
	"strings"

	"go.uber.org/zap"
)

func New(level string) (*zap.SugaredLogger, error) {
	cfg := zap.NewProductionConfig()
	cfg.Level = zap.NewAtomicLevel()

	normalized := strings.ToLower(strings.TrimSpace(level))
	switch normalized {
	case "debug":
		cfg.Level.SetLevel(zap.DebugLevel)
	case "info", "":
		cfg.Level.SetLevel(zap.InfoLevel)
	case "warn", "warning":
		cfg.Level.SetLevel(zap.WarnLevel)
	case "error":
		cfg.Level.SetLevel(zap.ErrorLevel)
	default:
		return nil, fmt.Errorf("unsupported log level: %s", level)
	}

	log, err := cfg.Build()
	if err != nil {
		return nil, err
	}

	return log.Sugar(), nil
}
