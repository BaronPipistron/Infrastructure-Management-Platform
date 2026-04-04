package detectors

import (
	"fmt"
	"strings"

	"drift-detector-svc/internal/domain"
)

const workloadSourceCadvisor = "cadvisor"

type Detector interface {
	Component() domain.Component
	Detect(cycleID string, desired domain.DesiredHost, actual domain.ActualHost) domain.DetectorResult
}

func buildCorrelationID(cycleID string, component domain.Component, fqdn string) string {
	return fmt.Sprintf("%s:%s:%s", cycleID, component, strings.ToLower(strings.TrimSpace(fqdn)))
}

func resolveTargetFQDN(desired domain.DesiredHost, actual domain.ActualHost) string {
	if strings.TrimSpace(desired.FQDN) != "" {
		return strings.TrimSpace(desired.FQDN)
	}
	return strings.TrimSpace(actual.FQDN)
}
