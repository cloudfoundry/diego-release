package event

type Event struct {
	ContainerID string
	Message     string
}

func NewOOMEvent(containerID string) Event {
	return Event{ContainerID: containerID, Message: "Out of memory"}
}
