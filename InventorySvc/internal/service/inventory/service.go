package inventory

import (
	"context"
	"fmt"
	"strings"
	"time"

	"inventory-svc/internal/domain"
	"inventory-svc/internal/sources"
	"inventory-svc/internal/store/memory"

	"go.uber.org/zap"
)

type Service struct {
	store          *memory.Store
	sources        []sources.HostSource
	bootstrapHosts []domain.HostSeed
	log            *zap.SugaredLogger
}

func NewService(store *memory.Store, hostSources []sources.HostSource, bootstrapHosts []domain.HostSeed, log *zap.SugaredLogger) *Service {
	normalizedHosts := make([]domain.HostSeed, 0, len(bootstrapHosts))
	for _, host := range bootstrapHosts {
		normalizedHosts = append(normalizedHosts, domain.HostSeed{
			ID:     strings.TrimSpace(host.ID),
			FQDN:   strings.TrimSpace(host.FQDN),
			Labels: cloneLabels(host.Labels),
		})
	}

	return &Service{
		store:          store,
		sources:        hostSources,
		bootstrapHosts: normalizedHosts,
		log:            log,
	}
}

func (s *Service) Sync(ctx context.Context) error {
	startedAt := time.Now().UTC()
	previousSnapshot := s.store.GetSnapshot()

	hosts := make([]domain.Host, 0, len(s.bootstrapHosts))
	failedHosts := 0
	successHosts := 0

	for _, bootstrapHost := range s.bootstrapHosts {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		host := domain.Host{
			ID:           bootstrapHost.ID,
			FQDN:         bootstrapHost.FQDN,
			Labels:       cloneLabels(bootstrapHost.Labels),
			Workloads:    []domain.Workload{},
			Status:       domain.HostStatusBootstrapOnly,
			SourceStatus: make(map[string]domain.SourceStatus, len(s.sources)),
			Errors:       []string{},
		}

		enabledSources := 0
		successfulSources := 0
		failedSources := 0

		for _, source := range s.sources {
			sourceName := source.Name()
			if !source.Enabled() {
				host.SourceStatus[sourceName] = domain.SourceStatus{
					Source:  sourceName,
					Enabled: false,
					Status:  domain.SourceStatusDisabled,
				}
				continue
			}

			enabledSources++

			data, err := source.CollectHost(ctx, bootstrapHost)
			if err != nil {
				failedSources++
				host.SourceStatus[sourceName] = domain.SourceStatus{
					Source:  sourceName,
					Enabled: true,
					Status:  domain.SourceStatusError,
					Error:   err.Error(),
				}
				host.Errors = append(host.Errors, fmt.Sprintf("%s: %v", sourceName, err))

				s.log.Warnw("source collection failed",
					"host_id", bootstrapHost.ID,
					"fqdn", bootstrapHost.FQDN,
					"source", sourceName,
					"error", err,
				)
				continue
			}

			successfulSources++
			observedAt := data.ObservedAt.UTC()
			host.SourceStatus[sourceName] = domain.SourceStatus{
				Source:     sourceName,
				Enabled:    true,
				Status:     domain.SourceStatusOK,
				ObservedAt: &observedAt,
			}

			host.Workloads = append(host.Workloads, cloneWorkloads(data.Workloads)...)
			if host.LastObservedAt == nil || observedAt.After(*host.LastObservedAt) {
				timestamp := observedAt
				host.LastObservedAt = &timestamp
			}
		}

		switch {
		case enabledSources == 0:
			host.Status = domain.HostStatusBootstrapOnly
		case failedSources > 0 && successfulSources > 0:
			host.Status = domain.HostStatusPartial
			failedHosts++
		case failedSources > 0 && successfulSources == 0:
			host.Status = domain.HostStatusError
			failedHosts++
		default:
			host.Status = domain.HostStatusOK
			successHosts++
		}

		hosts = append(hosts, host)
	}

	finishedAt := time.Now().UTC()
	metadata := domain.InventoryMetadata{
		IsPartial:      failedHosts > 0,
		TotalHosts:     len(hosts),
		FailedHosts:    failedHosts,
		LastSyncAt:     timePtr(finishedAt),
		SyncDurationMs: time.Since(startedAt).Milliseconds(),
	}

	if len(hosts) > 0 {
		metadata.LastSuccessfulSyncAt = timePtr(finishedAt)
	} else {
		metadata.LastSuccessfulSyncAt = previousSnapshot.Metadata.LastSuccessfulSyncAt
	}

	if metadata.IsPartial {
		metadata.LastPartialSyncAt = timePtr(finishedAt)
		metadata.LastFullSyncAt = previousSnapshot.Metadata.LastFullSyncAt
	} else {
		metadata.LastFullSyncAt = timePtr(finishedAt)
		metadata.LastPartialSyncAt = previousSnapshot.Metadata.LastPartialSyncAt
	}

	snapshot := domain.InventorySnapshot{
		Hosts:    hosts,
		Metadata: metadata,
	}
	s.store.Replace(snapshot)

	s.log.Infow("inventory sync completed",
		"hosts_total", len(hosts),
		"hosts_success", successHosts,
		"hosts_failed", failedHosts,
		"is_partial", metadata.IsPartial,
		"duration_ms", metadata.SyncDurationMs,
	)

	return nil
}

func (s *Service) ListHosts(labelFilters map[string]string) ([]domain.Host, domain.InventoryMetadata) {
	snapshot := s.store.GetSnapshot()
	return FilterHostsByLabels(snapshot.Hosts, labelFilters), snapshot.Metadata
}

func (s *Service) GetHostByID(id string) (domain.Host, bool) {
	return s.store.GetHost(id)
}

func (s *Service) SetReady(ready bool) {
	s.store.SetReady(ready)
}

func (s *Service) IsReady() bool {
	return s.store.IsReady()
}

func (s *Service) GetMetadata() domain.InventoryMetadata {
	return s.store.GetSnapshot().Metadata
}

func FilterHostsByLabels(hosts []domain.Host, filters map[string]string) []domain.Host {
	if len(filters) == 0 {
		result := make([]domain.Host, 0, len(hosts))
		for _, host := range hosts {
			result = append(result, host.Clone())
		}
		return result
	}

	result := make([]domain.Host, 0)
	for _, host := range hosts {
		if hostMatchesAllFilters(host, filters) {
			result = append(result, host.Clone())
		}
	}

	return result
}

func hostMatchesAllFilters(host domain.Host, filters map[string]string) bool {
	for key, expected := range filters {
		actual, ok := host.Labels[key]
		if !ok {
			return false
		}
		if actual != expected {
			return false
		}
	}

	return true
}

func cloneLabels(labels map[string]string) map[string]string {
	cloned := make(map[string]string, len(labels))
	for k, v := range labels {
		cloned[k] = v
	}
	return cloned
}

func cloneWorkloads(workloads []domain.Workload) []domain.Workload {
	cloned := make([]domain.Workload, 0, len(workloads))
	for _, workload := range workloads {
		item := workload
		if workload.LastSeenAt != nil {
			timestamp := workload.LastSeenAt.UTC()
			item.LastSeenAt = &timestamp
		}
		cloned = append(cloned, item)
	}
	return cloned
}

func timePtr(value time.Time) *time.Time {
	v := value.UTC()
	return &v
}
