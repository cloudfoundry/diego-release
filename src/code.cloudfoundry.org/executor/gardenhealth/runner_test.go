package gardenhealth_test

import (
	"errors"
	"os"
	"sync"
	"time"

	"code.cloudfoundry.org/executor/gardenhealth"
	loggregator "code.cloudfoundry.org/go-loggregator/v9"
	"github.com/tedsuo/ifrit"
	ginkgomon "github.com/tedsuo/ifrit/ginkgomon_v2"

	"code.cloudfoundry.org/clock/fakeclock"
	mfakes "code.cloudfoundry.org/diego-logging-client/testhelpers"
	fakeexecutor "code.cloudfoundry.org/executor/fakes"
	"code.cloudfoundry.org/executor/gardenhealth/fakegardenhealth"
	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/lager/v3/lagertest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Runner", func() {
	var (
		runner                          *gardenhealth.Runner
		process                         ifrit.Process
		logger                          *lagertest.TestLogger
		checker                         *fakegardenhealth.FakeChecker
		executorClient                  *fakeexecutor.FakeClient
		fakeClock                       *fakeclock.FakeClock
		fakeMetronClient                *mfakes.FakeIngressClient
		checkInterval, emissionInterval time.Duration
		timeoutDuration                 time.Duration
		metricMap                       map[string]float64
		m                               sync.RWMutex
	)

	const GardenHealthCheckFailed = "GardenHealthCheckFailed"
	const CellUnhealthyMetric = "CellUnhealthy"

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("test")
		checker = &fakegardenhealth.FakeChecker{}
		executorClient = &fakeexecutor.FakeClient{}
		fakeClock = fakeclock.NewFakeClock(time.Now())
		checkInterval = 2 * time.Minute
		timeoutDuration = 1 * time.Minute
		emissionInterval = 30 * time.Second

		fakeMetronClient = new(mfakes.FakeIngressClient)

		m = sync.RWMutex{}
	})

	getMetrics := func() map[string]float64 {
		m.Lock()
		defer m.Unlock()
		m := make(map[string]float64, len(metricMap))
		for k, v := range metricMap {
			m[k] = v
		}
		return m
	}

	JustBeforeEach(func() {
		metricMap = make(map[string]float64)
		fakeMetronClient.SendMetricStub = func(name string, value int, opts ...loggregator.EmitGaugeOption) error {
			m.Lock()
			metricMap[name] = float64(value)
			m.Unlock()
			return nil
		}

		runner = gardenhealth.NewRunner(checkInterval, emissionInterval, timeoutDuration, logger, checker, executorClient, fakeMetronClient, fakeClock)
		process = ifrit.Background(runner)

	})

	AfterEach(func() {
		ginkgomon.Interrupt(process)
	})

	Describe("Run", func() {
		Context("When garden is immediately unhealthy", func() {
			Context("because the health check fails with a transient error", func() {
				var checkErr = errors.New("transient")
				BeforeEach(func() {
					checker.HealthcheckReturns(checkErr)
					executorClient.HealthyReturns(false)
				})

				It("becomes ready immediately and retries until the initial timeout fires", func() {
					Eventually(process.Ready()).Should(BeClosed())
					// Transient errors no longer cause an immediate fatal exit; the
					// runner retries while marking the cell unhealthy.
					Consistently(process.Wait()).ShouldNot(Receive())
					// Only exits (with HealthcheckTimeoutError) once the timeout elapses.
					fakeClock.WaitForWatcherAndIncrement(timeoutDuration)
					Eventually(process.Wait()).Should(Receive(Equal(gardenhealth.HealthcheckTimeoutError{})))
				})

				It("emits a metric for unhealthy cell", func() {
					// setUnhealthy is called on each failed retry attempt.
					Eventually(getMetrics).Should(HaveKeyWithValue(GardenHealthCheckFailed, float64(1)))
				})

				It("emits the CellUnhealthy metric when it times out", func() {
					fakeClock.WaitForWatcherAndIncrement(timeoutDuration)
					Eventually(process.Wait()).Should(Receive(Equal(gardenhealth.HealthcheckTimeoutError{})))
					Eventually(getMetrics).Should(HaveKeyWithValue(CellUnhealthyMetric, float64(1)))
				})
			})

			Context("because the health check fails with an unrecoverable error", func() {
				var checkErr = gardenhealth.UnrecoverableError("nope")
				BeforeEach(func() {
					checker.HealthcheckReturns(checkErr)
					executorClient.HealthyReturns(false)
				})

				It("becomes ready immediately and exits immediately with the error", func() {
					Eventually(process.Ready()).Should(BeClosed())
					Eventually(process.Wait()).Should(Receive(Equal(checkErr)))
				})

				It("emits a metric for unhealthy cell", func() {
					Eventually(process.Wait()).Should(Receive(Equal(checkErr)))
					Eventually(getMetrics).Should(HaveKeyWithValue(GardenHealthCheckFailed, float64(1)))
				})
			})

			Context("because the health check timed out", func() {
				var blockHealthcheck chan struct{}

				BeforeEach(func() {
					blockHealthcheck = make(chan struct{})
					checker.HealthcheckStub = func(lager.Logger) error {
						<-blockHealthcheck
						return nil
					}
				})

				JustBeforeEach(func() {
					fakeClock.WaitForWatcherAndIncrement(timeoutDuration)
				})

				AfterEach(func() {
					// Send to the channel to unblock the goroutine after the runner has exited
					blockHealthcheck <- struct{}{}
					close(blockHealthcheck)
					blockHealthcheck = nil
				})

				It("becomes ready immediately then exits with timeout error", func() {
					Eventually(process.Ready()).Should(BeClosed())
					Eventually(process.Wait()).Should(Receive(Equal(gardenhealth.HealthcheckTimeoutError{})))
				})

				It("emits a metric for unhealthy cell", func() {
					Eventually(process.Wait()).Should(Receive(Equal(gardenhealth.HealthcheckTimeoutError{})))
					Eventually(getMetrics).Should(HaveKeyWithValue(GardenHealthCheckFailed, float64(1)))
				})

				It("emits the CellUnhealthy metric", func() {
					Eventually(process.Wait()).Should(Receive(Equal(gardenhealth.HealthcheckTimeoutError{})))
					Eventually(getMetrics).Should(HaveKeyWithValue(CellUnhealthyMetric, float64(1)))
				})

				It("cancels the existing health check", func() {
					Eventually(process.Wait()).Should(Receive(Equal(gardenhealth.HealthcheckTimeoutError{})))
					Eventually(checker.CancelCallCount).Should(Equal(1))
				})
			})
		})

		Context("When garden is healthy", func() {
			BeforeEach(func() {
				executorClient.HealthyReturns(true)
			})

			It("sets healthy to true only once", func() {
				Eventually(executorClient.SetHealthyCallCount).Should(Equal(1))
				_, healthy := executorClient.SetHealthyArgsForCall(0)
				Expect(healthy).Should(Equal(true))
				Expect(executorClient.SetHealthyCallCount()).To(Equal(1))
			})

			It("continues to check at the correct interval", func() {
				Eventually(checker.HealthcheckCallCount).Should(Equal(1))
				Eventually(process.Ready()).Should(BeClosed())

				fakeClock.WaitForNWatchersAndIncrement(checkInterval, 2)
				Eventually(checker.HealthcheckCallCount).Should(Equal(2))

				fakeClock.WaitForNWatchersAndIncrement(checkInterval, 2)
				Eventually(checker.HealthcheckCallCount).Should(Equal(3))

				fakeClock.WaitForNWatchersAndIncrement(checkInterval, 2)
				Eventually(checker.HealthcheckCallCount).Should(Equal(4))
			})

			It("emits a metric for healthy cell", func() {
				Eventually(executorClient.SetHealthyCallCount).Should(Equal(1))
				Eventually(getMetrics).Should(HaveKeyWithValue(GardenHealthCheckFailed, float64(0)))
			})
		})

		Context("when garden is intermittently healthy", func() {
			var (
				checkValues   chan error
				healthyValues chan bool
			)

			BeforeEach(func() {
				healthyValues = make(chan bool, 1)
				checkValues = make(chan error, 1)
				executorClient.HealthyStub = func(lager.Logger) bool {
					return <-healthyValues
				}
				checker.HealthcheckStub = func(lager.Logger) error {
					return <-checkValues
				}

				Expect(healthyValues).To(BeSent(true))
				checkValues <- nil

				// Set emission interval to a high value so that it doesn't trigger in this test
				emissionInterval = 100 * time.Minute
			})

			It("Sets healthy to false after it fails, then to true after success and emits respective metrics", func() {
				Eventually(executorClient.SetHealthyCallCount).Should(Equal(1))
				_, healthy := executorClient.SetHealthyArgsForCall(0)
				Expect(healthy).Should(Equal(true))
				Eventually(getMetrics).Should(HaveKeyWithValue(GardenHealthCheckFailed, float64(0)))

				Expect(healthyValues).To(BeSent(false))
				checkValues <- errors.New("boom")
				fakeClock.WaitForWatcherAndIncrement(checkInterval)

				Eventually(executorClient.SetHealthyCallCount).Should(Equal(2))
				_, healthy = executorClient.SetHealthyArgsForCall(1)
				Expect(healthy).Should(Equal(false))
				Eventually(getMetrics).Should(HaveKeyWithValue(GardenHealthCheckFailed, float64(1)))

				Expect(healthyValues).To(BeSent(true))
				checkValues <- nil
				fakeClock.WaitForNWatchersAndIncrement(checkInterval, 2)

				Eventually(executorClient.SetHealthyCallCount).Should(Equal(3))
				_, healthy = executorClient.SetHealthyArgsForCall(2)
				Expect(healthy).Should(Equal(true))
				Eventually(getMetrics).Should(HaveKeyWithValue(GardenHealthCheckFailed, float64(0)))
			})
		})

		Context("When the healthcheck times out", func() {
			var blockHealthcheck chan struct{}

			BeforeEach(func() {
				blockHealthcheck = make(chan struct{})
				checker.HealthcheckStub = func(lager.Logger) error {
					logger.Info("blocking")
					<-blockHealthcheck
					logger.Info("unblocking")
					return nil
				}
			})

			AfterEach(func() {
				close(blockHealthcheck)
			})

			JustBeforeEach(func() {
				Eventually(blockHealthcheck).Should(BeSent(struct{}{}))
				Eventually(executorClient.SetHealthyCallCount).Should(Equal(1))

				fakeClock.WaitForNWatchersAndIncrement(checkInterval, 2)
				Eventually(checker.HealthcheckCallCount).Should(Equal(2))

				fakeClock.WaitForNWatchersAndIncrement(timeoutDuration, 2)
			})

			It("sets the executor to unhealthy and emits the unhealthy metric", func() {
				Eventually(executorClient.SetHealthyCallCount).Should(Equal(2))
				_, healthy := executorClient.SetHealthyArgsForCall(1)
				Expect(healthy).Should(Equal(false))
				Eventually(getMetrics).Should(HaveKeyWithValue(GardenHealthCheckFailed, float64(1)))
			})

			It("cancels the existing health check", func() {
				Eventually(checker.CancelCallCount).Should(Equal(1))
			})

			It("emits the CellUnhealthy metric", func() {
				Eventually(getMetrics).Should(HaveKeyWithValue(CellUnhealthyMetric, float64(1)))
			})
		})

		Context("When the runner is signalled", func() {
			Context("during the initial health check", func() {
				var blockHealthcheck chan struct{}

				BeforeEach(func() {
					blockHealthcheck = make(chan struct{})
					checker.HealthcheckStub = func(lager.Logger) error {
						<-blockHealthcheck
						return nil
					}
				})

				JustBeforeEach(func() {
					process.Signal(os.Interrupt)
				})

				It("exits with no error", func() {
					Eventually(process.Wait()).Should(Receive(BeNil()))
				})
			})

			Context("After the initial health check", func() {
				It("exits imediately with no error", func() {
					Eventually(executorClient.SetHealthyCallCount).Should(Equal(1))

					process.Signal(os.Interrupt)
					Eventually(process.Wait()).Should(Receive(BeNil()))
				})
			})
		})

		Context("GardenHealthCheckFailed metric emission", func() {
			It("emits the GardenHealthCheckFailed metric every emitInterval", func() {
				Eventually(executorClient.HealthyCallCount).Should(Equal(1))
				Eventually(getMetrics).Should(HaveKeyWithValue(GardenHealthCheckFailed, float64(1)))

				executorClient.HealthyReturns(true)
				fakeClock.WaitForWatcherAndIncrement(emissionInterval)

				Eventually(executorClient.HealthyCallCount).Should(Equal(2))
				Eventually(getMetrics).Should(HaveKeyWithValue(GardenHealthCheckFailed, float64(0)))

				executorClient.HealthyReturns(false)
				fakeClock.WaitForWatcherAndIncrement(emissionInterval)

				Eventually(executorClient.HealthyCallCount).Should(Equal(3))
				Eventually(getMetrics).Should(HaveKeyWithValue(GardenHealthCheckFailed, float64(1)))
			})
		})
	})
})
