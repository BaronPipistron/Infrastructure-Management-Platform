package detectors

import (
	"strings"

	"drift-detector-svc/internal/domain"
)

type CadvisorDetector struct{}

func NewCadvisorDetector() *CadvisorDetector {
	return &CadvisorDetector{}
}

func (d *CadvisorDetector) Component() domain.Component {
	return domain.ComponentCadvisor
}

func (d *CadvisorDetector) Detect(cycleID string, desired domain.DesiredHost, actual domain.ActualHost) domain.DetectorResult {
	result := domain.DetectorResult{Component: d.Component()}

	desiredWorkload, found := desired.DesiredWorkload(d.Component())
	if !found || !desiredWorkload.Enabled {
		return result
	}
	if !strings.EqualFold(strings.TrimSpace(desiredWorkload.DeploymentMode), "container") {
		return result
	}

	result.Applicable = true
	targetFQDN := resolveTargetFQDN(desired, actual)
	hostID := strings.TrimSpace(desired.HostID)
	if hostID == "" {
		hostID = strings.TrimSpace(actual.HostID)
	}

	parameters := map[string]interface{}{}
	if strings.TrimSpace(desiredWorkload.Image) != "" {
		parameters["cadvisor_image"] = strings.TrimSpace(desiredWorkload.Image)
	}
	if desiredWorkload.Port != nil {
		parameters["cadvisor_port"] = *desiredWorkload.Port
	}

	if !actual.IsSourceHealthy(workloadSourceCadvisor) {
		// Bootstrap mode for local cold-start:
		// if cAdvisor source is unavailable, we still attempt to install cadvisor.
		result.Drift = true
		result.Finding = &domain.DriftFinding{
			Component: d.Component(),
			HostID:    hostID,
			FQDN:      targetFQDN,
			Reason:    "cadvisor source is not healthy, bootstrap reconcile is required",
		}
		result.ReconcileCommand = &domain.ReconcileCommand{
			HostID:        hostID,
			FQDN:          targetFQDN,
			Component:     d.Component(),
			CorrelationID: buildCorrelationID(cycleID, d.Component(), targetFQDN),
			Parameters:    parameters,
		}
		result.SkipReason = "bootstrap reconcile requested for cadvisor"
		return result
	}

	if actual.HasWorkload(d.Component()) {
		return result
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
	result.SkipReason = "drift detected for cadvisor"

	return result
}
