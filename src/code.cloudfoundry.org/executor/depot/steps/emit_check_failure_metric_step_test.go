package steps_test

import (
	"errors"
	"os"

	mfakes "code.cloudfoundry.org/diego-logging-client/testhelpers"
	"code.cloudfoundry.org/executor"
	"code.cloudfoundry.org/executor/depot/steps"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/tedsuo/ifrit"
	fake_runner "github.com/tedsuo/ifrit/fake_runner_v2"
)

var _ = Describe("EmitCheckFailureMetricStep", func() {
	var (
		subStep          *fake_runner.TestRunner
		step             ifrit.Runner
		fakeMetronClient *mfakes.FakeIngressClient
		errorToReturn    error
		checkProtocol    executor.CheckProtocol
		checkType        executor.HealthcheckType
	)

	BeforeEach(func() {
		checkProtocol = executor.HTTPCheck
		checkType = executor.IsLivenessCheck
		fakeMetronClient = new(mfakes.FakeIngressClient)
		errorToReturn = nil
		subStep = fake_runner.NewTestRunner()

		subStep.RunStub = func(signals <-chan os.Signal, ready chan<- struct{}) error {
			return errorToReturn
		}
	})

	AfterEach(func() {
		subStep.EnsureExit()

	})

	JustBeforeEach(func() {
		step = steps.NewEmitCheckFailureMetricStep(subStep, checkProtocol, checkType, fakeMetronClient)
	})

	Describe("Ready", func() {
		var (
			fakeRunner *fake_runner.TestRunner
		)

		Context("when substep is not nil", func() {
			BeforeEach(func() {
				fakeRunner = fake_runner.NewTestRunner()
				subStep = fakeRunner
			})

			It("becomes ready when the substep is ready", func() {
				p := ifrit.Background(step)
				Consistently(p.Ready()).ShouldNot(BeClosed())
				fakeRunner.TriggerReady()
				Eventually(p.Ready()).Should(BeClosed())
			})
		})
	})

	Describe("Running", func() {
		var (
			err error
		)

		JustBeforeEach(func() {
			err = <-ifrit.Invoke(step).Wait()
		})

		Context("when the substep succeeds", func() {
			It("should not emit any metric", func() {
				Expect(err).NotTo(HaveOccurred())
				Eventually(fakeMetronClient.IncrementCounterCallCount).Should(Equal(0))
			})
		})

		Context("when the substep fails", func() {
			BeforeEach(func() {
				errorToReturn = errors.New("check failed")
			})

			It("should pass the error along", func() {
				Expect(err).To(MatchError(errorToReturn))
			})

			Context("when HealthCheckType is Liveness and CheckProtocol is HTTP", func() {
				BeforeEach(func() {
					checkType = executor.IsLivenessCheck
					checkProtocol = executor.HTTPCheck
				})

				It("should emit metric for the correct health check protocol and type", func() {
					Eventually(fakeMetronClient.IncrementCounterCallCount).Should(Equal(1))
					name := fakeMetronClient.IncrementCounterArgsForCall(0)
					Expect(name).To(Equal("HTTPLivenessChecksFailedCount"))
				})
			})

			Context("when HealthCheckType is Liveness and CheckProtocol is TCP", func() {
				BeforeEach(func() {
					checkType = executor.IsLivenessCheck
					checkProtocol = executor.TCPCheck
				})

				It("should emit metric for the correct health check protocol and type", func() {
					Eventually(fakeMetronClient.IncrementCounterCallCount).Should(Equal(1))
					name := fakeMetronClient.IncrementCounterArgsForCall(0)
					Expect(name).To(Equal("TCPLivenessChecksFailedCount"))
				})
			})

			Context("when HealthCheckType is not Liveness", func() {
				BeforeEach(func() {
					checkType = executor.IsUntilFailureReadinessCheck
					checkProtocol = executor.TCPCheck
				})

				It("should not emit any metric", func() {
					Eventually(fakeMetronClient.IncrementCounterCallCount).Should(Equal(0))
				})
			})
		})
	})

	Describe("Signal", func() {
		var (
			p        ifrit.Process
			finished chan struct{}
		)

		BeforeEach(func() {
			finished = make(chan struct{})
			subStep.RunStub = func(signals <-chan os.Signal, ready chan<- struct{}) error {
				<-signals
				close(finished)
				return new(steps.CancelledError)
			}
		})

		JustBeforeEach(func() {
			p = ifrit.Background(step)
		})

		It("should pass the signal", func() {
			Consistently(finished).ShouldNot(BeClosed())
			p.Signal(os.Interrupt)
			Eventually(finished).Should(BeClosed())
		})

		It("should not emit metric when cancelled", func() {
			p.Signal(os.Interrupt)
			Eventually(fakeMetronClient.IncrementCounterCallCount).Should(Equal(0))
		})
	})
})
