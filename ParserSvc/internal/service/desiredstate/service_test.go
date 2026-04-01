package desiredstate

import (
	"testing"
	"time"

	"parser-svc/internal/domain"
	"parser-svc/internal/store/memory"

	"github.com/stretchr/testify/require"
)

func TestFilterByLabels(t *testing.T) {
	t.Parallel()

	hosts := []domain.DesiredHost{
		{HostID: "srv-1", FQDN: "srv-1.example.local", Labels: map[string]string{"env": "prod", "managed_by": "platform-team"}},
		{HostID: "srv-2", FQDN: "srv-2.example.local", Labels: map[string]string{"env": "dev", "managed_by": "platform-team"}},
	}

	filtered := FilterHosts(hosts, Query{LabelFilters: map[string]string{"env": "prod", "managed_by": "platform-team"}})
	require.Len(t, filtered, 1)
	require.Equal(t, "srv-1", filtered[0].HostID)
}

func TestFilterByFQDN(t *testing.T) {
	t.Parallel()

	hosts := []domain.DesiredHost{
		{HostID: "srv-1", FQDN: "srv-1.example.local", Labels: map[string]string{}},
		{HostID: "srv-2", FQDN: "srv-2.example.local", Labels: map[string]string{}},
	}

	filtered := FilterHosts(hosts, Query{FQDN: "srv-2.example.local"})
	require.Len(t, filtered, 1)
	require.Equal(t, "srv-2", filtered[0].HostID)
}

func TestServiceListHosts(t *testing.T) {
	t.Parallel()

	store := memory.NewStore()
	store.Replace(domain.Snapshot{
		State: domain.DesiredState{
			Hosts: []domain.DesiredHost{
				{HostID: "srv-1", FQDN: "srv-1.example.local", Labels: map[string]string{"env": "prod"}},
				{HostID: "srv-2", FQDN: "srv-2.example.local", Labels: map[string]string{"env": "dev"}},
			},
		},
		Metadata: domain.SnapshotMetadata{LoadedAt: time.Now().UTC()},
	})

	svc := NewService(store)
	filtered := svc.ListHosts(Query{LabelFilters: map[string]string{"env": "prod"}})
	require.Len(t, filtered, 1)
	require.Equal(t, "srv-1", filtered[0].HostID)
}
