package steps

import (
	"fmt"
	"os"
	"time"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/executor/depot/log_streamer"
	"code.cloudfoundry.org/lager/v3"
	"github.com/tedsuo/ifrit"
)

const (
	startupFailureMessage   = "Failed after %s: startup health check never passed.\n"
	timeoutCrashReason      = "Instance never healthy after %s: %s"
	healthcheckNowUnhealthy = "Instance became unhealthy: %s"
)

type healthCheckStep struct {
	startupCheck  ifrit.Runner
	livenessCheck ifrit.Runner

	logger              lager.Logger
	clock               clock.Clock
	logStreamer         log_streamer.LogStreamer
	healthCheckStreamer log_streamer.LogStreamer

	startTimeout time.Duration
}

func NewHealthCheckStep(
	startupCheck ifrit.Runner,
	livenessCheck ifrit.Runner,
	logger lager.Logger,
	clock clock.Clock,
	logStreamer log_streamer.LogStreamer,
	healthcheckStreamer log_streamer.LogStreamer,
	startTimeout time.Duration,
) ifrit.Runner {
	logger = logger.Session("health-check-step")

	return &healthCheckStep{
		startupCheck:        startupCheck,
		livenessCheck:       livenessCheck,
		logger:              logger,
		clock:               clock,
		logStreamer:         logStreamer,
		healthCheckStreamer: healthcheckStreamer,
		startTimeout:        startTimeout,
	}
}

func (step *healthCheckStep) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	//TODO: make this use metron agent directly, don't use log streamer, shouldn't be rate limited.
	fmt.Fprint(step.logStreamer.Stdout(), "Starting health monitoring of container\n")

	startupProcess := ifrit.Background(step.startupCheck)

	healthCheckStartedTime := time.Now()

	select {
	case err := <-startupProcess.Wait():
		if err != nil {
			healthCheckFailedTime := time.Since(healthCheckStartedTime).Round(time.Millisecond)
			//TODO: make this use metron agent directly, don't use log streamer, shouldn't be rate limited.
			fmt.Fprintf(step.healthCheckStreamer.Stderr(), "%s\n", err.Error())
			fmt.Fprintf(step.logStreamer.Stderr(), startupFailureMessage, healthCheckFailedTime)
			step.logger.Info("timed-out-before-healthy", lager.Data{
				"step-error": err.Error(),
			})
			return NewEmittableError(err, timeoutCrashReason, healthCheckFailedTime, err.Error())
		}
	case s := <-signals:
		startupProcess.Signal(s)
		<-startupProcess.Wait()
		return new(CancelledError)
	}

	step.logger.Info("transitioned-to-healthy")
	//TODO: make this use metron agent directly, don't use log streamer, shouldn't be rate limited.
	fmt.Fprint(step.logStreamer.Stdout(), "Container became healthy\n")
	close(ready)

	livenessProcess := ifrit.Background(step.livenessCheck)

	select {
	case err := <-livenessProcess.Wait():
		step.logger.Info("transitioned-to-unhealthy")
		//TODO: make this use metron agent directly, don't use log streamer, shouldn't be rate limited.
		fmt.Fprintf(step.healthCheckStreamer.Stderr(), "%s\n", err.Error())
		fmt.Fprint(step.logStreamer.Stderr(), "Container became unhealthy\n")
		return NewEmittableError(err, healthcheckNowUnhealthy, err.Error())
	case s := <-signals:
		livenessProcess.Signal(s)
		<-livenessProcess.Wait()
		return new(CancelledError)
	}
}
