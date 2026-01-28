package emitter_test

import (
	"errors"

	mfakes "code.cloudfoundry.org/diego-logging-client/testhelpers"
	"code.cloudfoundry.org/lager/v3/lagertest"
	"code.cloudfoundry.org/route-emitter/diegonats"
	"code.cloudfoundry.org/route-emitter/emitter"
	"code.cloudfoundry.org/route-emitter/routingtable"
	"code.cloudfoundry.org/workpool"
	"github.com/nats-io/nats.go"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("NatsEmitter", func() {
	var natsEmitter emitter.NATSEmitter
	var natsClient *diegonats.FakeNATSClient
	var fakeMetronClient *mfakes.FakeIngressClient
	var logger *lagertest.TestLogger

	messagesToEmit := routingtable.MessagesToEmit{
		RegistrationMessages: []routingtable.RegistryMessage{
			{URIs: []string{"foo.com", "bar.com"}, Host: "1.1.1.1", Port: 11},
			{URIs: []string{"baz.com"}, Host: "2.2.2.2", Port: 22},
		},
		UnregistrationMessages: []routingtable.RegistryMessage{
			{URIs: []string{"wibble.com"}, Host: "1.1.1.1", Port: 11},
			{URIs: []string{"baz.com"}, Host: "3.3.3.3", Port: 33},
		},
		InternalRegistrationMessages: []routingtable.RegistryMessage{
			{URIs: []string{"internal-foo.com", "internal-bar.com"}, Host: "1.2.1.1", Port: 11},
			{URIs: []string{"internal-baz.com"}, Host: "2.2.2.2", Port: 22},
		},
		InternalUnregistrationMessages: []routingtable.RegistryMessage{
			{URIs: []string{"internal-wibble.com"}, Host: "1.2.1.1", Port: 11},
			{URIs: []string{"internal-baz.com"}, Host: "3.2.3.3", Port: 33},
		},
	}

	BeforeEach(func() {
		natsClient = diegonats.NewFakeClient()
		logger = lagertest.NewTestLogger("test")
		workPool, err := workpool.NewWorkPool(1)
		Expect(err).NotTo(HaveOccurred())
		fakeMetronClient = &mfakes.FakeIngressClient{}
		natsEmitter = emitter.NewNATSEmitter(natsClient, workPool, logger, fakeMetronClient, true)
	})

	Describe("Emitting", func() {
		It("should emit register and unregister messages", func() {
			err := natsEmitter.Emit(messagesToEmit)
			Expect(err).NotTo(HaveOccurred())

			Expect(natsClient.PublishedMessages("router.register")).To(HaveLen(2))
			Expect(natsClient.PublishedMessages("router.unregister")).To(HaveLen(2))

			Expect(natsClient.PublishedMessages("service-discovery.register")).To(HaveLen(2))
			Expect(natsClient.PublishedMessages("service-discovery.unregister")).To(HaveLen(2))

			registeredPayloads := [][]byte{
				natsClient.PublishedMessages("router.register")[0].Data,
				natsClient.PublishedMessages("router.register")[1].Data,
			}

			unregisteredPayloads := [][]byte{
				natsClient.PublishedMessages("router.unregister")[0].Data,
				natsClient.PublishedMessages("router.unregister")[1].Data,
			}

			internalRegisteredPayloads := [][]byte{
				natsClient.PublishedMessages("service-discovery.register")[0].Data,
				natsClient.PublishedMessages("service-discovery.register")[1].Data,
			}

			internalUnregisteredPayloads := [][]byte{
				natsClient.PublishedMessages("service-discovery.unregister")[0].Data,
				natsClient.PublishedMessages("service-discovery.unregister")[1].Data,
			}

			Expect(registeredPayloads).To(ContainElement(MatchJSON(`
        {
          "uris":["foo.com", "bar.com"],
          "host":"1.1.1.1",
          "port":11
        }
      `)))

			Expect(registeredPayloads).To(ContainElement(MatchJSON(`
        {
          "uris":["baz.com"],
          "host":"2.2.2.2",
          "port":22
        }
      `)))

			Expect(unregisteredPayloads).To(ContainElement(MatchJSON(`
        {
          "uris":["wibble.com"],
          "host":"1.1.1.1",
          "port":11
        }
      `)))

			Expect(unregisteredPayloads).To(ContainElement(MatchJSON(`
        {
          "uris":["baz.com"],
          "host":"3.3.3.3",
          "port":33
        }
      `)))

			Expect(internalRegisteredPayloads).To(ContainElement(MatchJSON(`
        {
          "uris":["internal-foo.com", "internal-bar.com"],
          "host":"1.2.1.1",
          "port":11
        }
      `)))

			Expect(internalRegisteredPayloads).To(ContainElement(MatchJSON(`
        {
          "uris":["internal-baz.com"],
          "host":"2.2.2.2",
          "port":22
        }
      `)))

			Expect(internalUnregisteredPayloads).To(ContainElement(MatchJSON(`
        {
          "uris":["internal-wibble.com"],
          "host":"1.2.1.1",
          "port":11
        }
      `)))

			Expect(internalUnregisteredPayloads).To(ContainElement(MatchJSON(`
        {
          "uris":["internal-baz.com"],
          "host":"3.2.3.3",
          "port":33
        }
      `)))

			Eventually(fakeMetronClient.IncrementCounterWithDeltaCallCount).Should(Equal(2))
			name, delta := fakeMetronClient.IncrementCounterWithDeltaArgsForCall(0)
			Expect(name).To(Equal("HTTPRouteNATSMessagesEmitted"))
			Expect(delta).To(BeEquivalentTo(4))

			name, delta = fakeMetronClient.IncrementCounterWithDeltaArgsForCall(1)
			Expect(name).To(Equal("InternalRouteNATSMessagesEmitted"))
			Expect(delta).To(BeEquivalentTo(4))
		})

		Context("when the nats emitter is configured to not emit internal routes", func() {
			BeforeEach(func() {
				logger := lagertest.NewTestLogger("test")
				workPool, err := workpool.NewWorkPool(1)
				Expect(err).NotTo(HaveOccurred())
				natsEmitter = emitter.NewNATSEmitter(natsClient, workPool, logger, fakeMetronClient, false)
			})

			It("only emits http routes", func() {
				err := natsEmitter.Emit(messagesToEmit)
				Expect(err).NotTo(HaveOccurred())

				Expect(natsClient.PublishedMessages("router.register")).To(HaveLen(2))
				Expect(natsClient.PublishedMessages("router.unregister")).To(HaveLen(2))

				Expect(natsClient.PublishedMessages("service-discovery.register")).To(HaveLen(0))
				Expect(natsClient.PublishedMessages("service-discovery.unregister")).To(HaveLen(0))

				registeredPayloads := [][]byte{
					natsClient.PublishedMessages("router.register")[0].Data,
					natsClient.PublishedMessages("router.register")[1].Data,
				}

				unregisteredPayloads := [][]byte{
					natsClient.PublishedMessages("router.unregister")[0].Data,
					natsClient.PublishedMessages("router.unregister")[1].Data,
				}

				Expect(registeredPayloads).To(ContainElement(MatchJSON(`
        {
          "uris":["foo.com", "bar.com"],
          "host":"1.1.1.1",
          "port":11
        }
      `)))

				Expect(registeredPayloads).To(ContainElement(MatchJSON(`
        {
          "uris":["baz.com"],
          "host":"2.2.2.2",
          "port":22
        }
      `)))

				Expect(unregisteredPayloads).To(ContainElement(MatchJSON(`
        {
          "uris":["wibble.com"],
          "host":"1.1.1.1",
          "port":11
        }
      `)))

				Expect(unregisteredPayloads).To(ContainElement(MatchJSON(`
        {
          "uris":["baz.com"],
          "host":"3.3.3.3",
          "port":33
        }
      `)))

				Eventually(fakeMetronClient.IncrementCounterWithDeltaCallCount).Should(Equal(1))
				name, delta := fakeMetronClient.IncrementCounterWithDeltaArgsForCall(0)
				Expect(name).To(Equal("HTTPRouteNATSMessagesEmitted"))
				Expect(delta).To(BeEquivalentTo(4))
			})
		})

		Context("when the nats client errors", func() {
			BeforeEach(func() {
				natsClient.WhenPublishing("router.register", func(*nats.Msg) error {
					return errors.New("bam")
				})
			})

			It("should error", func() {
				Expect(natsEmitter.Emit(messagesToEmit)).To(MatchError(errors.New("bam")))
			})
		})

		Context("when the metron client errors", func() {
			BeforeEach(func() {
				fakeMetronClient.IncrementCounterWithDeltaReturns(errors.New("boo"))
			})

			It("should log the error message", func() {
				err := natsEmitter.Emit(messagesToEmit)
				Expect(err).NotTo(HaveOccurred())

				Expect(logger).To(gbytes.Say("cannot-emit-number-of-internal-messages.*boo"))
			})
		})
	})
})
