package steps_test

import (
	"errors"
	"os"

	"code.cloudfoundry.org/executor/depot/steps"
	"code.cloudfoundry.org/lager/v3/lagertest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/tedsuo/ifrit"
	fake_runner "github.com/tedsuo/ifrit/fake_runner_v2"
)

var _ = Describe("BackgroundStep", func() {
	var (
		substepRunError error
		substep         *fake_runner.TestRunner
		logger          *lagertest.TestLogger
		process         ifrit.Process
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("test")
		substep = fake_runner.NewTestRunner()
		process = ifrit.Background(steps.NewBackground(substep, logger))
	})

	Describe("Run", func() {
		Context("when the substep exits", func() {
			JustBeforeEach(func() {
				substep.TriggerExit(substepRunError)
			})

			Context("when the substep returns an error", func() {
				BeforeEach(func() {
					substepRunError = errors.New("some error")
				})

				It("performs the substep", func() {
					Eventually(substep.RunCallCount).Should(Equal(1))
				})

				It("returns this error", func() {
					Eventually(process.Wait()).Should(Receive(Equal(substepRunError)))
				})
			})

			Context("when the substep does not error", func() {
				BeforeEach(func() {
					substepRunError = nil
				})

				It("performs the substep", func() {
					Eventually(substep.RunCallCount).Should(Equal(1))
				})

				It("does not error", func() {
					Eventually(process.Wait()).Should(Receive(BeNil()))
				})
			})
		})

		Context("when the substep does not exit", func() {
			AfterEach(func() {
				substep.EnsureExit()
			})

			It("does not exit", func() {
				Consistently(process.Wait()).ShouldNot(Receive())
			})
		})

		Context("readiness", func() {
			AfterEach(func() {
				substep.EnsureExit()
			})

			It("becomes ready when the substep is ready", func() {
				Consistently(process.Ready()).ShouldNot(BeClosed())
				substep.TriggerReady()
				Eventually(process.Ready()).Should(BeClosed())
			})
		})
	})

	Describe("Signalling", func() {
		AfterEach(func() {
			substep.EnsureExit()
		})

		It("never signals the substep", func() {
			Eventually(substep.RunCallCount).Should(Equal(1))
			process.Signal(os.Interrupt)
			Consistently(substep.WaitForCall()).ShouldNot(Receive())
		})
	})
})
