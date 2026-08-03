package integration_test

import (
	"net/http"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/bbs/models"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("desired-lrp-scheduling-infos", func() {
	itValidatesBBSFlags("desired-lrp-scheduling-infos")
	itHasNoArgs("desired-lrp-scheduling-infos", false)

	Context("when extra arguments are passed", func() {
		It("exits with status code of 3 and prints the usage", func() {
			sess := RunCFDot("desired-lrp-scheduling-infos", "extra-arg")
			Eventually(sess).Should(gexec.Exit(3))
			Expect(sess.Err).To(gbytes.Say("cfdot desired-lrp-scheduling-infos \\[flags\\]"))
		})
	})

	Context("when no filters are passed", func() {
		var (
			serverTimeout int
		)

		BeforeEach(func() {
			serverTimeout = 0
		})

		JustBeforeEach(func() {
			bbsServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/v1/desired_lrp_scheduling_infos/list"),
					func(w http.ResponseWriter, req *http.Request) {
						time.Sleep(time.Duration(serverTimeout) * time.Second)
					},
					ghttp.RespondWithProto(200, &models.DesiredLRPSchedulingInfosResponse{
						DesiredLrpSchedulingInfos: []*models.DesiredLRPSchedulingInfo{
							{
								Instances: 1,
							},
						},
					}),
				),
			)
		})

		It("exits with 0  and returns the json encoding of the desired lrp scheduling info", func() {
			sess := RunCFDot("desired-lrp-scheduling-infos")
			Eventually(sess).Should(gexec.Exit(0))
			Expect(sess.Out).To(gbytes.Say(`"instances":1`))
		})

		Context("when timeout flag is present", func() {
			var sess *gexec.Session

			BeforeEach(func() {
				sess = RunCFDot("desired-lrp-scheduling-infos", "--timeout", "1")
			})

			Context("when request exceeds timeout", func() {
				BeforeEach(func() {
					serverTimeout = 2
				})

				It("exits with code 4 and a timeout message", func() {
					Eventually(sess, 2).Should(gexec.Exit(4))
					Expect(sess.Err).To(gbytes.Say(`Timeout exceeded`))
				})
			})

			Context("when request is within the timeout", func() {
				It("exits with status code of 0", func() {
					Eventually(sess).Should(gexec.Exit(0))
					Expect(sess.Out).To(gbytes.Say(`"instances":1`))
				})
			})
		})
	})

	Context("when passing filters", func() {
		BeforeEach(func() {
			bbsServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/v1/desired_lrp_scheduling_infos/list"),
					ghttp.VerifyProtoRepresenting(&models.DesiredLRPsRequest{
						Domain: "cf-apps",
					}),
					ghttp.RespondWithProto(200, &models.DesiredLRPSchedulingInfosResponse{
						DesiredLrpSchedulingInfos: []*models.DesiredLRPSchedulingInfo{
							{
								Instances: 1,
							},
						},
					}),
				),
			)
		})

		It("exits with status code of 0", func() {
			sess := RunCFDot("desired-lrp-scheduling-infos", "-d", "cf-apps")
			Eventually(sess).Should(gexec.Exit(0))
		})

		It("exits with status code of 0", func() {
			sess := RunCFDot("desired-lrp-scheduling-infos", "--domain", "cf-apps")
			Eventually(sess).Should(gexec.Exit(0))
		})
	})
})
