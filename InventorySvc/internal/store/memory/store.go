package memory

import (
	"sync"

	"inventory-svc/internal/domain"
)

type Store struct {
	mu        sync.RWMutex
	snapshot  domain.InventorySnapshot
	hostsByID map[string]domain.Host
	ready     bool
}

func NewStore() *Store {
	return &Store{
		hostsByID: make(map[string]domain.Host),
	}
}

func (s *Store) Replace(snapshot domain.InventorySnapshot) {
	s.mu.Lock()
	defer s.mu.Unlock()

	clonedSnapshot := snapshot.Clone()
	s.snapshot = clonedSnapshot

	hostsByID := make(map[string]domain.Host, len(clonedSnapshot.Hosts))
	for _, host := range clonedSnapshot.Hosts {
		hostsByID[host.ID] = host.Clone()
	}
	s.hostsByID = hostsByID
}

func (s *Store) GetSnapshot() domain.InventorySnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.snapshot.Clone()
}

func (s *Store) GetHost(id string) (domain.Host, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	host, found := s.hostsByID[id]
	if !found {
		return domain.Host{}, false
	}

	return host.Clone(), true
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
