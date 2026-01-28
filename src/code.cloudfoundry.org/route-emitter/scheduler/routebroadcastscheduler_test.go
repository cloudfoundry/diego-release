package scheduler_test

import (
	"fmt"
	"os"
	"sync"
	"time"

	"code.cloudfoundry.org/clock/fakeclock"
	"code.cloudfoundry.org/lager/v3/lagertest"
	"code.cloudfoundry.org/route-emitter/cmd/route-emitter/config"
	"code.cloudfoundry.org/route-emitter/diegonats"
	"code.cloudfoundry.org/route-emitter/scheduler"
	"github.com/nats-io/nats.go"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/tedsuo/ifrit"
)

var _ = Describe("RouteBroadcastScheduler", func() {
	var (
		natsClient      *diegonats.FakeNATSClient
		schedulerRunner *scheduler.RouteBroadcastScheduler
		process         ifrit.Process
		clock           *fakeclock.FakeClock
		cfg             *config.RouteEmitterConfig
		emitCh          chan struct{}

		shutdown chan struct{}

		natsStartMessages chan<- *nats.Msg

		maxGreetAttempts                int
		blockTestOnProcessBecomingReady bool
		expectProcessToExitWithFailure  bool
	)

	testRouteBroadcastScheduler := func(prefix string) {
		Context(prefix, func() {
			BeforeEach(func() {
				natsClient = diegonats.NewFakeClient()
				cfg = &config.RouteEmitterConfig{}
				cfg.JitterFactor = 0.2

				clock = fakeclock.NewFakeClock(time.Now())

				emitCh = make(chan struct{}, 1)
				startMessages := make(chan *nats.Msg)
				natsStartMessages = startMessages

				natsClient.WhenSubscribing(fmt.Sprintf("%s.start", prefix), func(callback nats.MsgHandler) error {
					go func() {
						for msg := range startMessages {
							callback(msg)
						}
					}()

					return nil
				})
				maxGreetAttempts = 30
			})

			JustBeforeEach(func() {
				logger := lagertest.NewTestLogger("test")
				schedulerRunner = scheduler.NewRouteBroadcastScheduler(clock, natsClient, logger, prefix, cfg, emitCh)
				schedulerRunner.MaxGreetAttempts = maxGreetAttempts

				shutdown = make(chan struct{})

				if blockTestOnProcessBecomingReady {
					process = ifrit.Invoke(schedulerRunner)
				} else {
					process = ifrit.Background(schedulerRunner)
				}
			})

			AfterEach(func() {
				process.Signal(os.Interrupt)
				if !expectProcessToExitWithFailure {
					Eventually(process.Wait()).Should(Receive(BeNil()))
				}
				close(shutdown)
				close(natsStartMessages)
			})

			Describe("getting the heartbeat interval from the external service", func() {
				var greetings chan *nats.Msg
				BeforeEach(func() {
					greetings = make(chan *nats.Msg, 3)
					natsClient.WhenPublishing(fmt.Sprintf("%s.greet", prefix), func(msg *nats.Msg) error {
						greetings <- msg
						return nil
					})
				})

				Context("when the external service emits a *.start", func() {
					Context("using a one second interval", func() {
						JustBeforeEach(func() {
							natsStartMessages <- &nats.Msg{
								Data: []byte(`{
						"minimumRegisterIntervalInSeconds":2,
						"pruneThresholdInSeconds": 3
						}`),
							}
						})

						It("should emit routes with the frequency of the passed-in-interval", func() {
							Eventually(greetings).Should(Receive())
							Consistently(schedulerRunner.EmitCh()).ShouldNot(Receive())

							clock.WaitForWatcherAndIncrement(3 * time.Second)
							Eventually(schedulerRunner.EmitCh()).Should(Receive())

							clock.WaitForWatcherAndIncrement(2 * time.Second)
							Eventually(schedulerRunner.EmitCh()).Should(Receive())
						})

						It("should only greet the external service once", func() {
							Eventually(greetings).Should(Receive())
							Consistently(greetings, 1).ShouldNot(Receive())
						})
					})
				})

				Context("when the external service does not emit a *.start", func() {
					BeforeEach(func() {
						blockTestOnProcessBecomingReady = false
					})

					It("should keep greeting the external service until it gets an interval", func() {
						isReady := false
						var wg sync.WaitGroup
						wg.Add(1)
						go func() {
							<-process.Ready()
							isReady = true
							wg.Done()
						}()

						//get the first greeting
						Eventually(greetings).Should(Receive())

						//get the second greeting, and respond
						clock.WaitForWatcherAndIncrement(time.Second)
						var msg *nats.Msg
						Eventually(greetings).Should(Receive(&msg))
						Expect(isReady).To(Equal(false))

						go natsClient.Publish(msg.Reply, []byte(`{"minimumRegisterIntervalInSeconds":1, "pruneThresholdInSeconds": 3}`))
						wg.Wait()
						Eventually(isReady).Should(Equal(true))

						//should no longer be greeting the external service
						Consistently(greetings).ShouldNot(Receive())
					})

					Context("when it has been greeting for a very long time", func() {
						BeforeEach(func() {
							maxGreetAttempts = 3
							expectProcessToExitWithFailure = true
						})

						It("eventually fails", func() {
							//get the first greeting
							Eventually(greetings).Should(Receive())

							//attempt to greet 3 times
							clock.WaitForWatcherAndIncrement(time.Second)
							clock.WaitForWatcherAndIncrement(time.Second)
							clock.WaitForWatcherAndIncrement(time.Second)

							//should exit with an error
							err := <-process.Wait()
							Expect(err).To(MatchError(ContainSubstring("attempted to greet")))
						})
					})
				})

				Context("after getting the first interval, when a second interval arrives", func() {
					JustBeforeEach(func() {
						natsStartMessages <- &nats.Msg{
							Data: []byte(`{"minimumRegisterIntervalInSeconds":1, "pruneThresholdInSeconds": 3}`),
						}
					})

					It("should modify its update rate", func() {
						natsStartMessages <- &nats.Msg{
							Data: []byte(`{"minimumRegisterIntervalInSeconds":2, "pruneThresholdInSeconds": 6}`),
						}

						// first emit should wait a jitter time in response to the incoming heartbeat interval
						Consistently(schedulerRunner.EmitCh()).ShouldNot(Receive())
						clock.Increment(400 * time.Millisecond)
						Eventually(schedulerRunner.EmitCh()).Should(Receive())

						clock.WaitForWatcherAndIncrement(time.Second)
						Consistently(schedulerRunner.EmitCh()).ShouldNot(Receive())

						//i subsequent emit should follow the interval
						clock.WaitForWatcherAndIncrement(time.Second)
						Eventually(schedulerRunner.EmitCh()).Should(Receive())
					})

					Context("using different interval", func() {
						It("jitter respects the interval while sleeping", func() {
							natsStartMessages <- &nats.Msg{
								Data: []byte(`{"minimumRegisterIntervalInSeconds":5, "pruneThresholdInSeconds": 180}`),
							}

							// first emit should wait a jitter time in response to the incoming heartbeat interval
							Consistently(schedulerRunner.EmitCh()).ShouldNot(Receive())
							clock.Increment(1 * time.Second)
							Eventually(schedulerRunner.EmitCh()).Should(Receive())

							// subsequent emit should follow the interval
							clock.WaitForWatcherAndIncrement(5 * time.Second)
							Eventually(schedulerRunner.EmitCh()).Should(Receive())
						})
					})

					It("the jitter should be random", func() {
						// This test uses the fact that the probability of the jitter being
						// less than 100 milliseconds should be 50%.
						// However, this test has a 1/512 chance of failing in the case that
						// 10 samples of the jitter are less than 100ms or all 10 samples are
						// greater than 100ms.
						emitted := []bool{}
						for i := 0; i < 10; i++ {
							natsStartMessages <- &nats.Msg{
								Data: []byte(`{"minimumRegisterIntervalInSeconds":1, "pruneThresholdInSeconds": 180}`),
							}

							Consistently(schedulerRunner.EmitCh()).ShouldNot(Receive())
							// Wait 10% of the minimum register interval
							clock.Increment(100 * time.Millisecond)
							select {
							case <-schedulerRunner.EmitCh():
								// Statistically 50% of the jitters should end up here
								emitted = append(emitted, true)
								continue
							case <-time.After(10 * time.Millisecond):
								emitted = append(emitted, false)
							}
							// Wait the last 10% to guarantee that an emit happens
							clock.Increment(100 * time.Millisecond)
							Eventually(schedulerRunner.EmitCh()).Should(Receive())
						}
						trueCount := 0
						Expect(emitted).To(HaveLen(10))
						for _, e := range emitted {
							if e {
								trueCount++
							}
						}
						Expect(trueCount).To(BeNumerically(">", 0))
						Expect(trueCount).To(BeNumerically("<", 10))
					})
				})

				Context("if it never hears anything from a external service anywhere", func() {
					It("should still be able to shutdown", func() {
						process.Signal(os.Interrupt)
						Eventually(process.Wait()).Should(Receive(BeNil()))
					})
				})
			})
		})
	}

	testRouteBroadcastScheduler("router")
	testRouteBroadcastScheduler("service-discovery")
})
