package inventory

import (
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
	cfg        config.InventoryClientConfig
	httpClient *http.Client
	log        *zap.SugaredLogger
}

func NewClient(cfg config.InventoryClientConfig, log *zap.SugaredLogger) *Client {
	return &Client{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: cfg.Timeout,
		},
		log: log,
	}
}

func (c *Client) FetchActualState(ctx context.Context) (domain.ActualState, error) {
	url := c.cfg.BaseURL + c.cfg.HostsPath

	var lastErr error
	for attempt := 1; attempt <= c.cfg.Retry.MaxAttempts; attempt++ {
		state, statusCode, err := c.fetchOnce(ctx, url)
		if err == nil {
			return state, nil
		}

		lastErr = err
		if !shouldRetry(statusCode) || attempt == c.cfg.Retry.MaxAttempts {
			break
		}

		c.log.Warnw("inventory request failed, retrying",
			"attempt", attempt,
			"max_attempts", c.cfg.Retry.MaxAttempts,
			"status_code", statusCode,
			"error", err,
		)

		if !sleepWithContext(ctx, c.cfg.Retry.Backoff) {
			return domain.ActualState{}, ctx.Err()
		}
	}

	return domain.ActualState{}, fmt.Errorf("fetch inventory actual state: %w", lastErr)
}

func (c *Client) fetchOnce(ctx context.Context, requestURL string) (domain.ActualState, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return domain.ActualState{}, 0, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return domain.ActualState{}, 0, err
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return domain.ActualState{}, resp.StatusCode, fmt.Errorf("read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return domain.ActualState{}, resp.StatusCode, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var payload hostsResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return domain.ActualState{}, resp.StatusCode, fmt.Errorf("decode response: %w", err)
	}

	return payload.toDomain(), resp.StatusCode, nil
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

type hostsResponse struct {
	Metadata hostsMetadataDTO `json:"metadata"`
	Hosts    []hostDTO        `json:"hosts"`
}

type hostsMetadataDTO struct {
	IsPartial   bool       `json:"isPartial"`
	TotalHosts  int        `json:"totalHosts"`
	FailedHosts int        `json:"failedHosts"`
	LastSyncAt  *time.Time `json:"lastSyncAt"`
}

type hostDTO struct {
	ID           string                     `json:"id"`
	FQDN         string                     `json:"fqdn"`
	Labels       map[string]string          `json:"labels"`
	Status       string                     `json:"status"`
	Workloads    []workloadDTO              `json:"workloads"`
	SourceStatus map[string]sourceStatusDTO `json:"sourceStatus"`
}

type workloadDTO struct {
	Name string `json:"name"`
}

type sourceStatusDTO struct {
	Source  string `json:"source"`
	Enabled bool   `json:"enabled"`
	Status  string `json:"status"`
	Error   string `json:"error"`
}

func (r hostsResponse) toDomain() domain.ActualState {
	hosts := make([]domain.ActualHost, 0, len(r.Hosts))
	for _, host := range r.Hosts {
		labels := make(map[string]string, len(host.Labels))
		for k, v := range host.Labels {
			labels[k] = v
		}

		workloads := make([]domain.ActualWorkload, 0, len(host.Workloads))
		for _, workload := range host.Workloads {
			workloads = append(workloads, domain.ActualWorkload{Name: workload.Name})
		}

		sourceStatuses := make(map[string]domain.ActualSourceStatus, len(host.SourceStatus))
		for key, sourceStatus := range host.SourceStatus {
			sourceStatuses[key] = domain.ActualSourceStatus{
				Source:  sourceStatus.Source,
				Enabled: sourceStatus.Enabled,
				Status:  sourceStatus.Status,
				Error:   sourceStatus.Error,
			}
		}

		hosts = append(hosts, domain.ActualHost{
			HostID:       host.ID,
			FQDN:         host.FQDN,
			Labels:       labels,
			Status:       host.Status,
			Workloads:    workloads,
			SourceStatus: sourceStatuses,
		})
	}

	return domain.ActualState{
		Hosts: hosts,
		Metadata: domain.ActualMetadata{
			IsPartial:   r.Metadata.IsPartial,
			TotalHosts:  r.Metadata.TotalHosts,
			FailedHosts: r.Metadata.FailedHosts,
			LastSyncAt:  r.Metadata.LastSyncAt,
		},
	}
}
