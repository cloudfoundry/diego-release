package steps_test

import (
	"errors"
	"os"

	"github.com/hashicorp/go-multierror"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/tedsuo/ifrit"
	fake_runner "github.com/tedsuo/ifrit/fake_runner_v2"

	"code.cloudfoundry.org/executor/depot/steps"
)

var _ = Describe("CodependentStep", func() {
	var (
		step    ifrit.Runner
		process ifrit.Process

		subStep1 *fake_runner.TestRunner
		subStep2 *fake_runner.TestRunner

		errorOnExit        bool
		cancelOthersOnExit bool
	)

	BeforeEach(func() {
		errorOnExit = false
		cancelOthersOnExit = false

		subStep1 = fake_runner.NewTestRunner()
		subStep2 = fake_runner.NewTestRunner()
	})

	AfterEach(func() {
		subStep1.EnsureExit()
		subStep2.EnsureExit()
	})

	JustBeforeEach(func() {
		step = steps.NewCodependent([]ifrit.Runner{subStep1, subStep2}, errorOnExit, cancelOthersOnExit)
		process = ifrit.Background(step)
		Eventually(subStep1.RunCallCount).Should(Equal(1))
		Eventually(subStep2.RunCallCount).Should(Equal(1))
	})

	Describe("Run", func() {
		It("runs its substeps in parallel", func() {
			subStep1.TriggerExit(nil)
			subStep2.TriggerExit(nil)

			Eventually(process.Wait()).Should(Receive(BeNil()))
		})

		Context("when one of the substeps fails", func() {
			It("signals remaining steps and returns an aggregate of the failures", func() {
				disaster := errors.New("oh no!")
				subStep1.TriggerExit(disaster)

				stepErrorsWhenSignalled(subStep2)
				var err *multierror.Error
				Eventually(process.Wait()).Should(Receive(&err))
				Expect(err.WrappedErrors()).To(ConsistOf(disaster, new(steps.CancelledError)))
			})
		})

		Context("when step fails with a non displayable error", func() {
			It("does not add non displayable error to message", func() {
				disaster := errors.New("oh no!")

				subStep1.TriggerExit(disaster)

				nonDisplayableErr := new(NonDisplayableError)
				subStep2.TriggerExit(nonDisplayableErr)

				var err *multierror.Error
				Eventually(process.Wait()).Should(Receive(&err))
				Expect(err.Error()).To(Equal("oh no!"))
			})
		})

		Context("when one of the substeps exits without failure", func() {
			It("continues to run the other step", func() {
				subStep1.TriggerExit(nil)
				Consistently(process.Wait()).ShouldNot(Receive())
			})

			Context("when cancelOthersOnExit is set to true", func() {
				BeforeEach(func() {
					cancelOthersOnExit = true
				})

				It("should cancel all other steps", func() {
					subStep1.TriggerExit(nil)
					stepErrorsWhenSignalled(subStep2)

					Eventually(process.Wait()).Should(Receive())
				})
			})

			Context("when errorOnExit is set to true", func() {
				BeforeEach(func() {
					errorOnExit = true
				})

				It("signals all other steps and returns an aggregate of the failures once they exit", func() {
					subStep1.TriggerExit(nil)
					stepErrorsWhenSignalled(subStep2)

					var err *multierror.Error
					Eventually(process.Wait()).Should(Receive(&err))
					Expect(err.WrappedErrors()).To(ConsistOf(steps.CodependentStepExitedError, new(steps.CancelledError)))
				})

				It("should cancel all other steps regardless of the step that failed", func() {
					subStep2.TriggerExit(nil)
					stepErrorsWhenSignalled(subStep1)
					Eventually(process.Wait()).Should(Receive())
				})
			})
		})

		Context("when multiple substeps fail", func() {
			It("joins the error messages with a semicolon", func() {
				disaster1 := errors.New("oh no")
				disaster2 := errors.New("oh my")

				subStep1.TriggerExit(disaster1)
				subStep2.TriggerExit(disaster2)

				var err *multierror.Error
				Eventually(process.Wait()).Should(Receive(&err))
				errMsg := err.Error()
				Expect(errMsg).NotTo(HavePrefix(";"))
				Expect(errMsg).To(ContainSubstring("oh no"))
				Expect(errMsg).To(ContainSubstring("oh my"))
				Expect(errMsg).To(MatchRegexp(`\w+; \w+`))
			})
		})
	})

	Describe("Ready", func() {
		It("becomes ready only when all substeps are ready", func() {
			Consistently(process.Ready()).ShouldNot(BeClosed())
			subStep1.TriggerReady()
			Consistently(process.Ready()).ShouldNot(BeClosed())
			subStep2.TriggerReady()
			Eventually(process.Ready()).Should(BeClosed())
		})
	})

	Describe("Signal", func() {
		It("signals all sub-steps", func() {
			Consistently(process.Wait()).ShouldNot(Receive())

			process.Signal(os.Interrupt)

			stepErrorsWhenSignalled(subStep1)
			stepErrorsWhenSignalled(subStep2)

			Eventually(process.Wait()).Should(Receive())
		})
	})
})

func stepErrorsWhenSignalled(step *fake_runner.TestRunner) {
	Eventually(step.WaitForCall()).Should(Receive())
	step.TriggerExit(new(steps.CancelledError))
}
