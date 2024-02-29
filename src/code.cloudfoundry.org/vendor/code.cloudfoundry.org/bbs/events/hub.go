package events

import (
	"errors"
	"sync"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/lager/v3"
)

const MAX_PENDING_SUBSCRIBER_EVENTS = 1024

var ErrReadFromClosedSource = errors.New("read from closed source")
var ErrSendToClosedSource = errors.New("send to closed source")
var ErrSourceAlreadyClosed = errors.New("source already closed")
var ErrSlowConsumer = errors.New("slow consumer")

var ErrSubscribedToClosedHub = errors.New("subscribed to closed hub")
var ErrHubAlreadyClosed = errors.New("hub already closed")

//counterfeiter:generate -o eventfakes/fake_hub.go . Hub
type Hub interface {
	Subscribe() (EventSource, error)
	Emit(models.Event)
	Close() error

	RegisterCallback(func(count int))
	UnregisterCallback()
}

type hub struct {
	subscribers map[*hubSource]struct{}
	closed      bool
	lock        sync.Mutex
	logger      lager.Logger

	cb func(count int)
}

func NewHub(logger lager.Logger) Hub {
	return &hub{
		subscribers: make(map[*hubSource]struct{}),
		logger:      logger,
	}
}

func (hub *hub) RegisterCallback(cb func(int)) {
	hub.lock.Lock()
	hub.cb = cb
	size := len(hub.subscribers)
	hub.lock.Unlock()
	if cb != nil {
		cb(size)
	}
}

func (hub *hub) UnregisterCallback() {
	hub.lock.Lock()
	hub.cb = nil
	hub.lock.Unlock()
}

func (hub *hub) Subscribe() (EventSource, error) {
	hub.lock.Lock()

	if hub.closed {
		hub.lock.Unlock()

		return nil, ErrSubscribedToClosedHub
	}

	sub := newSource(MAX_PENDING_SUBSCRIBER_EVENTS, hub.subscriberClosed)
	hub.subscribers[sub] = struct{}{}
	cb := hub.cb
	size := len(hub.subscribers)
	hub.lock.Unlock()

	if cb != nil {
		cb(size)
	}
	return sub, nil
}

func (hub *hub) Emit(event models.Event) {
	hub.lock.Lock()
	size := len(hub.subscribers)

	for sub, _ := range hub.subscribers {
		err := sub.send(event)
		if err != nil {
			hub.logger.Error("got-error-sending-event", err)
			delete(hub.subscribers, sub)
		}
	}

	var cb func(int)
	if len(hub.subscribers) != size {
		cb = hub.cb
		size = len(hub.subscribers)
	}
	hub.lock.Unlock()

	if cb != nil {
		cb(size)
	}
}

func (hub *hub) Close() error {
	hub.lock.Lock()
	defer hub.lock.Unlock()

	if hub.closed {
		return ErrHubAlreadyClosed
	}

	hub.closeSubscribers()
	hub.closed = true
	if hub.cb != nil {
		hub.cb(0)
	}
	return nil
}

func (hub *hub) closeSubscribers() {
	for sub, _ := range hub.subscribers {
		_ = sub.Close()
	}
	hub.subscribers = nil
}

func (hub *hub) subscriberClosed(source *hubSource) {
	hub.lock.Lock()
	delete(hub.subscribers, source)
	cb := hub.cb
	count := len(hub.subscribers)
	hub.lock.Unlock()

	if cb != nil {
		cb(count)
	}
}

type hubSource struct {
	events        chan models.Event
	closeCallback func(*hubSource)
	closed        bool
	lock          sync.Mutex
}

func newSource(maxPendingEvents int, closeCallback func(*hubSource)) *hubSource {
	return &hubSource{
		events:        make(chan models.Event, maxPendingEvents),
		closeCallback: closeCallback,
	}
}

func (source *hubSource) Next() (models.Event, error) {
	event, ok := <-source.events
	if !ok {
		return nil, ErrReadFromClosedSource
	}
	return event, nil
}

func (source *hubSource) Close() error {
	source.lock.Lock()
	defer source.lock.Unlock()

	if source.closed {
		return ErrSourceAlreadyClosed
	}
	close(source.events)
	source.closed = true
	go source.closeCallback(source)
	return nil
}

func (source *hubSource) send(event models.Event) error {
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
