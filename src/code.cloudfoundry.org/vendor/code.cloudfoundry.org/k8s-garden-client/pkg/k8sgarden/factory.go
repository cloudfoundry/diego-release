package k8sgarden

import (
	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/lager/v3"
)

type Factory struct {
	client garden.Client
}

func NewFactory(client garden.Client) *Factory {
	return &Factory{
		client: client,
	}
}

func (f *Factory) NewGardenClient(logger lager.Logger, traceID string) garden.Client {
	return f.client
}
