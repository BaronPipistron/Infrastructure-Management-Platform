package desiredstate

import (
	"strings"

	"parser-svc/internal/domain"
	"parser-svc/internal/store/memory"
)

type Query struct {
	FQDN         string
	LabelFilters map[string]string
}

type Service struct {
	store *memory.Store
}

func NewService(store *memory.Store) *Service {
	return &Service{store: store}
}

func (s *Service) GetSnapshot() domain.Snapshot {
	return s.store.GetSnapshot()
}

func (s *Service) ListHosts(query Query) []domain.DesiredHost {
	snapshot := s.store.GetSnapshot()
	return FilterHosts(snapshot.State.Hosts, query)
}

func (s *Service) GetHostByID(hostID string) (domain.DesiredHost, bool) {
	return s.store.GetHost(hostID)
}

func (s *Service) IsReady() bool {
	return s.store.IsReady()
}

func FilterHosts(hosts []domain.DesiredHost, query Query) []domain.DesiredHost {
	result := make([]domain.DesiredHost, 0)
	for _, host := range hosts {
		if !matchFQDN(host, query.FQDN) {
			continue
		}
		if !matchLabels(host, query.LabelFilters) {
			continue
		}
		result = append(result, host.Clone())
	}

	return result
}

func matchFQDN(host domain.DesiredHost, fqdn string) bool {
	normalized := strings.TrimSpace(fqdn)
	if normalized == "" {
		return true
	}

	return strings.EqualFold(host.FQDN, normalized)
}

func matchLabels(host domain.DesiredHost, filters map[string]string) bool {
	if len(filters) == 0 {
		return true
	}

	for key, expected := range filters {
		actual, found := host.Labels[key]
		if !found {
			return false
		}
		if actual != expected {
			return false
		}
	}

	return true
}
