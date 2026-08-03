package containermetrics

import (
	"os"
	"time"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/executor"
	"code.cloudfoundry.org/lager/v3"
)

type ReportersRunner struct {
	logger lager.Logger

	interval         time.Duration
	clock            clock.Clock
	executorClient   executor.Client
	metricsReporters []MetricsReporter
}

type MetricsReporter interface {
	Report(logger lager.Logger, containers []executor.Container, metrics map[string]executor.Metrics, timeStamp time.Time) error
}

func NewReportersRunner(logger lager.Logger,
	interval time.Duration,
	clock clock.Clock,
	executorClient executor.Client,
	metricsReporters ...MetricsReporter,
) *ReportersRunner {
	return &ReportersRunner{
		logger: logger,

		interval:         interval,
		clock:            clock,
		executorClient:   executorClient,
		metricsReporters: metricsReporters,
	}
}

func (reporterRunner *ReportersRunner) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	logger := reporterRunner.logger.Session("container-metrics-reporterRunner")

	ticker := reporterRunner.clock.NewTicker(reporterRunner.interval)
	defer ticker.Stop()

	close(ready)

	for {
		select {
		case signal := <-signals:
			logger.Info("signalled", lager.Data{"signal": signal.String()})
			return nil

		case now := <-ticker.C():
			reporterRunner.fetchMetrics(logger, now, reporterRunner.metricsReporters...)
		}
	}
}

func (reporterRunner *ReportersRunner) fetchMetrics(logger lager.Logger, timeStamp time.Time, reporters ...MetricsReporter) {
	logger = logger.Session("tick")

	startTime := reporterRunner.clock.Now()

	logger.Debug("started")
	defer func() {
		logger.Debug("done", lager.Data{
			"took": reporterRunner.clock.Now().Sub(startTime).String(),
		})
	}()

	metricsCache, err := reporterRunner.executorClient.GetBulkMetrics(logger)
	if err != nil {
		logger.Error("failed-to-get-all-metrics", err)
		return
	}

	logger.Debug("emitting", lager.Data{
		"total-containers": len(metricsCache),
		"get-metrics-took": reporterRunner.clock.Now().Sub(startTime).String(),
	})

	containers, err := reporterRunner.executorClient.ListContainers(logger)
	if err != nil {
		logger.Error("failed-to-fetch-containers", err)
		return
	}

	for _, reporter := range reporters {
		err := reporter.Report(logger, containers, metricsCache, timeStamp)
		if err != nil {
			logger.Error("failed-to-report-metric", err, lager.Data{"containers": containers, "metrics": metricsCache, "reporter": reporter})
		}
	}
}
