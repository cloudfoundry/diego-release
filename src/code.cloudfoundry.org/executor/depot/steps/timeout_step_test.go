package steps_test

import (
	"errors"
	"os"
	"time"

	"code.cloudfoundry.org/clock/fakeclock"
	"code.cloudfoundry.org/executor/depot/steps"
	"code.cloudfoundry.org/lager/v3/lagertest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/tedsuo/ifrit"
	fake_runner "github.com/tedsuo/ifrit/fake_runner_v2"
)

var _ = Describe("TimeoutStep", func() {
	var (
		substep    *fake_runner.TestRunner
		substepErr error

		clock *fakeclock.FakeClock

		timeout time.Duration
		logger  *lagertest.TestLogger
	)

	BeforeEach(func() {
		timeout = 100 * time.Millisecond
		clock = fakeclock.NewFakeClock(time.Now())
		substep = fake_runner.NewTestRunner()
		logger = lagertest.NewTestLogger("test")
	})

	AfterEach(func() {
		substep.EnsureExit()
	})

	Describe("Ready", func() {
		It("becomes ready when the substep is ready", func() {
			runner := steps.NewTimeout(substep, timeout, clock, logger)
			p := ifrit.Background(runner)
			Consistently(p.Ready()).ShouldNot(BeClosed())
			substep.TriggerReady()
			Eventually(p.Ready()).Should(BeClosed())
		})
	})

	Describe("Run", func() {
		var (
			err error
			p   ifrit.Process
		)

		JustBeforeEach(func() {
			runner := steps.NewTimeout(substep, timeout, clock, logger)
			p = ifrit.Background(runner)
		})

		Context("When the substep finishes before the timeout expires", func() {
			JustBeforeEach(func() {
				substep.TriggerExit(substepErr)
				err = <-p.Wait()
			})

			Context("when the substep returns an error", func() {
				BeforeEach(func() {
					substepErr = errors.New("some error")
				})

				It("runs the substep", func() {
					Eventually(substep.RunCallCount).Should(Equal(1))
				})

				It("returns this error", func() {
					Expect(err).To(HaveOccurred())
					Expect(err).To(Equal(substepErr))
				})
			})

			Context("when the substep does not error", func() {
				BeforeEach(func() {
					substepErr = nil
				})

				It("runs the substep", func() {
					Eventually(substep.RunCallCount).Should(Equal(1))
				})

				It("does not error", func() {
					Expect(err).NotTo(HaveOccurred())
				})
			})
		})

		Context("When the timeout expires before the substep finishes", func() {
			BeforeEach(func() {
				substepErr = new(steps.CancelledError)
			})

			JustBeforeEach(func() {
				clock.WaitForWatcherAndIncrement(timeout)
			})

			It("signals the sub process", func() {
				signals := substep.WaitForCall()
				Eventually(signals).Should(Receive())
			})

			Context("and the subprocess is signaled", func() {
				JustBeforeEach(func() {
					signals := substep.WaitForCall()
					Eventually(signals).Should(Receive())
					substep.TriggerExit(substepErr)
					err = <-p.Wait()
				})

				It("logs the timeout", func() {
					Eventually(logger.TestSink.LogMessages).Should(ConsistOf([]string{
						"test.timeout-step.timed-out",
					}))
				})

				Context("when the substep does not error", func() {
					BeforeEach(func() {
						substepErr = nil
					})

					It("returns an emittable error", func() {
						Expect(err).To(HaveOccurred())
						Expect(err).To(BeAssignableToTypeOf(&steps.EmittableError{}))
					})
				})

				Context("when the substep returns an error", func() {
					Context("when the error is not emittable", func() {
						BeforeEach(func() {
							substepErr = errors.New("some error")
						})

						It("returns a timeout error which does not include the error returned by the substep", func() {
							Expect(err).To(HaveOccurred())
							Expect(err).To(BeAssignableToTypeOf(&steps.EmittableError{}))
							Expect(err.Error()).NotTo(ContainSubstring("some error"))
							Expect(err.(*steps.EmittableError).WrappedError()).To(Equal(substepErr))
						})
					})

					Context("when the error is emittable", func() {
						BeforeEach(func() {
							substepErr = steps.NewEmittableError(nil, "some error")
						})

						It("returns a timeout error which includes the error returned by the substep", func() {
							Expect(err).To(HaveOccurred())
							Expect(err).To(BeAssignableToTypeOf(&steps.EmittableError{}))
							Expect(err.Error()).To(ContainSubstring("some error"))
							Expect(err.(*steps.EmittableError).WrappedError()).To(Equal(substepErr))
						})
					})
				})
			})
		})
	})

	Describe("Signal", func() {
		It("signals the nested step", func() {
			step := steps.NewTimeout(substep, timeout, clock, logger)
			p := ifrit.Background(step)
			p.Signal(os.Interrupt)

			signals := substep.WaitForCall()
			Eventually(signals).Should(Receive())
		})
	})
})
