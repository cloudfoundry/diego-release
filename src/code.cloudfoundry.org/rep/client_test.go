package rep_test

import (
	"net/http"
	"path"
	"strings"
	"time"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/bbs/trace"
	cfhttp "code.cloudfoundry.org/cfhttp/v2"
	"code.cloudfoundry.org/lager/v3/lagertest"
	"code.cloudfoundry.org/rep"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("ClientFactory", func() {
	var (
		httpClient                    *http.Client
		fixturePath                   string
		certFile, keyFile, caCertFile string
	)

	BeforeEach(func() {
		fixturePath = path.Join("cmd", "rep", "fixtures")
		certFile = path.Join(fixturePath, "blue-certs/client.crt")
		keyFile = path.Join(fixturePath, "blue-certs/client.key")
		caCertFile = path.Join(fixturePath, "blue-certs/server-ca.crt")
	})

	Describe("NewClientFactory", func() {
		Context("when no TLS configuration is provided", func() {
			It("returns a new client", func() {
				httpClient = cfhttp.NewClient(
					cfhttp.WithRequestTimeout(cfHttpTimeout),
				)
				client, err := rep.NewClientFactory(httpClient, httpClient, nil)
				Expect(err).NotTo(HaveOccurred())
				Expect(client).NotTo(BeNil())
			})
		})

		Context("when TLS is preferred", func() {
			var tlsConfig *rep.TLSConfig
			BeforeEach(func() {
				tlsConfig = &rep.TLSConfig{RequireTLS: false}
				httpClient = cfhttp.NewClient(
					cfhttp.WithRequestTimeout(cfHttpTimeout),
				)
			})

			Context("no cert files are provided", func() {
				It("returns a new client", func() {
					client, err := rep.NewClientFactory(httpClient, httpClient, tlsConfig)
					Expect(err).NotTo(HaveOccurred())
					Expect(client).NotTo(BeNil())
				})
			})

			Context("valid cert files are provided", func() {
				It("returns a new client", func() {
					tlsConfig.CertFile = certFile
					tlsConfig.KeyFile = keyFile
					tlsConfig.CaCertFile = caCertFile

					client, err := rep.NewClientFactory(httpClient, httpClient, tlsConfig)
					Expect(err).NotTo(HaveOccurred())
					Expect(client).NotTo(BeNil())
				})
			})
		})
	})

	Describe("CreateClient", func() {
		var fakeServer *ghttp.Server

		BeforeEach(func() {
			fakeServer = ghttp.NewServer()
			var err error
			Expect(err).NotTo(HaveOccurred())

			fakeServer.RouteToHandler("GET", "/state", func(resp http.ResponseWriter, req *http.Request) {
			})
		})

		AfterEach(func() {
			fakeServer.Close()
		})

		Context("when trace ID is provided", func() {
			It("returns client that adds trace ID to request", func() {
				httpClient = cfhttp.NewClient(
					cfhttp.WithRequestTimeout(cfHttpTimeout),
				)
				clientFactory, err := rep.NewClientFactory(httpClient, httpClient, nil)
				Expect(err).NotTo(HaveOccurred())
				client, err := clientFactory.CreateClient(fakeServer.URL(), "", "some-trace-id")
				Expect(err).NotTo(HaveOccurred())
				logger := lagertest.NewTestLogger("test-rep-client")
				client.State(logger)

				Expect(fakeServer.ReceivedRequests()).To(HaveLen(1))
				Expect(fakeServer.ReceivedRequests()[0].Header.Get(trace.RequestIdHeader)).To(Equal("some-trace-id"))
			})
		})

		Context("when trace ID is not provided", func() {
			It("returns client that does not add trace ID to request", func() {
				httpClient = cfhttp.NewClient(
					cfhttp.WithRequestTimeout(cfHttpTimeout),
				)
				clientFactory, err := rep.NewClientFactory(httpClient, httpClient, nil)
				Expect(err).NotTo(HaveOccurred())
				client, err := clientFactory.CreateClient(fakeServer.URL(), "", "")
				Expect(err).NotTo(HaveOccurred())
				logger := lagertest.NewTestLogger("test-rep-client")
				client.State(logger)

				Expect(fakeServer.ReceivedRequests()).To(HaveLen(1))
				Expect(fakeServer.ReceivedRequests()[0].Header.Get(trace.RequestIdHeader)).To(Equal(""))
			})
		})
	})
})

var _ = Describe("Client", func() {
	var fakeServer *ghttp.Server
	var client models.RepClient

	BeforeEach(func() {
		fakeServer = ghttp.NewServer()
		var err error
		client, err = factory.CreateClient(fakeServer.URL(), "", "")
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		fakeServer.Close()
	})

	Describe("State", func() {
		var (
			logger *lagertest.TestLogger
			addrs  map[string]struct{}
		)

		BeforeEach(func() {
			str := strings.Repeat("x", 4096)
			addrs = make(map[string]struct{})

			fakeServer.RouteToHandler("GET", "/state", func(resp http.ResponseWriter, req *http.Request) {
				addrs[req.RemoteAddr] = struct{}{}
				resp.Write([]byte("{}" + str))
			})
		})

		// test that the http client read the entire body, otherwise the http
		// connections won't be recycled. **Note** this test is slightly ugly but I
		// cannot think of a better way. see
		// https://www.pivotaltracker.com/story/show/144907419 for more info
		It("reads the entire body", func() {
			client.State(logger)
			client.State(logger)
			Expect(addrs).To(HaveLen(1))
		})
	})

	Describe("UpdateLRPInstance", func() {
		var (
			logger    = lagertest.NewTestLogger("test")
			updateErr error
			lrpUpdate models.LRPUpdate
		)

		BeforeEach(func() {
			lrpUpdate = models.LRPUpdate{
				InstanceGUID: "some-instance-guid",
				ActualLRPKey: models.NewActualLRPKey("some-process-guid", 2, "test-domain"),
				InternalRoutes: models.InternalRoutes{
					{Hostname: "a.apps.internal"},
					{Hostname: "b.apps.internal"},
				},
				MetricTags: map[string]string{"app-name": "some-app"},
			}
		})

		JustBeforeEach(func() {
			updateErr = client.UpdateLRPInstance(logger, lrpUpdate)
		})

		Context("when the request is successful", func() {
			BeforeEach(func() {
				fakeServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("PUT", "/v2/lrps/some-process-guid/instances/some-instance-guid"),
						ghttp.RespondWith(http.StatusAccepted, ""),
					),
				)
			})

			It("makes the request and does not return an error", func() {
				Expect(updateErr).NotTo(HaveOccurred())
				Expect(fakeServer.ReceivedRequests()).To(HaveLen(1))
			})

			It("logs start and complete", func() {
				Eventually(logger.Buffer()).Should(gbytes.Say("update-lrp.starting"))
				Eventually(logger.Buffer()).Should(gbytes.Say("update-lrp.completed"))
				Eventually(logger.Buffer()).Should(gbytes.Say("duration"))
			})
		})

		Context("when the request returns 500", func() {
			BeforeEach(func() {
				fakeServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("PUT", "/v2/lrps/some-process-guid/instances/some-instance-guid"),
						ghttp.RespondWith(http.StatusInternalServerError, ""),
					),
				)
			})

			It("makes the request and returns an error", func() {
				Expect(updateErr).To(HaveOccurred())
				Expect(updateErr.Error()).To(ContainSubstring("http error: status code 500"))
				Expect(fakeServer.ReceivedRequests()).To(HaveLen(1))
			})

			It("logs that it failed", func() {
				Eventually(logger.Buffer()).Should(gbytes.Say("update-lrp.failed-with-status"))
			})
		})

		Context("when the request returns 404", func() {
			BeforeEach(func() {
				fakeServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("PUT", "/v2/lrps/some-process-guid/instances/some-instance-guid"),
						ghttp.RespondWith(http.StatusNotFound, ""),
					),
				)
			})

			Context("when the v1 request is successful", func() {
				BeforeEach(func() {
					fakeServer.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("PUT", "/v1/lrps/some-process-guid/instances/some-instance-guid"),
							ghttp.RespondWith(http.StatusAccepted, ""),
						),
					)
				})

				It("logs that the v2 request failed", func() {
					Eventually(logger.Buffer()).Should(gbytes.Say("update-lrp.failed-with-status"))
				})

				It("makes both requests and does not return an error", func() {
					Expect(updateErr).NotTo(HaveOccurred())
					Expect(fakeServer.ReceivedRequests()).To(HaveLen(2))
				})

				It("logs start and complete", func() {
					Eventually(logger.Buffer()).Should(gbytes.Say("update-lrp-r0.starting"))
					Eventually(logger.Buffer()).Should(gbytes.Say("update-lrp-r0.completed"))
					Eventually(logger.Buffer()).Should(gbytes.Say("duration"))
				})
			})

			Context("when the v1 request fails", func() {
				BeforeEach(func() {
					fakeServer.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("PUT", "/v1/lrps/some-process-guid/instances/some-instance-guid"),
							ghttp.RespondWith(http.StatusInternalServerError, ""),
						),
					)
				})

				It("logs that the v2 request failed", func() {
					Eventually(logger.Buffer()).Should(gbytes.Say("update-lrp.failed-with-status"))
				})

				It("makes both requests and returns the v1 request error", func() {
					Expect(updateErr).To(HaveOccurred())
					Expect(updateErr.Error()).To(ContainSubstring("http error: status code 500"))
					Expect(fakeServer.ReceivedRequests()).To(HaveLen(2))
				})

				It("logs that it fails", func() {
					Eventually(logger.Buffer()).Should(gbytes.Say("update-lrp-r0.failed-with-status"))
				})
			})

			Context("when the v1 request returns 404", func() {
				BeforeEach(func() {
					fakeServer.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("PUT", "/v1/lrps/some-process-guid/instances/some-instance-guid"),
							ghttp.RespondWith(http.StatusNotFound, ""),
						),
					)
				})

				It("logs that the v2 request failed", func() {
					Eventually(logger.Buffer()).Should(gbytes.Say("update-lrp.failed-with-status"))
				})

				// We are assuming that this 404 means that the rep has not updated
				// yet, but will roll soon.  This is for backwards compatibility.
				It("makes both requests and does not return an error", func() {
					Expect(updateErr).ToNot(HaveOccurred())
					Expect(fakeServer.ReceivedRequests()).To(HaveLen(2))
				})

				It("logs that it fails", func() {
					Eventually(logger.Buffer()).Should(gbytes.Say("update-lrp-r0.failed-with-status"))
				})
			})
		})

		Context("when the connection fails", func() {
			BeforeEach(func() {
				fakeServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("PUT", "/v2/lrps/some-process-guid/instances/some-instance-guid"),
						func(w http.ResponseWriter, r *http.Request) {
							fakeServer.CloseClientConnections()
						},
					),
				)
			})

			It("makes the request and returns an error", func() {
				Expect(updateErr).To(HaveOccurred())
				Expect(updateErr.Error()).To(ContainSubstring("EOF"))
				Expect(fakeServer.ReceivedRequests()).To(HaveLen(1))
			})
		})

		Context("when the connection times out", func() {
			BeforeEach(func() {
				fakeServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("PUT", "/v2/lrps/some-process-guid/instances/some-instance-guid"),
						func(w http.ResponseWriter, r *http.Request) {
							time.Sleep(cfHttpTimeout + 100*time.Millisecond)
						},
					),
				)
			})

			It("makes the request and returns an error", func() {
				Expect(updateErr).To(HaveOccurred())
				Expect(updateErr.Error()).To(MatchRegexp("use of closed network connection|Client.Timeout exceeded while awaiting headers"))
				Expect(fakeServer.ReceivedRequests()).To(HaveLen(1))
			})

			It("logs start and complete", func() {
				Eventually(logger.Buffer()).Should(gbytes.Say("update-lrp.request-failed"))
			})
		})
	})

	Describe("StopLRPInstance", func() {
		var (
			logger    = lagertest.NewTestLogger("test")
			stopErr   error
			actualLRP = models.ActualLRP{
				ActualLRPKey:         models.NewActualLRPKey("some-process-guid", 2, "test-domain"),
				ActualLRPInstanceKey: models.NewActualLRPInstanceKey("some-instance-guid", "some-cell-id"),
			}
		)

		JustBeforeEach(func() {
			stopErr = client.StopLRPInstance(logger, actualLRP.ActualLRPKey, actualLRP.ActualLRPInstanceKey)
		})

		Context("when the request is successful", func() {
			BeforeEach(func() {
				fakeServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/v1/lrps/some-process-guid/instances/some-instance-guid/stop"),
						ghttp.RespondWith(http.StatusAccepted, ""),
					),
				)
			})

			It("makes the request and does not return an error", func() {
				Expect(stopErr).NotTo(HaveOccurred())
				Expect(fakeServer.ReceivedRequests()).To(HaveLen(1))
			})

			It("logs start and complete", func() {
				Eventually(logger.Buffer()).Should(gbytes.Say("stop-lrp.starting"))
				Eventually(logger.Buffer()).Should(gbytes.Say("stop-lrp.completed"))
				Eventually(logger.Buffer()).Should(gbytes.Say("duration"))
			})
		})

		Context("when the request returns 500", func() {
			BeforeEach(func() {
				fakeServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/v1/lrps/some-process-guid/instances/some-instance-guid/stop"),
						ghttp.RespondWith(http.StatusInternalServerError, ""),
					),
				)
			})

			It("makes the request and returns an error", func() {
				Expect(stopErr).To(HaveOccurred())
				Expect(stopErr.Error()).To(ContainSubstring("http error: status code 500"))
				Expect(fakeServer.ReceivedRequests()).To(HaveLen(1))
			})

			It("logs start and complete", func() {
				Eventually(logger.Buffer()).Should(gbytes.Say("stop-lrp.failed-with-status"))
			})
		})

		Context("when the connection fails", func() {
			BeforeEach(func() {
				fakeServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/v1/lrps/some-process-guid/instances/some-instance-guid/stop"),
						func(w http.ResponseWriter, r *http.Request) {
							fakeServer.CloseClientConnections()
						},
					),
				)
			})

			It("makes the request and returns an error", func() {
				Expect(stopErr).To(HaveOccurred())
				Expect(stopErr.Error()).To(ContainSubstring("EOF"))
				Expect(fakeServer.ReceivedRequests()).To(HaveLen(1))
			})
		})

		Context("when the connection times out", func() {
			BeforeEach(func() {
				fakeServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/v1/lrps/some-process-guid/instances/some-instance-guid/stop"),
						func(w http.ResponseWriter, r *http.Request) {
							time.Sleep(cfHttpTimeout + 100*time.Millisecond)
						},
					),
				)
			})

			It("makes the request and returns an error", func() {
				Expect(stopErr).To(HaveOccurred())
				Expect(stopErr.Error()).To(MatchRegexp("use of closed network connection|Client.Timeout exceeded while awaiting headers"))
				Expect(fakeServer.ReceivedRequests()).To(HaveLen(1))
			})

			It("logs start and complete", func() {
				Eventually(logger.Buffer()).Should(gbytes.Say("stop-lrp.request-failed"))
			})
		})
	})

	Describe("CancelTask", func() {
		var (
			logger    = lagertest.NewTestLogger("test")
			cancelErr error
			taskGuid  = "some-task-guid"
		)

		JustBeforeEach(func() {
			cancelErr = client.CancelTask(logger, taskGuid)
		})

		Context("when the request is successful", func() {
			BeforeEach(func() {
				fakeServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/v1/tasks/some-task-guid/cancel"),
						ghttp.RespondWith(http.StatusAccepted, ""),
					),
				)
			})

			It("makes the request and does not return an error", func() {
				Expect(cancelErr).NotTo(HaveOccurred())
				Expect(fakeServer.ReceivedRequests()).To(HaveLen(1))
			})

			It("logs start and complete", func() {
				Eventually(logger.Buffer()).Should(gbytes.Say("cancel-task.starting"))
				Eventually(logger.Buffer()).Should(gbytes.Say("cancel-task.completed"))
			})

		})

		Context("when the request returns 500", func() {
			BeforeEach(func() {
				fakeServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/v1/tasks/some-task-guid/cancel"),
						ghttp.RespondWith(http.StatusInternalServerError, ""),
					),
				)
			})

			It("makes the request and returns an error", func() {
				Expect(cancelErr).To(HaveOccurred())
				Expect(cancelErr.Error()).To(ContainSubstring("http error: status code 500"))
				Expect(fakeServer.ReceivedRequests()).To(HaveLen(1))
			})

			It("logs start and complete", func() {
				Eventually(logger.Buffer()).Should(gbytes.Say("cancel-task.failed-with-status"))
			})
		})

		Context("when the connection fails", func() {
			BeforeEach(func() {
				fakeServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/v1/tasks/some-task-guid/cancel"),
						func(w http.ResponseWriter, r *http.Request) {
							fakeServer.CloseClientConnections()
						},
					),
				)
			})

			It("makes the request and returns an error", func() {
				Expect(cancelErr).To(HaveOccurred())
				Expect(cancelErr.Error()).To(ContainSubstring("EOF"))
				Expect(fakeServer.ReceivedRequests()).To(HaveLen(1))
			})
		})

		Context("when the connection times out", func() {
			BeforeEach(func() {
				fakeServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/v1/tasks/some-task-guid/cancel"),
						func(w http.ResponseWriter, r *http.Request) {
							time.Sleep(cfHttpTimeout + 100*time.Millisecond)
						},
					),
				)
			})

			It("makes the request and returns an error", func() {
				Expect(cancelErr).To(HaveOccurred())
				Expect(cancelErr.Error()).To(MatchRegexp("use of closed network connection|Client.Timeout exceeded while awaiting headers"))
				Expect(fakeServer.ReceivedRequests()).To(HaveLen(1))
			})

			It("logs start and complete", func() {
				Eventually(logger.Buffer()).Should(gbytes.Say("cancel-task.request-failed"))
			})
		})
	})
})
