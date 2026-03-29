package cadvisor

import (
	"context"
	"fmt"
	"time"

	"inventory-svc/internal/config"
	"inventory-svc/internal/domain"

	"go.uber.org/zap"
)

type Source struct {
	cfg    config.CAdvisorConfig
	client *Client
	log    *zap.SugaredLogger
}

func NewSource(cfg config.CAdvisorConfig, log *zap.SugaredLogger) *Source {
	return &Source{
		cfg:    cfg,
		client: NewClient(cfg),
		log:    log,
	}
}

func (s *Source) Name() string {
	return sourceName
}

func (s *Source) Enabled() bool {
	return s.cfg.IsEnabled
}

func (s *Source) CollectHost(ctx context.Context, host domain.HostSeed) (domain.SourceData, error) {
	if !s.cfg.IsEnabled {
		return domain.SourceData{}, fmt.Errorf("source %s is disabled", sourceName)
	}

	requestCtx, cancel := deadlineContext(ctx, s.cfg.Timeout)
	defer cancel()

	start := time.Now()
	containers, err := s.client.FetchContainers(requestCtx, host.FQDN)
	if err != nil {
		return domain.SourceData{}, err
	}

	workloads, _ := NormalizeContainers(containers)
	workloads = FilterWorkloads(workloads, s.cfg.IncludeSystemWorkloads)
	observedAt := time.Now().UTC()

	s.log.Debugw("cadvisor host collected",
		"host_id", host.ID,
		"fqdn", host.FQDN,
		"workloads_count", len(workloads),
		"duration_ms", time.Since(start).Milliseconds(),
	)

	return domain.SourceData{
		Workloads:  workloads,
		ObservedAt: observedAt,
	}, nil
}
