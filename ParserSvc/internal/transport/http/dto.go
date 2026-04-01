package httpapi

import (
	"time"

	"parser-svc/internal/domain"
)

type HealthResponse struct {
	Status string `json:"status" example:"ok"`
}

type ReadinessResponse struct {
	Status      string `json:"status" example:"ready"`
	ReadyReason string `json:"readyReason,omitempty"`
	LoadedFiles int    `json:"loadedFiles"`
	BrokenFiles int    `json:"brokenFiles"`
	HostsTotal  int    `json:"hostsTotal"`
}

type APIError struct {
	Message string `json:"message" example:"host not found"`
}

type DesiredStateResponse struct {
	Metadata DesiredStateMetadata `json:"metadata"`
	State    DesiredStateDTO      `json:"desiredState"`
}

type DesiredStateMetadata struct {
	LoadedAt       time.Time `json:"loadedAt"`
	ManifestMode   string    `json:"manifestMode"`
	ManifestPath   string    `json:"manifestPath"`
	FilesTotal     int       `json:"filesTotal"`
	FilesLoaded    int       `json:"filesLoaded"`
	FilesBroken    int       `json:"filesBroken"`
	FilesIgnored   int       `json:"filesIgnored"`
	HostsTotal     int       `json:"hostsTotal"`
	WorkloadsTotal int       `json:"workloadsTotal"`
	Ready          bool      `json:"ready"`
	ReadyReason    string    `json:"readyReason,omitempty"`
}

type DesiredStateDTO struct {
	Hosts []DesiredHostDTO `json:"hosts"`
}

type HostsResponse struct {
	Metadata HostsResponseMetadata `json:"metadata"`
	Hosts    []DesiredHostDTO      `json:"hosts"`
}

type HostsResponseMetadata struct {
	LoadedAt       time.Time `json:"loadedAt"`
	TotalHosts     int       `json:"totalHosts"`
	ReturnedHosts  int       `json:"returnedHosts"`
	WorkloadsTotal int       `json:"workloadsTotal"`
	FilesLoaded    int       `json:"filesLoaded"`
	FilesBroken    int       `json:"filesBroken"`
}

type HostResponse struct {
	Metadata HostResponseMetadata `json:"metadata"`
	Host     DesiredHostDTO       `json:"host"`
}

type HostResponseMetadata struct {
	LoadedAt    time.Time `json:"loadedAt"`
	FilesLoaded int       `json:"filesLoaded"`
	FilesBroken int       `json:"filesBroken"`
}

type DesiredHostDTO struct {
	HostID    string               `json:"host_id"`
	FQDN      string               `json:"fqdn"`
	IP        string               `json:"ip,omitempty"`
	Labels    map[string]string    `json:"labels"`
	Env       string               `json:"env"`
	ManagedBy string               `json:"managed_by"`
	Purpose   string               `json:"purpose"`
	Workloads []DesiredWorkloadDTO `json:"workloads"`
}

type DesiredWorkloadDTO struct {
	Name           string  `json:"name"`
	Enabled        bool    `json:"enabled"`
	DeploymentMode string  `json:"deployment_mode"`
	Image          string  `json:"image,omitempty"`
	Version        *string `json:"version,omitempty"`
	Port           *int    `json:"port,omitempty"`
}

func toDesiredStateResponse(snapshot domain.Snapshot) DesiredStateResponse {
	hosts := make([]DesiredHostDTO, 0, len(snapshot.State.Hosts))
	for _, host := range snapshot.State.Hosts {
		hosts = append(hosts, toDesiredHostDTO(host))
	}

	return DesiredStateResponse{
		Metadata: DesiredStateMetadata{
			LoadedAt:       snapshot.Metadata.LoadedAt,
			ManifestMode:   snapshot.Metadata.ManifestMode,
			ManifestPath:   snapshot.Metadata.ManifestPath,
			FilesTotal:     snapshot.Metadata.FilesTotal,
			FilesLoaded:    snapshot.Metadata.FilesLoaded,
			FilesBroken:    snapshot.Metadata.FilesBroken,
			FilesIgnored:   snapshot.Metadata.FilesIgnored,
			HostsTotal:     snapshot.Metadata.HostsTotal,
			WorkloadsTotal: snapshot.Metadata.WorkloadsTotal,
			Ready:          snapshot.Metadata.Ready,
			ReadyReason:    snapshot.Metadata.ReadyReason,
		},
		State: DesiredStateDTO{Hosts: hosts},
	}
}

func toDesiredHostDTO(host domain.DesiredHost) DesiredHostDTO {
	labels := make(map[string]string, len(host.Labels))
	for k, v := range host.Labels {
		labels[k] = v
	}

	workloads := make([]DesiredWorkloadDTO, 0, len(host.Workloads))
	for _, workload := range host.Workloads {
		item := DesiredWorkloadDTO{
			Name:           workload.Name,
			Enabled:        workload.Enabled,
			DeploymentMode: workload.DeploymentMode,
			Image:          workload.Image,
			Version:        workload.Version,
			Port:           workload.Port,
		}
		workloads = append(workloads, item)
	}

	return DesiredHostDTO{
		HostID:    host.HostID,
		FQDN:      host.FQDN,
		IP:        host.IP,
		Labels:    labels,
		Env:       host.Env,
		ManagedBy: host.ManagedBy,
		Purpose:   host.Purpose,
		Workloads: workloads,
	}
}
