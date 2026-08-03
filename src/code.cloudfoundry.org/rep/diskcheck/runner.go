package diskcheck

import (
	"errors"
	"os"
	"time"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/executor"
	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/rep/evacuation/evacuation_context"
)

// ErrDiskReadOnly is returned by Run when a read-only filesystem is detected.
var ErrDiskReadOnly = errors.New("disk read-only detected, evacuating")

// Runner is an ifrit.Runner that periodically checks configured paths for
// read-only filesystems and triggers graceful evacuation when one is detected.
type Runner struct {
	logger           lager.Logger
	clock            clock.Clock
	paths            []string
	interval         time.Duration
	failureThreshold int
	executorClient   executor.Client
	evacuatable      evacuation_context.Evacuatable
	checkPath        func(string) (bool, error)
}

// NewRunner constructs a Runner. checkPath is called once per path per tick;
// pass diskcheck.IsReadOnly for production use. failureThreshold controls how
// many consecutive periodic-check failures must occur before evacuation is
// triggered; the startup check is always immediate regardless of this value.
// Values <= 0 are treated as 3.
func NewRunner(
	logger lager.Logger,
	clk clock.Clock,
	paths []string,
	interval time.Duration,
	failureThreshold int,
	executorClient executor.Client,
	evacuatable evacuation_context.Evacuatable,
	checkPath func(string) (bool, error),
) *Runner {
	if failureThreshold <= 0 {
		failureThreshold = 3
	}
	return &Runner{
		logger:           logger.Session("disk-check"),
		clock:            clk,
		paths:            paths,
		interval:         interval,
		failureThreshold: failureThreshold,
		executorClient:   executorClient,
		evacuatable:      evacuatable,
		checkPath:        checkPath,
	}
}

// Run implements ifrit.Runner.
//
// Sequence:
//  1. Check all paths immediately in a goroutine. Any failure triggers
//     evacuation at once — a disk already read-only at startup is a real
//     problem, not a transient glitch. A signal received while the startup
//     check is in progress returns nil immediately.
//  2. If startup check passes, signal ready.
//  3. Tick at interval. On each tick check all paths.
//  4. If a tick check fails, increment consecutiveFailures. When
//     consecutiveFailures reaches failureThreshold, trigger evacuation.
//     A passing tick resets the counter, tolerating transient errors.
//  5. On signal (before any failure) return nil.
func (r *Runner) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	logger := r.logger.Session("run")
	logger.Info("starting")

	startupDone := make(chan bool, 1)
	go func() { startupDone <- r.pathsUnhealthy(logger) }()

	select {
	case <-signals:
		logger.Info("signalled-during-startup")
		return nil
	case unhealthy := <-startupDone:
		if unhealthy {
			return r.triggerFailureAndWait(logger, signals)
		}
	}

	close(ready)
	logger.Info("started")

	ticker := r.clock.NewTicker(r.interval)
	defer ticker.Stop()

	consecutiveFailures := 0

	for {
		select {
		case <-signals:
			logger.Info("signalled")
			return nil

		case <-ticker.C():
			if r.pathsUnhealthy(logger) {
				consecutiveFailures++
				logger.Info("disk-check-failure", lager.Data{
					"consecutive_failures": consecutiveFailures,
					"threshold":            r.failureThreshold,
				})
				if consecutiveFailures >= r.failureThreshold {
					return r.triggerFailureAndWait(logger, signals)
				}
			} else {
				if consecutiveFailures > 0 {
					logger.Info("disk-check-recovered", lager.Data{"was": consecutiveFailures})
					consecutiveFailures = 0
				}
			}
		}
	}
}

// pathsUnhealthy returns true if any path is read-only or its check returns an
// error. A check error is treated as unhealthy to fail safe: if statfs cannot
// determine whether the disk is read-only, we assume the worst.
func (r *Runner) pathsUnhealthy(logger lager.Logger) bool {
	for _, path := range r.paths {
		ro, err := r.checkPath(path)
		if err != nil {
			logger.Error("check-path-error", err, lager.Data{"path": path})
			return true
		}
		if ro {
			logger.Error("path-read-only", ErrDiskReadOnly, lager.Data{"path": path})
			return true
		}
	}
	return false
}

// triggerFailureAndWait marks the cell unhealthy, triggers evacuation, waits
// for the cascade signal from the grouper, then returns ErrDiskReadOnly so
// monitor.Wait() propagates it to os.Exit(1).
func (r *Runner) triggerFailureAndWait(logger lager.Logger, signals <-chan os.Signal) error {
	logger.Error("triggering-evacuation", ErrDiskReadOnly)
	r.executorClient.SetHealthy(logger, false)
	r.evacuatable.Evacuate()
	<-signals
	return ErrDiskReadOnly
}
