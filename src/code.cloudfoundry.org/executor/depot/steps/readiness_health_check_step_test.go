package steps_test

import (
	"errors"
	"fmt"
	"os"

	"code.cloudfoundry.org/executor/depot/log_streamer/fake_log_streamer"
	"code.cloudfoundry.org/executor/depot/steps"
	"code.cloudfoundry.org/lager/v3/lagertest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/tedsuo/ifrit"
	fake_runner "github.com/tedsuo/ifrit/fake_runner_v2"
)

var _ = Describe("NewReadinessHealthCheckStep", func() {
	var (
		fakeStreamer                *fake_log_streamer.FakeLogStreamer
		logger                      *lagertest.TestLogger
		untilReadyCheck             *fake_runner.TestRunner
		untilFailureCheck           *fake_runner.TestRunner
		needToKillUntilFailureCheck bool
		needToKillUntilReadyCheck   bool
		process                     ifrit.Process
		readinessChan               chan steps.ReadinessState

		step ifrit.Runner
	)

	BeforeEach(func() {
		fakeStreamer = newFakeStreamer()
		untilReadyCheck = fake_runner.NewTestRunner()
		untilFailureCheck = fake_runner.NewTestRunner()
		readinessChan = make(chan steps.ReadinessState)
		needToKillUntilFailureCheck = false
		needToKillUntilReadyCheck = false
		logger = lagertest.NewTestLogger("test")
	})

	JustBeforeEach(func() {
		step = steps.NewReadinessHealthCheckStep(
			untilReadyCheck,
			untilFailureCheck,
			fakeStreamer,
			readinessChan,
			logger,
		)
		process = ifrit.Background(step)
	})

	AfterEach(func() {
		process.Signal(os.Interrupt)
		if needToKillUntilReadyCheck == true {
			Eventually(untilReadyCheck.RunCallCount).Should(Equal(2))
			Eventually(func() <-chan os.Signal {
				signal, _ := untilReadyCheck.RunArgsForCall(1)
				return signal
			}).Should(Receive(Equal(os.Interrupt)))
			untilReadyCheck.TriggerExit(errors.New("boom"))
		}
		untilReadyCheck.EnsureExit()
		if needToKillUntilFailureCheck == true {
			Eventually(untilFailureCheck.RunCallCount).Should(Equal(1))
			Eventually(func() <-chan os.Signal {
				signal, _ := untilFailureCheck.RunArgsForCall(0)
				return signal
			}).Should(Receive(Equal(os.Interrupt)))
			untilFailureCheck.TriggerExit(errors.New("boom"))
		}
		untilFailureCheck.EnsureExit()
	})

	Describe("Run", func() {
		Context("the untilReady check succeeds", func() {
			BeforeEach(func() {
				needToKillUntilReadyCheck = false
				needToKillUntilFailureCheck = true
			})

			JustBeforeEach(func() {
				Consistently(fakeStreamer.Stdout().(*gbytes.Buffer)).ShouldNot(
					gbytes.Say("Container became ready\n"),
				)

				untilReadyCheck.TriggerExit(nil)
				state := <-readinessChan
				Expect(state).To(Equal(steps.IsReady))
			})

			It("emits a message to the applications log stream", func() {
				Eventually(fakeStreamer.Stdout().(*gbytes.Buffer)).Should(
					gbytes.Say("Starting readiness health monitoring of container\n"),
				)
			})

			It("runs the untilReady check", func() {
				Eventually(untilReadyCheck.RunCallCount).Should(Equal(1))
			})

			It("emits a message to the application log stream", func() {
				Eventually(fakeStreamer.Stdout().(*gbytes.Buffer)).Should(
					gbytes.Say("Container passed the readiness health check. Container marked ready and added to route pool.\n"),
				)
			})

			It("becomes ready", func() {
				Eventually(process.Ready()).Should(BeClosed())
			})

			It("runs the untilFailure check", func() {
				Eventually(untilFailureCheck.RunCallCount).Should(Equal(1))
			})

			Context("when the untilFailure check exits with an error", func() {
				BeforeEach(func() {
					needToKillUntilReadyCheck = true
					needToKillUntilFailureCheck = false
				})

				JustBeforeEach(func() {
					Eventually(untilFailureCheck.RunCallCount).Should(Equal(1))
					untilFailureCheck.TriggerExit(errors.New("crash"))
					state := <-readinessChan
					Expect(state).To(Equal(steps.IsNotReady))
				})

				It("emits a message to the application log stream", func() {
					Eventually(fakeStreamer.Stdout().(*gbytes.Buffer)).Should(
						gbytes.Say("Container failed the readiness health check. Container marked not ready and removed from route pool.\n"),
					)
				})

				It("restarts untilReady check", func() {
					Eventually(untilReadyCheck.RunCallCount).Should(Equal(2))
				})
			})
		})

		Context("the untilReady check fails to run", func() {
			var disaster error
			BeforeEach(func() {
				needToKillUntilFailureCheck = false
				needToKillUntilReadyCheck = false
			})

			JustBeforeEach(func() {
				disaster = errors.New("booom!")
				untilReadyCheck.TriggerExit(disaster)
			})

			It("becomes ready (process is running)", func() {
				Eventually(process.Ready()).Should(BeClosed())
			})

			It("emits a message to the applications log stream", func() {
				Eventually(fakeStreamer.Stdout().(*gbytes.Buffer)).Should(
					gbytes.Say("Starting readiness health monitoring of container\n"),
				)
			})

			It("runs the untilReady check", func() {
				Eventually(untilReadyCheck.RunCallCount).Should(Equal(1))
			})

			It("the step exits with an error", func() {
				var err error
				Eventually(process.Wait()).Should(Receive(&err))
				Expect(err).To(MatchError(ContainSubstring("booom!")))
			})

			It("does not run the untilFailure check", func() {
				Consistently(untilFailureCheck.RunCallCount).Should(Equal(0))
			})

			It("emits a message to the application log stream", func() {
				Eventually(fakeStreamer.Stdout().(*gbytes.Buffer)).Should(
					gbytes.Say("Failed to run the readiness check\n"),
				)
			})
		})
	})

	Describe("Signalling", func() {
		BeforeEach(func() {
			needToKillUntilReadyCheck = false
			needToKillUntilFailureCheck = false
		})

		Context("while doing untilReady check", func() {
			JustBeforeEach(func() {
				Eventually(untilReadyCheck.RunCallCount).Should(Equal(1))
				process.Signal(os.Interrupt)
				Eventually(untilReadyCheck.WaitForCall()).Should(Receive(Equal(os.Interrupt)))
				untilReadyCheck.TriggerExit(nil)
			})

			It("cancels the in-flight check", func() {
				Eventually(process.Wait()).Should(Receive(Equal(new(steps.CancelledError))))
			})

			It("does not run the untilFailure check", func() {
				Consistently(untilFailureCheck.RunCallCount).Should(Equal(0))
			})
		})

		Context("while waiting on the readines channel to receive", func() {
			Context("while doing the untilReadyCheck", func() {
				JustBeforeEach(func() {
					Eventually(untilReadyCheck.RunCallCount).Should(Equal(1))
					untilReadyCheck.TriggerExit(nil)
				})
				It("cancels the in-flight check", func() {
					process.Signal(os.Interrupt)
					Eventually(process.Wait()).Should(Receive(Equal(new(steps.CancelledError))))
				})
			})
			Context("while doing the untilFailureCheck", func() {
				JustBeforeEach(func() {
					untilReadyCheck.TriggerExit(nil)
					state := <-readinessChan
					Expect(state).To(Equal(steps.IsReady))
					Eventually(untilFailureCheck.RunCallCount).Should(Equal(1))
				})

				It("cancels the in-flight check", func() {
					untilFailureCheck.TriggerExit(fmt.Errorf("meow"))
					process.Signal(os.Interrupt)
					Eventually(process.Wait()).Should(Receive(Equal(new(steps.CancelledError))))
				})
			})
		})

		Context("while doing the untilFailure check", func() {
			JustBeforeEach(func() {
				untilReadyCheck.TriggerExit(nil)
				state := <-readinessChan
				Expect(state).To(Equal(steps.IsReady))
				Eventually(untilFailureCheck.RunCallCount).Should(Equal(1))
			})

			It("cancels the in-flight check", func() {
				process.Signal(os.Interrupt)
				Eventually(untilFailureCheck.WaitForCall()).Should(Receive(Equal(os.Interrupt)))
				untilFailureCheck.TriggerExit(fmt.Errorf("meow"))
				Eventually(process.Wait()).Should(Receive(Equal(new(steps.CancelledError))))
			})
		})
	})
})
