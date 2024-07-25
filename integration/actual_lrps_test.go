package integration_test

import (
	"net/http"
	"time"

	"code.cloudfoundry.org/bbs/models"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("actual-lrps", func() {
	itValidatesBBSFlags("actual-lrps")
	itHasNoArgs("actual-lrps", false)

	Context("when no filters are passed", func() {
		var (
			serverTimeout int
		)

		BeforeEach(func() {
			serverTimeout = 0
		})

		JustBeforeEach(func() {
			request := &models.ActualLRPsRequest{}
			response := &models.ActualLRPsResponse{
				ActualLrps: []*models.ActualLRP{
					&models.ActualLRP{
						State: "running",
					},
				},
			}
			bbsServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/v1/actual_lrps/list"),
					ghttp.VerifyProtoRepresenting(request.ToProto()),
					func(w http.ResponseWriter, req *http.Request) {
						time.Sleep(time.Duration(serverTimeout) * time.Second)
					},
					ghttp.RespondWithProto(200, response.ToProto()),
				),
			)
		})

		It("returns the json encoding of the actual lrp", func() {
			sess := RunCFDot("actual-lrps")
			Eventually(sess).Should(gexec.Exit(0))
			Expect(sess.Out).To(gbytes.Say(`"state":"running"`))
		})

		Context("when timeout flag is present", func() {
			Context("when request exceeds timeout", func() {
				BeforeEach(func() {
					serverTimeout = 2
				})

				It("exits with code 4 and a timeout message", func() {
					sess := RunCFDot("actual-lrps", "--timeout", "1")
					Eventually(sess, 2).Should(gexec.Exit(4))
					Expect(sess.Err).To(gbytes.Say(`Timeout exceeded`))
				})
			})

			Context("when request is within the timeout", func() {
				It("exits with status code of 0", func() {
					sess := RunCFDot("actual-lrps", "--timeout", "1")
					Eventually(sess).Should(gexec.Exit(0))
					Expect(sess.Out).To(gbytes.Say(`"state":"running"`))
				})
			})
		})
	})

	Context("when passing filters", func() {
		BeforeEach(func() {
			request := models.ActualLRPsRequest{
				Domain:      "cf-apps",
				CellId:      "cell_z1-0",
				ProcessGuid: "pg-0",
			}
			index := int32(1)
			request.SetIndex(&index)
			response := &models.ActualLRPsResponse{
				ActualLrps: []*models.ActualLRP{
					&models.ActualLRP{
						State: "running",
					},
				},
			}

			bbsServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/v1/actual_lrps/list"),
					ghttp.VerifyProtoRepresenting(request.ToProto()),
					ghttp.RespondWithProto(200, response.ToProto()),
				),
			)
		})

		It("exits with status code of 0", func() {
			sess := RunCFDot("actual-lrps",
				"-d", "cf-apps",
				"-c", "cell_z1-0",
				"-p", "pg-0",
				"-i", "1",
			)
			Eventually(sess).Should(gexec.Exit(0))
		})

		It("exits with status code of 0", func() {
			sess := RunCFDot("actual-lrps",
				"-d", "cf-apps",
				"--domain", "cf-apps",
				"--cell-id", "cell_z1-0",
				"--process-guid", "pg-0",
				"--index", "1",
			)
			Eventually(sess).Should(gexec.Exit(0))
		})
	})
})
