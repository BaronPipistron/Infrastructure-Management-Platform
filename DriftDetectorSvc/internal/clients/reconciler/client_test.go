package reconciler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"drift-detector-svc/internal/config"
	"drift-detector-svc/internal/domain"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestSendReconcileSuccess(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "/api/v1/reconcile", r.URL.Path)

		var payload map[string]interface{}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
		require.Equal(t, "srv-001.example.local", payload["fqdn"])
		require.Equal(t, "node_exporter", payload["component"])

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{
			"status":"accepted",
			"request_id":"req-001",
			"component":"node_exporter",
			"fqdn":"srv-001.example.local",
			"correlation_id":"cycle-1:node_exporter:srv-001.example.local",
			"accepted_at":"2026-04-01T10:00:00Z"
		}`))
	}))
	defer server.Close()

	log := zap.NewNop().Sugar()
	client := NewClient(config.ReconcilerClientConfig{
		BaseURL:       server.URL,
		ReconcilePath: "/api/v1/reconcile",
		Timeout:       time.Second,
		Retry: config.RetryConfig{
			MaxAttempts: 2,
			Backoff:     10 * time.Millisecond,
		},
	}, log)

	accepted, err := client.SendReconcile(context.Background(), domain.ReconcileCommand{
		HostID:        "srv-001",
		FQDN:          "srv-001.example.local",
		Component:     domain.ComponentNodeExporter,
		CorrelationID: "cycle-1:node_exporter:srv-001.example.local",
		Parameters:    map[string]interface{}{"node_exporter_port": 9100},
	})
	require.NoError(t, err)
	require.Equal(t, "req-001", accepted.RequestID)
	require.Equal(t, domain.ComponentNodeExporter, accepted.Component)
}

func TestSendReconcileUnexpectedStatus(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "queue full", http.StatusServiceUnavailable)
	}))
	defer server.Close()

	log := zap.NewNop().Sugar()
	client := NewClient(config.ReconcilerClientConfig{
		BaseURL:       server.URL,
		ReconcilePath: "/api/v1/reconcile",
		Timeout:       time.Second,
		Retry: config.RetryConfig{
			MaxAttempts: 1,
			Backoff:     0,
		},
	}, log)

	_, err := client.SendReconcile(context.Background(), domain.ReconcileCommand{
		FQDN:      "srv-001.example.local",
		Component: domain.ComponentNodeExporter,
	})
	require.Error(t, err)
}
