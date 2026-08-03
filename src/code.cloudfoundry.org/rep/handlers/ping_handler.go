package handlers

import (
	"net/http"

	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/locket/metrics/helpers"
)

type pingHandler struct {
	metrics helpers.RequestMetrics
}

// Ping Handler serves a route that is called by the rep ctl script
func newPingHandler(metrics helpers.RequestMetrics) *pingHandler {
	return &pingHandler{
		metrics: metrics,
	}
}

func (h *pingHandler) ServeHTTP(w http.ResponseWriter, r *http.Request, logger lager.Logger) {
	w.WriteHeader(http.StatusOK)
}
