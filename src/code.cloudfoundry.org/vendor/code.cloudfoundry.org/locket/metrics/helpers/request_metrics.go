package helpers

import (
	"fmt"
	"os"
	"sync/atomic"
	"time"

	"code.cloudfoundry.org/clock"
	loggingclient "code.cloudfoundry.org/diego-logging-client"
	loggregator "code.cloudfoundry.org/go-loggregator/v9"
	"code.cloudfoundry.org/lager/v3"
)

const (
	requestsStartedMetric     = "RequestsStarted"
	requestsSucceededMetric   = "RequestsSucceeded"
	requestsFailedMetric      = "RequestsFailed"
	requestsInFlightMetric    = "RequestsInFlight"
	requestsCancelledMetric   = "RequestsCancelled"
	requestLatencyMaxDuration = "RequestLatencyMax"
)

type requestMetric struct {
	RequestsStarted   uint64
	RequestsSucceeded uint64
	RequestsFailed    uint64
	RequestsInFlight  uint64
	RequestsCancelled uint64
	RequestLatencyMax int64
}

type RequestMetrics interface {
	IncrementRequestsStartedCounter(requestType string, delta int)
	IncrementRequestsSucceededCounter(requestType string, delta int)
	IncrementRequestsFailedCounter(requestType string, delta int)
	IncrementRequestsInFlightCounter(requestType string, delta int)
	DecrementRequestsInFlightCounter(requestType string, delta int)
	IncrementRequestsCancelledCounter(requestType string, delta int)
	UpdateLatency(requestType string, dur time.Duration)
}

type RequestMetricsNotifier struct {
	logger          lager.Logger
	clock           clock.Clock
	metronClient    loggingclient.IngressClient
	metricsInterval time.Duration
	metrics         map[string]*requestMetric
}

func NewRequestMetricsNotifier(logger lager.Logger, clock clock.Clock, metronClient loggingclient.IngressClient, metricsInterval time.Duration, requestTypes []string) *RequestMetricsNotifier {
	metrics := map[string]*requestMetric{}
	for _, requestType := range requestTypes {
		metrics[requestType] = &requestMetric{}
	}

	return &RequestMetricsNotifier{
		logger:          logger,
		clock:           clock,
		metronClient:    metronClient,
		metricsInterval: metricsInterval,
		metrics:         metrics,
	}
}

func (notifier *RequestMetricsNotifier) requestMetricsForType(requestType string) *requestMetric {
	metric, exist := notifier.metrics[requestType]
	if !exist {
		panic(fmt.Sprintf("unknown request type %s", requestType))
	}
	return metric
}

func (notifier *RequestMetricsNotifier) IncrementRequestsStartedCounter(requestType string, delta int) {
	atomic.AddUint64(&notifier.requestMetricsForType(requestType).RequestsStarted, uint64(delta))
}

func (notifier *RequestMetricsNotifier) IncrementRequestsSucceededCounter(requestType string, delta int) {
	atomic.AddUint64(&notifier.requestMetricsForType(requestType).RequestsSucceeded, uint64(delta))
}

func (notifier *RequestMetricsNotifier) IncrementRequestsFailedCounter(requestType string, delta int) {
	atomic.AddUint64(&notifier.requestMetricsForType(requestType).RequestsFailed, uint64(delta))
}

func (notifier *RequestMetricsNotifier) IncrementRequestsInFlightCounter(requestType string, delta int) {
	atomic.AddUint64(&notifier.requestMetricsForType(requestType).RequestsInFlight, uint64(delta))
}

func (notifier *RequestMetricsNotifier) DecrementRequestsInFlightCounter(requestType string, delta int) {
	atomic.AddUint64(&notifier.requestMetricsForType(requestType).RequestsInFlight, uint64(-delta))
}

func (notifier *RequestMetricsNotifier) IncrementRequestsCancelledCounter(requestType string, delta int) {
	atomic.AddUint64(&notifier.requestMetricsForType(requestType).RequestsCancelled, uint64(delta))
}

func (notifier *RequestMetricsNotifier) UpdateLatency(requestType string, dur time.Duration) {
	addr := &notifier.requestMetricsForType(requestType).RequestLatencyMax
	for {
		val := atomic.LoadInt64(addr)
		newval := int64(dur)
		if newval < val {
			return
		}

		if atomic.CompareAndSwapInt64(addr, val, newval) {
			return
		}
	}
}

func (notifier *RequestMetricsNotifier) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	logger := notifier.logger.Session("request-metrics-notifier")
	logger.Info("starting", lager.Data{"interval": notifier.metricsInterval})
	defer logger.Info("completed")
	close(ready)

	tick := notifier.clock.NewTicker(notifier.metricsInterval)

	for {
		select {
		case <-signals:
			return nil

		case <-tick.C():
			logger.Debug("emitting-metrics")

			var err error
			for requestType := range notifier.metrics {
				metric := notifier.requestMetricsForType(requestType)
				opt := loggregator.WithEnvelopeTag("request-type", requestType)

				value := int(atomic.LoadUint64(&metric.RequestsStarted))
				err = notifier.metronClient.SendMetric(requestsStartedMetric, value, opt)

				if err != nil {
					logger.Error("failed-to-emit-requests-started-metric", err)
				}
				err = notifier.metronClient.SendMetric(requestsSucceededMetric, int(atomic.LoadUint64(&metric.RequestsSucceeded)), opt)
				if err != nil {
					logger.Error("failed-to-emit-requests-succeeded-metric", err)
				}
				err = notifier.metronClient.SendMetric(requestsFailedMetric, int(atomic.LoadUint64(&metric.RequestsFailed)), opt)
				if err != nil {
					logger.Error("failed-to-emit-requests-failed-metric", err)
				}
				err = notifier.metronClient.SendMetric(requestsInFlightMetric, int(atomic.LoadUint64(&metric.RequestsInFlight)), opt)
				if err != nil {
					logger.Error("failed-to-emit-requests-in-flight-metric", err)
				}
				err = notifier.metronClient.SendMetric(requestsCancelledMetric, int(atomic.LoadUint64(&metric.RequestsCancelled)), opt)
				if err != nil {
					logger.Error("failed-to-emit-requests-cancelled-metric", err)
				}
				latency := atomic.SwapInt64(&metric.RequestLatencyMax, 0)
				err = notifier.metronClient.SendDuration(requestLatencyMaxDuration, time.Duration(latency), opt)
				if err != nil {
					logger.Error("failed-to-emit-requests-latency-max-metric", err)
				}
			}
		}
	}
}
