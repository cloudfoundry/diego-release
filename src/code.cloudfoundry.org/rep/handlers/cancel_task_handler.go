package handlers

import (
	"net/http"
	"time"

	"code.cloudfoundry.org/bbs/trace"
	"code.cloudfoundry.org/executor"
	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/locket/metrics/helpers"
)

type cancelTaskHandler struct {
	executorClient executor.Client
	metrics        helpers.RequestMetrics
}

func newCancelTaskHandler(executorClient executor.Client, requestMetrics helpers.RequestMetrics) *cancelTaskHandler {
	return &cancelTaskHandler{
		executorClient: executorClient,
		metrics:        requestMetrics,
	}
}

func (h *cancelTaskHandler) ServeHTTP(w http.ResponseWriter, r *http.Request, logger lager.Logger) {
	start := time.Now()
	requestType := "CancelTask"
	startMetrics(h.metrics, requestType)
	defer func() {
		h.metrics.DecrementRequestsInFlightCounter(requestType, 1)
		h.metrics.UpdateLatency(requestType, time.Since(start))
	}()

	taskGuid := r.FormValue(":task_guid")
	logger = logger.Session("cancel-task", lager.Data{"instance-guid": taskGuid}).WithTraceInfo(r)

	w.WriteHeader(http.StatusAccepted)

	go func() {
		logger.Info("deleting-container")

		err := h.executorClient.DeleteContainer(logger, trace.RequestIdFromRequest(r), taskGuid)
		switch err {
		case nil:
			logger.Info("succeeded-deleting-container")
			h.metrics.IncrementRequestsSucceededCounter(requestType, 1)
		case executor.ErrContainerNotFound:
			logger.Info("container-not-found")
			h.metrics.IncrementRequestsSucceededCounter(requestType, 1)
		default:
			logger.Error("failed-deleting-container", err)
			h.metrics.IncrementRequestsFailedCounter(requestType, 1)
		}
	}()
}
