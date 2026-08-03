package steps_test

import (
	"errors"
	"os"
	"time"

	"code.cloudfoundry.org/clock/fakeclock"
	"code.cloudfoundry.org/executor/depot/log_streamer/fake_log_streamer"
	"code.cloudfoundry.org/executor/depot/steps"
	"code.cloudfoundry.org/lager/v3/lagertest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/tedsuo/ifrit"
	fake_runner "github.com/tedsuo/ifrit/fake_runner_v2"
)

var _ = Describe("NewHealthCheckStep", func() {
	var (
		startupCheck, livenessCheck *fake_runner.TestRunner
		clock                       *fakeclock.FakeClock
		fakeStreamer                *fake_log_streamer.FakeLogStreamer
		fakeHealthCheckStreamer     *fake_log_streamer.FakeLogStreamer

		startTimeout time.Duration

		step    ifrit.Runner
		process ifrit.Process
		logger  *lagertest.TestLogger
	)

	BeforeEach(func() {
		startTimeout = 1 * time.Second

		startupCheck = fake_runner.NewTestRunner()
		livenessCheck = fake_runner.NewTestRunner()

		clock = fakeclock.NewFakeClock(time.Now())

		fakeHealthCheckStreamer = newFakeStreamer()
		fakeStreamer = newFakeStreamer()

		logger = lagertest.NewTestLogger("test")
	})

	JustBeforeEach(func() {
		fakeStreamer.WithSourceReturns(fakeStreamer)

		step = steps.NewHealthCheckStep(
			startupCheck,
			livenessCheck,
			logger,
			clock,
			fakeStreamer,
			fakeHealthCheckStreamer,
			startTimeout,
		)

		process = ifrit.Background(step)
	})

	AfterEach(func() {
		Eventually(startupCheck.RunCallCount).Should(Equal(1))
		startupCheck.EnsureExit()
		if livenessCheck != nil {
			Eventually(livenessCheck.RunCallCount).Should(Equal(1))
			livenessCheck.TriggerExit(errors.New("booom!")) // liveness check must exit with non-nil error
		}
	})

	Describe("Run", func() {
		It("emits a message to the applications log stream", func() {
			Eventually(fakeStreamer.Stdout().(*gbytes.Buffer)).Should(
				gbytes.Say("Starting health monitoring of container\n"),
			)
		})

		Context("when the start upcheck fails", func() {
			JustBeforeEach(func() {
				startupCheck.TriggerExit(errors.New("booom!"))
				livenessCheck = nil
			})

			It("completes with failure", func() {
				var err *steps.EmittableError
				Eventually(process.Wait()).Should(Receive(&err))
				Expect(err.WrappedError()).To(MatchError(ContainSubstring("booom!")))
			})

			It("logs the step", func() {
				Eventually(logger.TestSink.LogMessages).Should(ConsistOf([]string{
					"test.health-check-step.timed-out-before-healthy",
				}))
			})

			It("emits the last healthcheck process response to the log stream", func() {
				Eventually(fakeHealthCheckStreamer.Stderr().(*gbytes.Buffer)).Should(
					gbytes.Say("booom!\n"),
				)
			})

			It("emits a log message explaining the timeout", func() {
				Eventually(fakeStreamer.Stderr().(*gbytes.Buffer)).Should(gbytes.Say(
					"Failed after .*: startup health check never passed.\n",
				))
			})
		})

		Context("when the startup check passes", func() {
			JustBeforeEach(func() {
				startupCheck.TriggerExit(nil)
			})

			It("becomes ready", func() {
				Eventually(process.Ready()).Should(BeClosed())
			})

			It("emits a log message for the success", func() {
				Eventually(fakeStreamer.Stdout().(*gbytes.Buffer)).Should(
					gbytes.Say("Container became healthy\n"),
				)
			})

			It("logs the step", func() {
				Eventually(logger.TestSink.LogMessages).Should(ConsistOf([]string{
					"test.health-check-step.transitioned-to-healthy",
				}))
			})

			Context("and the liveness check fails", func() {
				disaster := errors.New("oh no!")

				JustBeforeEach(func() {
					livenessCheck.TriggerExit(disaster)
					livenessCheck = nil
				})

				It("logs the step", func() {
					Eventually(func() []string { return logger.TestSink.LogMessages() }).Should(ConsistOf([]string{
						"test.health-check-step.transitioned-to-healthy",
						"test.health-check-step.transitioned-to-unhealthy",
					}))
				})

				It("emits a log message for the failure", func() {
					Eventually(fakeStreamer.Stderr().(*gbytes.Buffer)).Should(
						gbytes.Say("Container became unhealthy\n"),
					)
				})

				It("emits the healthcheck process response for the failure", func() {
					Eventually(fakeHealthCheckStreamer.Stderr().(*gbytes.Buffer)).Should(
						gbytes.Say("oh no!\n"),
					)
				})

				It("completes with failure", func() {
					var err *steps.EmittableError
					Eventually(process.Wait()).Should(Receive(&err))
					Expect(err.WrappedError()).To(Equal(disaster))
				})
			})
		})
	})

	Describe("Signalling", func() {
		Context("while doing startup check", func() {
			BeforeEach(func() {
				livenessCheck = nil
			})

			It("cancels the in-flight check", func() {
				Eventually(startupCheck.RunCallCount).Should(Equal(1))

				process.Signal(os.Interrupt)
				Eventually(startupCheck.WaitForCall()).Should(Receive(Equal(os.Interrupt)))
				startupCheck.TriggerExit(nil)
				Eventually(process.Wait()).Should(Receive(Equal(new(steps.CancelledError))))
			})
		})

		Context("when startup check passes", func() {
			Context("and while doing liveness check", func() {
				It("cancels the in-flight check", func() {
					startupCheck.TriggerExit(nil)

					Eventually(livenessCheck.RunCallCount).Should(Equal(1))

					process.Signal(os.Interrupt)
					Eventually(livenessCheck.WaitForCall()).Should(Receive(Equal(os.Interrupt)))
					livenessCheck.TriggerExit(nil)
					livenessCheck = nil
					Eventually(process.Wait()).Should(Receive(Equal(new(steps.CancelledError))))
				})
			})
		})
	})
})
