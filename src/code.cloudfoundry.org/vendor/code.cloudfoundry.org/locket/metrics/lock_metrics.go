package metrics

import (
	"context"
	"os"
	"time"

	"code.cloudfoundry.org/clock"
	loggingclient "code.cloudfoundry.org/diego-logging-client"
	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/locket/db"
	"code.cloudfoundry.org/locket/models"
	"github.com/tedsuo/ifrit"
)

const (
	activeLocksMetric     = "ActiveLocks"
	activePresencesMetric = "ActivePresences"
)

type lockMetricsNotifier struct {
	logger          lager.Logger
	ticker          clock.Clock
	metricsInterval time.Duration
	lockDB          db.LockDB
	metronClient    loggingclient.IngressClient
}

func NewLockMetricsNotifier(logger lager.Logger, ticker clock.Clock, metronClient loggingclient.IngressClient, metricsInterval time.Duration, lockDB db.LockDB) ifrit.Runner {
	return &lockMetricsNotifier{
		logger:          logger,
		ticker:          ticker,
		metricsInterval: metricsInterval,
		lockDB:          lockDB,
		metronClient:    metronClient,
	}
}

func (notifier *lockMetricsNotifier) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	logger := notifier.logger.Session("lock-metrics-notifier")
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

			locks, err := notifier.lockDB.Count(context.Background(), logger, models.LockType)
			if err != nil {
				logger.Error("failed-to-retrieve-lock-count", err)
			} else {
				err = notifier.metronClient.SendMetric(activeLocksMetric, locks)
				if err != nil {
					logger.Error("failed-sending-lock-count", err)
				}
			}

			presences, err := notifier.lockDB.Count(context.Background(), logger, models.PresenceType)
			if err != nil {
				logger.Error("failed-to-retrieve-presence-count", err)
			} else {
				err = notifier.metronClient.SendMetric(activePresencesMetric, presences)
				if err != nil {
					logger.Error("failed-sending-presences-count", err)
				}
			}

			logger.Debug("emitted-metrics")
		}
	}
}
