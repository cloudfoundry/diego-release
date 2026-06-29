package handlers

import (
	"net/http"

	"code.cloudfoundry.org/bbs/format"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/lager/v3"
)

func (h *LRPGroupEventsHandler) commonSubscribe(logger lager.Logger, w http.ResponseWriter, req *http.Request, target format.Version) {
	logger = logger.Session("subscribe-r0").WithTraceInfo(req)

	request := &models.EventsByCellId{}
	err := parseRequest(logger, req, request)
	if err != nil {
		logger.Error("failed-parsing-request", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	logger.Info("subscribed-to-event-stream", lager.Data{"cell_id": request.CellId})

	desiredSource, err := h.desiredHub.Subscribe()
	if err != nil {
		logger.Error("failed-to-subscribe-to-desired-event-hub", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer desiredSource.Close()

	actualSource, err := h.actualHub.Subscribe()
	if err != nil {
		logger.Error("failed-to-subscribe-to-actual-event-hub", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer actualSource.Close()

	eventChan := make(chan models.Event)
	errorChan := make(chan error)
	closeChan := make(chan struct{})
	defer close(closeChan)

	actualEventsFetcher := actualSource.Next
	if request.CellId != "" {
		actualEventsFetcher = func() (models.Event, error) {
			for {
				event, err := actualSource.Next()
				if err != nil {
					return event, err
				}

				if matches, err := filterByCellID(request.CellId, event, err); err != nil {
					return nil, err
				} else if matches {
					return event, nil
				}
			}
		}
	}

	desiredEventsFetcher := func() (models.Event, error) {
		event, err := desiredSource.Next()
		if err != nil {
			return event, err
		}
		event = models.VersionDesiredLRPsTo(event, target)
		return event, err
	}

	go streamSource(eventChan, errorChan, closeChan, desiredEventsFetcher)
	go streamSource(eventChan, errorChan, closeChan, actualEventsFetcher)

	streamEventsToResponse(logger, w, eventChan, errorChan)
}

func (h *LRPGroupEventsHandler) Subscribe_r0(logger lager.Logger, w http.ResponseWriter, req *http.Request) {
	h.commonSubscribe(logger, w, req, format.V0)
}

func (h *LRPGroupEventsHandler) Subscribe_r1(logger lager.Logger, w http.ResponseWriter, req *http.Request) {
	h.commonSubscribe(logger, w, req, format.V3)
}

func (h *LRPInstanceEventHandler) commonSubscribe(logger lager.Logger, w http.ResponseWriter, req *http.Request, target format.Version) {
	logger = logger.Session("subscribe-r0").WithTraceInfo(req)

	request := &models.EventsByCellId{}
	err := parseRequest(logger, req, request)
	if err != nil {
		logger.Error("failed-parsing-request", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	logger.Info("subscribed-to-instance-event-stream", lager.Data{"cell_id": request.CellId})

	desiredSource, err := h.desiredHub.Subscribe()
	if err != nil {
		logger.Error("failed-to-subscribe-to-desired-event-hub", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer desiredSource.Close()

	lrpInstanceSource, err := h.lrpInstanceHub.Subscribe()
	if err != nil {
		logger.Error("failed-to-subscribe-to-actual-instance-event-hub", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer lrpInstanceSource.Close()

	eventChan := make(chan models.Event)
	errorChan := make(chan error)
	closeChan := make(chan struct{})
	defer close(closeChan)

	lrpInstanceEventFetcher := lrpInstanceSource.Next
	if request.CellId != "" {
		lrpInstanceEventFetcher = func() (models.Event, error) {
			for {
				event, err := lrpInstanceSource.Next()
				if err != nil {
					return event, err
				}

				if filterInstanceEventByCellID(request.CellId, event, err) {
					return event, nil
				}
			}
		}
	}

	desiredEventsFetcher := func() (models.Event, error) {
		event, err := desiredSource.Next()
		if err != nil {
			return event, err
		}
		event = models.VersionDesiredLRPsTo(event, target)
		return event, err
	}

	go streamSource(eventChan, errorChan, closeChan, desiredEventsFetcher)
	go streamSource(eventChan, errorChan, closeChan, lrpInstanceEventFetcher)

	streamEventsToResponse(logger, w, eventChan, errorChan)
}

func (h *LRPInstanceEventHandler) Subscribe_r0(logger lager.Logger, w http.ResponseWriter, req *http.Request) {
	h.commonSubscribe(logger, w, req, format.V0)
}

func (h *LRPInstanceEventHandler) Subscribe_r1(logger lager.Logger, w http.ResponseWriter, req *http.Request) {
	h.commonSubscribe(logger, w, req, format.V3)
}

func (h *TaskEventHandler) commonSubscribe(logger lager.Logger, w http.ResponseWriter, req *http.Request, target format.Version) {
	logger = logger.Session("tasks-subscribe-r0").WithTraceInfo(req)
	logger.Info("subscribed-to-tasks-event-stream")

	taskSource, err := h.taskHub.Subscribe()
	if err != nil {
		logger.Error("failed-to-subscribe-to-task-event-hub", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer taskSource.Close()

	eventChan := make(chan models.Event)
	errorChan := make(chan error)
	closeChan := make(chan struct{})
	defer close(closeChan)

	taskEventsFetcher := func() (models.Event, error) {
		event, err := taskSource.Next()
		if err != nil {
			return event, err
		}
		event = models.VersionTaskDefinitionsTo(event, target)
		return event, err
	}

	go streamSource(eventChan, errorChan, closeChan, taskEventsFetcher)

	streamEventsToResponse(logger, w, eventChan, errorChan)
}

func (h *TaskEventHandler) Subscribe_r0(logger lager.Logger, w http.ResponseWriter, req *http.Request) {
	h.commonSubscribe(logger, w, req, format.V0)
}

func (h *TaskEventHandler) Subscribe_r1(logger lager.Logger, w http.ResponseWriter, req *http.Request) {
	h.commonSubscribe(logger, w, req, format.V3)
}

func filterByCellID(cellID string, bbsEvent models.Event, err error) (bool, error) {
	switch x := bbsEvent.(type) {
	//lint:ignore SA1019 - need to support this event until the deprecation becomes deletion
	case *models.ActualLRPCreatedEvent:
		//lint:ignore SA1019 - calling deprecated model while unit testing deprecated method
		lrp, _, resolveError := x.ActualLrpGroup.Resolve()
		if resolveError != nil {
			return false, resolveError
		}

		if lrp.CellId != cellID {
			return false, nil
		}

	//lint:ignore SA1019 - need to support this event until the deprecation becomes deletion
	case *models.ActualLRPChangedEvent:
		//lint:ignore SA1019 - calling deprecated model while unit testing deprecated method
		beforeLRP, _, beforeResolveError := x.Before.Resolve()
		if beforeResolveError != nil {
			return false, beforeResolveError
		}
		//lint:ignore SA1019 - calling deprecated model while unit testing deprecated method
		afterLRP, _, afterResolveError := x.After.Resolve()
		if afterResolveError != nil {
			return false, afterResolveError
		}
		if afterLRP.CellId != cellID && beforeLRP.CellId != cellID {
			return false, nil
		}

	//lint:ignore SA1019 - need to support this event until the deprecation becomes deletion
	case *models.ActualLRPRemovedEvent:
		//lint:ignore SA1019 - calling deprecated model while unit testing deprecated method
		lrp, _, resolveError := x.ActualLrpGroup.Resolve()
		if resolveError != nil {
			return false, resolveError
		}
		if lrp.CellId != cellID {
			return false, nil
		}

	case *models.ActualLRPCrashedEvent:
		if x.ActualLRPInstanceKey.CellId != cellID {
			return false, nil
		}
	}

	return true, nil
}

func filterInstanceEventByCellID(cellID string, bbsEvent models.Event, err error) bool {
	switch x := bbsEvent.(type) {
	case *models.ActualLRPInstanceCreatedEvent:
		lrp := x.ActualLrp
		if lrp.CellId != cellID {
			return false
		}

	case *models.ActualLRPInstanceChangedEvent:
		if x.CellId != cellID {
			return false
		}

	case *models.ActualLRPInstanceRemovedEvent:
		lrp := x.ActualLrp
		if lrp.CellId != cellID {
			return false
		}

	case *models.ActualLRPCrashedEvent:
		if x.ActualLRPInstanceKey.CellId != cellID {
			return false
		}
	}

	return true
}
