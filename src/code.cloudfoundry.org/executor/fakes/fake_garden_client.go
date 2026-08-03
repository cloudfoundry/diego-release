package fakes

import (
	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/garden/client"
	"code.cloudfoundry.org/garden/client/connection/connectionfakes"
)

type FakeGardenClient struct {
	garden.Client

	Connection *connectionfakes.FakeConnection
}

func NewGardenClient() *FakeGardenClient {
	connection := new(connectionfakes.FakeConnection)

	return &FakeGardenClient{
		Connection: connection,

		Client: client.New(connection),
	}
}
