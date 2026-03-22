package httpapi

import (
	"time"

	"inventory-svc/internal/domain"
)

type HealthResponse struct {
	Status string `json:"status" example:"ok"`
}

type ReadinessResponse struct {
	Status     string     `json:"status" example:"ready"`
	IsPartial  bool       `json:"isPartial"`
	LastSyncAt *time.Time `json:"lastSyncAt,omitempty"`
}

type APIError struct {
	Message string `json:"message" example:"host not found"`
}

type HostsResponse struct {
	Metadata HostsResponseMetadata `json:"metadata"`
	Hosts    []HostDTO             `json:"hosts"`
}

type HostsResponseMetadata struct {
	IsPartial            bool       `json:"isPartial"`
	TotalHosts           int        `json:"totalHosts"`
	ReturnedHosts        int        `json:"returnedHosts"`
	FailedHosts          int        `json:"failedHosts"`
	LastSyncAt           *time.Time `json:"lastSyncAt,omitempty"`
	LastSuccessfulSyncAt *time.Time `json:"lastSuccessfulSyncAt,omitempty"`
	LastFullSyncAt       *time.Time `json:"lastFullSyncAt,omitempty"`
	LastPartialSyncAt    *time.Time `json:"lastPartialSyncAt,omitempty"`
	SyncDurationMs       int64      `json:"syncDurationMs"`
}

type HostResponse struct {
	Metadata HostResponseMetadata `json:"metadata"`
	Host     HostDTO              `json:"host"`
}

type HostResponseMetadata struct {
	IsPartial  bool       `json:"isPartial"`
	LastSyncAt *time.Time `json:"lastSyncAt,omitempty"`
}

type HostDTO struct {
	ID             string                     `json:"id"`
	FQDN           string                     `json:"fqdn"`
	Labels         map[string]string          `json:"labels"`
	Workloads      []WorkloadDTO              `json:"workloads"`
	LastObservedAt *time.Time                 `json:"lastObservedAt,omitempty"`
	Status         domain.HostStatus          `json:"status"`
	SourceStatus   map[string]SourceStatusDTO `json:"sourceStatus"`
	Errors         []string                   `json:"errors,omitempty"`
}

type WorkloadDTO struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Image      string     `json:"image,omitempty"`
	Runtime    string     `json:"runtime,omitempty"`
	Source     string     `json:"source"`
	LastSeenAt *time.Time `json:"lastSeenAt,omitempty"`
}

type SourceStatusDTO struct {
	Source     string                  `json:"source"`
	Enabled    bool                    `json:"enabled"`
	Status     domain.SourceStatusType `json:"status"`
	ObservedAt *time.Time              `json:"observedAt,omitempty"`
	Error      string                  `json:"error,omitempty"`
}

func toHostDTO(host domain.Host) HostDTO {
	sourceStatuses := make(map[string]SourceStatusDTO, len(host.SourceStatus))
	for sourceName, sourceStatus := range host.SourceStatus {
		status := SourceStatusDTO{
			Source:  sourceStatus.Source,
			Enabled: sourceStatus.Enabled,
			Status:  sourceStatus.Status,
			Error:   sourceStatus.Error,
		}
		if sourceStatus.ObservedAt != nil {
			timestamp := sourceStatus.ObservedAt.UTC()
			status.ObservedAt = &timestamp
		}
		sourceStatuses[sourceName] = status
	}

	workloads := make([]WorkloadDTO, 0, len(host.Workloads))
	for _, workload := range host.Workloads {
		w := WorkloadDTO{
			ID:      workload.ID,
			Name:    workload.Name,
			Image:   workload.Image,
			Runtime: workload.Runtime,
			Source:  workload.Source,
		}
		if workload.LastSeenAt != nil {
			timestamp := workload.LastSeenAt.UTC()
			w.LastSeenAt = &timestamp
		}
		workloads = append(workloads, w)
	}

	labels := make(map[string]string, len(host.Labels))
	for k, v := range host.Labels {
		labels[k] = v
	}

	errors := make([]string, len(host.Errors))
	copy(errors, host.Errors)

	result := HostDTO{
		ID:           host.ID,
		FQDN:         host.FQDN,
		Labels:       labels,
		Workloads:    workloads,
		Status:       host.Status,
		SourceStatus: sourceStatuses,
		Errors:       errors,
	}

	if host.LastObservedAt != nil {
		timestamp := host.LastObservedAt.UTC()
		result.LastObservedAt = &timestamp
	}

	return result
}

func toHostsResponseMetadata(metadata domain.InventoryMetadata, returnedHosts int) HostsResponseMetadata {
	return HostsResponseMetadata{
		IsPartial:            metadata.IsPartial,
		TotalHosts:           metadata.TotalHosts,
		ReturnedHosts:        returnedHosts,
		FailedHosts:          metadata.FailedHosts,
		LastSyncAt:           metadata.LastSyncAt,
		LastSuccessfulSyncAt: metadata.LastSuccessfulSyncAt,
		LastFullSyncAt:       metadata.LastFullSyncAt,
		LastPartialSyncAt:    metadata.LastPartialSyncAt,
		SyncDurationMs:       metadata.SyncDurationMs,
	}
}
