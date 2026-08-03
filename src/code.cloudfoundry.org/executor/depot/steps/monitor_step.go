package steps

import (
	"time"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/executor/depot/log_streamer"
	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/workpool"
	"github.com/tedsuo/ifrit"
)

func NewMonitor(
	checkFunc func() ifrit.Runner,
	logger lager.Logger,
	clock clock.Clock,
	logStreamer log_streamer.LogStreamer,
	startTimeout time.Duration,
	healthyInterval time.Duration,
	unhealthyInterval time.Duration,
	workPool *workpool.WorkPool,
	proxyStartupChecks ...ifrit.Runner,
) ifrit.Runner {
	throttledCheckFunc := func() ifrit.Runner {
		return NewThrottle(checkFunc(), workPool)
	}

	startupCheck := NewEventuallySucceedsStep(throttledCheckFunc, unhealthyInterval, startTimeout, clock)
	liveness := NewConsistentlySucceedsStep(throttledCheckFunc, healthyInterval, clock)

	// add the proxy startup checks (if any)
	startupCheck = NewParallel(append(proxyStartupChecks, startupCheck))

	return NewHealthCheckStep(startupCheck, liveness, logger, clock, logStreamer, logStreamer, startTimeout)
}
