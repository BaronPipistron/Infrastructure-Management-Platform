package domain

import (
	"fmt"
	"strings"
	"time"
)

type Component string

const (
	ComponentNodeExporter Component = "node_exporter"
	ComponentCadvisor     Component = "cadvisor"
)

func ParseComponent(value string) (Component, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case string(ComponentNodeExporter):
		return ComponentNodeExporter, nil
	case string(ComponentCadvisor):
		return ComponentCadvisor, nil
	default:
		return "", fmt.Errorf("unsupported component: %s", value)
	}
}

type DesiredState struct {
	Hosts    []DesiredHost
	Metadata DesiredMetadata
}

type DesiredMetadata struct {
	Ready          bool
	ReadyReason    string
	FilesLoaded    int
	FilesBroken    int
	HostsTotal     int
	WorkloadsTotal int
	LoadedAt       *time.Time
}

type DesiredHost struct {
	HostID    string
	FQDN      string
	Labels    map[string]string
	Workloads []DesiredWorkload
}

type DesiredWorkload struct {
	Name           string
	Enabled        bool
	DeploymentMode string
	Image          string
	Version        *string
	Port           *int
}

func (h DesiredHost) DesiredWorkload(component Component) (DesiredWorkload, bool) {
	for _, workload := range h.Workloads {
		if strings.EqualFold(strings.TrimSpace(workload.Name), string(component)) {
			return workload, true
		}
	}

	return DesiredWorkload{}, false
}

type ActualState struct {
	Hosts    []ActualHost
	Metadata ActualMetadata
}

type ActualMetadata struct {
	IsPartial   bool
	TotalHosts  int
	FailedHosts int
	LastSyncAt  *time.Time
}

type ActualHost struct {
	HostID       string
	FQDN         string
	Labels       map[string]string
	Status       string
	Workloads    []ActualWorkload
	SourceStatus map[string]ActualSourceStatus
}

type ActualSourceStatus struct {
	Source  string
	Enabled bool
	Status  string
	Error   string
}

type ActualWorkload struct {
	Name string
}

func (h ActualHost) HasWorkload(component Component) bool {
	for _, workload := range h.Workloads {
		if strings.EqualFold(strings.TrimSpace(workload.Name), string(component)) {
			return true
		}
	}

	return false
}

func (h ActualHost) IsSourceHealthy(source string) bool {
	status, found := h.SourceStatus[source]
	if !found {
		return false
	}
	if !status.Enabled {
		return false
	}

	return strings.EqualFold(strings.TrimSpace(status.Status), "ok")
}

type DriftFinding struct {
	Component Component
	HostID    string
	FQDN      string
	Reason    string
}

type ReconcileCommand struct {
	HostID        string
	FQDN          string
	Component     Component
	CorrelationID string
	Parameters    map[string]interface{}
}

type ReconcileAccepted struct {
	RequestID     string
	AcceptedAt    *time.Time
	Component     Component
	FQDN          string
	CorrelationID string
}

type DetectorResult struct {
	Component         Component
	Applicable        bool
	MissingActualData bool
	MissingActualHost bool
	Drift             bool
	SkipReason        string
	Finding           *DriftFinding
	ReconcileCommand  *ReconcileCommand
}

type DetectionStats struct {
	DesiredHosts             int
	ComparedHosts            int
	SkippedHostsNoActualHost int
	SkippedHostsNoActualData int
	DriftsFound              int
	ReconcileSent            int
	ReconcileSuppressed      int
	Errors                   int
}

type DetectionStageTimings struct {
	InventoryFetchMs    int64
	ParserFetchMs       int64
	DriftComparisonMs   int64
	ReconcileDispatchMs int64
}

type DetectionCycleResult struct {
	CycleID                string
	Trigger                string
	StartedAt              time.Time
	FinishedAt             time.Time
	DurationMs             int64
	StageTimings           DetectionStageTimings
	Partial                bool
	InventoryMarkedPartial bool
	ParserReady            bool
	Stats                  DetectionStats
	Warnings               []string
	ErrorMessages          []string
}

func (r DetectionCycleResult) Clone() DetectionCycleResult {
	clonedWarnings := make([]string, len(r.Warnings))
	copy(clonedWarnings, r.Warnings)

	clonedErrors := make([]string, len(r.ErrorMessages))
	copy(clonedErrors, r.ErrorMessages)

	return DetectionCycleResult{
		CycleID:                r.CycleID,
		Trigger:                r.Trigger,
		StartedAt:              r.StartedAt,
		FinishedAt:             r.FinishedAt,
		DurationMs:             r.DurationMs,
		StageTimings:           r.StageTimings,
		Partial:                r.Partial,
		InventoryMarkedPartial: r.InventoryMarkedPartial,
		ParserReady:            r.ParserReady,
		Stats:                  r.Stats,
		Warnings:               clonedWarnings,
		ErrorMessages:          clonedErrors,
	}
}
