package main

import (
	"context"
	"errors"
	"os"
	"time"

	"code.cloudfoundry.org/bbs"
	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager/v3"
)

// bbsHealthCheckRunner periodically probes BBS connectivity from the
// auctioneer host and exits (via ifrit error) when N consecutive checks
// fail. This lets monit restart the auctioneer, whose normal SIGTERM
// path cleanly releases the Locket lock — allowing a healthy-AZ
// standby to acquire leadership.
//
// The auctioneer's Locket lock is renewed
// over a persistent gRPC connection (DNS-free) while auction work
// (fetching cell reps from BBS) resolves bbs.service.cf.internal on
// every call. Under a DNS-only partition, renewals succeed and the
// leader retains the lock indefinitely despite being functionally
// dead. A dedicated health check breaks the tie by making
// functional-path degradation observable to the process supervisor.
type bbsHealthCheckRunner struct {
	logger           lager.Logger
	bbsClient        bbs.InternalClient
	clock            clock.Clock
	interval         time.Duration
	timeout          time.Duration
	failureThreshold int
}

func newBBSHealthCheckRunner(
	logger lager.Logger,
	bbsClient bbs.InternalClient,
	clk clock.Clock,
	interval, timeout time.Duration,
	failureThreshold int,
) *bbsHealthCheckRunner {
	return &bbsHealthCheckRunner{
		logger:           logger.Session("bbs-health-check-runner"),
		bbsClient:        bbsClient,
		clock:            clk,
		interval:         interval,
		timeout:          timeout,
		failureThreshold: failureThreshold,
	}
}

// Run implements ifrit.Runner. Signals a ready channel immediately;
// on each tick, probes BBS. On threshold consecutive failures, returns
// a non-nil error so the ifrit group tears down and monit restarts.
func (r *bbsHealthCheckRunner) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	logger := r.logger.Session("run")
	logger.Info("starting", lager.Data{
		"interval":          r.interval.String(),
		"timeout":           r.timeout.String(),
		"failure_threshold": r.failureThreshold,
	})

	close(ready)

	ticker := r.clock.NewTicker(r.interval)
	defer ticker.Stop()

	failures := 0
	for {
		select {
		case <-signals:
			logger.Info("received-signal")
			return nil

		case <-ticker.C():
			if r.probe(logger) {
				if failures > 0 {
					logger.Info("health-check-recovered", lager.Data{
						"consecutive_failures": failures,
					})
				}
				failures = 0
				continue
			}

			failures++
			logger.Error("health-check-failed", nil, lager.Data{
				"failures":  failures,
				"threshold": r.failureThreshold,
			})

			if failures >= r.failureThreshold {
				err := errors.New("bbs connectivity degraded")
				logger.Error("bbs-connectivity-degraded-restarting-auctioneer",
					err, lager.Data{"failures": failures})
				return err
			}
		}
	}
}

// probe performs one BBS Ping with an enforced timeout. Returns true
// on success. The ping uses the same bbs.Client the auctioneer uses
// for real work, so it exercises the same DNS/TLS/HTTP path — any
// degradation visible to auction requests is visible here.
func (r *bbsHealthCheckRunner) probe(logger lager.Logger) bool {
	ctx, cancel := context.WithTimeout(context.Background(), r.timeout)
	defer cancel()

	done := make(chan bool, 1)
	go func() {
		// bbs.InternalClient.Ping does not accept a context directly,
		// but the underlying HTTP client honors the per-request
		// timeout configured on the auctioneer's cfhttp client. The
		// context.WithTimeout above enforces an outer bound so a
		// truly hung Ping cannot stall the health check goroutine.
		done <- r.bbsClient.Ping(logger, "auctioneer-bbs-health-check")
	}()

	select {
	case ok := <-done:
		return ok
	case <-ctx.Done():
		logger.Error("probe-timeout", ctx.Err(), lager.Data{
			"timeout": r.timeout.String(),
		})
		return false
	}
}
