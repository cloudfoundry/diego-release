//go:build !linux

package initializer

import (
	"errors"

	"code.cloudfoundry.org/executor/depot/containerstore"
	"code.cloudfoundry.org/executor/initializer/configuration"
	GardenClient "code.cloudfoundry.org/garden/client"
	"code.cloudfoundry.org/lager/v3"
)

// The Kubernetes garden client depends on containerd/kubelet/linux_command_runner,
// which are linux-only. Non-linux cells never set UseKubernetesGardenClient.
func newKubernetesGardenClient(logger lager.Logger, config ExecutorConfig, sidecarRootFSPath string) (GardenClient.Client, containerstore.GardenClientFactory, configuration.RootFSSizer, error) {
	return nil, nil, nil, errors.New("kubernetes garden client is not supported on this platform")
}
