package selfprovisioning

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"inventory-svc/internal/domain"

	"gopkg.in/yaml.v3"
)

var requiredLabels = []string{"managed_by", "purpose", "env"}

func Load(path string) ([]domain.HostSeed, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read selfProvisioning file: %w", err)
	}

	var cfg File
	if err := yaml.Unmarshal(content, &cfg); err != nil {
		return nil, fmt.Errorf("unmarshal selfProvisioning: %w", err)
	}

	if err := validate(cfg.Hosts); err != nil {
		return nil, err
	}

	return cfg.Hosts, nil
}

func validate(hosts []domain.HostSeed) error {
	if len(hosts) == 0 {
		return errors.New("selfProvisioning must contain at least one host")
	}

	seenIDs := make(map[string]struct{}, len(hosts))
	seenFQDN := make(map[string]struct{}, len(hosts))
	var errs []string

	for i, host := range hosts {
		host.ID = strings.TrimSpace(host.ID)
		host.FQDN = strings.TrimSpace(host.FQDN)

		if host.ID == "" {
			errs = append(errs, fmt.Sprintf("hosts[%d].id is required", i))
		} else {
			if _, exists := seenIDs[host.ID]; exists {
				errs = append(errs, fmt.Sprintf("hosts[%d].id is duplicated: %q", i, host.ID))
			}
			seenIDs[host.ID] = struct{}{}
		}

		if host.FQDN == "" {
			errs = append(errs, fmt.Sprintf("hosts[%d].fqdn is required", i))
		} else {
			if _, exists := seenFQDN[host.FQDN]; exists {
				errs = append(errs, fmt.Sprintf("hosts[%d].fqdn is duplicated: %q", i, host.FQDN))
			}
			seenFQDN[host.FQDN] = struct{}{}
		}

		if host.Labels == nil {
			errs = append(errs, fmt.Sprintf("hosts[%d].labels is required", i))
			continue
		}

		for _, requiredLabel := range requiredLabels {
			value, ok := host.Labels[requiredLabel]
			if !ok || strings.TrimSpace(value) == "" {
				errs = append(errs, fmt.Sprintf("hosts[%d].labels.%s is required", i, requiredLabel))
			}
		}
	}

	if len(errs) > 0 {
		return errors.New("invalid selfProvisioning config: " + strings.Join(errs, "; "))
	}

	return nil
}
