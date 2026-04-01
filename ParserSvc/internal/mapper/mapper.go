package mapper

import (
	"fmt"
	"strconv"
	"strings"

	"parser-svc/internal/domain"
	"parser-svc/internal/parser"
	"parser-svc/internal/validator"
)

type Warning struct {
	File     string `json:"file"`
	NodeID   string `json:"nodeId,omitempty"`
	HostID   string `json:"hostId,omitempty"`
	Workload string `json:"workload,omitempty"`
	Message  string `json:"message"`
}

type Result struct {
	Hosts     []domain.DesiredHost
	Warnings  []Warning
	Workloads int
}

func MapManifest(manifest parser.StructurizrManifest, sourceFile string) Result {
	containerNamesByID := buildContainerMap(manifest)
	result := Result{
		Hosts:    make([]domain.DesiredHost, 0),
		Warnings: make([]Warning, 0),
	}

	for _, node := range manifest.Model.DeploymentNodes {
		walkDeploymentNode(node, sourceFile, containerNamesByID, &result)
	}

	return result
}

func walkDeploymentNode(node parser.DeploymentNode, sourceFile string, containerNamesByID map[string]string, result *Result) {
	host, hostErr := mapHost(node)
	if hostErr != nil {
		result.Warnings = append(result.Warnings, Warning{
			File:    sourceFile,
			NodeID:  node.ID,
			Message: fmt.Sprintf("skip deployment node: %v", hostErr),
		})
	} else {
		for _, instance := range node.ContainerInstances {
			workload, err := mapWorkload(instance, containerNamesByID)
			if err != nil {
				result.Warnings = append(result.Warnings, Warning{
					File:     sourceFile,
					NodeID:   node.ID,
					HostID:   host.HostID,
					Workload: instance.ID,
					Message:  fmt.Sprintf("skip workload: %v", err),
				})
				continue
			}
			host.Workloads = append(host.Workloads, workload)
			result.Workloads++
		}

		result.Hosts = append(result.Hosts, host)
	}

	for _, child := range node.ChildNodes() {
		walkDeploymentNode(child, sourceFile, containerNamesByID, result)
	}
}

func buildContainerMap(manifest parser.StructurizrManifest) map[string]string {
	containerNamesByID := make(map[string]string)
	for _, system := range manifest.Model.SoftwareSystems {
		for _, container := range system.Containers {
			containerID := strings.TrimSpace(container.ID)
			if containerID == "" {
				continue
			}
			containerNamesByID[containerID] = strings.TrimSpace(container.Name)
		}
	}

	return containerNamesByID
}

func mapHost(node parser.DeploymentNode) (domain.DesiredHost, error) {
	hostID, err := getRequiredString(node.Properties, "host_id")
	if err != nil {
		return domain.DesiredHost{}, err
	}

	fqdn, err := getRequiredString(node.Properties, "fqdn")
	if err != nil {
		return domain.DesiredHost{}, err
	}

	env, err := getRequiredString(node.Properties, "env")
	if err != nil {
		return domain.DesiredHost{}, err
	}

	managedBy, err := getRequiredString(node.Properties, "managed_by")
	if err != nil {
		return domain.DesiredHost{}, err
	}

	purpose, err := getRequiredString(node.Properties, "purpose")
	if err != nil {
		return domain.DesiredHost{}, err
	}

	ip, _ := getOptionalString(node.Properties, "ip")

	host := domain.DesiredHost{
		HostID:    hostID,
		FQDN:      fqdn,
		IP:        ip,
		Env:       env,
		ManagedBy: managedBy,
		Purpose:   purpose,
		Labels:    extractLabels(node.Properties, env, managedBy, purpose),
		Workloads: make([]domain.DesiredWorkload, 0),
	}

	if err := validator.ValidateHost(host); err != nil {
		return domain.DesiredHost{}, err
	}

	return host, nil
}

func mapWorkload(instance parser.ContainerInstance, containerNamesByID map[string]string) (domain.DesiredWorkload, error) {
	containerID := strings.TrimSpace(instance.ContainerID)
	name, found := containerNamesByID[containerID]
	if !found || strings.TrimSpace(name) == "" {
		return domain.DesiredWorkload{}, fmt.Errorf("container definition not found for containerId=%s", containerID)
	}

	enabled, err := getRequiredBool(instance.Properties, "enabled")
	if err != nil {
		return domain.DesiredWorkload{}, err
	}

	deploymentMode, err := getRequiredString(instance.Properties, "deployment_mode")
	if err != nil {
		return domain.DesiredWorkload{}, err
	}

	image, _ := getOptionalString(instance.Properties, "image")

	var port *int
	portRaw, hasPort := instance.Properties["port"]
	if hasPort {
		parsedPort, parseErr := normalizeInt(portRaw)
		if parseErr != nil {
			return domain.DesiredWorkload{}, fmt.Errorf("invalid port: %w", parseErr)
		}
		port = &parsedPort
	}

	var version *string
	versionRaw, hasVersion := instance.Properties["version"]
	if hasVersion {
		parsedVersion, parseErr := normalizeString(versionRaw)
		if parseErr != nil {
			return domain.DesiredWorkload{}, fmt.Errorf("invalid version: %w", parseErr)
		}
		if parsedVersion != "" {
			version = &parsedVersion
		}
	}

	workload := domain.DesiredWorkload{
		Name:           name,
		Enabled:        enabled,
		DeploymentMode: deploymentMode,
		Image:          image,
		Version:        version,
		Port:           port,
	}

	if err := validator.ValidateWorkload(workload); err != nil {
		return domain.DesiredWorkload{}, err
	}

	return workload, nil
}

