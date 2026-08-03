package lockheldmetrics

import (
	"os"
	"sync/atomic"

	"github.com/tedsuo/ifrit"

	"code.cloudfoundry.org/clock"
	loggingclient "code.cloudfoundry.org/diego-logging-client"
	"code.cloudfoundry.org/lager/v3"
)

const (
	lockHeldMetric = "LockHeld"
)

type LockHeldMetronNotifier struct {
	logger       lager.Logger
	ticker       clock.Ticker
	lockHeld     *uint64
	metronClient loggingclient.IngressClient
}

func NewLockHeldMetronNotifier(logger lager.Logger, ticker clock.Ticker, metronClient loggingclient.IngressClient) *LockHeldMetronNotifier {
	lockHeld := uint64(0)
	return &LockHeldMetronNotifier{
		logger:       logger,
		ticker:       ticker,
		metronClient: metronClient,
		lockHeld:     &lockHeld,
	}
}

func (notifier *LockHeldMetronNotifier) SetLock() {
	atomic.StoreUint64(notifier.lockHeld, uint64(1))
}

func (notifier *LockHeldMetronNotifier) UnsetLock() {
	atomic.StoreUint64(notifier.lockHeld, 0)
}

func (notifier *LockHeldMetronNotifier) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	logger := notifier.logger.Session("lock-held-metrics-notifier")
	close(ready)

	logger.Info("started")
	defer logger.Info("finished")

	for {
		select {
		case <-notifier.ticker.C():
			value := atomic.LoadUint64(notifier.lockHeld)
			err := notifier.metronClient.SendMetric(lockHeldMetric, int(value))
			if err != nil {
				logger.Debug("failed-to-emit-lock-heldmetric", lager.Data{"error": err})
			}

		case <-signals:
			return nil
		}
	}
}

func SetLockHeldRunner(logger lager.Logger, notifier LockHeldMetronNotifier) ifrit.Runner {
	return ifrit.RunFunc(func(signals <-chan os.Signal, ready chan<- struct{}) error {
		logger = logger.Session("set-lock-held-metron-notifier")

		logger.Info("started")
		defer logger.Info("exited")

		notifier.SetLock()
		close(ready)

		<-signals
		return nil
	})
}
