package staging

import (
	"strings"

	"github.com/buildpacks/lifecycle/platform/files"
)

const LifecycleType = "cnb"

type LifecycleMetadata struct {
	Buildpacks []BuildpackMetadata `json:"buildpacks"`
}

type BuildpackMetadata struct {
	ID      string `json:"key" yaml:"key"`
	Name    string `json:"name" yaml:"name"`
	Version string `json:"version,omitempty" yaml:"version,omitempty"`
}

type ProcessTypes map[string]string

type StagingResult struct {
	LifecycleMetadata `json:"lifecycle_metadata"`
	ProcessTypes      `json:"process_types"`
	ExecutionMetadata string `json:"execution_metadata"`
	LifecycleType     string `json:"lifecycle_type"`
}

func StagingResultFromMetadata(buildMeta *files.BuildMetadata) *StagingResult {
	result := &StagingResult{
		LifecycleType: LifecycleType,
		LifecycleMetadata: LifecycleMetadata{
			Buildpacks: []BuildpackMetadata{},
		},
		ProcessTypes: ProcessTypes{},
	}

	for _, buildpack := range buildMeta.Buildpacks {
		result.Buildpacks = append(result.Buildpacks, BuildpackMetadata{
			ID:      buildpack.ID,
			Name:    buildpack.String(),
			Version: buildpack.Version,
		})
	}

	for _, process := range buildMeta.Processes {
		result.ProcessTypes[process.Type] = strings.Join(append(process.Command.Entries, process.Args...), " ")
	}

	return result
}
