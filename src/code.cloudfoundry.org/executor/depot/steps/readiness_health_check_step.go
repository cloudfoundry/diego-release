package steps

import (
	"fmt"
	"os"

	"code.cloudfoundry.org/executor/depot/log_streamer"
	"code.cloudfoundry.org/lager/v3"
	"github.com/tedsuo/ifrit"
)

type ReadinessState int

const (
	IsReady ReadinessState = iota
	IsNotReady
)

type readinessHealthCheckStep struct {
	untilReadyCheck   ifrit.Runner
	untilFailureCheck ifrit.Runner
	logger            lager.Logger
	logStreamer       log_streamer.LogStreamer
	readinessChan     chan ReadinessState
}

func NewReadinessHealthCheckStep(
	untilReadyCheck ifrit.Runner,
	untilFailureCheck ifrit.Runner,
	logStreamer log_streamer.LogStreamer,
	readinessChan chan ReadinessState,
	logger lager.Logger,
) ifrit.Runner {
	return &readinessHealthCheckStep{
		untilReadyCheck:   untilReadyCheck,
		untilFailureCheck: untilFailureCheck,
		logStreamer:       logStreamer,
		readinessChan:     readinessChan,
		logger:            logger,
	}
}

func (step *readinessHealthCheckStep) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	fmt.Fprint(step.logStreamer.Stdout(), "Starting readiness health monitoring of container\n")
	close(ready)

	for {
		err := step.runUntilReadyProcess(signals)
		if err != nil {
			return err
		}
		err = step.runUntilFailureProcess(signals)
		if err != nil {
			return err
		}
	}
}

func (step *readinessHealthCheckStep) runUntilReadyProcess(signals <-chan os.Signal) error {
	untilReadyProcess := ifrit.Background(step.untilReadyCheck)
	select {
	case err := <-untilReadyProcess.Wait():
		if err != nil {
			fmt.Fprint(step.logStreamer.Stdout(), "Failed to run the readiness check\n")
			return err
		}
		step.logger.Info("transitioned-to-ready")
		select {
		case step.readinessChan <- IsReady:
			fmt.Fprint(step.logStreamer.Stdout(), "Container passed the readiness health check. Container marked ready and added to route pool.\n")
			return nil
		case <-signals:
			return new(CancelledError)
		}
	case s := <-signals:
		untilReadyProcess.Signal(s)
		<-untilReadyProcess.Wait()
		return new(CancelledError)
	}
}

func (step *readinessHealthCheckStep) runUntilFailureProcess(signals <-chan os.Signal) error {
	untilFailureProcess := ifrit.Background(step.untilFailureCheck)
	select {
	case err := <-untilFailureProcess.Wait():
		if err != nil {
			step.logger.Info("transitioned-to-not-ready")
			select {
			case step.readinessChan <- IsNotReady:
				fmt.Fprint(step.logStreamer.Stdout(), "Container failed the readiness health check. Container marked not ready and removed from route pool.\n")
				return nil
			case <-signals:
				return new(CancelledError)
			}
		}
		step.logger.Error("unexpected-until-failure-check-result", err)
		return nil
	case s := <-signals:
		untilFailureProcess.Signal(s)
		<-untilFailureProcess.Wait()
		return new(CancelledError)
	}
}
