package inventory

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"drift-detector-svc/internal/config"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestFetchActualStateSuccess(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/v1/hosts", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"metadata":{"isPartial":false,"totalHosts":1,"failedHosts":0},
			"hosts":[
				{
					"id":"srv-001",
					"fqdn":"srv-001.example.local",
					"labels":{"env":"prod"},
					"status":"ok",
					"workloads":[{"name":"node_exporter"}],
					"sourceStatus":{"cadvisor":{"source":"cadvisor","enabled":true,"status":"ok"}}
				}
			]
		}`))
	}))
	defer server.Close()

	log := zap.NewNop().Sugar()
	client := NewClient(config.InventoryClientConfig{
		BaseURL:   server.URL,
		HostsPath: "/api/v1/hosts",
		Timeout:   time.Second,
		Retry: config.RetryConfig{
			MaxAttempts: 2,
			Backoff:     10 * time.Millisecond,
		},
	}, log)

	state, err := client.FetchActualState(context.Background())
	require.NoError(t, err)
	require.Len(t, state.Hosts, 1)
	require.Equal(t, "srv-001", state.Hosts[0].HostID)
	require.True(t, state.Hosts[0].HasWorkload("node_exporter"))
}

func TestFetchActualStateRetriesOnServerError(t *testing.T) {
	t.Parallel()

	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		current := atomic.AddInt32(&calls, 1)
		if current == 1 {
			http.Error(w, "temporary error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"metadata":{"isPartial":false,"totalHosts":0,"failedHosts":0},"hosts":[]}`))
	}))
	defer server.Close()

	log := zap.NewNop().Sugar()
	client := NewClient(config.InventoryClientConfig{
		BaseURL:   server.URL,
		HostsPath: "/api/v1/hosts",
		Timeout:   time.Second,
		Retry: config.RetryConfig{
			MaxAttempts: 2,
			Backoff:     10 * time.Millisecond,
		},
	}, log)

	_, err := client.FetchActualState(context.Background())
	require.NoError(t, err)
	require.Equal(t, int32(2), atomic.LoadInt32(&calls))
}
