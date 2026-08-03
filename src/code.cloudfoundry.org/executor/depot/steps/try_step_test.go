package steps_test

import (
	"errors"
	"os"

	"code.cloudfoundry.org/lager/v3/lagertest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/tedsuo/ifrit"
	fake_runner "github.com/tedsuo/ifrit/fake_runner_v2"

	"code.cloudfoundry.org/executor/depot/steps"
)

var _ = Describe("TryStep", func() {
	var step ifrit.Runner
	var subStep *fake_runner.TestRunner
	var logger *lagertest.TestLogger

	BeforeEach(func() {
		subStep = fake_runner.NewTestRunner()

		logger = lagertest.NewTestLogger("test")
	})

	JustBeforeEach(func() {
		step = steps.NewTry(subStep, logger)
	})

	AfterEach(func() {
		subStep.EnsureExit()
	})

	It("runs its substep", func() {
		ifrit.Background(step)
		Eventually(subStep.RunCallCount).Should(Equal(1))
	})

	Context("when the substep fails", func() {
		disaster := errors.New("oh no!")

		BeforeEach(func() {
			go subStep.TriggerExit(disaster)
		})

		It("succeeds anyway", func() {
			Eventually(ifrit.Invoke(step).Wait()).Should(Receive(BeNil()))
		})

		It("logs the failure", func() {
			Eventually(ifrit.Invoke(step).Wait()).Should(Receive(BeNil()))

			Expect(logger).To(gbytes.Say("failed"))
			Expect(logger).To(gbytes.Say("oh no!"))
		})
	})

	It("does not become ready", func() {
		p := ifrit.Background(step)
		Consistently(p.Ready()).ShouldNot(BeClosed())
	})

	Context("when the substep is ready", func() {
		var (
			p ifrit.Process
		)

		JustBeforeEach(func() {
			p = ifrit.Background(step)
			subStep.TriggerReady()
		})

		It("becomes ready", func() {
			Eventually(p.Ready()).Should(BeClosed())
		})

		It("does not exit", func() {
			Consistently(p.Wait()).ShouldNot(Receive())
		})
	})

	Context("when signalled", func() {
		It("passes the message along", func() {
			p := ifrit.Background(step)
			signals := subStep.WaitForCall()
			Consistently(signals).ShouldNot(Receive())
			p.Signal(os.Interrupt)
			Eventually(signals).Should(Receive())
		})
	})
})
