package executor

import "code.cloudfoundry.org/bbs/models"

func EnvironmentVariablesToModel(envVars []EnvironmentVariable) []models.EnvironmentVariable {
	out := make([]models.EnvironmentVariable, len(envVars))
	for i, val := range envVars {
		out[i].Name = val.Name
		out[i].Value = val.Value
	}
	return out
}

func EnvironmentVariablesFromModel(envVars []*models.EnvironmentVariable) []EnvironmentVariable {
	out := make([]EnvironmentVariable, len(envVars))
	for i, val := range envVars {
		out[i].Name = val.Name
		out[i].Value = val.Value
	}
	return out
}

func VolumeMountedFilesFromModel(envFiles []*models.File) []VolumeMountedFiles {
	out := make([]VolumeMountedFiles, len(envFiles))
	for i, envFile := range envFiles {
		out[i].Path = envFile.Path
		out[i].Content = envFile.Content
	}

	return out
}
