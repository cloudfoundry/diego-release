package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/locket/metrics/helpers"
	"code.cloudfoundry.org/rep/evacuation/evacuation_context"
)

type evacuationHandler struct {
	evacuatable evacuation_context.Evacuatable
	metrics     helpers.RequestMetrics
}

// Evacuation Handler serves a route that is called by the rep drain script
func newEvacuationHandler(evacuatable evacuation_context.Evacuatable, requestMetrics helpers.RequestMetrics) *evacuationHandler {
	return &evacuationHandler{
		evacuatable: evacuatable,
		metrics:     requestMetrics,
	}
}

func (h *evacuationHandler) ServeHTTP(w http.ResponseWriter, r *http.Request, logger lager.Logger) {
	h.evacuatable.Evacuate()

	jsonBytes, err := json.Marshal(map[string]string{"ping_path": "/ping"})
	if err != nil {
		//THIS SHOULD NEVER HAPPEN
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Length", strconv.Itoa(len(jsonBytes)))
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	// #nosec G104 - ignore errors when writing HTTP responses so we don't spam our logs during a DoS
	w.Write(jsonBytes)
}
