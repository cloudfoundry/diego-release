package steps

import (
	"os"

	"code.cloudfoundry.org/lager/v3"
	"github.com/tedsuo/ifrit"
)

type tryStep struct {
	substep ifrit.Runner
	logger  lager.Logger
}

func NewTry(substep ifrit.Runner, logger lager.Logger) ifrit.Runner {
	logger = logger.Session("try-step")
	return &tryStep{
		substep: substep,
		logger:  logger,
	}
}

func (step *tryStep) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	subStepSignals := make(chan os.Signal, 1)
	errCh := make(chan error)
	go func() {
		errCh <- step.substep.Run(subStepSignals, ready)
	}()

	logErr := func(err error) {
		if err != nil {
			step.logger.Info("failed", lager.Data{
				"error": err.Error(),
			})
		}
	}

	select {
	case s := <-signals:
		subStepSignals <- s
		logErr(<-errCh)
		return nil
	case err := <-errCh:
		logErr(err)
		return nil
	}
}
