package handlers

import (
	"time"

	"code.cloudfoundry.org/locket/metrics/helpers"
)

func startMetrics(metrics helpers.RequestMetrics, requestType string) {
	metrics.IncrementRequestsStartedCounter(requestType, 1)
	metrics.IncrementRequestsInFlightCounter(requestType, 1)
}

func stopMetrics(metrics helpers.RequestMetrics, requestType string, start time.Time, deferErr *error) {
	metrics.DecrementRequestsInFlightCounter(requestType, 1)
	metrics.UpdateLatency(requestType, time.Since(start))

	if deferErr == nil || *deferErr == nil {
		metrics.IncrementRequestsSucceededCounter(requestType, 1)
	} else {
		metrics.IncrementRequestsFailedCounter(requestType, 1)
	}
}
