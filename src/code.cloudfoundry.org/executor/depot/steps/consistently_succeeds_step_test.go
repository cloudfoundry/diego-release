package steps_test

import (
	"errors"
	"os"
	"time"

	"code.cloudfoundry.org/clock/fakeclock"
	"code.cloudfoundry.org/executor/depot/steps"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/tedsuo/ifrit"
	fake_runner "github.com/tedsuo/ifrit/fake_runner_v2"
)

var _ = Describe("ConsistentlySucceedsStep", func() {
	var (
		step    ifrit.Runner
		process ifrit.Process

		fakeRunner *fake_runner.TestRunner
		fakeClock  *fakeclock.FakeClock
	)

	BeforeEach(func() {
		fakeClock = fakeclock.NewFakeClock(time.Now())
		fakeRunner = fake_runner.NewTestRunner()

		step = steps.NewConsistentlySucceedsStep(func() ifrit.Runner { return fakeRunner }, time.Second, fakeClock)
	})

	JustBeforeEach(func() {
		process = ifrit.Background(step)
	})

	Context("when the process is started", func() {
		AfterEach(func() {
			fakeClock.WaitForWatcherAndIncrement(time.Second)
			fakeRunner.TriggerExit(errors.New("boooom!"))
		})

		It("should not exit", func() {
			Consistently(process.Wait()).ShouldNot(Receive(BeNil()))
		})

		It("does not perform the substep initially", func() {
			Consistently(fakeRunner.RunCallCount).Should(BeZero())
		})

		It("becomes ready immediately", func() {
			Eventually(process.Ready()).Should(BeClosed())
		})
	})

	Context("when the step is cancelled", func() {
		JustBeforeEach(func() {
			process.Signal(os.Interrupt)
		})

		It("cancels the substep", func() {
			Eventually(process.Wait()).Should(Receive(MatchError(new(steps.CancelledError))))
		})
	})

	Context("when the step is stuck", func() {
		Context("and the step is cancelled", func() {
			JustBeforeEach(func() {
				fakeClock.WaitForWatcherAndIncrement(time.Second)
				Eventually(fakeRunner.RunCallCount).Should(Equal(1))
				process.Signal(os.Interrupt)
			})

			It("cancels the substep", func() {
				signals := fakeRunner.WaitForCall()
				Eventually(signals).Should(Receive())
				fakeRunner.TriggerExit(errors.New("BOOOM"))
				Eventually(process.Wait()).Should(Receive(MatchError("BOOOM")))
			})
		})
	})

	Context("when the step is succeeding", func() {
		JustBeforeEach(func() {
			fakeClock.WaitForWatcherAndIncrement(time.Second)
			Eventually(fakeRunner.RunCallCount).Should(Equal(1))
			fakeRunner.TriggerExit(nil)
		})

		It("retries after the timeout has elapsed", func() {
			fakeClock.WaitForWatcherAndIncrement(time.Second)
			Eventually(fakeRunner.RunCallCount).Should(Equal(2))
			fakeRunner.TriggerExit(errors.New("boooom!"))
		})

		Context("when it later fails", func() {
			JustBeforeEach(func() {
				fakeClock.WaitForWatcherAndIncrement(time.Second)
				Eventually(fakeRunner.RunCallCount).Should(Equal(2))
				fakeRunner.TriggerExit(errors.New("BOOOM"))
			})

			It("should fail", func() {
				Eventually(process.Wait()).Should(Receive(MatchError("BOOOM")))
			})
		})
	})
})
