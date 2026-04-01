package memory

import (
	"testing"
	"time"

	"parser-svc/internal/domain"

	"github.com/stretchr/testify/require"
)

func TestStoreReplaceAndGet(t *testing.T) {
	t.Parallel()

	store := NewStore()
	now := time.Now().UTC()
	snapshot := domain.Snapshot{
		State: domain.DesiredState{
			Hosts: []domain.DesiredHost{
				{
					HostID: "srv-001",
					FQDN:   "srv-001.example.local",
					Labels: map[string]string{"env": "prod"},
				},
			},
		},
		Metadata: domain.SnapshotMetadata{LoadedAt: now},
	}

	store.Replace(snapshot)
	store.SetReady(true)

	storedSnapshot := store.GetSnapshot()
	require.Equal(t, now, storedSnapshot.Metadata.LoadedAt)
	require.Len(t, storedSnapshot.State.Hosts, 1)
	require.True(t, store.IsReady())

	host, found := store.GetHost("srv-001")
	require.True(t, found)
	require.Equal(t, "srv-001.example.local", host.FQDN)
}

func TestStoreGetHostMissing(t *testing.T) {
	t.Parallel()

	store := NewStore()
	_, found := store.GetHost("missing")
	require.False(t, found)
}
