package cooldown

import (
	"strings"
	"sync"
	"time"

	"drift-detector-svc/internal/domain"
)

type Store struct {
	mu       sync.RWMutex
	lastSent map[string]time.Time
}

func NewStore() *Store {
	return &Store{
		lastSent: make(map[string]time.Time),
	}
}

func BuildKey(component domain.Component, fqdn string) string {
	return strings.ToLower(string(component)) + "|" + strings.ToLower(strings.TrimSpace(fqdn))
}

func (s *Store) CanSend(key string, now time.Time, cooldown time.Duration) (bool, time.Duration) {
	if cooldown <= 0 {
		return true, 0
	}

	s.mu.RLock()
	last, found := s.lastSent[key]
	s.mu.RUnlock()

	if !found {
		return true, 0
	}

	elapsed := now.Sub(last)
	if elapsed >= cooldown {
		return true, 0
	}

	return false, cooldown - elapsed
}

func (s *Store) MarkSent(key string, at time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.lastSent[key] = at.UTC()
}
