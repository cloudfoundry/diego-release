package steps_test

import (
	"errors"
	"os"

	"code.cloudfoundry.org/executor/depot/steps"
	"code.cloudfoundry.org/workpool"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/tedsuo/ifrit"
	fake_runner "github.com/tedsuo/ifrit/fake_runner_v2"
)

var _ = Describe("ThrottleStep", func() {
	const numConcurrentSteps = 3

	var (
		step     ifrit.Runner
		workPool *workpool.WorkPool
		fakeStep *fake_runner.TestRunner
	)

	BeforeEach(func() {
		fakeStep = fake_runner.NewTestRunner()
		var err error
		workPool, err = workpool.NewWorkPool(numConcurrentSteps)
		Expect(err).NotTo(HaveOccurred())
		step = steps.NewThrottle(fakeStep, workPool)
	})

	AfterEach(func() {
		fakeStep.EnsureExit()
		workPool.Stop()
	})

	Describe("Run", func() {
		It("throttles its substep", func() {
			for i := 0; i < 5; i++ {
				go step.Run(nil, nil)
			}

			Eventually(fakeStep.RunCallCount).Should(Equal(numConcurrentSteps))
			Consistently(fakeStep.RunCallCount).Should(Equal(numConcurrentSteps))

			fakeStep.TriggerExit(nil)
			Eventually(fakeStep.RunCallCount).Should(Equal(numConcurrentSteps + 1))

			fakeStep.TriggerExit(nil)
			Eventually(fakeStep.RunCallCount).Should(Equal(5))

			fakeStep.TriggerExit(nil)
			fakeStep.TriggerExit(nil)
			fakeStep.TriggerExit(nil)
		})

		It("becomes ready once the substep is ready", func() {
			process := ifrit.Background(step)
			Consistently(process.Ready()).ShouldNot(BeClosed())
			fakeStep.TriggerReady()
			Eventually(process.Ready()).Should(BeClosed())
		})

		It("exits with the result of the substep", func() {
			process := ifrit.Background(step)
			Eventually(fakeStep.RunCallCount).Should(Equal(1))
			Consistently(process.Wait()).ShouldNot(Receive())
			fakeStep.TriggerExit(errors.New("substep exited"))
			Eventually(process.Wait()).Should(Receive(MatchError("substep exited")))
		})
	})

	Describe("Signalling", func() {
		It("signals the throttled substep", func() {
			process := ifrit.Background(step)
			Eventually(fakeStep.RunCallCount).Should(Equal(1))
			Consistently(fakeStep.WaitForCall()).ShouldNot(Receive())
			process.Signal(os.Interrupt)
			Eventually(fakeStep.WaitForCall()).Should(Receive())
		})
	})
})
