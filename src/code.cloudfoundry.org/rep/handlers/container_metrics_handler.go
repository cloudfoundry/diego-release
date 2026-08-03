package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/locket/metrics/helpers"
	"code.cloudfoundry.org/rep"
)

//go:generate counterfeiter . MetricCollector
type MetricCollector interface {
	Metrics(logger lager.Logger) (*rep.ContainerMetricsCollection, error)
}

type containerMetrics struct {
	rep     MetricCollector
	metrics helpers.RequestMetrics
}

func newContainerMetricsHandler(rep MetricCollector, metrics helpers.RequestMetrics) *containerMetrics {
	return &containerMetrics{rep: rep, metrics: metrics}
}

func (h *containerMetrics) ServeHTTP(w http.ResponseWriter, r *http.Request, logger lager.Logger) {
	var deferErr error

	start := time.Now()
	requestType := "ContainerMetrics"
	startMetrics(h.metrics, requestType)
	defer stopMetrics(h.metrics, requestType, start, &deferErr)

	logger = logger.Session("container-metrics-handler").WithTraceInfo(r)

	var metricsCollector *rep.ContainerMetricsCollection

	metricsCollector, deferErr = h.rep.Metrics(logger)
	if deferErr != nil {
		w.WriteHeader(http.StatusInternalServerError)
		logger.Error("failed-to-fetch-container-metrics", deferErr)
		return
	}

	// #nosec G104 - ignore errors when writing HTTP responses so we don't spam our logs during a DoS
	json.NewEncoder(w).Encode(metricsCollector)
}
