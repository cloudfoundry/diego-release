package models

// Rep client interfaces moved here from code.cloudfoundry.org/rep
// to break the bbs <-> rep module cycle.

import (
	"net/http"
	"time"

	"code.cloudfoundry.org/lager/v3"
)

// RepClient is the interface bbs uses to communicate with rep agents.
// rep implements this interface. Defined here to break the bbs <-> rep cycle.
//
//go:generate counterfeiter -o fakes/fake_rep_client.go . RepClient
type RepClient interface {
	State(logger lager.Logger) (CellState, error)
	Perform(logger lager.Logger, work Work) (Work, error)
	UpdateLRPInstance(logger lager.Logger, update LRPUpdate) error
	StopLRPInstance(logger lager.Logger, key ActualLRPKey, instanceKey ActualLRPInstanceKey) error
	CancelTask(logger lager.Logger, taskGuid string) error
	SetStateClient(stateClient *http.Client)
	StateClientTimeout() time.Duration
}

// RepClientFactory creates RepClient instances.
//
//go:generate counterfeiter -o fakes/fake_rep_client_factory.go . RepClientFactory
type RepClientFactory interface {
	CreateClient(address, url, traceID string) (RepClient, error)
}

// RepSimClient extends RepClient with simulation capabilities.
type RepSimClient interface {
	RepClient
	Reset() error
}
