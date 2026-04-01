package memory

import (
	"sync"

	"parser-svc/internal/domain"
)

type Store struct {
	mu        sync.RWMutex
	snapshot  domain.Snapshot
	hostsByID map[string]domain.DesiredHost
	ready     bool
}

func NewStore() *Store {
	return &Store{
		hostsByID: make(map[string]domain.DesiredHost),
	}
}

func (s *Store) Replace(snapshot domain.Snapshot) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cloned := snapshot.Clone()
	s.snapshot = cloned
	updatedHostsByID := make(map[string]domain.DesiredHost, len(cloned.State.Hosts))
	for _, host := range cloned.State.Hosts {
		updatedHostsByID[host.HostID] = host.Clone()
	}
	s.hostsByID = updatedHostsByID
}

func (s *Store) SetReady(ready bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ready = ready
}

func (s *Store) IsReady() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.ready
}

func (s *Store) GetSnapshot() domain.Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.snapshot.Clone()
}

func (s *Store) GetHost(hostID string) (domain.DesiredHost, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	host, found := s.hostsByID[hostID]
	if !found {
		return domain.DesiredHost{}, false
	}

	return host.Clone(), true
}
