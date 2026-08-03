package steps

import (
	"os"

	"code.cloudfoundry.org/executor/depot/log_streamer"
	"github.com/tedsuo/ifrit"

	"code.cloudfoundry.org/lager/v3"
)

type emitProgressStep struct {
	substep        ifrit.Runner
	logger         lager.Logger
	startMessage   string
	successMessage string
	failureMessage string
	streamer       log_streamer.LogStreamer
}

func NewEmitProgress(
	substep ifrit.Runner,
	startMessage,
	successMessage,
	failureMessage string,
	streamer log_streamer.LogStreamer,
	logger lager.Logger,
) *emitProgressStep {
	logger = logger.Session("emit-progress-step")
	return &emitProgressStep{
		substep:        substep,
		logger:         logger,
		startMessage:   startMessage,
		successMessage: successMessage,
		failureMessage: failureMessage,
		streamer:       streamer,
	}
}

func (step *emitProgressStep) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	if step.startMessage != "" {
		_, writeErr := step.streamer.Stdout().Write([]byte(step.startMessage + "\n"))
		if writeErr != nil {
			step.logger.Debug("failed-writing-to-stdout", lager.Data{"error": writeErr})
		}
	}

	err := step.substep.Run(signals, ready)

	if err != nil {
		if step.failureMessage != "" {
			_, writeErr := step.streamer.Stderr().Write([]byte(step.failureMessage))
			if writeErr != nil {
				step.logger.Debug("failed-writing-to-stderr", lager.Data{"error": writeErr})
			}

			if emittableError, ok := err.(*EmittableError); ok {
				_, writeErr := step.streamer.Stderr().Write([]byte(": "))
				if writeErr != nil {
					step.logger.Debug("failed-writing-to-stderr", lager.Data{"error": writeErr})
				}
				_, writeErr = step.streamer.Stderr().Write([]byte(emittableError.Error()))
				if writeErr != nil {
					step.logger.Debug("failed-writing-to-stderr", lager.Data{"error": writeErr})
				}

				wrappedMessage := ""
				if emittableError.WrappedError() != nil {
					wrappedMessage = emittableError.WrappedError().Error()
				}

				step.logger.Info("errored", lager.Data{
					"wrapped-error":   wrappedMessage,
					"message-emitted": emittableError.Error(),
				})
			}

			_, writeErr = step.streamer.Stderr().Write([]byte("\n"))
			if writeErr != nil {
				step.logger.Debug("failed-writing-to-stderr", lager.Data{"error": writeErr})
			}
		}
	} else {
		if step.successMessage != "" {
			_, writeErr := step.streamer.Stdout().Write([]byte(step.successMessage + "\n"))
			if writeErr != nil {
				step.logger.Debug("failed-writing-to-stdout", lager.Data{"error": writeErr})
			}
		}
	}

	return err
}
