package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"inventory-svc/internal/domain"

	"github.com/stretchr/testify/require"
)

type inventoryStub struct {
	hosts    []domain.Host
	metadata domain.InventoryMetadata
	ready    bool
}

func (s *inventoryStub) ListHosts(labelFilters map[string]string) ([]domain.Host, domain.InventoryMetadata) {
	filtered := make([]domain.Host, 0)
	for _, host := range s.hosts {
		match := true
		for key, value := range labelFilters {
			if host.Labels[key] != value {
				match = false
				break
			}
		}
		if match {
			filtered = append(filtered, host)
		}
	}
	return filtered, s.metadata
}

func (s *inventoryStub) GetHostByID(id string) (domain.Host, bool) {
	for _, host := range s.hosts {
		if host.ID == id {
			return host, true
		}
	}
	return domain.Host{}, false
}

func (s *inventoryStub) IsReady() bool {
	return s.ready
}

func (s *inventoryStub) GetMetadata() domain.InventoryMetadata {
	return s.metadata
}

func TestListHostsHandler(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	stub := &inventoryStub{
		hosts: []domain.Host{
			{ID: "host-1", FQDN: "a", Labels: map[string]string{"env": "prod"}, Status: domain.HostStatusOK},
			{ID: "host-2", FQDN: "b", Labels: map[string]string{"env": "dev"}, Status: domain.HostStatusOK},
		},
		metadata: domain.InventoryMetadata{IsPartial: false, TotalHosts: 2, LastSyncAt: &now},
		ready:    true,
	}

	router := NewRouter(NewHandler(stub))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/hosts?env=prod", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusOK, resp.Code)

	var payload HostsResponse
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &payload))
	require.Len(t, payload.Hosts, 1)
	require.Equal(t, "host-1", payload.Hosts[0].ID)
	require.Equal(t, 1, payload.Metadata.ReturnedHosts)
}

func TestGetHostNotFoundHandler(t *testing.T) {
	t.Parallel()

	stub := &inventoryStub{hosts: []domain.Host{}, ready: true}
	router := NewRouter(NewHandler(stub))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/hosts/missing", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusNotFound, resp.Code)
}
