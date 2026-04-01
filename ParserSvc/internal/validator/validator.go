package validator

import (
	"errors"
	"fmt"
	"strings"

	"parser-svc/internal/domain"
)

func ValidateHost(host domain.DesiredHost) error {
	var errs []string

	if strings.TrimSpace(host.HostID) == "" {
		errs = append(errs, "host_id is required")
	}
	if strings.TrimSpace(host.FQDN) == "" {
		errs = append(errs, "fqdn is required")
	}
	if strings.TrimSpace(host.Env) == "" {
		errs = append(errs, "env is required")
	}
	if strings.TrimSpace(host.ManagedBy) == "" {
		errs = append(errs, "managed_by is required")
	}
	if strings.TrimSpace(host.Purpose) == "" {
		errs = append(errs, "purpose is required")
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}

	return nil
}

func ValidateWorkload(workload domain.DesiredWorkload) error {
	var errs []string

	if strings.TrimSpace(workload.Name) == "" {
		errs = append(errs, "workload name is required")
	}
	if strings.TrimSpace(workload.DeploymentMode) == "" {
		errs = append(errs, "deployment_mode is required")
	}

	if strings.EqualFold(strings.TrimSpace(workload.DeploymentMode), "container") && strings.TrimSpace(workload.Image) == "" {
		errs = append(errs, "image is required when deployment_mode=container")
	}

	if len(errs) > 0 {
		return fmt.Errorf("invalid workload: %s", strings.Join(errs, "; "))
	}

	return nil
}
