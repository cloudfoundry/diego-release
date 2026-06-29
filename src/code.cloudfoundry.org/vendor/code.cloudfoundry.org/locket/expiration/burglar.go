package expiration

import (
	"context"
	"os"
	"time"

	"code.cloudfoundry.org/clock"
	loggingclient "code.cloudfoundry.org/diego-logging-client"
	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/locket"
	"code.cloudfoundry.org/locket/db"
)

const (
	locksExpiredCounter    = "LocksExpired"
	presenceExpiredCounter = "PresenceExpired"
)

type burglar struct {
	logger        lager.Logger
	lockDB        db.LockDB
	lockPick      LockPick
	clock         clock.Clock
	checkInterval time.Duration
	metronClient  loggingclient.IngressClient
}

func NewBurglar(logger lager.Logger, lockDB db.LockDB, lockPick LockPick, clock clock.Clock, checkInterval time.Duration, metronClient loggingclient.IngressClient) burglar {
	return burglar{
		logger:        logger,
		lockDB:        lockDB,
		lockPick:      lockPick,
		clock:         clock,
		checkInterval: checkInterval,
		metronClient:  metronClient,
	}
}

func (b burglar) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	logger := b.logger.Session("burglar")

	logger.Info("started")
	defer logger.Info("complete")

	locks, err := b.lockDB.FetchAll(context.Background(), logger, "")
	if err != nil {
		logger.Error("failed-fetching-locks", err)
	}

	for _, lock := range locks {
		b.lockPick.RegisterTTL(logger, lock)
	}

	check := b.clock.NewTicker(b.checkInterval)
	expirationCheck := b.clock.NewTicker(locket.ExpirationMetricsInterval)

	close(ready)

	for {
		select {
		case sig := <-signals:
			logger.Info("signalled", lager.Data{"signal": sig})
			return nil
		case <-check.C():
			locks, err := b.lockDB.FetchAll(context.Background(), logger, "")
			if err != nil {
				logger.Error("failed-fetching-locks", err)
				continue
			}

			for _, lock := range locks {
				b.lockPick.RegisterTTL(logger, lock)
			}
		case <-expirationCheck.C():
			locksExpired, presencesExpired := b.lockPick.ExpirationCounts()
			err := b.metronClient.SendMetric(locksExpiredCounter, int(locksExpired))
			if err != nil {
				logger.Debug("failed-to-send-locks-expired-metric", lager.Data{"error": err})
			}
			err = b.metronClient.SendMetric(presenceExpiredCounter, int(presencesExpired))
			if err != nil {
				logger.Debug("failed-to-send-presences-expired-metric", lager.Data{"error": err})
			}
		}
	}
}
