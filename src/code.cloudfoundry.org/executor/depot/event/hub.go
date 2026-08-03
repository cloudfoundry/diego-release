package event

import (
	"code.cloudfoundry.org/eventhub"
	"code.cloudfoundry.org/executor"
)

const SUBSCRIBER_BUFFER = 1024

//go:generate counterfeiter -o fakes/fake_hub.go . Hub
type Hub interface {
	Emit(executor.Event)
	Subscribe() (executor.EventSource, error)
	Close() error
}

func NewHub() Hub {
	return &hub{
		rawHub: eventhub.NewNonBlocking(SUBSCRIBER_BUFFER),
	}
}

type hub struct {
	rawHub eventhub.Hub
}

func (hub *hub) Subscribe() (executor.EventSource, error) {
	rawSource, err := hub.rawHub.Subscribe()
	if err != nil {
		return nil, err
	}

	return executorSource{rawSource}, nil
}

func (hub *hub) Emit(ev executor.Event) {
	hub.rawHub.Emit(ev)
}

func (hub *hub) Close() error {
	return hub.rawHub.Close()
}

type executorSource struct {
	rawSource eventhub.Source
}

func (source executorSource) Next() (executor.Event, error) {
	ev, err := source.rawSource.Next()
	if err != nil {
		return nil, err
	}

	return ev.(executor.Event), nil
}

func (source executorSource) Close() error {
	return source.rawSource.Close()
}
