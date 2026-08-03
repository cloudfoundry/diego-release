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

var _ = Describe("ParallelStep", func() {
	var (
		step    ifrit.Runner
		process ifrit.Process

		subStep1 *fake_runner.TestRunner
		subStep2 *fake_runner.TestRunner
	)

	BeforeEach(func() {
		subStep1 = fake_runner.NewTestRunner()
		subStep2 = fake_runner.NewTestRunner()
	})

	JustBeforeEach(func() {
		step = steps.NewParallel([]ifrit.Runner{subStep1, subStep2})
		process = ifrit.Background(step)
	})

	AfterEach(func() {
		subStep1.EnsureExit()
		subStep2.EnsureExit()
	})

	It("performs its substeps in parallel", func() {
		Eventually(subStep1.RunCallCount).Should(Equal(1))
		Eventually(subStep2.RunCallCount).Should(Equal(1))
		subStep1.TriggerExit(nil)
		subStep2.TriggerExit(nil)

		Eventually(process.Wait()).Should(Receive(BeNil()))
	})

	Context("when multiple substeps fail", func() {
		disaster1 := errors.New("oh no")
		disaster2 := errors.New("oh my")

		It("joins the error messages with a semicolon", func() {
			subStep1.TriggerExit(disaster1)
			subStep2.TriggerExit(disaster2)

			var err error
			Eventually(process.Wait()).Should(Receive(&err))
			Expect(err).To(HaveOccurred())
			errMsg := err.Error()
			Expect(errMsg).NotTo(HavePrefix(";"))
			Expect(errMsg).To(ContainSubstring("oh no"))
			Expect(errMsg).To(ContainSubstring("oh my"))
			Expect(errMsg).To(MatchRegexp(`\w+; \w+`))
		})

		Context("when step fails with a non displayable error", func() {
			It("does not add non displayable error to message", func() {
				nonDisplayableErr := new(NonDisplayableError)
				subStep1.TriggerExit(nonDisplayableErr)
				subStep2.TriggerExit(disaster2)

				var err error
				Eventually(process.Wait()).Should(Receive(&err))
				Expect(err).To(HaveOccurred())
				errMsg := err.Error()
				Expect(errMsg).NotTo(HavePrefix(";"))
				Expect(errMsg).To(ContainSubstring("oh my"))
				Expect(errMsg).NotTo(ContainSubstring("some-non-displaybale-error"))
			})
		})
	})

	Context("when first substep fails", func() {
		disaster := errors.New("oh no!")

		It("waits for the rest to finish", func() {
			subStep1.TriggerExit(disaster)

			Consistently(process.Wait()).ShouldNot(Receive())

			subStep2.TriggerExit(nil)

			var err error
			Eventually(process.Wait()).Should(Receive(&err))
			Expect(err.(*multierror.Error).WrappedErrors()).To(ConsistOf(disaster))
		})
	})

	Context("when told to cancel", func() {
		It("cancels its substeps", func() {
			process.Signal(os.Interrupt)

			Eventually(subStep1.WaitForCall()).Should(Receive(Equal(os.Interrupt)))
			Eventually(subStep2.WaitForCall()).Should(Receive(Equal(os.Interrupt)))
		})
	})

	Describe("process readiness", func() {
		It("does not become ready until its subprocesses are", func() {
			Consistently(process.Ready()).ShouldNot(BeClosed())

			subStep1.TriggerReady()
			Consistently(process.Ready()).ShouldNot(BeClosed())

			subStep2.TriggerReady()
			Eventually(process.Ready()).Should(BeClosed())
		})

		It("never becomes ready if a subprocess exits without becoming ready", func() {
			Consistently(process.Ready()).ShouldNot(BeClosed())

			subStep1.TriggerReady()
			Consistently(process.Ready()).ShouldNot(BeClosed())

			subStep2.TriggerExit(errors.New("some error"))
			Consistently(process.Ready()).ShouldNot(BeClosed())
		})
	})
})
