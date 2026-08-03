package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/bbs/trace"
	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/locket/metrics/helpers"
	"code.cloudfoundry.org/rep/auctioncellrep"
)

type perform struct {
	rep     auctioncellrep.AuctionCellClient
	metrics helpers.RequestMetrics
}

func newPerformHandler(rep auctioncellrep.AuctionCellClient, metrics helpers.RequestMetrics) *perform {
	return &perform{rep: rep, metrics: metrics}
}

func (h *perform) ServeHTTP(w http.ResponseWriter, r *http.Request, logger lager.Logger) {
	var deferErr error

	start := time.Now()
	requestType := "Perform"
	startMetrics(h.metrics, requestType)
	defer stopMetrics(h.metrics, requestType, start, &deferErr)

	logger = logger.Session("auction-perform-work").WithTraceInfo(r)
	var work models.Work
	deferErr = json.NewDecoder(r.Body).Decode(&work)
	if deferErr != nil {
		w.WriteHeader(http.StatusBadRequest)
		logger.Error("failed-to-unmarshal", deferErr)
		return
	}

	var failedWork models.Work
	failedWork, deferErr = h.rep.Perform(logger, trace.RequestIdFromRequest(r), work)
	if deferErr != nil {
		w.WriteHeader(http.StatusInternalServerError)
		logger.Error("failed-to-perform-work", deferErr)
		return
	}

	// #nosec G104 - ignore errors when writing HTTP responses so we don't spam our logs during a DoS
	json.NewEncoder(w).Encode(failedWork)
}
