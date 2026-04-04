package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"drift-detector-svc/internal/domain"
	"drift-detector-svc/internal/service/detection"

	"github.com/stretchr/testify/require"
)

type detectionServiceStub struct {
	ready      bool
	lastResult domain.DetectionCycleResult
	hasResult  bool
	runResult  domain.DetectionCycleResult
	runErr     error
}

func (s *detectionServiceStub) RunCycle(ctx context.Context, trigger string) (domain.DetectionCycleResult, error) {
	return s.runResult, s.runErr
}

func (s *detectionServiceStub) IsReady() bool {
	return s.ready
}

func (s *detectionServiceStub) LastCycleResult() (domain.DetectionCycleResult, bool) {
	return s.lastResult, s.hasResult
}

func TestHealthzAndReadyz(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	stub := &detectionServiceStub{
		ready:     true,
		hasResult: true,
		lastResult: domain.DetectionCycleResult{
			CycleID:    "cycle-1",
			FinishedAt: now,
			Partial:    false,
		},
	}

	router := NewRouter(NewHandler(stub))

	healthReq := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	healthResp := httptest.NewRecorder()
	router.ServeHTTP(healthResp, healthReq)
	require.Equal(t, http.StatusOK, healthResp.Code)

	readyReq := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	readyResp := httptest.NewRecorder()
	router.ServeHTTP(readyResp, readyReq)
	require.Equal(t, http.StatusOK, readyResp.Code)
}

func TestReadyzNotReady(t *testing.T) {
	t.Parallel()

	stub := &detectionServiceStub{ready: false}
	router := NewRouter(NewHandler(stub))

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusServiceUnavailable, resp.Code)
}

func TestManualRunSuccess(t *testing.T) {
	t.Parallel()

	stub := &detectionServiceStub{
		ready: true,
		runResult: domain.DetectionCycleResult{
			CycleID:    "cycle-2",
			Trigger:    "manual_api",
			StartedAt:  time.Now().UTC(),
			FinishedAt: time.Now().UTC(),
			Stats:      domain.DetectionStats{DriftsFound: 1, ReconcileSent: 1},
		},
	}

	router := NewRouter(NewHandler(stub))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/detection/run", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	require.Equal(t, http.StatusOK, resp.Code)
	var payload DetectionRunResponse
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &payload))
	require.Equal(t, "completed", payload.Status)
	require.Equal(t, "cycle-2", payload.Result.CycleID)
}

func TestManualRunConflict(t *testing.T) {
	t.Parallel()

	stub := &detectionServiceStub{
		runErr: detection.ErrCycleAlreadyRunning,
	}
	router := NewRouter(NewHandler(stub))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/detection/run", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusConflict, resp.Code)
}
