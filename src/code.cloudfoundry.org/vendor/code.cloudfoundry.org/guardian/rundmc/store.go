package rundmc

import (
	"strings"
	"sync"
)

//go:generate counterfeiter . Properties

type Properties interface {
	Set(handle string, key string, value string)
	Get(handle string, key string) (string, bool)
}

type events struct {
	props Properties
	mu    sync.Mutex
}

func NewEventStore(props Properties) *events {
	return &events{
		props: props,
	}
}

func (e *events) OnEvent(handle, event string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	events := append(e.Events(handle), event)
	e.props.Set(handle, "rundmc.events", strings.Join(events, ","))
	return nil
}

func (e *events) Events(handle string) []string {
	if value, ok := e.props.Get(handle, "rundmc.events"); ok {
		return strings.Split(value, ",")
	}

	return nil
}

type states struct {
	props Properties
}

func NewStateStore(props Properties) *states {
	return &states{
		props: props,
	}
}

func (s *states) StoreStopped(handle string) {
	s.props.Set(handle, "rundmc.state", "stopped")
}

func (s *states) IsStopped(handle string) bool {
	value, ok := s.props.Get(handle, "rundmc.state")
	if !ok {
		return false
	}

	return value == "stopped"
}
