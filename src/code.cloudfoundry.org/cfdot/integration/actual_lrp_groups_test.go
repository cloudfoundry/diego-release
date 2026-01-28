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

var _ = Describe("actual-lrp-groups", func() {
	itValidatesBBSFlags("actual-lrp-groups")
	itHasNoArgs("actual-lrp-groups", false)

	Context("when no filters are passed", func() {
		var (
			serverTimeout int
		)

		BeforeEach(func() {
			serverTimeout = 0
		})

		JustBeforeEach(func() {
			//lint:ignore SA1019 - calling deprecated model while unit testing deprecated method
			response := &models.ActualLRPGroupsResponse{
				//lint:ignore SA1019 - calling deprecated model while unit testing deprecated method
				ActualLrpGroups: []*models.ActualLRPGroup{
					{
						Instance: &models.ActualLRP{
							State: "running",
						},
					},
				},
			}
			bbsServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/v1/actual_lrp_groups/list"),
					func(w http.ResponseWriter, req *http.Request) {
						time.Sleep(time.Duration(serverTimeout) * time.Second)
					},
					ghttp.RespondWithProto(200, response.ToProto()),
				),
			)
		})

		It("returns the json encoding of the actual lrp", func() {
			sess := RunCFDot("actual-lrp-groups")
			Eventually(sess).Should(gexec.Exit(0))
			Expect(sess.Out).To(gbytes.Say(`"state":"running"`))
		})

		Context("when timeout flag is present", func() {
			Context("when request exceeds timeout", func() {
				BeforeEach(func() {
					serverTimeout = 2
				})

				It("exits with code 4 and a timeout message", func() {
					sess := RunCFDot("actual-lrp-groups", "--timeout", "1")
					Eventually(sess, 2).Should(gexec.Exit(4))
					Expect(sess.Err).To(gbytes.Say(`Timeout exceeded`))
				})
			})

			Context("when request is within the timeout", func() {
				It("exits with status code of 0", func() {
					sess := RunCFDot("actual-lrp-groups", "--timeout", "1")
					Eventually(sess).Should(gexec.Exit(0))
					Expect(sess.Out).To(gbytes.Say(`"state":"running"`))
				})
			})
		})
	})

	Context("when passing filters", func() {
		BeforeEach(func() {
			//lint:ignore SA1019 - calling deprecated model while unit testing deprecated method
			request := &models.ActualLRPGroupsRequest{
				Domain: "cf-apps",
				CellId: "cell_z1-0",
			}
			//lint:ignore SA1019 - calling deprecated model while unit testing deprecated method
			response := &models.ActualLRPGroupsResponse{
				//lint:ignore SA1019 - calling deprecated model while unit testing deprecated method
				ActualLrpGroups: []*models.ActualLRPGroup{
					{
						Instance: &models.ActualLRP{
							State: "running",
						},
					},
				},
			}
			bbsServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/v1/actual_lrp_groups/list"),
					ghttp.VerifyProtoRepresenting(request.ToProto()),
					ghttp.RespondWithProto(200, response.ToProto()),
				),
			)
		})

		It("exits with status code of 0", func() {
			sess := RunCFDot("actual-lrp-groups",
				"-d", "cf-apps",
				"-c", "cell_z1-0",
			)
			Eventually(sess).Should(gexec.Exit(0))
		})

		It("exits with status code of 0", func() {
			sess := RunCFDot("actual-lrp-groups",
				"-d", "cf-apps",
				"--domain", "cf-apps",
				"--cell-id", "cell_z1-0",
			)
			Eventually(sess).Should(gexec.Exit(0))
		})
	})
})
