package steps_test

import (
	"errors"
	"os"
	"time"

	"code.cloudfoundry.org/clock/fakeclock"
	mfakes "code.cloudfoundry.org/diego-logging-client/testhelpers"
	"code.cloudfoundry.org/executor/depot/steps"
	"code.cloudfoundry.org/lager/v3/lagertest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/tedsuo/ifrit"
	fake_runner "github.com/tedsuo/ifrit/fake_runner_v2"
)

var _ = Describe("TimedStep", func() {
	Describe("Run", func() {
		var (
			innerStep        *fake_runner.TestRunner
			timedStep        ifrit.Runner
			process          ifrit.Process
			logger           *lagertest.TestLogger
			clock            *fakeclock.FakeClock
			fakeMetronClient *mfakes.FakeIngressClient
		)

		BeforeEach(func() {
			logger = lagertest.NewTestLogger("test")
			clock = fakeclock.NewFakeClock(time.Now())
			fakeMetronClient = new(mfakes.FakeIngressClient)
		})

		Context("with inner step set to non-nil process", func() {
			BeforeEach(func() {
				innerStep = fake_runner.NewTestRunner()
			})

			JustBeforeEach(func() {
				timedStep = steps.NewTimedStep(logger, innerStep, fakeMetronClient, clock, clock.Now())
				process = ifrit.Background(timedStep)
			})

			AfterEach(func() {
				innerStep.EnsureExit()
			})

			Context("when step takes time to be ready", func() {
				It("it should close the ready channel when the step is ready", func() {
					innerStep.TriggerReady()
					Eventually(process.Ready()).Should(BeClosed())
					innerStep.TriggerExit(nil)
				})
			})

			Context("when the step runs for some time", func() {
				BeforeEach(func() {
					innerStep.RunStub = func(signals <-chan os.Signal, ready chan<- struct{}) error {
						clock.Increment(1 * time.Second)
						return nil
					}
				})

				It("reports the duration of the given step on the duration channel", func() {
					Eventually(logger).Should(gbytes.Say("container-setup-succeeded.*duration.*1000000000"))
				})

				It("emits metrics when contanier setup succeeds", func() {
					Eventually(fakeMetronClient.SendDurationCallCount).Should(Equal(1))
					name, val, _ := fakeMetronClient.SendDurationArgsForCall(0)
					Expect(name).To(Equal(steps.ContainerSetupSucceededDuration))
					Expect(val).To(Equal(1 * time.Second))
				})
			})

			Context("when the step errors", func() {
				BeforeEach(func() {
					innerStep.RunStub = func(signals <-chan os.Signal, ready chan<- struct{}) error {
						clock.Increment(1 * time.Second)
						return errors.New("failed")
					}
				})

				It("reports the duration with a failed setup message", func() {
					Eventually(logger).Should(gbytes.Say("container-setup-failed.*duration.*1000000000"))
				})

				It("emits metrics when contanier setup fails", func() {
					Eventually(fakeMetronClient.SendDurationCallCount).Should(Equal(1))
					name, val, _ := fakeMetronClient.SendDurationArgsForCall(0)
					Expect(name).To(Equal(steps.ContainerSetupFailedDuration))
					Expect(val).To(Equal(1 * time.Second))
				})
			})
		})

		Context("when the inner step is nil", func() {
			It("should still log the time for container creation", func() {
				startTime := clock.Now().Add(-1 * time.Second)
				timedStep = steps.NewTimedStep(logger, nil, fakeMetronClient, clock, startTime)
				ifrit.Background(timedStep)
				Eventually(logger).Should(gbytes.Say("container-setup-succeeded.*duration.*:1000000000"))
			})

			It("should still emit metrics for container creation", func() {
				startTime := clock.Now().Add(-1 * time.Second)
				timedStep = steps.NewTimedStep(logger, nil, fakeMetronClient, clock, startTime)
				ifrit.Background(timedStep)
				Eventually(fakeMetronClient.SendDurationCallCount).Should(Equal(1))
				name, val, _ := fakeMetronClient.SendDurationArgsForCall(0)
				Expect(name).To(Equal(steps.ContainerSetupSucceededDuration))
				Expect(val).To(Equal(1 * time.Second))
			})
		})
	})
})
