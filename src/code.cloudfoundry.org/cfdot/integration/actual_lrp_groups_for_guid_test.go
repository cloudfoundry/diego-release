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

var _ = Describe("actual-lrp-groups-for-guid", func() {
	itValidatesBBSFlags("actual-lrp-groups-for-guid", "test-guid")

	Context("when there is no filter", func() {
		Context("when the server returns a valid response", func() {
			var serverTimeout int

			BeforeEach(func() {
				serverTimeout = 0
			})

			JustBeforeEach(func() {
				bbsServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/v1/actual_lrp_groups/list_by_process_guid"),
						func(w http.ResponseWriter, req *http.Request) {
							time.Sleep(time.Duration(serverTimeout) * time.Second)
						},
						//lint:ignore SA1019 - deprecated model used for testing deprecated functionality
						ghttp.VerifyProtoRepresenting(&models.ActualLRPGroupsByProcessGuidRequest{
							ProcessGuid: "random-guid",
						}),
						//lint:ignore SA1019 - deprecated model used for testing deprecated functionality
						ghttp.RespondWithProto(200, &models.ActualLRPGroupsResponse{
							//lint:ignore SA1019 - deprecated model used for testing deprecated functionality
							ActualLrpGroups: []*models.ActualLRPGroup{
								{
									Instance: &models.ActualLRP{
										State: "running",
									},
								},
							},
						}),
					),
				)
			})

			It("returns the json encoding of the actual lrp", func() {
				sess := RunCFDot("actual-lrp-groups-for-guid", "random-guid")
				Eventually(sess).Should(gexec.Exit(0))
				Expect(sess.Out).To(gbytes.Say(`"state":"running"`))
			})

			Context("when timeout flag is present", func() {
				Context("when request exceeds timeout", func() {
					BeforeEach(func() {
						serverTimeout = 2
					})

					It("exits with code 4 and a timeout message", func() {
						sess := RunCFDot("actual-lrp-groups-for-guid", "random-guid", "--timeout", "1")
						Eventually(sess, 2).Should(gexec.Exit(4))
						Expect(sess.Err).To(gbytes.Say(`Timeout exceeded`))
					})
				})

				Context("when request is within the timeout", func() {
					It("exits with status code of 0", func() {
						sess := RunCFDot("actual-lrp-groups-for-guid", "random-guid", "--timeout", "1")
						Eventually(sess).Should(gexec.Exit(0))
						Expect(sess.Out).To(gbytes.Say(`"state":"running"`))
					})
				})
			})
		})

		Context("when the server returns an error", func() {
			BeforeEach(func() {
				bbsServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/v1/actual_lrp_groups/list_by_process_guid"),
						ghttp.RespondWithProto(500, &models.DomainsResponse{
							Error: &models.Error{
								Type:    models.Error_Deadlock,
								Message: "the request failed due to deadlock",
							},
							Domains: nil,
						}),
					),
				)
			})

			It("exits with status code 4 and should print the type and message of the error", func() {
				sess := RunCFDot("actual-lrp-groups-for-guid", "random-guid")
				Eventually(sess).Should(gexec.Exit(4))
				Expect(sess.Err).To(gbytes.Say("BBS error"))
				Expect(sess.Err).To(gbytes.Say("Type 28: Deadlock"))
				Expect(sess.Err).To(gbytes.Say("Message: the request failed due to deadlock"))
			})

			It("should not print the usage", func() {
				sess := RunCFDot("actual-lrp-groups-for-guid", "random-guid")
				Expect(sess.Err).NotTo(gbytes.Say("Usage:"))
			})
		})
	})

	Context("when passing index as filter", func() {
		BeforeEach(func() {
			bbsServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/v1/actual_lrp_groups/get_by_process_guid_and_index"),
					//lint:ignore SA1019 - deprecated model used for testing deprecated functionality
					ghttp.VerifyProtoRepresenting(&models.ActualLRPGroupByProcessGuidAndIndexRequest{
						ProcessGuid: "test-process-guid",
						Index:       1,
					}),
					//lint:ignore SA1019 - deprecated model used for testing deprecated functionality
					ghttp.RespondWithProto(200, &models.ActualLRPGroupsResponse{
						//lint:ignore SA1019 - deprecated model used for testing deprecated functionality
						ActualLrpGroups: []*models.ActualLRPGroup{
							{
								Instance: &models.ActualLRP{
									ActualLRPKey: models.ActualLRPKey{
										Index: 1,
									},
									State: "running",
								},
							},
						},
					}),
				),
			)
		})

		It("returns the json encoding of the actual lrp", func() {
			sess := RunCFDot("actual-lrp-groups-for-guid", "test-process-guid", "-i", "1")
			Eventually(sess).Should(gexec.Exit(0))
			Expect(sess.Out).To(gbytes.Say(`"state":"running"`))
		})
	})
})
