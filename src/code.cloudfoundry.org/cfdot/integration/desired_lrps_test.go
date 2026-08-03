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

var _ = Describe("desired-lrps", func() {
	itValidatesBBSFlags("desired-lrps")
	itHasNoArgs("desired-lrps", false)

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
					ghttp.VerifyRequest("POST", "/v1/desired_lrps/list.r3"),
					func(w http.ResponseWriter, req *http.Request) {
						time.Sleep(time.Duration(serverTimeout) * time.Second)
					},
					ghttp.RespondWithProto(200, &models.DesiredLRPsResponse{
						DesiredLrps: []*models.DesiredLRP{
							{
								Instances: 1,
							},
						},
					}),
				),
			)
		})

		It("returns the json encoding of the desired lrp scheduling info", func() {
			sess := RunCFDot("desired-lrps")
			Eventually(sess).Should(gexec.Exit(0))
			Expect(sess.Out).To(gbytes.Say(`"instances":1`))
		})

		Context("when timeout flag is present", func() {
			Context("when request exceeds timeout", func() {
				BeforeEach(func() {
					serverTimeout = 2
				})

				It("exits with code 4 and a timeout message", func() {
					sess := RunCFDot("desired-lrps", "--timeout", "1")
					Eventually(sess, 2).Should(gexec.Exit(4))
					Expect(sess.Err).To(gbytes.Say(`Timeout exceeded`))
				})
			})

			Context("when request is within the timeout", func() {
				It("exits with status code of 0", func() {
					sess := RunCFDot("desired-lrps", "--timeout", "1")
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
					ghttp.VerifyRequest("POST", "/v1/desired_lrps/list.r3"),
					ghttp.VerifyProtoRepresenting(&models.DesiredLRPsRequest{
						Domain: "cf-apps",
					}),
					ghttp.RespondWithProto(200, &models.DesiredLRPsResponse{
						DesiredLrps: []*models.DesiredLRP{
							{
								Instances: 1,
							},
						},
					}),
				),
			)
		})

		Context("when -d is used as a filter flag", func() {
			It("exits with a status code of 0", func() {
				sess := RunCFDot("desired-lrps", "-d", "cf-apps")
				Eventually(sess).Should(gexec.Exit(0))
			})
		})

		Context("when --domain is used as a filter flag", func() {
			It("exits with a status code of 0", func() {
				sess := RunCFDot("desired-lrps", "--domain", "cf-apps")
				Eventually(sess).Should(gexec.Exit(0))
			})
		})

		Context("when --domain and -d are supplied as filter flags", func() {
			It("exits with a status code of 3", func() {
				sess := RunCFDot("desired-lrps", "--domain", "cf-apps", "-d", "cf-apps")
				Eventually(sess).Should(gexec.Exit(3))
			})
		})
	})
})
