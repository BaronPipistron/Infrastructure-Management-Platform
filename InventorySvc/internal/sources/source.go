package sources

import (
	"context"
	"fmt"

	"inventory-svc/internal/domain"
)

type HostSource interface {
	Name() string
	Enabled() bool
	CollectHost(ctx context.Context, host domain.HostSeed) (domain.SourceData, error)
}

type PlaceholderSource struct {
	SourceName string
	IsOn       bool
}

func (p PlaceholderSource) Name() string {
	return p.SourceName
}

func (p PlaceholderSource) Enabled() bool {
	return p.IsOn
}

func (p PlaceholderSource) CollectHost(ctx context.Context, host domain.HostSeed) (domain.SourceData, error) {
	_ = ctx
	_ = host
	return domain.SourceData{}, fmt.Errorf("source %q is enabled but not implemented in MVP", p.SourceName)
}
