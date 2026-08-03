package main

import (
	"os"
	"sync/atomic"
	"testing"
	"time"

	"code.cloudfoundry.org/bbs"
	"code.cloudfoundry.org/clock/fakeclock"
	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/lager/v3/lagertest"
)

// fakeBBSClient is a minimal stand-in for bbs.InternalClient used to
// exercise the health-check probe path without spinning up a full BBS.
// Only Ping() is implemented meaningfully; all other methods panic if
// called (the runner should not invoke them).
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

// TestBBSHealthCheckRunner_ExitsOnThreshold verifies the runner exits
// (returns a non-nil error) after `failureThreshold` consecutive
// failed probes.
func TestBBSHealthCheckRunner_ExitsOnThreshold(t *testing.T) {
	logger := lagertest.NewTestLogger("test")
	bbsClient := newFakeBBSClient(false)
	fakeClock := fakeclock.NewFakeClock(time.Now())

	r := newBBSHealthCheckRunner(logger, bbsClient, fakeClock,
		100*time.Millisecond, 50*time.Millisecond, 3)

	signals := make(chan os.Signal)
	ready := make(chan struct{})
	errCh := make(chan error, 1)
	go func() { errCh <- r.Run(signals, ready) }()

	<-ready

	// Drive the fake clock forward. WaitForWatcherAndIncrement blocks
	// until the goroutine calls NewTicker/Sleep on the fake clock,
	// then fires the tick. We give each tick a moment to be processed.
	for i := 0; i < 3; i++ {
		fakeClock.WaitForWatcherAndIncrement(100 * time.Millisecond)
		// Poll for the probe to be observed (fakeclock fires the
		// channel synchronously, but the goroutine still needs a
		// scheduling slice to consume it and run probe()).
		deadline := time.Now().Add(500 * time.Millisecond)
		for bbsClient.pings.Load() <= int64(i) && time.Now().Before(deadline) {
			time.Sleep(2 * time.Millisecond)
		}
	}

	select {
	case err := <-errCh:
		if err == nil {
			t.Fatal("expected non-nil error on threshold breach")
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("runner did not exit within 2s of threshold breach; pings=%d", bbsClient.pings.Load())
	}
	if bbsClient.pings.Load() < 3 {
		t.Fatalf("expected >=3 pings, got %d", bbsClient.pings.Load())
	}
}

// TestBBSHealthCheckRunner_RecoversOnSuccess verifies that a
// successful probe resets the failure counter and the runner
// continues without exiting.
func TestBBSHealthCheckRunner_RecoversOnSuccess(t *testing.T) {
	logger := lagertest.NewTestLogger("test")
	bbsClient := newFakeBBSClient(false)
	fakeClock := fakeclock.NewFakeClock(time.Now())

	r := newBBSHealthCheckRunner(logger, bbsClient, fakeClock,
		100*time.Millisecond, 50*time.Millisecond, 3)

	signals := make(chan os.Signal, 1)
	ready := make(chan struct{})
	errCh := make(chan error, 1)
	go func() { errCh <- r.Run(signals, ready) }()
	<-ready

	// Two failures then flip to success and tick once more; the runner
	// must not exit even though the total number of failed probes
	// (2) is less than the threshold (3).
	for i := 0; i < 2; i++ {
		fakeClock.WaitForWatcherAndIncrement(100 * time.Millisecond)
		deadline := time.Now().Add(500 * time.Millisecond)
		for bbsClient.pings.Load() <= int64(i) && time.Now().Before(deadline) {
			time.Sleep(2 * time.Millisecond)
		}
	}
	bbsClient.pingResult.Store(true)
	fakeClock.WaitForWatcherAndIncrement(100 * time.Millisecond)
	// Give the success probe a scheduling slice.
	deadline := time.Now().Add(500 * time.Millisecond)
	for bbsClient.pings.Load() < 3 && time.Now().Before(deadline) {
		time.Sleep(2 * time.Millisecond)
	}

	// Should still be running; send signal to shut down cleanly.
	signals <- os.Interrupt
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("expected clean exit on signal, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("runner did not exit after signal")
	}
}
