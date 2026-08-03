package steps

import (
	"os"
	"time"

	"code.cloudfoundry.org/clock"
	loggingclient "code.cloudfoundry.org/diego-logging-client"
	"code.cloudfoundry.org/lager/v3"
	"github.com/tedsuo/ifrit"
)

type timedStep struct {
	step         ifrit.Runner
	startTime    time.Time
	clock        clock.Clock
	logger       lager.Logger
	metronClient loggingclient.IngressClient
}

const (
	ContainerSetupSucceededDuration = "ContainerSetupSucceededDuration"
	ContainerSetupFailedDuration    = "ContainerSetupFailedDuration"
)

func NewTimedStep(logger lager.Logger, step ifrit.Runner, metronClient loggingclient.IngressClient, clock clock.Clock, startTime time.Time) ifrit.Runner {
	return &timedStep{
		step:         step,
		startTime:    startTime,
		metronClient: metronClient,
		clock:        clock,
		logger:       logger,
	}
}

func (runner *timedStep) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	var err error
	defer func() {
		duration := runner.clock.Since(runner.startTime)
		if err == nil {
			runner.logger.Info("container-setup-succeeded", lager.Data{"duration": duration})
			go runner.metronClient.SendDuration(ContainerSetupSucceededDuration, duration)
		} else {
			runner.logger.Info("container-setup-failed", lager.Data{"duration": duration})
			go runner.metronClient.SendDuration(ContainerSetupFailedDuration, duration)
		}
	}()

	if runner.step == nil {
		return nil
	}

	subStepSignals := make(chan os.Signal, 1)
	errCh := make(chan error)

	go func() {
		errCh <- runner.step.Run(subStepSignals, ready)
	}()

	select {
	case s := <-signals:
		subStepSignals <- s
		return nil
	case err = <-errCh:
		return err
	}
}