func extractLabels(properties map[string]interface{}, env, managedBy, purpose string) map[string]string {
	labels := make(map[string]string)

	for key, value := range properties {
		if strings.HasPrefix(key, "structurizr.") {
			continue
		}
		switch key {
		case "host_id", "fqdn", "ip", "env", "managed_by", "purpose":
			continue
		}

		normalized, err := normalizeString(value)
		if err != nil || normalized == "" {
			continue
		}
		labels[key] = normalized
	}

	if env != "" {
		labels["env"] = env
	}
	if managedBy != "" {
		labels["managed_by"] = managedBy
	}
	if purpose != "" {
		labels["purpose"] = purpose
	}

	return labels
}

func getRequiredString(properties map[string]interface{}, key string) (string, error) {
	value, ok := properties[key]
	if !ok {
		return "", fmt.Errorf("%s is required", key)
	}

	normalized, err := normalizeString(value)
	if err != nil {
		return "", fmt.Errorf("%s: %w", key, err)
	}
	if normalized == "" {
		return "", fmt.Errorf("%s is required", key)
	}

	return normalized, nil
}

func getOptionalString(properties map[string]interface{}, key string) (string, error) {
	value, ok := properties[key]
	if !ok {
		return "", nil
	}

	normalized, err := normalizeString(value)
	if err != nil {
		return "", fmt.Errorf("%s: %w", key, err)
	}

	return normalized, nil
}

func getRequiredBool(properties map[string]interface{}, key string) (bool, error) {
	value, ok := properties[key]
	if !ok {
		return false, fmt.Errorf("%s is required", key)
	}

	normalized, err := normalizeBool(value)
	if err != nil {
		return false, fmt.Errorf("%s: %w", key, err)
	}

	return normalized, nil
}

func normalizeString(value interface{}) (string, error) {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed), nil
	case float64:
		if typed == float64(int64(typed)) {
			return strconv.FormatInt(int64(typed), 10), nil
		}
		return strconv.FormatFloat(typed, 'f', -1, 64), nil
	case bool:
		if typed {
			return "true", nil
		}
		return "false", nil
	case int:
		return strconv.Itoa(typed), nil
	case int64:
		return strconv.FormatInt(typed, 10), nil
	case nil:
		return "", nil
	default:
		return "", fmt.Errorf("unsupported type %T", value)
	}
}

func normalizeBool(value interface{}) (bool, error) {
	switch typed := value.(type) {
	case bool:
		return typed, nil
	case string:
		normalized := strings.ToLower(strings.TrimSpace(typed))
		switch normalized {
		case "true", "1", "yes", "y":
			return true, nil
		case "false", "0", "no", "n":
			return false, nil
		default:
			return false, fmt.Errorf("unsupported boolean value: %s", typed)
		}
	case float64:
		if typed == 1 {
			return true, nil
		}
		if typed == 0 {
			return false, nil
		}
		return false, fmt.Errorf("unsupported boolean number: %v", typed)
	case int:
		if typed == 1 {
			return true, nil
		}
		if typed == 0 {
			return false, nil
		}
		return false, fmt.Errorf("unsupported boolean number: %d", typed)
	default:
		return false, fmt.Errorf("unsupported type %T", value)
	}
}

func normalizeInt(value interface{}) (int, error) {
	switch typed := value.(type) {
	case int:
		return typed, nil
	case int64:
		return int(typed), nil
	case float64:
		if typed != float64(int(typed)) {
			return 0, fmt.Errorf("non-integer number: %v", typed)
		}
		return int(typed), nil
	case string:
		normalized := strings.TrimSpace(typed)
		if normalized == "" {
			return 0, fmt.Errorf("empty numeric value")
		}
		parsed, err := strconv.Atoi(normalized)
		if err != nil {
			return 0, err
		}
		return parsed, nil
	default:
		return 0, fmt.Errorf("unsupported type %T", value)
	}
}
