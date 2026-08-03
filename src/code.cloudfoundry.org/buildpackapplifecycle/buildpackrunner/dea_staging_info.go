package buildpackrunner

import "code.cloudfoundry.org/buildpackapplifecycle"

const DeaStagingInfoFilename = "staging_info.yml"

// Used to generate YAML file read by the DEA
type DeaStagingInfo struct {
	DetectedBuildpack string                                 `json:"detected_buildpack" yaml:"detected_buildpack"`
	StartCommand      string                                 `json:"start_command" yaml:"start_command"`
	Config            *buildpackapplifecycle.BuildpackConfig `json:"config,omitempty" yaml:"config,omitempty"`
}

func (stagingInfo DeaStagingInfo) GetEntrypointPrefix() string {
	if stagingInfo.Config != nil {
		return stagingInfo.Config.EntrypointPrefix
	}

	return ""
}
