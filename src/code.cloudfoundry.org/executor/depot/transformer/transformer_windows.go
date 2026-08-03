package transformer

import "code.cloudfoundry.org/bbs/models"

func envoyRunAction(envoyArgs []string) models.RunAction {
	envoyArgs = append(envoyArgs, "--id-creds",
		"C:\\etc\\cf-assets\\envoy_config\\sds-id-cert-and-key.yaml",
		"--c2c-creds",
		"C:\\etc\\cf-assets\\envoy_config\\sds-c2c-cert-and-key.yaml",
		"--id-validation",
		"C:\\etc\\cf-assets\\envoy_config\\sds-id-validation-context.yaml",
	)
	return models.RunAction{
		LogSource: "PROXY",
		Path:      "/etc/cf-assets/envoy/envoy",
		Args:      envoyArgs,
	}
}
