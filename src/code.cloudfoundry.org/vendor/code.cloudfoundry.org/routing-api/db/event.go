package db

import "encoding/json"

type Event struct {
	Type  EventType
	Value string
}

type EventType int

const (
	InvalidEvent = EventType(iota)
	CreateEvent
	DeleteEvent
	ExpireEvent
	UpdateEvent
)

func (e EventType) String() string {
	switch e {
	case CreateEvent:
		return "Upsert"
	case UpdateEvent:
		return "Upsert"
	case DeleteEvent, ExpireEvent:
		return "Delete"
	default:
		return "Invalid"
	}
}

func NewEventFromInterface(eventType EventType, obj interface{}) (Event, error) {
	data, err := json.Marshal(obj)
	if err != nil {
		return Event{}, err
	}

	return Event{
		Type:  eventType,
		Value: string(data),
	}, nil
}
