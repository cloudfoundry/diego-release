package containerstore

import (
	"net/http"

	"code.cloudfoundry.org/garden"
	gardenclient "code.cloudfoundry.org/garden/client"
	"code.cloudfoundry.org/garden/client/connection"
	"code.cloudfoundry.org/lager/v3"
)

//go:generate counterfeiter -o containerstorefakes/fake_garden_client_factory.go . GardenClientFactory
type GardenClientFactory interface {
	NewGardenClient(logger lager.Logger, traceID string) garden.Client
}

type gardenClientFactory struct {
	network string
	address string
}

func NewGardenClientFactory(network, address string) GardenClientFactory {
	return &gardenClientFactory{
		network: network,
		address: address,
	}
}

func (f *gardenClientFactory) NewGardenClient(logger lager.Logger, traceID string) garden.Client {
	hijacker := connection.NewHijackStreamerWithHeaders(f.network, f.address, http.Header{lager.RequestIdHeader: []string{traceID}})
	return gardenclient.New(connection.NewWithHijacker(hijacker, logger))
}
