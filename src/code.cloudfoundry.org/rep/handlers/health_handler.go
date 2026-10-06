package handlers

import (
	"net/http"

	"code.cloudfoundry.org/executor"
	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/rep"
	"github.com/tedsuo/rata"
)

type healthHandler struct {
	executorClient executor.Client
}

// NewHealth serves rep.RoutesHealth; /health is 503 while the executor reports the cell unhealthy.
func NewHealth(executorClient executor.Client, logger lager.Logger) rata.Handlers {
	h := &healthHandler{executorClient: executorClient}
	return rata.Handlers{
		rep.HealthRoute: logWrap(h.ServeHTTP, logger),
	}
}

func (h *healthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request, logger lager.Logger) {
	if !h.executorClient.Healthy(logger) {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusOK)
}
