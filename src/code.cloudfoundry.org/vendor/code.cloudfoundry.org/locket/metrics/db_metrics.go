package metrics

import (
	"os"
	"time"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/diego-db-helpers/sqldb/helpers"
	"code.cloudfoundry.org/diego-db-helpers/sqldb/helpers/monitor"
	loggingclient "code.cloudfoundry.org/diego-logging-client"
	"code.cloudfoundry.org/lager/v3"
	"github.com/tedsuo/ifrit"
)

const (
	dbOpenConnectionsMetric  = "DBOpenConnections"
	dbWaitDurationMetric     = "DBWaitDuration"
	dbWaitCountMetric        = "DBWaitCount"
	dbQueriesTotalMetric     = "DBQueriesTotal"
	dbQueriesSucceededMetric = "DBQueriesSucceeded"
	dbQueriesFailedMetric    = "DBQueriesFailed"
	dbQueriesInFlightMetric  = "DBQueriesInFlight"
	dbQueryDurationMaxMetric = "DBQueryDurationMax"
)

type dbMetricsNotifier struct {
	logger          lager.Logger
	ticker          clock.Clock
	metricsInterval time.Duration
	lockDB          helpers.QueryableDB
	metronClient    loggingclient.IngressClient
	queryMonitor    monitor.Monitor
}

func NewDBMetricsNotifier(logger lager.Logger, ticker clock.Clock, metronClient loggingclient.IngressClient, metricsInterval time.Duration, lockDB helpers.QueryableDB, queryMonitor monitor.Monitor) ifrit.Runner {
	return &dbMetricsNotifier{
		logger:          logger,
		ticker:          ticker,
		metricsInterval: metricsInterval,
		lockDB:          lockDB,
		metronClient:    metronClient,
		queryMonitor:    queryMonitor,
	}
}

func (notifier *dbMetricsNotifier) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	logger := notifier.logger.Session("metrics-notifier")
	logger.Info("starting", lager.Data{"interval": notifier.metricsInterval})
	defer logger.Info("completed")
	close(ready)

	tick := notifier.ticker.NewTicker(notifier.metricsInterval)
	for {
		select {
		case <-signals:
			return nil
		case <-tick.C():
			logger.Debug("emitting-metrics")

			openConnections := notifier.lockDB.OpenConnections()
			waitDuration := notifier.lockDB.WaitDuration()
			waitCount := notifier.lockDB.WaitCount()
			queriesTotal := notifier.queryMonitor.Total()
			queriesSucceeded := notifier.queryMonitor.Succeeded()
			queriesFailed := notifier.queryMonitor.Failed()
			queriesInFlightMax := notifier.queryMonitor.ReadAndResetInFlightMax()
			queryDurationMax := notifier.queryMonitor.ReadAndResetDurationMax()

			err := notifier.metronClient.SendMetric(dbOpenConnectionsMetric, openConnections)
			if err != nil {
				logger.Error("failed-sending-db-open-connections-count", err)
			}

			err = notifier.metronClient.SendDuration(dbWaitDurationMetric, waitDuration)
			if err != nil {
				logger.Error("failed-sending-db-wait-duration", err)
			}

			err = notifier.metronClient.SendMetric(dbWaitCountMetric, int(waitCount))
			if err != nil {
				logger.Error("failed-sending-db-wait-count", err)
			}

			logger.Debug("sending-queries-total-metric", lager.Data{"value": queriesTotal})
			err = notifier.metronClient.SendMetric(dbQueriesTotalMetric, int(queriesTotal))
			if err != nil {
				logger.Error("failed-sending-db-queries-total-count", err)
			}

			err = notifier.metronClient.SendMetric(dbQueriesSucceededMetric, int(queriesSucceeded))
			if err != nil {
				logger.Error("failed-sending-db-queries-succeeded-count", err)
			}

			err = notifier.metronClient.SendMetric(dbQueriesFailedMetric, int(queriesFailed))
			if err != nil {
				logger.Error("failed-sending-db-queries-failed-count", err)
			}

			err = notifier.metronClient.SendMetric(dbQueriesInFlightMetric, int(queriesInFlightMax))
			if err != nil {
				logger.Error("failed-sending-db-queries-in-flight-count", err)
			}

			err = notifier.metronClient.SendDuration(dbQueryDurationMaxMetric, queryDurationMax)
			if err != nil {
				logger.Error("failed-sending-db-query-duration-max", err)
			}

			logger.Debug("emitted-metrics")
		}
	}
}
