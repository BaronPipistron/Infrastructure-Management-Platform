package domain

import "time"

type HostStatus string

type SourceStatusType string

const (
	HostStatusOK            HostStatus = "ok"
	HostStatusPartial       HostStatus = "partial"
	HostStatusError         HostStatus = "error"
	HostStatusBootstrapOnly HostStatus = "bootstrap_only"
)

const (
	SourceStatusOK       SourceStatusType = "ok"
	SourceStatusError    SourceStatusType = "error"
	SourceStatusDisabled SourceStatusType = "disabled"
)

type HostSeed struct {
	ID     string            `json:"id" yaml:"id"`
	FQDN   string            `json:"fqdn" yaml:"fqdn"`
	Labels map[string]string `json:"labels" yaml:"labels"`
}

type Workload struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Image      string     `json:"image,omitempty"`
	Runtime    string     `json:"runtime,omitempty"`
	Source     string     `json:"source"`
	LastSeenAt *time.Time `json:"lastSeenAt,omitempty"`
}

type SourceData struct {
	Workloads  []Workload
	ObservedAt time.Time
}

type SourceStatus struct {
	Source     string           `json:"source"`
	Enabled    bool             `json:"enabled"`
	Status     SourceStatusType `json:"status"`
	ObservedAt *time.Time       `json:"observedAt,omitempty"`
	Error      string           `json:"error,omitempty"`
}

type Host struct {
	ID             string                  `json:"id"`
	FQDN           string                  `json:"fqdn"`
	Labels         map[string]string       `json:"labels"`
	Workloads      []Workload              `json:"workloads"`
	LastObservedAt *time.Time              `json:"lastObservedAt,omitempty"`
	Status         HostStatus              `json:"status"`
	SourceStatus   map[string]SourceStatus `json:"sourceStatus"`
	Errors         []string                `json:"errors,omitempty"`
}

type InventoryMetadata struct {
	IsPartial            bool       `json:"isPartial"`
	TotalHosts           int        `json:"totalHosts"`
	FailedHosts          int        `json:"failedHosts"`
	LastSyncAt           *time.Time `json:"lastSyncAt,omitempty"`
	LastSuccessfulSyncAt *time.Time `json:"lastSuccessfulSyncAt,omitempty"`
	LastFullSyncAt       *time.Time `json:"lastFullSyncAt,omitempty"`
	LastPartialSyncAt    *time.Time `json:"lastPartialSyncAt,omitempty"`
	SyncDurationMs       int64      `json:"syncDurationMs"`
}

type InventorySnapshot struct {
	Hosts    []Host            `json:"hosts"`
	Metadata InventoryMetadata `json:"metadata"`
}

func (s InventorySnapshot) Clone() InventorySnapshot {
	clonedHosts := make([]Host, 0, len(s.Hosts))
	for _, host := range s.Hosts {
		clonedHosts = append(clonedHosts, host.Clone())
	}

	metadata := s.Metadata
	metadata.LastSyncAt = cloneTimePtr(metadata.LastSyncAt)
	metadata.LastSuccessfulSyncAt = cloneTimePtr(metadata.LastSuccessfulSyncAt)
	metadata.LastFullSyncAt = cloneTimePtr(metadata.LastFullSyncAt)
	metadata.LastPartialSyncAt = cloneTimePtr(metadata.LastPartialSyncAt)

	return InventorySnapshot{
		Hosts:    clonedHosts,
		Metadata: metadata,
	}
}

func (h Host) Clone() Host {
	clonedLabels := make(map[string]string, len(h.Labels))
	for k, v := range h.Labels {
		clonedLabels[k] = v
	}

	clonedSourceStatus := make(map[string]SourceStatus, len(h.SourceStatus))
	for source, status := range h.SourceStatus {
		status.ObservedAt = cloneTimePtr(status.ObservedAt)
		clonedSourceStatus[source] = status
	}

	clonedErrors := make([]string, len(h.Errors))
	copy(clonedErrors, h.Errors)

	clonedWorkloads := make([]Workload, 0, len(h.Workloads))
	for _, workload := range h.Workloads {
		w := workload
		w.LastSeenAt = cloneTimePtr(workload.LastSeenAt)
		clonedWorkloads = append(clonedWorkloads, w)
	}

	return Host{
		ID:             h.ID,
		FQDN:           h.FQDN,
		Labels:         clonedLabels,
		Workloads:      clonedWorkloads,
		LastObservedAt: cloneTimePtr(h.LastObservedAt),
		Status:         h.Status,
		SourceStatus:   clonedSourceStatus,
		Errors:         clonedErrors,
	}
}

func cloneTimePtr(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	clone := *value
	return &clone
}
