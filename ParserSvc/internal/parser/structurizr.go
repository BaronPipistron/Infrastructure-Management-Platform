package parser

import (
	"encoding/json"
	"errors"
	"fmt"
)

type StructurizrManifest struct {
	Model StructurizrModel `json:"model"`
}

type StructurizrModel struct {
	DeploymentNodes []DeploymentNode `json:"deploymentNodes"`
	SoftwareSystems []SoftwareSystem `json:"softwareSystems"`
}

type SoftwareSystem struct {
	ID         string      `json:"id"`
	Name       string      `json:"name"`
	Containers []Container `json:"containers"`
}

type Container struct {
	ID         string                 `json:"id"`
	Name       string                 `json:"name"`
	Technology string                 `json:"technology"`
	Properties map[string]interface{} `json:"properties"`
}

type DeploymentNode struct {
	ID                 string                 `json:"id"`
	Name               string                 `json:"name"`
	Environment        string                 `json:"environment"`
	Properties         map[string]interface{} `json:"properties"`
	ContainerInstances []ContainerInstance    `json:"containerInstances"`
	Children           []DeploymentNode       `json:"children"`
	DeploymentNodes    []DeploymentNode       `json:"deploymentNodes"`
}

type ContainerInstance struct {
	ID          string                 `json:"id"`
	ContainerID string                 `json:"containerId"`
	Properties  map[string]interface{} `json:"properties"`
}

func ParseStructurizrManifest(data []byte) (StructurizrManifest, error) {
	var manifest StructurizrManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return StructurizrManifest{}, fmt.Errorf("unmarshal structurizr json: %w", err)
	}

	if len(manifest.Model.DeploymentNodes) == 0 {
		return StructurizrManifest{}, errors.New("manifest contains no deployment nodes")
	}

	return manifest, nil
}

func (d DeploymentNode) ChildNodes() []DeploymentNode {
	if len(d.Children) == 0 {
		return d.DeploymentNodes
	}

	if len(d.DeploymentNodes) == 0 {
		return d.Children
	}

	result := make([]DeploymentNode, 0, len(d.Children)+len(d.DeploymentNodes))
	result = append(result, d.Children...)
	result = append(result, d.DeploymentNodes...)
	return result
}
