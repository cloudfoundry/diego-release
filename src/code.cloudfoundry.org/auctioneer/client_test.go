package auctioneer_test

import (
	"context"
	"net/http"
	"path"
	"time"

	"code.cloudfoundry.org/auctioneer"
	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/lager/v3/lagertest"
	"code.cloudfoundry.org/tlsconfig"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("Auctioneer Client", func() {
	const (
		traceID = "dd6f938-9e14-4abd-974f-63c6138f1cca"
	)
	Describe("NewClient", func() {
		var (
			fakeAuctioneerServer *ghttp.Server
			dummyLogger          lager.Logger
		)

		BeforeEach(func() {
			fakeAuctioneerServer = ghttp.NewServer()

			fakeAuctioneerServer.AppendHandlers(ghttp.CombineHandlers(
				func(rw http.ResponseWriter, r *http.Request) {
					time.Sleep(2 * time.Second)
				},
				ghttp.RespondWith(http.StatusAccepted, nil),
			))

			dummyLogger = lagertest.NewTestLogger("client_test")
		})

		It("requests task auctions", func() {
			c := auctioneer.NewClient(fakeAuctioneerServer.URL(), 5*time.Second)

			err := c.RequestTaskAuctions(dummyLogger, traceID, []*auctioneer.TaskStartRequest{})
			Expect(err).NotTo(HaveOccurred())

			firstRequest := fakeAuctioneerServer.ReceivedRequests()[0]
			vCapRequestIdHeader := firstRequest.Header["X-Vcap-Request-Id"]
			Expect(len(vCapRequestIdHeader)).To(Equal(1))
			Expect(vCapRequestIdHeader[0]).To(Equal(traceID))
		})

		It("requests auctions", func() {
			c := auctioneer.NewClient(fakeAuctioneerServer.URL(), 5*time.Second)

			err := c.RequestLRPAuctions(dummyLogger, traceID, []*auctioneer.LRPStartRequest{})
			Expect(err).NotTo(HaveOccurred())

			firstRequest := fakeAuctioneerServer.ReceivedRequests()[0]
			vCapRequestIdHeader := firstRequest.Header["X-Vcap-Request-Id"]
			Expect(len(vCapRequestIdHeader)).To(Equal(1))
			Expect(vCapRequestIdHeader[0]).To(Equal(traceID))
		})

		It("times out if the request takes too long", func() {
			c := auctioneer.NewClient(fakeAuctioneerServer.URL(), 1*time.Second)

			err := c.RequestLRPAuctions(dummyLogger, traceID, []*auctioneer.LRPStartRequest{})
			Expect(err.Error()).To(ContainSubstring(context.DeadlineExceeded.Error()))
		})
	})

	Describe("NewSecureClient", func() {
		var (
			caFile, certFile, keyFile string
			fakeAuctioneerServer      *ghttp.Server
			dummyLogger               lager.Logger
		)

		BeforeEach(func() {
			basePath := path.Join("cmd", "auctioneer", "fixtures")
			caFile = path.Join(basePath, "green-certs", "ca.crt")

			certFile = path.Join(basePath, "green-certs", "client.crt")
			keyFile = path.Join(basePath, "green-certs", "client.key")

			fakeAuctioneerServer = ghttp.NewUnstartedServer()
			tlsConfig, err := tlsconfig.Build(
				tlsconfig.WithInternalServiceDefaults(),
				tlsconfig.WithIdentityFromFile(
					path.Join(basePath, "green-certs", "server.crt"),
					path.Join(basePath, "green-certs", "server.key"),
				),
			).Server(tlsconfig.WithClientAuthenticationFromFile(caFile))
			Expect(err).NotTo(HaveOccurred())

			fakeAuctioneerServer.HTTPTestServer.TLS = tlsConfig
			fakeAuctioneerServer.HTTPTestServer.StartTLS()

			fakeAuctioneerServer.AppendHandlers(ghttp.CombineHandlers(
				func(rw http.ResponseWriter, r *http.Request) {
					time.Sleep(2 * time.Second)
				},
				ghttp.RespondWith(http.StatusAccepted, nil),
			))

			dummyLogger = lagertest.NewTestLogger("client_test")
		})

		It("requests task auctions", func() {
			c, err := auctioneer.NewSecureClient(fakeAuctioneerServer.URL(), caFile, certFile, keyFile, true, 5*time.Second)
			Expect(err).NotTo(HaveOccurred())

			err = c.RequestTaskAuctions(dummyLogger, traceID, []*auctioneer.TaskStartRequest{})
			Expect(err).NotTo(HaveOccurred())

			firstRequest := fakeAuctioneerServer.ReceivedRequests()[0]
			vCapRequestIdHeader := firstRequest.Header["X-Vcap-Request-Id"]
			Expect(len(vCapRequestIdHeader)).To(Equal(1))
			Expect(vCapRequestIdHeader[0]).To(Equal(traceID))
		})

		It("requests auctions", func() {
			c, err := auctioneer.NewSecureClient(fakeAuctioneerServer.URL(), caFile, certFile, keyFile, true, 5*time.Second)
			Expect(err).NotTo(HaveOccurred())

			err = c.RequestLRPAuctions(dummyLogger, traceID, []*auctioneer.LRPStartRequest{})
			Expect(err).NotTo(HaveOccurred())

			firstRequest := fakeAuctioneerServer.ReceivedRequests()[0]
			vCapRequestIdHeader := firstRequest.Header["X-Vcap-Request-Id"]
			Expect(len(vCapRequestIdHeader)).To(Equal(1))
			Expect(vCapRequestIdHeader[0]).To(Equal(traceID))
		})

		It("times out if the request takes too long", func() {
			c, err := auctioneer.NewSecureClient(fakeAuctioneerServer.URL(), caFile, certFile, keyFile, true, time.Second)
			Expect(err).NotTo(HaveOccurred())

			err = c.RequestLRPAuctions(dummyLogger, traceID, []*auctioneer.LRPStartRequest{})
			Expect(err.Error()).To(ContainSubstring(context.DeadlineExceeded.Error()))
		})

		Context("when the tls config is invalid", func() {
			BeforeEach(func() {
				certFile = "cmd/auctioneer/fixtures/blue-certs/client.crt"
			})

			It("returns an error", func() {
				_, err := auctioneer.NewSecureClient(fakeAuctioneerServer.URL(), caFile, certFile, keyFile, true, time.Second)
				Expect(err.Error()).To(MatchRegexp("failed to load keypair.*"))
			})
		})
	})

	Describe("Falls back to non-TLS when TLS is not required", func() {
		var (
			caFile, certFile, keyFile string
			fakeAuctioneerServer      *ghttp.Server
			dummyLogger               lager.Logger
			traceID                   string
		)

		AfterEach(func() {
			fakeAuctioneerServer.Close()
		})

		BeforeEach(func() {
			basePath := path.Join("cmd", "auctioneer", "fixtures")
			caFile = path.Join(basePath, "green-certs", "ca.crt")

			certFile = path.Join(basePath, "green-certs", "client.crt")
			keyFile = path.Join(basePath, "green-certs", "client.key")

			fakeAuctioneerServer = ghttp.NewServer()

			fakeAuctioneerServer.AppendHandlers(ghttp.CombineHandlers(
				func(rw http.ResponseWriter, r *http.Request) {
					time.Sleep(2 * time.Second)
				},
				ghttp.RespondWith(http.StatusAccepted, nil),
			),
				ghttp.CombineHandlers(
					func(rw http.ResponseWriter, r *http.Request) {
					},
					ghttp.RespondWith(http.StatusAccepted, nil),
				))
			dummyLogger = lagertest.NewTestLogger("client_test")
		})

		When("retrying with an insecure client", func() {
			It("retries requests for auctions", func() {
				c, err := auctioneer.NewSecureClient(fakeAuctioneerServer.URL(), caFile, certFile, keyFile, false, 1*time.Second)
				Expect(err).NotTo(HaveOccurred())

				err = c.RequestLRPAuctions(dummyLogger, traceID, []*auctioneer.LRPStartRequest{})
				Expect(err).NotTo(HaveOccurred())

				firstRequest := fakeAuctioneerServer.ReceivedRequests()[0]
				vCapRequestIdHeader := firstRequest.Header["X-Vcap-Request-Id"]
				Expect(len(vCapRequestIdHeader)).To(Equal(1))
				Expect(vCapRequestIdHeader[0]).To(Equal(traceID))
			})

			It("retries requests for task auctions", func() {
				c, err := auctioneer.NewSecureClient(fakeAuctioneerServer.URL(), caFile, certFile, keyFile, false, 1*time.Second)
				Expect(err).NotTo(HaveOccurred())

				err = c.RequestTaskAuctions(dummyLogger, traceID, []*auctioneer.TaskStartRequest{})
				Expect(err).NotTo(HaveOccurred())

				firstRequest := fakeAuctioneerServer.ReceivedRequests()[0]
				vCapRequestIdHeader := firstRequest.Header["X-Vcap-Request-Id"]
				Expect(len(vCapRequestIdHeader)).To(Equal(1))
				Expect(vCapRequestIdHeader[0]).To(Equal(traceID))
			})
		})
	})
})
