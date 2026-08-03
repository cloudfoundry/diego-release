package gardener

import (
	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/lager/v3"
)

//counterfeiter:generate . ContainerNetworkMetricsProvider

type ContainerNetworkMetricsProvider interface {
	Get(logger lager.Logger, handle string) (*garden.ContainerNetworkStat, error)
}

func NewNoopContainerNetworkMetricsProvider() *NoopContainerNetworkMetricsProvider {
	return &NoopContainerNetworkMetricsProvider{}
}

type NoopContainerNetworkMetricsProvider struct{}

func (n NoopContainerNetworkMetricsProvider) Get(_ lager.Logger, _ string) (*garden.ContainerNetworkStat, error) {
	return nil, nil
}
