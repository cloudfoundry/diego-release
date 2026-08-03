package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/executor"
	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/locket/metrics/helpers"
	"code.cloudfoundry.org/rep"
)

// This is public for testing purpose
type UpdateLRPInstanceHandler struct {
	client  executor.Client
	metrics helpers.RequestMetrics
}

// This is public for testing purpose
func NewUpdateLRPInstanceHandler(client executor.Client, metrics helpers.RequestMetrics) *UpdateLRPInstanceHandler {
	return &UpdateLRPInstanceHandler{
		client:  client,
		metrics: metrics,
	}
}

func (h *UpdateLRPInstanceHandler) ServeHTTP(w http.ResponseWriter, r *http.Request, logger lager.Logger) {
	var deferErr error

	start := time.Now()
	requestType := "UpdateLRPInstance"
	startMetrics(h.metrics, requestType)
	defer stopMetrics(h.metrics, requestType, start, &deferErr)

	processGuid := r.FormValue(":process_guid")
	instanceGuid := r.FormValue(":instance_guid")

	logger = logger.Session("handling-update-lrp-instance", lager.Data{
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

	var lrpUpdate models.LRPUpdate
	deferErr = json.NewDecoder(r.Body).Decode(&lrpUpdate)
	if deferErr != nil {
		w.WriteHeader(http.StatusBadRequest)
		logger.Error("failed-to-unmarshal", deferErr)
		return
	}

	updateReq := executor.NewUpdateRequest(
		rep.LRPContainerGuid(processGuid, instanceGuid),
		lrpUpdate.InternalRoutes,
		lrpUpdate.MetricTags,
	)

	deferErr = h.client.UpdateContainer(logger, &updateReq)
	if deferErr != nil {
		w.WriteHeader(http.StatusInternalServerError)
		logger.Error("failed-to-update-container", deferErr)
		return
	}

	w.WriteHeader(http.StatusAccepted)
}
