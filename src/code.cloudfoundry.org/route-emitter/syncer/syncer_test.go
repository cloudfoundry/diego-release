package syncer_test

import (
	"os"
	"time"

	"code.cloudfoundry.org/clock/fakeclock"
	"code.cloudfoundry.org/lager/v3/lagertest"
	"code.cloudfoundry.org/route-emitter/syncer"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/tedsuo/ifrit"
)

var _ = Describe("NatsSyncer", func() {
	var (
		syncerRunner *syncer.NatsSyncer
		process      ifrit.Process
		clock        *fakeclock.FakeClock
		syncInterval time.Duration

		shutdown chan struct{}
	)

	BeforeEach(func() {
		clock = fakeclock.NewFakeClock(time.Now())
		syncInterval = 10 * time.Second
	})

	JustBeforeEach(func() {
		logger := lagertest.NewTestLogger("test")
		syncerRunner = syncer.NewSyncer(clock, syncInterval, logger)

		shutdown = make(chan struct{})

		process = ifrit.Invoke(syncerRunner)
	})

	AfterEach(func() {
		process.Signal(os.Interrupt)
		Eventually(process.Wait()).Should(Receive(BeNil()))
		close(shutdown)
	})

	BeforeEach(func() {
		syncInterval = 500 * time.Millisecond
	})

	Context("on a specified interval", func() {
		It("should sync", func() {
			clock.WaitForWatcherAndIncrement(syncInterval)
			Eventually(syncerRunner.SyncCh()).Should(Receive())

			clock.WaitForWatcherAndIncrement(syncInterval)
			Eventually(syncerRunner.SyncCh()).Should(Receive())
		})
	})
})
