package steps

import (
	"os"

	"code.cloudfoundry.org/lager/v3"
	"github.com/tedsuo/ifrit"
)

type backgroundStep struct {
	substep ifrit.Runner
	logger  lager.Logger
}

func NewBackground(substep ifrit.Runner, logger lager.Logger) ifrit.Runner {
	logger = logger.Session("background-step")
	return &backgroundStep{
		substep: substep,
		logger:  logger,
	}
}

func (step *backgroundStep) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	errCh := make(chan error, 1)
	go func() {
		errCh <- step.substep.Run(make(chan os.Signal, 1), ready)
	}()

	select {
	case err := <-errCh:
		if err != nil {
			step.logger.Info("failed", lager.Data{
				"error": err.Error(),
			})
		}
		return err
	case <-signals:
		step.logger.Info("detaching-from-substep")
		return nil
	}
}
