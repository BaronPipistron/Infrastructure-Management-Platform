package cadvisor

import "time"

type containersResponse []containerInfo

type containerInfo struct {
	Name      string          `json:"name"`
	Aliases   []string        `json:"aliases"`
	Namespace string          `json:"namespace"`
	Spec      containerSpec   `json:"spec"`
	Stats     []containerStat `json:"stats"`
}

type containerSpec struct {
	Image string `json:"image"`
}

type containerStat struct {
	Timestamp time.Time `json:"timestamp"`
}
