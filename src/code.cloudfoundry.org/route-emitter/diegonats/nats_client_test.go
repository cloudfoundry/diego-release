package diegonats_test

import (
	"fmt"
	"os"

	"code.cloudfoundry.org/inigo/helpers/certauthority"
	. "code.cloudfoundry.org/route-emitter/diegonats"
	"code.cloudfoundry.org/tlsconfig"
	"github.com/nats-io/nats.go"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("NatsClient", func() {
	var natsClient NATSClient
	var natsUrls []string

	BeforeEach(func() {
		natsUrls = []string{fmt.Sprintf("nats://127.0.0.1:%d", natsPort)}
	})

	AfterEach(func() {
		stopNATS()
	})

	verifyConnect := func() {
		Describe("Connect", func() {
			It("returns an error when connecting to an invalid address", func() {
				_, err := natsClient.Connect([]string{"nats://cats:bats@127.0.0.1:4223"})

				Expect(err).Should(HaveOccurred())
			})
		})
	}

	verifySubscription := func() {
		Describe("Subscription", func() {
			BeforeEach(func() {
				_, err := natsClient.Connect(natsUrls)
				Expect(err).ShouldNot(HaveOccurred())
			})

			AfterEach(func() {
				if natsClient != nil {
					natsClient.Close()
				}
			})

			It("can subscribe/unsubscribe", func() {
				payload1 := make(chan []byte)
				payload2 := make(chan []byte)

				sid1, _ := natsClient.Subscribe("some.subject", func(msg *nats.Msg) {
					payload1 <- msg.Data
				})

				natsClient.Subscribe("some.subject", func(msg *nats.Msg) {
					payload2 <- msg.Data
				})

				natsClient.Publish("some.subject", []byte("hello!"))

				Eventually(payload1).Should(Receive(Equal([]byte("hello!"))))
				Eventually(payload2).Should(Receive(Equal([]byte("hello!"))))

				natsClient.Unsubscribe(sid1)

				natsClient.Publish("some.subject", []byte("hello!"))

				Consistently(payload1).ShouldNot(Receive())
				Eventually(payload2).Should(Receive(Equal([]byte("hello!"))))
			})

			It("can subscribe/unsubscribe with a queue", func() {
				payload := make(chan []byte)

				natsClient.QueueSubscribe("some.subject", "some-queue", func(msg *nats.Msg) {
					payload <- msg.Data
				})

				natsClient.QueueSubscribe("some.subject", "some-queue", func(msg *nats.Msg) {
					payload <- msg.Data
				})

				natsClient.Publish("some.subject", []byte("hello!"))

				Eventually(payload).Should(Receive(Equal([]byte("hello!"))))
				Consistently(payload).ShouldNot(Receive())
			})

			It("can subscribe/unsubscribe with a request/response", func() {
				payload := make(chan []byte)

				natsClient.Subscribe("some.request", func(msg *nats.Msg) {
					natsClient.Publish(msg.Reply, []byte("response!"))
				})

				natsClient.Subscribe("some.reply", func(msg *nats.Msg) {
					payload <- msg.Data
				})

				natsClient.PublishRequest("some.request", "some.reply", []byte("hello!"))

				Eventually(payload).Should(Receive(Equal([]byte("response!"))))
			})
		})
	}

	Context("without TLS", func() {
		BeforeEach(func() {
			startNATS()
			natsClient = NewClient()
		})

		verifyConnect()

		verifySubscription()
	})

	Context("when configured with TLS", func() {
		var (
			certDepot string
			tlsConfig tlsconfig.Config
		)

		BeforeEach(func() {
			var err error
			certDepot, err = os.MkdirTemp("", "")
			Expect(err).NotTo(HaveOccurred())
			certAuthority, err := certauthority.NewCertAuthority(certDepot, "nats")
			Expect(err).NotTo(HaveOccurred())
			_, caFile := certAuthority.CAAndKey()
			keyFile, certFile, err := certAuthority.GenerateSelfSignedCertAndKey("nats", []string{}, false)
			Expect(err).NotTo(HaveOccurred())
			startNATSWithTLS(caFile, certFile, keyFile)

			tlsConfig = tlsconfig.Build(
				tlsconfig.WithInternalServiceDefaults(),
				tlsconfig.WithIdentityFromFile(certFile, keyFile),
			)

			clientTLSConfig, err := tlsConfig.Client(
				tlsconfig.WithAuthorityFromFile(caFile),
			)
			Expect(err).NotTo(HaveOccurred())
			natsClient = NewClientWithTLSConfig(clientTLSConfig)
		})

		AfterEach(func() {
			Expect(os.RemoveAll(certDepot)).To(Succeed())
		})

		Context("when the server presents a certificate that the client does not trust", func() {
			BeforeEach(func() {
				if natsClient != nil {
					natsClient.Close()
				}

				certAuthority, err := certauthority.NewCertAuthority(certDepot, "nats-incorrect")
				Expect(err).NotTo(HaveOccurred())
				_, caFile := certAuthority.CAAndKey()

				clientTLSConfig, err := tlsConfig.Client(
					tlsconfig.WithAuthorityFromFile(caFile),
				)
				Expect(err).NotTo(HaveOccurred())
				natsClient = NewClientWithTLSConfig(clientTLSConfig)
			})

			It("it fails to connect", func() {
				_, err := natsClient.Connect(natsUrls)

				Expect(err).Should(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("x509: certificate signed by unknown authority"))
			})
		})

		verifyConnect()

		verifySubscription()
	})
})
