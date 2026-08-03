package unregistration_test

import (
	"os"
	"time"

	"github.com/tedsuo/ifrit"

	"code.cloudfoundry.org/clock/fakeclock"
	"code.cloudfoundry.org/lager/v3/lagertest"
	"code.cloudfoundry.org/route-emitter/emitter/fakes"
	"code.cloudfoundry.org/route-emitter/routingtable"
	"code.cloudfoundry.org/route-emitter/unregistration"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Sender", func() {
	var (
		sender        ifrit.Runner
		senderProcess ifrit.Process
		natsEmitter   *fakes.FakeNATSEmitter
		cache         unregistration.Cache
		clock         *fakeclock.FakeClock
		sendInterval  time.Duration
	)

	BeforeEach(func() {
		logger := lagertest.NewTestLogger("sender")
		cache = unregistration.NewCache(logger)
		natsEmitter = &fakes.FakeNATSEmitter{}
		clock = fakeclock.NewFakeClock(time.Now())
		sendInterval = 500 * time.Millisecond
		sender = unregistration.NewSender(logger, clock, cache, natsEmitter, sendInterval, 3)
		senderProcess = ifrit.Background(sender)
	})

	AfterEach(func() {
		if senderProcess != nil {
			senderProcess.Signal(os.Interrupt)
			Eventually(senderProcess.Wait(), 5).Should(Receive())
		}
	})

	Context("when there are messages in cache", func() {
		var (
			endpoint1, endpoint2 routingtable.Endpoint
			route1, route2       routingtable.Route
		)

		BeforeEach(func() {
			endpoint1 = routingtable.Endpoint{
				InstanceGUID:  "instance-guid-1",
				Host:          "1.1.1.1",
				Port:          61001,
				ContainerPort: 11,
			}
			endpoint2 = routingtable.Endpoint{
				InstanceGUID:  "instance-guid-2",
				Host:          "2.2.2.2",
				Port:          61002,
				ContainerPort: 22,
			}

			route1 = routingtable.Route{
				Hostname: "host-1.example.com",
			}
			route2 = routingtable.Route{
				Hostname: "host-2.example.com",
			}
			cache.Add([]routingtable.RegistryMessage{
				routingtable.RegistryMessageFor(endpoint1, route1, false),
				routingtable.RegistryMessageFor(endpoint2, route2, false),
			})
		})

		It("emits unregistration for each message the required number of times", func() {
			clock.WaitForWatcherAndIncrement(sendInterval)
			Eventually(natsEmitter.EmitCallCount).Should(Equal(2))

			clock.WaitForWatcherAndIncrement(sendInterval)
			Eventually(natsEmitter.EmitCallCount).Should(Equal(4))

			clock.WaitForWatcherAndIncrement(sendInterval)
			Eventually(natsEmitter.EmitCallCount).Should(Equal(6))

			clock.WaitForWatcherAndIncrement(sendInterval)
			Consistently(natsEmitter.EmitCallCount).Should(Equal(6))
		})

		Context("when one of the messages is removed", func() {
			It("stops emitting unregistration messages", func() {
				clock.WaitForWatcherAndIncrement(sendInterval)
				Eventually(natsEmitter.EmitCallCount).Should(Equal(2))

				clock.WaitForWatcherAndIncrement(sendInterval)
				Eventually(natsEmitter.EmitCallCount).Should(Equal(4))

				cache.Remove([]routingtable.RegistryMessage{
					routingtable.RegistryMessageFor(endpoint1, route1, false),
				})

				clock.WaitForWatcherAndIncrement(sendInterval)
				Eventually(natsEmitter.EmitCallCount).Should(Equal(5))

				clock.WaitForWatcherAndIncrement(sendInterval)
				Consistently(natsEmitter.EmitCallCount).Should(Equal(5))
			})
		})

		Context("when another message is added", func() {
			It("stops emitting unregistration messages", func() {
				clock.WaitForWatcherAndIncrement(sendInterval)
				Eventually(natsEmitter.EmitCallCount).Should(Equal(2))

				clock.WaitForWatcherAndIncrement(sendInterval)
				Eventually(natsEmitter.EmitCallCount).Should(Equal(4))

				endpoint3 := routingtable.Endpoint{
					InstanceGUID:  "instance-guid-3",
					Host:          "3.3.3.3",
					Port:          61003,
					ContainerPort: 33,
				}
				route3 := routingtable.Route{
					Hostname: "host-3.example.com",
				}

				cache.Add([]routingtable.RegistryMessage{
					routingtable.RegistryMessageFor(endpoint3, route3, false),
				})

				clock.WaitForWatcherAndIncrement(sendInterval)
				Eventually(natsEmitter.EmitCallCount).Should(Equal(7))
			})
		})
	})
})
