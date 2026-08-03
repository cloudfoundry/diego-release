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

var _ = Describe("EventuallySucceedsStep", func() {
	var (
		step    ifrit.Runner
		process ifrit.Process

		fakeStep  *fake_runner.TestRunner
		fakeClock *fakeclock.FakeClock
	)

	BeforeEach(func() {
		fakeClock = fakeclock.NewFakeClock(time.Now())
		fakeStep = fake_runner.NewTestRunner()

		step = steps.NewEventuallySucceedsStep(func() ifrit.Runner { return fakeStep }, time.Second, 10*time.Second, fakeClock)
	})

	JustBeforeEach(func() {
		process = ifrit.Background(step)
	})

	Context("when the process is started", func() {
		AfterEach(func() {
			fakeClock.WaitForWatcherAndIncrement(10 * time.Second)
			fakeStep.TriggerExit(nil)
		})

		It("should not trigger the substep initially", func() {
			Consistently(fakeStep.RunCallCount).Should(BeZero())
		})

		It("becomes ready immediately", func() {
			Eventually(process.Ready()).Should(BeClosed())
		})
	})

	Context("when the step succeeds", func() {
		JustBeforeEach(func() {
			fakeClock.WaitForWatcherAndIncrement(time.Second)
			fakeStep.TriggerExit(nil)
		})

		It("should exits with no errors", func() {
			Eventually(process.Wait()).Should(Receive(BeNil()))
		})
	})

	Context("when the step is stuck", func() {
		Context("and the step is signalled", func() {
			JustBeforeEach(func() {
				fakeClock.WaitForWatcherAndIncrement(time.Second)
				Eventually(fakeStep.RunCallCount).Should(Equal(1))
				process.Signal(os.Interrupt)
			})

			It("cancels the substep", func() {
				Eventually(fakeStep.WaitForCall()).Should(Receive())
				fakeStep.TriggerExit(errors.New("BOOOOM"))
				Eventually(process.Wait()).Should(Receive(MatchError("BOOOOM")))
			})
		})
	})

	Context("when the step is signalled", func() {
		JustBeforeEach(func() {
			process.Signal(os.Interrupt)
		})

		It("returns CancelledError", func() {
			Eventually(process.Wait()).Should(Receive(MatchError(new(steps.CancelledError))))
		})
	})

	Context("when the step is failing", func() {
		JustBeforeEach(func() {
			fakeClock.WaitForWatcherAndIncrement(time.Second)
			fakeStep.TriggerExit(errors.New("BOOOOM"))
		})

		It("retries after the timeout has elapsed", func() {
			fakeClock.WaitForWatcherAndIncrement(time.Second)
			Eventually(fakeStep.RunCallCount).Should(Equal(2))
			fakeStep.TriggerExit(nil)
		})

		Context("and the timeout elapsed", func() {
			JustBeforeEach(func() {
				Eventually(fakeStep.RunCallCount).Should(Equal(1))
				fakeClock.WaitForWatcherAndIncrement(10 * time.Second)
				fakeStep.TriggerExit(errors.New("BOOOOM"))
			})

			It("returns the last error received from the substep", func() {
				Eventually(process.Wait()).Should(Receive(MatchError(ContainSubstring("BOOOOM"))))
			})
		})

		Context("when it later succeed", func() {
			JustBeforeEach(func() {
				Eventually(fakeStep.RunCallCount).Should(Equal(1))
				fakeClock.WaitForWatcherAndIncrement(time.Second)
				fakeStep.TriggerExit(nil)
			})

			It("should succeed", func() {
				Eventually(process.Wait()).Should(Receive(BeNil()))
			})
		})
	})
})
