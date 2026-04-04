package detectors

import (
	"sort"

	"drift-detector-svc/internal/domain"
)

type Registry struct {
	detectors map[domain.Component]Detector
}

func NewRegistry(items ...Detector) *Registry {
	detectorsByComponent := make(map[domain.Component]Detector, len(items))
	for _, detector := range items {
		detectorsByComponent[detector.Component()] = detector
	}

	return &Registry{detectors: detectorsByComponent}
}

func (r *Registry) Get(component domain.Component) (Detector, bool) {
	detector, found := r.detectors[component]
	return detector, found
}

func (r *Registry) Components() []domain.Component {
	result := make([]domain.Component, 0, len(r.detectors))
	for component := range r.detectors {
		result = append(result, component)
	}

	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result
}
