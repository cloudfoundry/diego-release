package dockerapplifecycle

type ProcessTypes map[string]string

type LifecycleMetadata struct {
	DockerImage string `json:"docker_image"`
}

type StagingResult struct {
	LifecycleType     string `json:"lifecycle_type"`
	LifecycleMetadata `json:"lifecycle_metadata"`
	ProcessTypes      `json:"process_types"`
	ExecutionMetadata string `json:"execution_metadata"`
}

func NewStagingResult(procTypes ProcessTypes, lifeMeta LifecycleMetadata, execMeta string) StagingResult {
	return StagingResult{
		LifecycleType:     "docker",
		LifecycleMetadata: lifeMeta,
		ProcessTypes:      procTypes,
		ExecutionMetadata: execMeta,
	}
}
