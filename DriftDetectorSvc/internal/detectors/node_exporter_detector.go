package detectors

import (
	"fmt"
	"strings"

	"drift-detector-svc/internal/domain"
)

type NodeExporterDetector struct{}

func NewNodeExporterDetector() *NodeExporterDetector {
	return &NodeExporterDetector{}
}

func (d *NodeExporterDetector) Component() domain.Component {
	return domain.ComponentNodeExporter
}

func (d *NodeExporterDetector) Detect(cycleID string, desired domain.DesiredHost, actual domain.ActualHost) domain.DetectorResult {
	result := domain.DetectorResult{Component: d.Component()}

	desiredWorkload, found := desired.DesiredWorkload(d.Component())
	if !found || !desiredWorkload.Enabled {
		return result
	}
	if !strings.EqualFold(strings.TrimSpace(desiredWorkload.DeploymentMode), "container") {
		return result
	}

	result.Applicable = true
	if !actual.IsSourceHealthy(workloadSourceCadvisor) {
		result.MissingActualData = true
		result.SkipReason = "cadvisor source data is not healthy"
		return result
	}

	if actual.HasWorkload(d.Component()) {
		return result
	}

	targetFQDN := resolveTargetFQDN(desired, actual)
	hostID := strings.TrimSpace(desired.HostID)
	if hostID == "" {
		hostID = strings.TrimSpace(actual.HostID)
	}

	parameters := map[string]interface{}{}
	if strings.TrimSpace(desiredWorkload.Image) != "" {
		parameters["node_exporter_image"] = strings.TrimSpace(desiredWorkload.Image)
	}
	if desiredWorkload.Port != nil {
		parameters["node_exporter_port"] = *desiredWorkload.Port
	}

	result.Drift = true
	result.Finding = &domain.DriftFinding{
		Component: d.Component(),
		HostID:    hostID,
		FQDN:      targetFQDN,
		Reason:    "component is required in desired state but missing in actual state",
	}
	result.ReconcileCommand = &domain.ReconcileCommand{
		HostID:        hostID,
		FQDN:          targetFQDN,
		Component:     d.Component(),
		CorrelationID: buildCorrelationID(cycleID, d.Component(), targetFQDN),
		Parameters:    parameters,
	}
	result.SkipReason = fmt.Sprintf("drift detected for %s", d.Component())

	return result
}
