package main

import (
	"os"
	"sync/atomic"
	"time"

	"code.cloudfoundry.org/bbs"
	"code.cloudfoundry.org/clock/fakeclock"
	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/lager/v3/lagertest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// fakeBBSClient is a minimal stand-in for bbs.InternalClient used to
// exercise the health-check probe path without spinning up a full BBS.
// Only Ping() is implemented meaningfully; the embedded interface
// leaves every other method nil so the runner panics if it invokes
// anything other than Ping.
type fakeBBSClient struct {
	pingResult atomic.Bool
	pings      atomic.Int64
	bbs.InternalClient
}

func newFakeBBSClient(initialResult bool) *fakeBBSClient {
	c := &fakeBBSClient{}
	c.pingResult.Store(initialResult)
	return c
}

func (f *fakeBBSClient) Ping(_ lager.Logger, _ string) bool {
	f.pings.Add(1)
	return f.pingResult.Load()
}

var _ = Describe("BBSHealthCheckRunner", func() {
	const (
		interval         = 100 * time.Millisecond
		timeout          = 50 * time.Millisecond
		failureThreshold = 3
	)

	var (
		testLogger *lagertest.TestLogger
		bbsClient  *fakeBBSClient
		fakeClock  *fakeclock.FakeClock
		runner     *bbsHealthCheckRunner

		signals chan os.Signal
		ready   chan struct{}
		errCh   chan error
	)

	BeforeEach(func() {
		testLogger = lagertest.NewTestLogger("test")
		fakeClock = fakeclock.NewFakeClock(time.Now())
		signals = make(chan os.Signal, 1)
		ready = make(chan struct{})
		errCh = make(chan error, 1)
	})

	JustBeforeEach(func() {
		runner = newBBSHealthCheckRunner(testLogger, bbsClient, fakeClock, interval, timeout, failureThreshold)
		go func() {
			errCh <- runner.Run(signals, ready)
		}()
		Eventually(ready).Should(BeClosed())
	})

	// tick advances the fake clock by one interval and waits for the
	// runner goroutine to observe the resulting probe. fakeclock fires
	// the ticker channel synchronously, but the goroutine still needs a
	// scheduling slice to consume it and run probe(); waiting for the
	// ping count to advance keeps successive ticks from coalescing.
	tick := func(expectedPings int64) {
		fakeClock.WaitForWatcherAndIncrement(interval)
		Eventually(func() int64 {
			return bbsClient.pings.Load()
		}).Should(BeNumerically(">=", expectedPings))
	}

	Context("when BBS is continuously unreachable", func() {
		BeforeEach(func() {
			bbsClient = newFakeBBSClient(false)
		})

		It("exits with an error after the failure threshold is reached", func() {
			for i := int64(1); i <= failureThreshold; i++ {
				tick(i)
			}

			var runErr error
			Eventually(errCh).Should(Receive(&runErr))
			Expect(runErr).To(HaveOccurred())
			Expect(bbsClient.pings.Load()).To(BeNumerically(">=", int64(failureThreshold)))
		})
	})

	Context("when a probe succeeds before the failure threshold is reached", func() {
		BeforeEach(func() {
			bbsClient = newFakeBBSClient(false)
		})

		It("resets the failure counter and keeps running", func() {
			// Two failures — one short of the threshold.
			tick(1)
			tick(2)

			// Flip to success and tick once more; the counter resets.
			bbsClient.pingResult.Store(true)
			tick(3)

			// The runner must not have exited.
			Consistently(errCh).ShouldNot(Receive())

			// A signal still shuts it down cleanly.
			signals <- os.Interrupt
			Eventually(errCh).Should(Receive(BeNil()))
		})
	})

	Context("when a signal is received", func() {
		BeforeEach(func() {
			bbsClient = newFakeBBSClient(true)
		})

		It("exits cleanly without an error", func() {
			signals <- os.Interrupt
			Eventually(errCh).Should(Receive(BeNil()))
		})
	})
})
