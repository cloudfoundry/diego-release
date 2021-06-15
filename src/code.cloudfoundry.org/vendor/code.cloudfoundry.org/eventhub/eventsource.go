package eventhub

import "sync"

type Source interface {
	Next() (Event, error)
	Close() error
	send(Event) error
}

type source struct {
	events chan Event
	closed bool
	lock   sync.Mutex
}

func newSource(maxPendingEvents int) Source {
	return &source{
		events: make(chan Event, maxPendingEvents),
	}
}

func (source *source) Next() (Event, error) {
	event, ok := <-source.events
	if !ok {
		return nil, ErrReadFromClosedSource
	}
	return event, nil
}

func (source *source) Close() error {
	source.lock.Lock()
	defer source.lock.Unlock()

	if source.closed {
		return ErrSourceAlreadyClosed
	}

	close(source.events)
	source.closed = true

	return nil
}

func (source *source) send(event Event) error {
	source.lock.Lock()

	if source.closed {
		source.lock.Unlock()
		return ErrSendToClosedSource
	}

	select {
	case source.events <- event:
		source.lock.Unlock()
		return nil

	default:
		source.lock.Unlock()
		err := source.Close()
		if err != nil {
			return err
		}

		return ErrSlowConsumer
	}
}
