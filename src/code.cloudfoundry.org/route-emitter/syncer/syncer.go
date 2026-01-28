package syncer

import (
	"os"
	"time"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager/v3"
)

type NatsSyncer struct {
	clock                clock.Clock
	syncInterval         time.Duration
	syncCh               chan struct{}
	externalServiceStart chan time.Duration

	logger lager.Logger
}

func NewSyncer(
	clock clock.Clock,
	syncInterval time.Duration,
	logger lager.Logger,
) *NatsSyncer {
	return &NatsSyncer{
		clock:        clock,
		syncInterval: syncInterval,
		syncCh:       make(chan struct{}, 1),

		externalServiceStart: make(chan time.Duration),

		logger: logger.Session("syncer"),
	}
}

func (s *NatsSyncer) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	close(ready)
	s.logger.Info("started")

	s.sync()

	// now keep emitting at the desired interval, syncing every syncInterval
	syncTicker := s.clock.NewTicker(s.syncInterval)

	for {
		select {
		case <-syncTicker.C():
			s.sync()
		case <-signals:
			s.logger.Info("stopping")
			syncTicker.Stop()
			return nil
		}
	}
}

func (s *NatsSyncer) SyncCh() chan struct{} {
	return s.syncCh
}

func (s *NatsSyncer) sync() {
	s.syncCh <- struct{}{}
}
