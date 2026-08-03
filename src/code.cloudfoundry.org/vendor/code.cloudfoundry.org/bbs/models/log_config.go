package models

import "strconv"

// LogConfig contains container log routing configuration.
// Moved here from code.cloudfoundry.org/executor to break silk-release's
// dependency on the executor module.
type LogConfig struct {
	Guid       string            `json:"guid"`
	Index      int               `json:"index"`
	SourceName string            `json:"source_name"`
	Tags       map[string]string `json:"tags"`
}

func (l LogConfig) GetSourceNameAndTagsForLogging() (string, map[string]string) {
	sourceName := l.SourceName
	if sourceName == "" {
		sourceName = "LOG"
	}

	tags := map[string]string{}
	for k, v := range l.Tags {
		tags[k] = v
	}

	if _, ok := tags["source_id"]; !ok {
		tags["source_id"] = l.Guid
	}
	sourceIndex := strconv.Itoa(l.Index)
	if _, ok := tags["instance_id"]; !ok {
		tags["instance_id"] = sourceIndex
	}
	return sourceName, tags
}
