package cooldown

import (
	"testing"
	"time"

	"drift-detector-svc/internal/domain"

	"github.com/stretchr/testify/require"
)

func TestCanSendAndCooldown(t *testing.T) {
	t.Parallel()

	store := NewStore()
	key := BuildKey(domain.ComponentNodeExporter, "srv-001.example.local")
	now := time.Date(2026, 4, 1, 10, 0, 0, 0, time.UTC)

	allowed, remaining := store.CanSend(key, now, time.Minute)
	require.True(t, allowed)
	require.Equal(t, time.Duration(0), remaining)

	store.MarkSent(key, now)

	allowed, remaining = store.CanSend(key, now.Add(20*time.Second), time.Minute)
	require.False(t, allowed)
	require.Equal(t, 40*time.Second, remaining)

	allowed, remaining = store.CanSend(key, now.Add(61*time.Second), time.Minute)
	require.True(t, allowed)
	require.Equal(t, time.Duration(0), remaining)
}

func TestCooldownDisabled(t *testing.T) {
	t.Parallel()

	store := NewStore()
	key := BuildKey(domain.ComponentCadvisor, "srv-002.example.local")
	now := time.Now().UTC()
	store.MarkSent(key, now)

	allowed, remaining := store.CanSend(key, now, 0)
	require.True(t, allowed)
	require.Equal(t, time.Duration(0), remaining)
}
