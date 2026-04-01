package domain

import "time"

type DesiredState struct {
	Hosts []DesiredHost `json:"hosts"`
}

type DesiredHost struct {
	HostID    string            `json:"host_id"`
	FQDN      string            `json:"fqdn"`
	IP        string            `json:"ip,omitempty"`
	Labels    map[string]string `json:"labels"`
	Env       string            `json:"env"`
	ManagedBy string            `json:"managed_by"`
	Purpose   string            `json:"purpose"`
	Workloads []DesiredWorkload `json:"workloads"`
}

type DesiredWorkload struct {
	Name           string  `json:"name"`
	Enabled        bool    `json:"enabled"`
	DeploymentMode string  `json:"deployment_mode"`
	Image          string  `json:"image,omitempty"`
	Version        *string `json:"version,omitempty"`
	Port           *int    `json:"port,omitempty"`
}

type SnapshotMetadata struct {
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

type Snapshot struct {
	State    DesiredState     `json:"desiredState"`
	Metadata SnapshotMetadata `json:"metadata"`
}

func (s Snapshot) Clone() Snapshot {
	return Snapshot{
		State:    s.State.Clone(),
		Metadata: s.Metadata,
	}
}

func (s DesiredState) Clone() DesiredState {
	hosts := make([]DesiredHost, 0, len(s.Hosts))
	for _, host := range s.Hosts {
		hosts = append(hosts, host.Clone())
	}

	return DesiredState{Hosts: hosts}
}

func (h DesiredHost) Clone() DesiredHost {
	labels := make(map[string]string, len(h.Labels))
	for k, v := range h.Labels {
		labels[k] = v
	}

	workloads := make([]DesiredWorkload, 0, len(h.Workloads))
	for _, workload := range h.Workloads {
		workloads = append(workloads, workload.Clone())
	}

	return DesiredHost{
		HostID:    h.HostID,
		FQDN:      h.FQDN,
		IP:        h.IP,
		Labels:    labels,
		Env:       h.Env,
		ManagedBy: h.ManagedBy,
		Purpose:   h.Purpose,
		Workloads: workloads,
	}
}

func (w DesiredWorkload) Clone() DesiredWorkload {
	var port *int
	if w.Port != nil {
		p := *w.Port
		port = &p
	}

	var version *string
	if w.Version != nil {
		v := *w.Version
		version = &v
	}

	return DesiredWorkload{
		Name:           w.Name,
		Enabled:        w.Enabled,
		DeploymentMode: w.DeploymentMode,
		Image:          w.Image,
		Version:        version,
		Port:           port,
	}
}
