package steps_test

import (
	"bytes"
	"errors"
	"os"

	"code.cloudfoundry.org/lager/v3/lagertest"

	"code.cloudfoundry.org/executor/depot/log_streamer/fake_log_streamer"

	"code.cloudfoundry.org/executor/depot/steps"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/tedsuo/ifrit"
	fake_runner "github.com/tedsuo/ifrit/fake_runner_v2"
)

var _ = Describe("EmitProgressStep", func() {
	var step ifrit.Runner
	var subStep *fake_runner.FakeRunner
	var errorToReturn error
	var fakeStreamer *fake_log_streamer.FakeLogStreamer
	var startMessage, successMessage, failureMessage string
	var logger *lagertest.TestLogger
	var stderrBuffer *bytes.Buffer
	var stdoutBuffer *bytes.Buffer

	BeforeEach(func() {
		stderrBuffer = new(bytes.Buffer)
		stdoutBuffer = new(bytes.Buffer)
		errorToReturn = nil
		startMessage, successMessage, failureMessage = "", "", ""
		fakeStreamer = new(fake_log_streamer.FakeLogStreamer)

		fakeStreamer.StderrReturns(stderrBuffer)
		fakeStreamer.StdoutReturns(stdoutBuffer)

		subStep = &fake_runner.FakeRunner{}

		subStep.RunStub = func(signals <-chan os.Signal, ready chan<- struct{}) error {
			fakeStreamer.Stdout().Write([]byte("RUNNING\n"))
			return errorToReturn
		}

		logger = lagertest.NewTestLogger("test")
	})

	JustBeforeEach(func() {
		step = steps.NewEmitProgress(subStep, startMessage, successMessage, failureMessage, fakeStreamer, logger)
	})

	Context("Ready", func() {
		var (
			fakeRunner *fake_runner.TestRunner
		)

		BeforeEach(func() {
			fakeRunner = fake_runner.NewTestRunner()
			subStep = fakeRunner.FakeRunner
		})

		AfterEach(func() {
			fakeRunner.EnsureExit()
		})

		It("becomes ready when the substep is ready", func() {
			p := ifrit.Background(step)
			Consistently(p.Ready()).ShouldNot(BeClosed())
			fakeRunner.TriggerReady()
			Eventually(p.Ready()).Should(BeClosed())
		})
	})

	Context("running", func() {
		var (
			err error
		)

		JustBeforeEach(func() {
			err = <-ifrit.Invoke(step).Wait()
		})

		Context("when there is a start message", func() {
			BeforeEach(func() {
				startMessage = "STARTING"
			})

			It("should emit the start message before performing", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(stdoutBuffer.String()).To(Equal("STARTING\nRUNNING\n"))
			})
		})

		Context("when there is no start or success message", func() {
			It("should not emit the start message (i.e. a newline) before performing", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(stdoutBuffer.String()).To(Equal("RUNNING\n"))
			})
		})

		Context("when the substep succeeds and there is a success message", func() {
			BeforeEach(func() {
				successMessage = "SUCCESS"
			})

			It("should emit the sucess message", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(stdoutBuffer.String()).To(Equal("RUNNING\nSUCCESS\n"))
			})
		})

		Context("when the substep fails", func() {
			BeforeEach(func() {
				errorToReturn = errors.New("bam!")
			})

			It("should pass the error along", func() {
				Expect(err).To(MatchError(errorToReturn))
			})

			Context("and there is a failure message", func() {
				BeforeEach(func() {
					failureMessage = "FAIL"
				})

				It("should emit the failure message", func() {

					Expect(stdoutBuffer.String()).To(Equal("RUNNING\n"))
					Expect(stderrBuffer.String()).To(Equal("FAIL\n"))
				})

				Context("with an emittable error", func() {
					BeforeEach(func() {
						errorToReturn = steps.NewEmittableError(errors.New("bam!"), "Failed to reticulate")
					})

					It("should print out the emittable error", func() {

						Expect(stdoutBuffer.String()).To(Equal("RUNNING\n"))
						Expect(stderrBuffer.String()).To(Equal("FAIL: Failed to reticulate\n"))
					})

					It("logs the error", func() {

						logs := logger.TestSink.Logs()
						Expect(logs).To(HaveLen(1))

						Expect(logs[0].Message).To(ContainSubstring("errored"))
						Expect(logs[0].Data["wrapped-error"]).To(Equal("bam!"))
						Expect(logs[0].Data["message-emitted"]).To(Equal("Failed to reticulate"))
					})

					Context("without a wrapped error", func() {
						BeforeEach(func() {
							errorToReturn = steps.NewEmittableError(nil, "Failed to reticulate")
						})

						It("should print out the emittable error", func() {

							Expect(stdoutBuffer.String()).To(Equal("RUNNING\n"))
							Expect(stderrBuffer.String()).To(Equal("FAIL: Failed to reticulate\n"))
						})

						It("logs the error", func() {
							logs := logger.TestSink.Logs()
							Expect(logs).To(HaveLen(1))

							Expect(logs[0].Message).To(ContainSubstring("errored"))
							Expect(logs[0].Data["wrapped-error"]).To(BeEmpty())
							Expect(logs[0].Data["message-emitted"]).To(Equal("Failed to reticulate"))
						})
					})
				})
			})

			Context("and there is no failure message", func() {
				BeforeEach(func() {
					errorToReturn = steps.NewEmittableError(errors.New("bam!"), "Failed to reticulate")
				})

				It("should not emit the failure message or error, even with an emittable error", func() {
					Expect(stdoutBuffer.String()).To(Equal("RUNNING\n"))
					Expect(stderrBuffer.String()).To(BeEmpty())
				})
			})
		})
	})

	Context("Signal", func() {
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

		It("passes the message along", func() {
			Consistently(finished).ShouldNot(BeClosed())
			p.Signal(os.Interrupt)
			Eventually(finished).Should(BeClosed())
		})
	})
})
