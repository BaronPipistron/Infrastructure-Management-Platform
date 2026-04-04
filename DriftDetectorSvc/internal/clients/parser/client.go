package parser

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
	cfg        config.ParserClientConfig
	httpClient *http.Client
	log        *zap.SugaredLogger
}

func NewClient(cfg config.ParserClientConfig, log *zap.SugaredLogger) *Client {
	return &Client{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: cfg.Timeout,
		},
		log: log,
	}
}

func (c *Client) FetchDesiredState(ctx context.Context) (domain.DesiredState, error) {
	url := c.cfg.BaseURL + c.cfg.DesiredStatePath

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

		c.log.Warnw("parser request failed, retrying",
			"attempt", attempt,
			"max_attempts", c.cfg.Retry.MaxAttempts,
			"status_code", statusCode,
			"error", err,
		)

		if !sleepWithContext(ctx, c.cfg.Retry.Backoff) {
			return domain.DesiredState{}, ctx.Err()
		}
	}

	return domain.DesiredState{}, fmt.Errorf("fetch parser desired state: %w", lastErr)
}

func (c *Client) fetchOnce(ctx context.Context, requestURL string) (domain.DesiredState, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return domain.DesiredState{}, 0, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return domain.DesiredState{}, 0, err
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return domain.DesiredState{}, resp.StatusCode, fmt.Errorf("read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return domain.DesiredState{}, resp.StatusCode, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var payload desiredStateResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return domain.DesiredState{}, resp.StatusCode, fmt.Errorf("decode response: %w", err)
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

type desiredStateResponse struct {
	Metadata desiredMetadataDTO `json:"metadata"`
	State    desiredStateDTO    `json:"desiredState"`
}

type desiredMetadataDTO struct {
	Ready          bool       `json:"ready"`
	ReadyReason    string     `json:"readyReason"`
	FilesLoaded    int        `json:"filesLoaded"`
	FilesBroken    int        `json:"filesBroken"`
	HostsTotal     int        `json:"hostsTotal"`
	WorkloadsTotal int        `json:"workloadsTotal"`
	LoadedAt       *time.Time `json:"loadedAt"`
}

type desiredStateDTO struct {
	Hosts []desiredHostDTO `json:"hosts"`
}

type desiredHostDTO struct {
	HostID    string               `json:"host_id"`
	FQDN      string               `json:"fqdn"`
	Labels    map[string]string    `json:"labels"`
	Workloads []desiredWorkloadDTO `json:"workloads"`
}

type desiredWorkloadDTO struct {
	Name           string  `json:"name"`
	Enabled        bool    `json:"enabled"`
	DeploymentMode string  `json:"deployment_mode"`
	Image          string  `json:"image"`
	Version        *string `json:"version"`
	Port           *int    `json:"port"`
}

func (r desiredStateResponse) toDomain() domain.DesiredState {
	hosts := make([]domain.DesiredHost, 0, len(r.State.Hosts))
	for _, host := range r.State.Hosts {
		labels := make(map[string]string, len(host.Labels))
		for key, value := range host.Labels {
			labels[key] = value
		}

		workloads := make([]domain.DesiredWorkload, 0, len(host.Workloads))
		for _, workload := range host.Workloads {
			workloads = append(workloads, domain.DesiredWorkload{
				Name:           workload.Name,
				Enabled:        workload.Enabled,
				DeploymentMode: workload.DeploymentMode,
				Image:          workload.Image,
				Version:        workload.Version,
				Port:           workload.Port,
			})
		}

		hosts = append(hosts, domain.DesiredHost{
			HostID:    host.HostID,
			FQDN:      host.FQDN,
			Labels:    labels,
			Workloads: workloads,
		})
	}

	return domain.DesiredState{
		Hosts: hosts,
		Metadata: domain.DesiredMetadata{
			Ready:          r.Metadata.Ready,
			ReadyReason:    r.Metadata.ReadyReason,
			FilesLoaded:    r.Metadata.FilesLoaded,
			FilesBroken:    r.Metadata.FilesBroken,
			HostsTotal:     r.Metadata.HostsTotal,
			WorkloadsTotal: r.Metadata.WorkloadsTotal,
			LoadedAt:       r.Metadata.LoadedAt,
		},
	}
}
