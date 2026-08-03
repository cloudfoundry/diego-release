package handlers

import (
	"errors"
	"net/http"
	"time"

	"code.cloudfoundry.org/bbs/trace"
	"code.cloudfoundry.org/executor"
	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/locket/metrics/helpers"
	"code.cloudfoundry.org/rep"
)

// This is public for testing purpose
type StopLRPInstanceHandler struct {
	client  executor.Client
	metrics helpers.RequestMetrics
}

// This is public for testing purpose
func NewStopLRPInstanceHandler(client executor.Client, metrics helpers.RequestMetrics) *StopLRPInstanceHandler {
	return &StopLRPInstanceHandler{
		client:  client,
		metrics: metrics,
	}
}

func (h *StopLRPInstanceHandler) ServeHTTP(w http.ResponseWriter, r *http.Request, logger lager.Logger) {
	var deferErr error

	start := time.Now()
	requestType := "StopLRPInstance"
	startMetrics(h.metrics, requestType)
	defer stopMetrics(h.metrics, requestType, start, &deferErr)

	processGuid := r.FormValue(":process_guid")
	instanceGuid := r.FormValue(":instance_guid")

	logger = logger.Session("handling-stop-lrp-instance", lager.Data{
		"process-guid":  processGuid,
		"instance-guid": instanceGuid,
	}).WithTraceInfo(r)

	if processGuid == "" {
		deferErr = errors.New("process_guid missing from request")
		logger.Error("missing-process-guid", deferErr)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if instanceGuid == "" {
		deferErr = errors.New("instance_guid missing from request")
		logger.Error("missing-instance-guid", deferErr)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	traceID := trace.RequestIdFromRequest(r)
	deferErr = h.client.StopContainer(logger, traceID, rep.LRPContainerGuid(processGuid, instanceGuid))
	if deferErr != nil {
		w.WriteHeader(http.StatusInternalServerError)
		logger.Error("failed-to-stop-container", deferErr)
		return
	}

	w.WriteHeader(http.StatusAccepted)
}
