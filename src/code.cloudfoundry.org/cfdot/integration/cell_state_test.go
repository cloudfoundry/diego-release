package integration_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/rep"
	"code.cloudfoundry.org/tlsconfig"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("cell-state", func() {
	itValidatesTLSFlags("cell-state")

	Context("when cell-state command is called", func() {
		var (
			presence1, presence2   *models.CellPresence
			rep1Server, rep2Server *ghttp.Server
			cellState1, cellState2 *rep.CellState
		)

		BeforeEach(func() {
			rep1Server = ghttp.NewUnstartedServer()

			tlsConfig, err := tlsconfig.Build(
				tlsconfig.WithInternalServiceDefaults(),
				tlsconfig.WithIdentityFromFile(locketClientCertFile, locketClientKeyFile),
			).Client(tlsconfig.WithAuthorityFromFile(locketCACertFile))
			Expect(err).NotTo(HaveOccurred())
			rep1Server.HTTPTestServer.TLS = tlsConfig
			rep1Server.HTTPTestServer.StartTLS()

			rep2Server = ghttp.NewServer()

			presence1 = &models.CellPresence{
				CellId: "cell-1",
				RepUrl: rep1Server.URL(),
			}
			presence2 = &models.CellPresence{
				CellId: "cell-2",
				RepUrl: rep2Server.URL(),
			}

			cellState1 = &rep.CellState{
				RepURL:             rep1Server.URL(),
				CellID:             "cell-1",
				RootFSProviders:    rep.RootFSProviders{},
				AvailableResources: rep.Resources{},
				TotalResources:     rep.Resources{},
				LRPs:               []rep.LRP{},
				Tasks:              []rep.Task{},
			}
			cellState2 = &rep.CellState{
				RepURL:             rep2Server.URL(),
				CellID:             "cell-2",
				RootFSProviders:    rep.RootFSProviders{},
				AvailableResources: rep.Resources{},
				TotalResources:     rep.Resources{},
				LRPs:               []rep.LRP{},
				Tasks:              []rep.Task{},
			}

			response := &models.CellsResponse{
				Cells: []*models.CellPresence{presence1, presence2},
			}

			bbsServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/v1/cells/list.r1"),
					ghttp.RespondWithProto(200, response.ToProto()),
				),
			)
			rep1Server.RouteToHandler("GET", "/state", func(resp http.ResponseWriter, req *http.Request) {
				jsonData, err := json.Marshal(cellState1)
				Expect(err).NotTo(HaveOccurred())
				resp.Write(jsonData)
			})

			rep2Server.RouteToHandler("GET", "/state", func(resp http.ResponseWriter, req *http.Request) {
				jsonData, err := json.Marshal(cellState2)
				Expect(err).NotTo(HaveOccurred())
				resp.Write(jsonData)
			})
		})

		AfterEach(func() {
			rep1Server.CloseClientConnections()
			rep1Server.Close()
			rep2Server.CloseClientConnections()
			rep2Server.Close()
		})

		It("returns the json encoding of the correct cell-state", func() {
			sess := RunCFDot("cell-state", "cell-2")
			Eventually(sess).Should(gexec.Exit(0))

			jsonData, err := json.Marshal(cellState2)
			Expect(err).NotTo(HaveOccurred())
			Expect(bytes.TrimSpace(sess.Out.Contents())).To(Equal(jsonData))
		})

		Context("when timeout flag is present", func() {
			var (
				serverTimeout int
				sess          *gexec.Session
			)

			JustBeforeEach(func() {
				response := &models.CellsResponse{
					Cells: []*models.CellPresence{presence1, presence2},
				}
				bbsServer.SetHandler(0,
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/v1/cells/list.r1"),
						func(w http.ResponseWriter, req *http.Request) {
							time.Sleep(time.Duration(serverTimeout) * time.Second)
						},
						ghttp.RespondWithProto(200, response.ToProto()),
					),
				)

				sess = RunCFDot("--timeout", "1", "cell-state", "cell-2")
			})

			Context("when exceeds timeout", func() {
				BeforeEach(func() {
					serverTimeout = 2
				})

				It("exits with code 4 and a timeout message", func() {
					Eventually(sess, 2).Should(gexec.Exit(4))
					Expect(sess.Err).To(gbytes.Say(`Timeout exceeded`))
				})

			})

			Context("when within timeout", func() {
				BeforeEach(func() {
					serverTimeout = 0
				})

				It("should succeed", func() {
					Eventually(sess).Should(gexec.Exit(0))
					jsonData, err := json.Marshal(cellState2)
					Expect(err).NotTo(HaveOccurred())
					Expect(bytes.TrimSpace(sess.Out.Contents())).To(Equal(jsonData))
				})
			})
		})

		Context("when the rep has mutual TLS enabled", func() {
			It("uses the correct TLS config", func() {
				sess := RunCFDot("cell-state", "cell-1")
				Eventually(sess).Should(gexec.Exit(0))

				jsonData, err := json.Marshal(cellState1)
				Expect(err).NotTo(HaveOccurred())
				Expect(bytes.TrimSpace(sess.Out.Contents())).To(Equal(jsonData))
			})

			Context("cell-states", func() {
				It("returns the json encoding of the cell-states", func() {
					sess := RunCFDot("cell-states")
					Eventually(sess).Should(gexec.Exit(0))

					decoder := json.NewDecoder(io.NopCloser(bytes.NewBuffer(sess.Out.Contents())))
					var receivedState rep.CellState

					err := decoder.Decode(&receivedState)
					Expect(err).NotTo(HaveOccurred())
					Expect(receivedState).To(Equal(*cellState1))

					err = decoder.Decode(&receivedState)
					Expect(err).NotTo(HaveOccurred())
					Expect(receivedState).To(Equal(*cellState2))
				})
			})

			Context("when timeout flag is present", func() {
				var (
					serverTimeout int
					sess          *gexec.Session
				)

				JustBeforeEach(func() {
					response := &models.CellsResponse{
						Cells: []*models.CellPresence{presence1, presence2},
					}
					bbsServer.SetHandler(0,
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("POST", "/v1/cells/list.r1"),
							func(w http.ResponseWriter, req *http.Request) {
								time.Sleep(time.Duration(serverTimeout) * time.Second)
							},
							ghttp.RespondWithProto(200, response.ToProto()),
						),
					)

					sess = RunCFDot("--timeout", "1", "cell-states")
				})

				Context("when exceeds timeout", func() {
					BeforeEach(func() {
						serverTimeout = 2
					})

					It("exits with code 4 and a timeout message", func() {
						Eventually(sess, 2).Should(gexec.Exit(4))
						Expect(sess.Err).To(gbytes.Say(`Timeout exceeded`))
					})
				})

				Context("when within timeout", func() {
					BeforeEach(func() {
						serverTimeout = 0
					})

					It("should succeed", func() {
						Eventually(sess).Should(gexec.Exit(0))
						decoder := json.NewDecoder(io.NopCloser(bytes.NewBuffer(sess.Out.Contents())))
						var receivedState rep.CellState

						err := decoder.Decode(&receivedState)
						Expect(err).NotTo(HaveOccurred())
						Expect(receivedState).To(Equal(*cellState1))

						err = decoder.Decode(&receivedState)
						Expect(err).NotTo(HaveOccurred())
						Expect(receivedState).To(Equal(*cellState2))
					})
				})
			})
		})

		Context("when the cell does not exist", func() {
			It("exits with status code of 5", func() {
				sess := RunCFDot("cell-state", "cell-id-dsafasdklfjasdlkf")
				Eventually(sess).Should(gexec.Exit(5))
			})
		})

		Context("when the BBS request fails", func() {
			BeforeEach(func() {
				response := &models.CellsResponse{
					Error: models.ErrUnknownError,
				}
				bbsServer.SetHandler(0,
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/v1/cells/list.r1"),
						ghttp.RespondWithProto(503, response.ToProto()),
					),
				)
			})

			It("exits with status code of 4", func() {
				sess := RunCFDot("cell-state", "cell-2")
				Eventually(sess).Should(gexec.Exit(4))
				Expect(sess.Err).To(gbytes.Say("BBS error"))
			})

			Context("cell-states", func() {
				It("exits with status code of 4", func() {
					sess := RunCFDot("cell-states")
					Eventually(sess).Should(gexec.Exit(4))
					Expect(sess.Err).To(gbytes.Say("BBS error"))
					Expect(sess.Err).To(gbytes.Say("Failed to get cell registrations from BBS"))
				})
			})
		})

		Context("when the Rep request fails", func() {
			BeforeEach(func() {
				rep1Server.RouteToHandler("GET", "/state", func(resp http.ResponseWriter, req *http.Request) {
					resp.WriteHeader(503)
				})
				rep2Server.RouteToHandler("GET", "/state", func(resp http.ResponseWriter, req *http.Request) {
					resp.WriteHeader(503)
				})
			})

			It("exits with status code of 4", func() {
				sess := RunCFDot("cell-state", "cell-1")
				Eventually(sess).Should(gexec.Exit(4))
				Expect(sess.Err).To(gbytes.Say("Rep error"))
				Expect(sess.Err).To(gbytes.Say("Failed to get cell state for cell cell-1"))
			})

			Context("cell-states", func() {
				It("exits with status code of 4", func() {
					sess := RunCFDot("cell-states")
					Eventually(sess).Should(gexec.Exit(4))
					Expect(sess.Err).To(gbytes.Say("Rep error"))
					Expect(sess.Err).To(gbytes.Say("Failed to get cell state for cell cell-1"))
					Expect(sess.Err).To(gbytes.Say("Failed to get cell state for cell cell-2"))
				})
			})
		})

		Context("when cell command is called with extra arguments", func() {
			It("exits with status code of 3", func() {
				sess := RunCFDot("cell-state", "cell-id", "extra-argument")
				Eventually(sess).Should(gexec.Exit(3))
			})

			Context("cell-states", func() {
				It("exits with status code of 3", func() {
					sess := RunCFDot("cell-states", "extra-argument")
					Eventually(sess).Should(gexec.Exit(3))
				})
			})
		})
	})
})
