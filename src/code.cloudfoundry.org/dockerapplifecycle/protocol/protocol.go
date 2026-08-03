package protocol

type ExecutionMetadata struct {
	Cmd          []string `json:"cmd,omitempty"`
	Entrypoint   []string `json:"entrypoint,omitempty"`
	Workdir      string   `json:"workdir,omitempty"`
	ExposedPorts []Port   `json:"ports,omitempty"`
	User         string   `json:"user,omitempty"`
}

type DockerImageMetadata struct {
	ExecutionMetadata ExecutionMetadata
	DockerImage       string
}

type Port struct {
	Port     uint16
	Protocol string
}
