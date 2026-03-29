package cadvisor

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"inventory-svc/internal/config"
)

type Client struct {
	cfg        config.CAdvisorConfig
	httpClient *http.Client
}

func NewClient(cfg config.CAdvisorConfig) *Client {
	return &Client{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: cfg.Timeout,
		},
	}
}

func (c *Client) FetchContainers(ctx context.Context, fqdn string) (containersResponse, error) {
	url := c.buildURL(fqdn)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("unexpected status %d from %s: %s", resp.StatusCode, url, strings.TrimSpace(string(body)))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	payload, err := decodeContainersResponse(body)
	if err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return payload, nil
}

func (c *Client) buildURL(fqdn string) string {
	replacements := strings.NewReplacer(
		"{{scheme}}", strings.TrimSpace(c.cfg.Scheme),
		"{{fqdn}}", strings.TrimSpace(fqdn),
		"{{port}}", strconv.Itoa(c.cfg.Port),
		"{{basePath}}", c.cfg.BasePath,
		"{{containersPath}}", c.cfg.ContainersPath,
	)

	return replacements.Replace(c.cfg.URLTemplate)
}

func deadlineContext(parent context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout <= 0 {
		return context.WithCancel(parent)
	}
	return context.WithTimeout(parent, timeout)
}

func decodeContainersResponse(body []byte) (containersResponse, error) {
	var listResponse []containerInfo
	if err := json.Unmarshal(body, &listResponse); err == nil {
		return listResponse, nil
	}

	var mapResponse map[string]containerInfo
	if err := json.Unmarshal(body, &mapResponse); err == nil {
		keys := make([]string, 0, len(mapResponse))
		for key := range mapResponse {
			keys = append(keys, key)
		}
		sort.Strings(keys)

		result := make([]containerInfo, 0, len(keys))
		for _, key := range keys {
			item := mapResponse[key]
			if strings.TrimSpace(item.Name) == "" {
				item.Name = key
			}
			result = append(result, item)
		}
		return result, nil
	}

	var singleResponse containerInfo
	if err := json.Unmarshal(body, &singleResponse); err == nil && hasContainerPayload(singleResponse) {
		if strings.TrimSpace(singleResponse.Name) == "" {
			singleResponse.Name = "/"
		}
		return []containerInfo{singleResponse}, nil
	}

	var apiError struct {
		Error   string `json:"error"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &apiError); err == nil {
		message := strings.TrimSpace(apiError.Error)
		if message == "" {
			message = strings.TrimSpace(apiError.Message)
		}
		if message != "" {
			return nil, fmt.Errorf("cadvisor api error: %s", message)
		}
	}

	return nil, fmt.Errorf("unsupported cadvisor response format: %s", strings.TrimSpace(string(body)))
}

func hasContainerPayload(value containerInfo) bool {
	if strings.TrimSpace(value.Name) != "" {
		return true
	}
	if len(value.Aliases) > 0 {
		return true
	}
	if strings.TrimSpace(value.Namespace) != "" {
		return true
	}
	if strings.TrimSpace(value.Spec.Image) != "" {
		return true
	}
	if len(value.Stats) > 0 {
		return true
	}
	return false
}
