package eventhub

import "errors"

var ErrReadFromClosedSource = errors.New("read from closed source")
var ErrSendToClosedSource = errors.New("send to closed source")
var ErrSourceAlreadyClosed = errors.New("source already closed")
var ErrSlowConsumer = errors.New("slow consumer")

var ErrSubscribedToClosedHub = errors.New("subscribed to closed hub")
var ErrHubAlreadyClosed = errors.New("hub already closed")
