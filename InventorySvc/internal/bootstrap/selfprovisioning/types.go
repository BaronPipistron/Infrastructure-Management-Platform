package selfprovisioning

import "inventory-svc/internal/domain"

type File struct {
	Hosts []domain.HostSeed `yaml:"hosts"`
}
