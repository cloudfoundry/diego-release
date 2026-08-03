package handlers

import (
	"net/http"
	"strconv"

	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/routing-api/db"
	"code.cloudfoundry.org/routing-api/metrics"
	"code.cloudfoundry.org/routing-api/uaaclient"
	"github.com/vito/go-sse/sse"
)

type EventStreamHandler struct {
	uaaClient uaaclient.TokenValidator
	db        db.DB
	logger    lager.Logger
	stats     metrics.PartialStatsdClient
}

func NewEventStreamHandler(uaaClient uaaclient.TokenValidator, database db.DB, logger lager.Logger, stats metrics.PartialStatsdClient) *EventStreamHandler {
	return &EventStreamHandler{
		uaaClient: uaaClient,
		db:        database,
		logger:    logger,
		stats:     stats,
	}
}

func (h *EventStreamHandler) EventStream(w http.ResponseWriter, req *http.Request) {
	err := h.stats.GaugeDelta(metrics.TotalHttpSubscriptions, 1, 1.0)
	if err != nil {
		h.logger.Info("error-sending-metrics", lager.Data{"error": err, "metric": metrics.TotalHttpSubscriptions})
	}
	defer func() {
		err = h.stats.GaugeDelta(metrics.TotalHttpSubscriptions, -1, 1.0)
		if err != nil {
			h.logger.Info("error-sending-metrics", lager.Data{"error": err, "metric": metrics.TotalHttpSubscriptions})
		}
	}()
	log := h.logger.Session("event-stream-handler")
	h.handleEventStream(log, db.HTTP_WATCH, w, req)
}

func (h *EventStreamHandler) TcpEventStream(w http.ResponseWriter, req *http.Request) {
	err := h.stats.GaugeDelta(metrics.TotalTcpSubscriptions, 1, 1.0)
	if err != nil {
		h.logger.Info("error-sending-metrics", lager.Data{"error": err, "metric": metrics.TotalTcpSubscriptions})
	}
	defer func() {
		err = h.stats.GaugeDelta(metrics.TotalTcpSubscriptions, -1, 1.0)
		if err != nil {
			h.logger.Info("error-sending-metrics", lager.Data{"error": err, "metric": metrics.TotalTcpSubscriptions})
		}
	}()
	log := h.logger.Session("tcp-event-stream-handler")
	h.handleEventStream(log, db.TCP_WATCH, w, req)
}

func (h *EventStreamHandler) handleEventStream(log lager.Logger, filterKey string,
	w http.ResponseWriter, req *http.Request) {

	err := h.uaaClient.ValidateToken(req.Header.Get("Authorization"), RoutingRoutesReadScope)
	if err != nil {
		handleUnauthorizedError(w, err, log)
		return
	}
	flusher := w.(http.Flusher)
	reqCtx := req.Context()
	closeNotifier := reqCtx.Done()
	resultChan, errChan, cancelFunc := h.db.WatchChanges(filterKey)

	w.Header().Add("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Add("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Add("Connection", "keep-alive")

	w.WriteHeader(http.StatusOK)

	flusher.Flush()

	eventID := 0
	for {
		select {
		case event := <-resultChan:
			eventType := event.Type
			if eventType == db.InvalidEvent {
				h.logger.Info("invalid-event", lager.Data{"event": event})
				return
			}

			err = sse.Event{
				ID:   strconv.Itoa(eventID),
				Name: eventType.String(),
				Data: []byte(event.Value),
			}.Write(w)

			if err != nil {
				break
			}

			flusher.Flush()

			eventID++
		case err := <-errChan:
			log.Error("watch-error", err)
			return
		case <-closeNotifier:
			log.Info("connection-closed")
			cancelFunc()
			return
		}
	}
}
