package evacuation

import (
	"os"
	"time"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/executor"
	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/rep/evacuation/evacuation_context"
)

type Evacuator struct {
	logger             lager.Logger
	clock              clock.Clock
	executorClient     executor.Client
	evacuationNotifier evacuation_context.EvacuationNotifier
	cellID             string
	evacuationTimeout  time.Duration
	pollingInterval    time.Duration
}

func NewEvacuator(
	logger lager.Logger,
	clock clock.Clock,
	executorClient executor.Client,
	evacuationNotifier evacuation_context.EvacuationNotifier,
	cellID string,
	evacuationTimeout time.Duration,
	pollingInterval time.Duration,
) *Evacuator {
	return &Evacuator{
		logger:             logger,
		clock:              clock,
		executorClient:     executorClient,
		evacuationNotifier: evacuationNotifier,
		cellID:             cellID,
		evacuationTimeout:  evacuationTimeout,
		pollingInterval:    pollingInterval,
	}
}

func (e *Evacuator) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	logger := e.logger.Session("running-evacuator")
	logger.Info("started")
	defer logger.Info("finished")

	evacuationNotify := e.evacuationNotifier.EvacuateNotify()
	close(ready)

	select {
	case signal := <-signals:
		logger.Info("signaled", lager.Data{"signal": signal.String()})
		return nil
	case <-evacuationNotify:
		evacuationNotify = nil
		logger.Info("notified-of-evacuation")
	}

	timer := e.clock.NewTimer(e.evacuationTimeout)
	defer timer.Stop()

	doneCh := make(chan struct{})
	go e.evacuate(logger, doneCh)

	select {
	case <-doneCh:
		logger.Info("evacuation-complete")
		return nil
	case <-timer.C():
		logger.Error("failed-to-evacuate-before-timeout", nil)
		return nil
	case signal := <-signals:
		logger.Info("signaled", lager.Data{"signal": signal.String()})
		return nil
	}
}

func (e *Evacuator) evacuate(logger lager.Logger, doneCh chan<- struct{}) {
	logger = logger.Session("evacuating")
	logger.Info("started")

	timer := e.clock.NewTimer(e.pollingInterval)
	defer timer.Stop()

	for {
		evacuated := e.allContainersEvacuated(logger)

		if !evacuated {
			logger.Info("evacuation-incomplete", lager.Data{"polling-interval": e.pollingInterval})
			timer.Reset(e.pollingInterval)
			<-timer.C()
			continue
		}

		close(doneCh)
		logger.Info("succeeded")

		return
	}
}

func (e *Evacuator) allContainersEvacuated(logger lager.Logger) bool {
	containers, err := e.executorClient.ListContainers(logger)
	if err != nil {
		logger.Error("failed-to-list-containers", err)
		return false
	}

	return len(containers) == 0
}
