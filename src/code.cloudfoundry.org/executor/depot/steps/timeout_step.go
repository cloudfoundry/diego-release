package steps

import (
	"os"
	"time"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager/v3"
	"github.com/tedsuo/ifrit"
)

type timeoutStep struct {
	substep ifrit.Runner
	timeout time.Duration
	logger  lager.Logger
	clock   clock.Clock
}

func NewTimeout(substep ifrit.Runner, timeout time.Duration, clock clock.Clock, logger lager.Logger) ifrit.Runner {
	return &timeoutStep{
		substep: substep,
		timeout: timeout,
		clock:   clock,
		logger:  logger.Session("timeout-step"),
	}
}

func (step *timeoutStep) Run(signals <-chan os.Signal, ready chan<- struct{}) (err error) {
	timer := step.clock.NewTimer(step.timeout)
	defer timer.Stop()

	subStepSignals := make(chan os.Signal)
	resultCh := make(chan error)

	go func() {
		resultCh <- step.substep.Run(subStepSignals, ready)
	}()

	for {
		select {
		case s := <-signals:
			subStepSignals <- s
		case err := <-resultCh:
			return err
		case <-timer.C():
			step.logger.Error("timed-out", nil)
			subStepSignals <- os.Interrupt
			err := <-resultCh
			return NewEmittableError(err, "%s", emittableMessage(step.timeout, err))
		}
	}
}

func emittableMessage(timeout time.Duration, substepErr error) string {
	message := "exceeded " + timeout.String() + " timeout"

	if emittable, ok := substepErr.(*EmittableError); ok {
		message += "; " + emittable.Error()
	}

	return message
}
