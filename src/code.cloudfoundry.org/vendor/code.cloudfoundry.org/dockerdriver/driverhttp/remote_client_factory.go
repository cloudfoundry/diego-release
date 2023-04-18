package driverhttp

import (
	"code.cloudfoundry.org/dockerdriver"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate
//counterfeiter:generate -o ../dockerdriverfakes/fake_remote_client_factory.go . RemoteClientFactory
type RemoteClientFactory interface {
	NewRemoteClient(url string, tls *dockerdriver.TLSConfig) (dockerdriver.Driver, error)
}

func NewRemoteClientFactory() RemoteClientFactory {
	return &remoteClientFactory{}
}

type remoteClientFactory struct{}

func (_ *remoteClientFactory) NewRemoteClient(url string, tls *dockerdriver.TLSConfig) (dockerdriver.Driver, error) {
	return NewRemoteClient(url, tls)
}
