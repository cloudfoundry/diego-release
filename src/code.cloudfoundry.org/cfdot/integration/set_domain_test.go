package integration_test

import (
	"net/http"
	"os/exec"
	"time"

	"code.cloudfoundry.org/bbs/models"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("set-domain", func() {
	itValidatesBBSFlags("set-domain", "domain1")

	Context("when the server responds for set-domain", func() {
		var (
			serverTimeout int
		)

		BeforeEach(func() {
			serverTimeout = 0
		})

		JustBeforeEach(func() {
			bbsServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/v1/domains/upsert"),
					func(w http.ResponseWriter, req *http.Request) {
						time.Sleep(time.Duration(serverTimeout) * time.Second)
					},
					ghttp.RespondWithProto(200, &models.UpsertDomainResponse{}),
				),
			)
		})

		It("set-domain works with a TTL not specified", func() {
			sess := RunCFDot("set-domain", "any-domain")
			Eventually(sess).Should(gexec.Exit(0))
		})

		It("set-domain works with a TTL specified", func() {
			sess := RunCFDot("set-domain", "any-domain", "--ttl", "40s")
			Eventually(sess).Should(gexec.Exit(0))
		})

		It("set-domain prints to stderr when no domain specified", func() {
			sess := RunCFDot("set-domain", "", "--ttl", "40s")
			Eventually(sess).Should(gexec.Exit(3))
			Expect(sess.Err).To(gbytes.Say(`No domain given`))
			Expect(sess.Err).To(gbytes.Say(`Usage`))
		})

		It("set-domain prints to stderr for negative TTL", func() {
			sess := RunCFDot("set-domain", "any-domain", "--ttl", "-40s")
			Eventually(sess).Should(gexec.Exit(3))
			Expect(sess.Err).To(gbytes.Say(`ttl is negative`))
			Expect(sess.Err).To(gbytes.Say(`Usage:`))
		})

		It("set-domain prints to stderr for non-numeric TTL", func() {
			cfdotCmd := exec.Command(cfdotPath, "--bbsURL", bbsServer.URL(), "set-domain", "any-domain", "-t", "asdf")

			sess, err := gexec.Start(cfdotCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(sess).Should(gexec.Exit(3))
			Expect(sess.Err).To(gbytes.Say(`invalid duration`))
			Expect(sess.Err).To(gbytes.Say(`Usage:`))
		})

		Context("when timeout flag is present", func() {
			Context("when request exceeds timeout", func() {
				BeforeEach(func() {
					serverTimeout = 2
				})

				It("exits with code 4 and a timeout message", func() {
					sess := RunCFDot("--timeout", "1", "set-domain", "any-domain")
					Eventually(sess, 2).Should(gexec.Exit(4))
					Expect(sess.Err).To(gbytes.Say(`Timeout exceeded`))
				})
			})

			Context("when request is within the timeout", func() {
				It("exits with status code of 0", func() {
					sess := RunCFDot("--timeout", "1", "set-domain", "any-domain")
					Eventually(sess).Should(gexec.Exit(0))
				})
			})
		})
	})

	Context("when the server does not respond for set-domain", func() {
		BeforeEach(func() {
			bbsServer.RouteToHandler("POST", "/v1/domains/upsert",
				ghttp.RespondWith(500, []byte{}))
		})

		It("set-domain fails with a relevant error message", func() {
			sess := RunCFDot("set-domain", "any-domain")
			Eventually(sess, 2*time.Second).Should(gexec.Exit(4))
			Expect(sess.Err).To(gbytes.Say("Invalid Response with status code: 500"))
		})
	})
})
