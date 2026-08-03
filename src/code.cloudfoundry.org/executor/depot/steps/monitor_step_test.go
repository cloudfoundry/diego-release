package steps_test

import (
	"errors"
	"os"
	"time"

	"code.cloudfoundry.org/clock/fakeclock"
	"code.cloudfoundry.org/executor/depot/log_streamer/fake_log_streamer"
	"code.cloudfoundry.org/executor/depot/steps"
	"code.cloudfoundry.org/lager/v3/lagertest"
	"code.cloudfoundry.org/workpool"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/tedsuo/ifrit"
	fake_runner "github.com/tedsuo/ifrit/fake_runner_v2"
)

var _ = Describe("MonitorStep", func() {
	var (
		fakeStep1 *fake_runner.TestRunner
		fakeStep2 *fake_runner.TestRunner

		checkSteps chan ifrit.Runner

		checkFunc    func() ifrit.Runner
		clock        *fakeclock.FakeClock
		fakeStreamer *fake_log_streamer.FakeLogStreamer

		startTimeout      time.Duration
		healthyInterval   time.Duration
		unhealthyInterval time.Duration

		step   ifrit.Runner
		logger *lagertest.TestLogger
	)

	const numOfConcurrentMonitorSteps = 3

	BeforeEach(func() {
		// disable goroutine leak detection for this test suite. It doesn't add much
		// value since MonitorStep is just a wrapper of EventuallySucceedsStep,
		// ConsistentlySucceedsStep & HealthCheckStep. It is also proving difficult to
		// make it pass with all the timer stuff that is going on and the current
		// Context nesting structure
		checkGoroutines = false

		startTimeout = 0
		healthyInterval = 1 * time.Second
		unhealthyInterval = 500 * time.Millisecond

		fakeStep1 = fake_runner.NewTestRunner()
		fakeStep2 = fake_runner.NewTestRunner()

		checkSteps = make(chan ifrit.Runner, 2)
		checkSteps <- fakeStep1
		checkSteps <- fakeStep2

		clock = fakeclock.NewFakeClock(time.Now())

		fakeStreamer = newFakeStreamer()

		checkFunc = func() ifrit.Runner {
			return <-checkSteps
		}

		logger = lagertest.NewTestLogger("test")
	})

	JustBeforeEach(func() {
		workPool, err := workpool.NewWorkPool(numOfConcurrentMonitorSteps)
		Expect(err).NotTo(HaveOccurred())

		fakeStreamer.WithSourceReturns(fakeStreamer)

		step = steps.NewMonitor(
			checkFunc,
			logger,
			clock,
			fakeStreamer,
			startTimeout,
			healthyInterval,
			unhealthyInterval,
			workPool,
		)
	})

	expectCheckAfterInterval := func(fakeStep *fake_runner.TestRunner, d time.Duration) {
		previousCheckCount := fakeStep.RunCallCount()

		clock.Increment(d - 1*time.Microsecond)
		Consistently(fakeStep.RunCallCount, 0.05).Should(Equal(previousCheckCount))

		clock.WaitForWatcherAndIncrement(d)
		Eventually(fakeStep.RunCallCount).Should(Equal(previousCheckCount + 1))
	}

	Describe("Throttling", func() {
		var (
			fakeStep *fake_runner.TestRunner
		)

		BeforeEach(func() {
			fakeStep = fake_runner.NewTestRunner()
			checkFunc = func() ifrit.Runner {
				return fakeStep
			}

		})

		It("throttles concurrent health check", func() {
			for i := 0; i < 5; i++ {
				ifrit.Background(step)
			}

			Consistently(fakeStep.RunCallCount).Should(Equal(0))
			clock.Increment(501 * time.Millisecond)

			Eventually(fakeStep.RunCallCount).Should(Equal(3))
			Consistently(fakeStep.RunCallCount).Should(Equal(3))

			fakeStep.TriggerExit(nil)
			Eventually(fakeStep.RunCallCount).Should(Equal(numOfConcurrentMonitorSteps + 1))

			fakeStep.TriggerExit(nil)
			Eventually(fakeStep.RunCallCount).Should(Equal(numOfConcurrentMonitorSteps + 2))

			for i := 0; i < 3; i++ {
				fakeStep.TriggerExit(nil)
			}
		})
	})

	Describe("Run", func() {
		var (
			process ifrit.Process
		)

		JustBeforeEach(func() {
			process = ifrit.Background(step)
		})

		It("emits a message to the applications log stream", func() {
			Eventually(fakeStreamer.Stdout().(*gbytes.Buffer)).Should(
				gbytes.Say("Starting health monitoring of container\n"),
			)
		})

		Context("when the check succeeds", func() {
			JustBeforeEach(func() {
				go fakeStep1.TriggerExit(nil)
			})

			Context("and the unhealthy interval passes", func() {
				JustBeforeEach(func() {
					expectCheckAfterInterval(fakeStep1, unhealthyInterval)
				})

				It("emits a healthy event", func() {
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

				Context("and later the check begins to fail", func() {
					disaster := errors.New("oh no!")

					BeforeEach(func() {
						go fakeStep2.TriggerExit(disaster)
					})

					Context("and the healthy interval passes", func() {
						JustBeforeEach(func() {
							Eventually(process.Ready()).Should(BeClosed())
							expectCheckAfterInterval(fakeStep2, healthyInterval)
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
							Eventually(fakeStreamer.Stderr().(*gbytes.Buffer)).Should(
								gbytes.Say(disaster.Error()),
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
		})

		Context("when the check is failing immediately", func() {
			var expectedErr error
			BeforeEach(func() {
				expectedErr = errors.New("not up yet!")
				go fakeStep1.TriggerExit(expectedErr)
				go fakeStep2.TriggerExit(expectedErr)
			})

			Context("and the start timeout is exceeded", func() {
				BeforeEach(func() {
					startTimeout = 60 * time.Millisecond
					unhealthyInterval = 30 * time.Millisecond
				})

				It("completes with failure", func() {
					expectCheckAfterInterval(fakeStep1, unhealthyInterval)
					Consistently(process.Wait()).ShouldNot(Receive())
					expectCheckAfterInterval(fakeStep2, unhealthyInterval)
					var err *steps.EmittableError
					Eventually(process.Wait()).Should(Receive(&err))
					Expect(err.WrappedError()).To(MatchError("not up yet!"))
				})

				It("logs the step", func() {
					expectCheckAfterInterval(fakeStep1, unhealthyInterval)
					expectCheckAfterInterval(fakeStep2, unhealthyInterval)
					Eventually(logger.TestSink.LogMessages).Should(ConsistOf([]string{
						"test.health-check-step.timed-out-before-healthy",
					}))
				})

				It("emits the last healthcheck process response to the log stream", func() {
					expectCheckAfterInterval(fakeStep1, unhealthyInterval)
					expectCheckAfterInterval(fakeStep2, unhealthyInterval)
					Eventually(fakeStreamer.Stderr().(*gbytes.Buffer)).Should(
						gbytes.Say(expectedErr.Error()),
					)
				})

				It("emits a log message explaining the failure", func() {
					expectCheckAfterInterval(fakeStep1, unhealthyInterval)
					expectCheckAfterInterval(fakeStep2, unhealthyInterval)
					Eventually(fakeStreamer.Stderr().(*gbytes.Buffer)).Should(gbytes.Say(
						"Failed after .*: startup health check never passed.\n",
					))
				})
			})

			Context("and the unhealthy interval passes", func() {
				JustBeforeEach(func() {
					expectCheckAfterInterval(fakeStep1, unhealthyInterval)
				})

				It("does not emit an unhealthy event", func() {
					Consistently(process.Ready()).ShouldNot(BeClosed())
				})

				It("does not exit", func() {
					Consistently(process.Wait()).ShouldNot(Receive())
				})

				Context("and the unhealthy interval passes again", func() {
					JustBeforeEach(func() {
						expectCheckAfterInterval(fakeStep2, unhealthyInterval)
					})

					It("does not emit an unhealthy event", func() {
						Consistently(process.Ready()).ShouldNot(BeClosed())
					})

					It("does not exit", func() {
						Consistently(process.Wait()).ShouldNot(Receive())
					})
				})
			})
		})
	})

	Describe("Signalling", func() {
		var (
			process ifrit.Process
		)

		It("interrupts the monitoring", func() {
			process = ifrit.Background(step)
			process.Signal(os.Interrupt)
			Eventually(process.Wait()).Should(Receive(Equal(new(steps.CancelledError))))
		})

		Context("while checking", func() {
			It("cancels the in-flight check", func() {
				process = ifrit.Background(step)

				expectCheckAfterInterval(fakeStep1, unhealthyInterval)

				Eventually(fakeStep1.RunCallCount).Should(Equal(1))

				process.Signal(os.Interrupt)

				fakeStep1.TriggerExit(new(steps.CancelledError))

				Eventually(process.Wait()).Should(Receive(Equal(new(steps.CancelledError))))
			})
		})
	})
})
