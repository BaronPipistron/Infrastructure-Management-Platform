package reconciler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"drift-detector-svc/internal/config"
	"drift-detector-svc/internal/domain"

	"go.uber.org/zap"
)

type Client struct {
	cfg        config.ReconcilerClientConfig
	httpClient *http.Client
	log        *zap.SugaredLogger
}

func NewClient(cfg config.ReconcilerClientConfig, log *zap.SugaredLogger) *Client {
	return &Client{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: cfg.Timeout,
		},
		log: log,
	}
}

func (c *Client) SendReconcile(ctx context.Context, command domain.ReconcileCommand) (domain.ReconcileAccepted, error) {
	url := c.cfg.BaseURL + c.cfg.ReconcilePath

	payload := reconcileRequest{
		HostID:        command.HostID,
		FQDN:          command.FQDN,
		Component:     string(command.Component),
		CorrelationID: command.CorrelationID,
		Parameters:    command.Parameters,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return domain.ReconcileAccepted{}, fmt.Errorf("marshal reconcile request: %w", err)
	}

	var lastErr error
	for attempt := 1; attempt <= c.cfg.Retry.MaxAttempts; attempt++ {
		accepted, statusCode, callErr := c.sendOnce(ctx, url, body)
		if callErr == nil {
			return accepted, nil
		}

		lastErr = callErr
		if !shouldRetry(statusCode) || attempt == c.cfg.Retry.MaxAttempts {
			break
		}

		c.log.Warnw("reconciler request failed, retrying",
			"attempt", attempt,
			"max_attempts", c.cfg.Retry.MaxAttempts,
			"status_code", statusCode,
			"error", callErr,
			"component", command.Component,
			"fqdn", command.FQDN,
		)

		if !sleepWithContext(ctx, c.cfg.Retry.Backoff) {
			return domain.ReconcileAccepted{}, ctx.Err()
		}
	}

	return domain.ReconcileAccepted{}, fmt.Errorf("send reconcile command: %w", lastErr)
}

func (c *Client) sendOnce(ctx context.Context, requestURL string, body []byte) (domain.ReconcileAccepted, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, requestURL, bytes.NewReader(body))
	if err != nil {
		return domain.ReconcileAccepted{}, 0, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return domain.ReconcileAccepted{}, 0, err
	}
	defer func() { _ = resp.Body.Close() }()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return domain.ReconcileAccepted{}, resp.StatusCode, fmt.Errorf("read response body: %w", err)
	}

	if resp.StatusCode != http.StatusAccepted {
		return domain.ReconcileAccepted{}, resp.StatusCode, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, strings.TrimSpace(string(responseBody)))
	}

	var accepted reconcileAcceptedResponse
	if err := json.Unmarshal(responseBody, &accepted); err != nil {
		return domain.ReconcileAccepted{}, resp.StatusCode, fmt.Errorf("decode response: %w", err)
	}

	return accepted.toDomain(), resp.StatusCode, nil
}

func shouldRetry(statusCode int) bool {
	if statusCode == 0 {
		return true
	}

	return statusCode >= http.StatusInternalServerError
}

func sleepWithContext(ctx context.Context, delay time.Duration) bool {
	if delay <= 0 {
		return true
	}

	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

type reconcileRequest struct {
	HostID        string                 `json:"host_id,omitempty"`
	FQDN          string                 `json:"fqdn"`
	Component     string                 `json:"component"`
	CorrelationID string                 `json:"correlation_id,omitempty"`
	Parameters    map[string]interface{} `json:"parameters,omitempty"`
}

type reconcileAcceptedResponse struct {
	RequestID     string     `json:"request_id"`
	AcceptedAt    *time.Time `json:"accepted_at"`
	Component     string     `json:"component"`
	FQDN          string     `json:"fqdn"`
	CorrelationID string     `json:"correlation_id"`
}

func (r reconcileAcceptedResponse) toDomain() domain.ReconcileAccepted {
	component, _ := domain.ParseComponent(r.Component)
	return domain.ReconcileAccepted{
		RequestID:     r.RequestID,
		AcceptedAt:    r.AcceptedAt,
		Component:     component,
		FQDN:          r.FQDN,
		CorrelationID: r.CorrelationID,
	}
}
