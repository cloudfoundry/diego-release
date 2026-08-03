package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/locket/metrics/helpers"
	"code.cloudfoundry.org/rep/auctioncellrep"
)

type state struct {
	rep     auctioncellrep.AuctionCellClient
	metrics helpers.RequestMetrics
}

func newStateHandler(rep auctioncellrep.AuctionCellClient, metrics helpers.RequestMetrics) *state {
	return &state{rep: rep, metrics: metrics}
}

func (h *state) ServeHTTP(w http.ResponseWriter, r *http.Request, logger lager.Logger) {
	var deferErr error

	start := time.Now()
	requestType := "State"
	startMetrics(h.metrics, requestType)
	defer stopMetrics(h.metrics, requestType, start, &deferErr)

	logger = logger.Session("auction-fetch-state").WithTraceInfo(r)

	var state models.CellState
	var healthy bool
	state, healthy, deferErr = h.rep.State(logger)
	if deferErr != nil {
		w.WriteHeader(http.StatusInternalServerError)
		logger.Error("failed-to-fetch-state", deferErr)
		return
	}

	if !healthy {
		logger.Info("cell-not-healthy")
		w.WriteHeader(http.StatusServiceUnavailable)
	}

	// #nosec G104 - ignore errors when writing HTTP responses so we don't spam our logs during a DoS
	json.NewEncoder(w).Encode(state)
}
