package handlers

import (
	"net/http"
	"time"

	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/locket/metrics/helpers"
	"code.cloudfoundry.org/rep/auctioncellrep"
)

type reset struct {
	rep     auctioncellrep.AuctionCellClient
	metrics helpers.RequestMetrics
}

func newResetHandler(rep auctioncellrep.AuctionCellClient, metrics helpers.RequestMetrics) *reset {
	return &reset{rep: rep, metrics: metrics}
}

func (h *reset) ServeHTTP(w http.ResponseWriter, r *http.Request, logger lager.Logger) {
	var deferErr error

	start := time.Now()
	requestType := "Reset"
	startMetrics(h.metrics, requestType)
	defer stopMetrics(h.metrics, requestType, start, &deferErr)

	logger = logger.Session("sim-reset").WithTraceInfo(r)

	deferErr = h.rep.Reset()
	if deferErr != nil {
		w.WriteHeader(http.StatusInternalServerError)
		logger.Error("failed-to-reset", deferErr)
		return
	}
}
