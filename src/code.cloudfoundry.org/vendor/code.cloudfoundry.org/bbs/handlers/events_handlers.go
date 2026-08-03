package handlers

import (
	"bytes"
	"fmt"
	"net/http"

	"code.cloudfoundry.org/bbs/events"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/lager/v3"
)

type EventController interface {
	Subscribe_r0(logger lager.Logger, w http.ResponseWriter, req *http.Request)
	Subscribe_r1(logger lager.Logger, w http.ResponseWriter, req *http.Request)
}

// Deprecated: use LRPInstanceEventHandler instead
type LRPGroupEventsHandler struct {
	desiredHub events.Hub
	actualHub  events.Hub
}

type TaskEventHandler struct {
	taskHub events.Hub
}

type LRPInstanceEventHandler struct {
	desiredHub     events.Hub
	lrpInstanceHub events.Hub
}

// Deprecated: use LRPInstanceEventHandler instead
func NewLRPGroupEventsHandler(desiredHub, actualHub events.Hub) *LRPGroupEventsHandler {
	return &LRPGroupEventsHandler{
		desiredHub: desiredHub,
		actualHub:  actualHub,
	}
}

func NewTaskEventHandler(taskHub events.Hub) *TaskEventHandler {
	return &TaskEventHandler{
		taskHub: taskHub,
	}
}

func NewLRPInstanceEventHandler(desiredHub, lrpInstanceHub events.Hub) *LRPInstanceEventHandler {
	return &LRPInstanceEventHandler{
		desiredHub:     desiredHub,
		lrpInstanceHub: lrpInstanceHub,
	}
}

func streamEventsToResponse(logger lager.Logger, w http.ResponseWriter, eventChan <-chan models.Event, errorChan <-chan error) {
	w.Header().Add("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Add("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Add("Connection", "keep-alive")

	w.WriteHeader(http.StatusOK)

	conn, rw, err := w.(http.Hijacker).Hijack()
	if err != nil {
		return
	}

	defer func() {
		fmt.Fprintf(conn, "0\r\n\r\n")
		err := conn.Close()
		if err != nil {
			logger.Error("failed-to-close-connection", err)
		}
	}()

	if err := rw.Flush(); err != nil {
		logger.Error("failed-to-flush", err)
		return
	}

	var event models.Event
	eventID := 0
	done := make(chan bool, 1)
	go func() {
		// #nosec G104 - ignore errors when reading hijacked HTTP requests so we don't spam our logs during a DoS
		rw.ReadFrom(conn)
		done <- true
	}()

	for {
		select {
		case event = <-eventChan:
		case err := <-errorChan:
			logger.Error("failed-to-get-next-event", err)
			return
		case <-done:
			logger.Debug("received-close-notify")
			return
		}

		sseEvent, err := events.NewEventFromModelEvent(eventID, event)
		if err != nil {
			logger.Error("failed-to-marshal-event", err)
			return
		}

		buf := new(bytes.Buffer)

		err = sseEvent.Write(buf)
		if err != nil {
			logger.Error("failed-to-write-event", err)
			return
		}

		fmt.Fprintf(conn, "%x;\r\n", buf.Len())
		fmt.Fprintf(conn, "%s\r\n", buf.String())

		eventID++
	}
}

type EventFetcher func() (models.Event, error)

func streamSource(eventChan chan<- models.Event, errorChan chan<- error, closeChan chan struct{}, fetchEvent EventFetcher) {
	for {
		event, err := fetchEvent()
		if err != nil {
			select {
			case errorChan <- err:
			case <-closeChan:
			}
			return
		}
		select {
		case eventChan <- event:
		case <-closeChan:
			return
		}
	}
}
