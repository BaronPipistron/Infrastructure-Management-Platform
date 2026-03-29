package cadvisor

import (
	"path"
	"sort"
	"strings"
	"time"

	"inventory-svc/internal/domain"
)

const sourceName = "cadvisor"

func NormalizeContainers(response containersResponse) ([]domain.Workload, time.Time) {
	if len(response) == 0 {
		return []domain.Workload{}, time.Time{}
	}

	items := make([]containerInfo, 0, len(response))
	for _, item := range response {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool {
		return strings.TrimSpace(items[i].Name) < strings.TrimSpace(items[j].Name)
	})

	workloads := make([]domain.Workload, 0, len(items))
	latestTimestamp := time.Time{}

	for _, container := range items {
		name := strings.TrimSpace(container.Name)
		if name == "" {
			name = "/"
		}
		if name == "/" {
			continue
		}

		workloadID := pickWorkloadID(name, container.Aliases)
		workloadName := pickWorkloadName(name, container.Aliases)

		lastSeen := pickLastSeen(container.Stats)
		if lastSeen != nil && lastSeen.After(latestTimestamp) {
			latestTimestamp = *lastSeen
		}

		workloads = append(workloads, domain.Workload{
			ID:         workloadID,
			Name:       workloadName,
			Image:      strings.TrimSpace(container.Spec.Image),
			Runtime:    pickRuntime(container.Namespace),
			Source:     sourceName,
			LastSeenAt: lastSeen,
		})
	}

	return workloads, latestTimestamp
}

func FilterWorkloads(workloads []domain.Workload, includeSystemWorkloads bool) []domain.Workload {
	if includeSystemWorkloads {
		return workloads
	}

	filtered := make([]domain.Workload, 0, len(workloads))
	for _, workload := range workloads {
		if isLikelySystemWorkload(workload) {
			continue
		}
		filtered = append(filtered, workload)
	}

	return filtered
}

func isLikelySystemWorkload(workload domain.Workload) bool {
	image := strings.TrimSpace(workload.Image)
	runtime := strings.TrimSpace(strings.ToLower(workload.Runtime))

	// Heuristic for MVP:
	// cAdvisor system cgroups usually have no image and unknown runtime.
	return image == "" && (runtime == "" || runtime == "unknown")
}

func pickWorkloadID(key string, aliases []string) string {
	for _, alias := range aliases {
		alias = strings.TrimSpace(alias)
		if alias != "" {
			return alias
		}
	}

	if key == "" {
		return "unknown"
	}

	return key
}

func pickWorkloadName(key string, aliases []string) string {
	for _, alias := range aliases {
		alias = strings.TrimSpace(alias)
		if alias == "" {
			continue
		}

		cleaned := strings.Trim(alias, "/")
		if cleaned == "" {
			continue
		}

		return path.Base(cleaned)
	}

	cleaned := strings.Trim(key, "/")
	if cleaned == "" {
		return "unknown"
	}

	return path.Base(cleaned)
}

func pickRuntime(namespace string) string {
	namespace = strings.TrimSpace(namespace)
	if namespace == "" {
		return "unknown"
	}
	return namespace
}

func pickLastSeen(stats []containerStat) *time.Time {
	if len(stats) == 0 {
		return nil
	}
	last := stats[len(stats)-1].Timestamp
	if last.IsZero() {
		return nil
	}
	value := last.UTC()
	return &value
}
