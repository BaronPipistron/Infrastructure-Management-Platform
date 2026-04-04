package parser

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"drift-detector-svc/internal/config"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestFetchDesiredStateSuccess(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/v1/desired-state", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"metadata":{"ready":true,"filesLoaded":1,"filesBroken":0,"hostsTotal":1,"workloadsTotal":1},
			"desiredState":{
				"hosts":[
					{
						"host_id":"srv-001",
						"fqdn":"srv-001.example.local",
						"labels":{"env":"prod"},
						"workloads":[
							{"name":"node_exporter","enabled":true,"deployment_mode":"container","image":"quay.io/prometheus/node-exporter:v1.8.2"}
						]
					}
				]
			}
		}`))
	}))
	defer server.Close()

	log := zap.NewNop().Sugar()
	client := NewClient(config.ParserClientConfig{
		BaseURL:          server.URL,
		DesiredStatePath: "/api/v1/desired-state",
		Timeout:          time.Second,
		Retry: config.RetryConfig{
			MaxAttempts: 2,
			Backoff:     10 * time.Millisecond,
		},
	}, log)

	state, err := client.FetchDesiredState(context.Background())
	require.NoError(t, err)
	require.True(t, state.Metadata.Ready)
	require.Len(t, state.Hosts, 1)
	require.Equal(t, "srv-001", state.Hosts[0].HostID)
	require.Len(t, state.Hosts[0].Workloads, 1)
}

func TestFetchDesiredStateError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "bad gateway", http.StatusBadGateway)
	}))
	defer server.Close()

	log := zap.NewNop().Sugar()
	client := NewClient(config.ParserClientConfig{
		BaseURL:          server.URL,
		DesiredStatePath: "/api/v1/desired-state",
		Timeout:          time.Second,
		Retry: config.RetryConfig{
			MaxAttempts: 1,
			Backoff:     0,
		},
	}, log)

	_, err := client.FetchDesiredState(context.Background())
	require.Error(t, err)
}
