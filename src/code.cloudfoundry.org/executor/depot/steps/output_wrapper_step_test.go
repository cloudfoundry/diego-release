package steps_test

import (
	"bytes"
	"errors"
	"os"

	"code.cloudfoundry.org/executor/depot/steps"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/tedsuo/ifrit"
	fake_runner "github.com/tedsuo/ifrit/fake_runner_v2"
)

var _ = Describe("OutputWrapperStep", func() {
	var (
		subStep *fake_runner.TestRunner
		step    ifrit.Runner
		buffer  *bytes.Buffer
	)

	BeforeEach(func() {
		subStep = fake_runner.NewTestRunner()
		buffer = bytes.NewBuffer(nil)
		step = steps.NewOutputWrapper(subStep, buffer)
	})

	AfterEach(func() {
		subStep.EnsureExit()
	})

	Context("Ready", func() {
		It("becomes ready when the substep is ready", func() {
			p := ifrit.Background(step)
			Consistently(p.Ready()).ShouldNot(BeClosed())
			subStep.TriggerReady()
			Eventually(p.Ready()).Should(BeClosed())
		})
	})

	Context("Run", func() {
		It("calls perform on the substep", func() {
			ifrit.Background(step)
			Eventually(subStep.RunCallCount).Should(Equal(1))
		})

		Context("when the substep fails", func() {
			var (
				errStr string
			)

			BeforeEach(func() {
				go subStep.TriggerExit(errors.New("BOOOM!"))
				errStr = "error reason"
			})

			JustBeforeEach(func() {
				buffer.WriteString(errStr)
			})

			It("wraps the buffer content in an emittable error", func() {
				p := ifrit.Background(step)
				Eventually(p.Wait()).Should(Receive(MatchError("error reason")))
			})

			Context("when the output has whitespaces", func() {
				BeforeEach(func() {
					errStr = "\r\nerror reason\r\n"
				})

				It("trims the extra whitespace", func() {
					p := ifrit.Background(step)
					Eventually(p.Wait()).Should(Receive(MatchError("error reason")))
				})
			})
		})

		Context("when the substep is cancelled", func() {
			BeforeEach(func() {
				go subStep.TriggerExit(new(steps.CancelledError))
			})

			It("returns the CancelledError error", func() {
				p := ifrit.Background(step)
				Eventually(p.Wait()).Should(Receive(MatchError(steps.NewEmittableError(new(steps.CancelledError), "Failed to invoke process: cancelled"))))
			})

			Context("and the buffer has data", func() {
				BeforeEach(func() {
					buffer.WriteString("error reason")
				})

				It("wraps the buffer content in an emittable error", func() {
					p := ifrit.Background(step)
					Eventually(p.Wait()).Should(Receive(MatchError("error reason")))
				})
			})
		})
	})

	Context("Signal", func() {
		It("calls Signal on the substep", func() {
			ch := make(chan struct{})
			defer close(ch)

			p := ifrit.Background(step)
			p.Signal(os.Interrupt)
			signals := subStep.WaitForCall()
			Eventually(signals).Should(Receive())
		})
	})
})
