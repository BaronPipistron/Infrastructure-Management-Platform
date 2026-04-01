package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"parser-svc/internal/domain"
	"parser-svc/internal/service/desiredstate"

	"github.com/stretchr/testify/require"
)

type serviceStub struct {
	snapshot domain.Snapshot
	ready    bool
}

func (s *serviceStub) GetSnapshot() domain.Snapshot {
	return s.snapshot.Clone()
}

func (s *serviceStub) ListHosts(query desiredstate.Query) []domain.DesiredHost {
	return desiredstate.FilterHosts(s.snapshot.State.Hosts, query)
}

func (s *serviceStub) GetHostByID(hostID string) (domain.DesiredHost, bool) {
	for _, host := range s.snapshot.State.Hosts {
		if host.HostID == hostID {
			return host.Clone(), true
		}
	}
	return domain.DesiredHost{}, false
}

func (s *serviceStub) IsReady() bool {
	return s.ready
}

func TestListHostsHandler(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	stub := &serviceStub{
		snapshot: domain.Snapshot{
			State: domain.DesiredState{
				Hosts: []domain.DesiredHost{
					{
						HostID: "srv-1",
						FQDN:   "srv-1.example.local",
						Labels: map[string]string{"env": "prod", "managed_by": "platform-team"},
					},
					{
						HostID: "srv-2",
						FQDN:   "srv-2.example.local",
						Labels: map[string]string{"env": "dev", "managed_by": "platform-team"},
					},
				},
			},
			Metadata: domain.SnapshotMetadata{LoadedAt: now, HostsTotal: 2},
		},
		ready: true,
	}

	router := NewRouter(NewHandler(stub))
	request := httptest.NewRequest(http.MethodGet, "/api/v1/desired-state/hosts?fqdn=srv-1.example.local&env=prod", nil)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)
	require.Equal(t, http.StatusOK, response.Code)

	var payload HostsResponse
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &payload))
	require.Len(t, payload.Hosts, 1)
	require.Equal(t, "srv-1", payload.Hosts[0].HostID)
}

func TestGetHostNotFoundHandler(t *testing.T) {
	t.Parallel()

	stub := &serviceStub{snapshot: domain.Snapshot{}, ready: true}
	router := NewRouter(NewHandler(stub))
	request := httptest.NewRequest(http.MethodGet, "/api/v1/desired-state/hosts/missing", nil)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)
	require.Equal(t, http.StatusNotFound, response.Code)
}

func TestReadyzNotReadyHandler(t *testing.T) {
	t.Parallel()

	stub := &serviceStub{
		snapshot: domain.Snapshot{Metadata: domain.SnapshotMetadata{ReadyReason: "no valid manifest files were loaded"}},
		ready:    false,
	}
	router := NewRouter(NewHandler(stub))
	request := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)
	require.Equal(t, http.StatusServiceUnavailable, response.Code)
}
