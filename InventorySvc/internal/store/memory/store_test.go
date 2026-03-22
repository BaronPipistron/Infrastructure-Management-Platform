package memory

import (
	"testing"
	"time"

	"inventory-svc/internal/domain"

	"github.com/stretchr/testify/require"
)

func TestStoreReplaceAndGetSnapshotAreSafeCopies(t *testing.T) {
	t.Parallel()

	timestamp := time.Date(2026, 3, 22, 9, 0, 0, 0, time.UTC)
	store := NewStore()

	store.Replace(domain.InventorySnapshot{
		Hosts: []domain.Host{
			{
				ID:   "host-1",
				FQDN: "db01.prod.example.internal",
				Labels: map[string]string{
					"env": "prod",
				},
				Status: domain.HostStatusOK,
			},
		},
		Metadata: domain.InventoryMetadata{LastSyncAt: &timestamp},
	})

	snapshot := store.GetSnapshot()
	require.Len(t, snapshot.Hosts, 1)
	snapshot.Hosts[0].Labels["env"] = "changed"

	snapshot2 := store.GetSnapshot()
	require.Equal(t, "prod", snapshot2.Hosts[0].Labels["env"])

	host, found := store.GetHost("host-1")
	require.True(t, found)
	require.Equal(t, "db01.prod.example.internal", host.FQDN)
}

func TestStoreReadyFlag(t *testing.T) {
	t.Parallel()

	store := NewStore()
	require.False(t, store.IsReady())
	store.SetReady(true)
	require.True(t, store.IsReady())
}
