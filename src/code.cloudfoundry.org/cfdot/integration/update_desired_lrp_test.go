package integration_test

import (
	"encoding/json"
	"net/http"
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/bbs/models"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("update-desired-lrp", func() {
	itValidatesBBSFlags("update-desired-lrp")

	Context("when not enough args are provided", func() {
		It("exits with status 3 and prints an error on stderr", func() {
			sess := RunCFDot("update-desired-lrp")
			Eventually(sess).Should(gexec.Exit(3))
			Expect(sess.Err).To(gbytes.Say(`Missing arguments`))
			Expect(sess.Err).To(gbytes.Say("cfdot update-desired-lrp process-guid \\(SPEC\\|@FILE\\) .*"))
		})
	})

	Context("when bbs responds with 200 status code", func() {
		var (
			lrpUpdate     *models.DesiredLRPUpdate
			serverTimeout int
		)

		BeforeEach(func() {
			lrpUpdate = &models.DesiredLRPUpdate{}
			lrpUpdate.SetInstances(int32(5))
			serverTimeout = 0
		})

		JustBeforeEach(func() {
			bbsServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/v1/desired_lrp/update"),
					func(w http.ResponseWriter, req *http.Request) {
						time.Sleep(time.Duration(serverTimeout) * time.Second)
					},
					ghttp.VerifyProtoRepresenting(&models.UpdateDesiredLRPRequest{
						Update:      lrpUpdate,
						ProcessGuid: "process-guid",
					}),
					ghttp.RespondWithProto(200, &models.DesiredLRPLifecycleResponse{
						Error: nil,
					}),
				),
			)
		})

		Context("as json", func() {
			var lrpArg string

			BeforeEach(func() {
				spec, err := json.Marshal(lrpUpdate)
				Expect(err).NotTo(HaveOccurred())
				lrpArg = string(spec)
			})

			It("exits with status code of 0", func() {
				sess := RunCFDot("update-desired-lrp", "process-guid", lrpArg)
				Eventually(sess).Should(gexec.Exit(0))
			})

			Context("when timeout flag is present", func() {
				Context("when request exceeds timeout", func() {
					BeforeEach(func() {
						serverTimeout = 2
					})

					It("exits with code 4 and a timeout message", func() {
						sess := RunCFDot("update-desired-lrp", "process-guid", lrpArg, "--timeout", "1")
						Eventually(sess, 2).Should(gexec.Exit(4))
						Expect(sess.Err).To(gbytes.Say(`Timeout exceeded`))
					})
				})

				Context("when request is within the timeout", func() {
					It("exits with status code of 0", func() {
						sess := RunCFDot("update-desired-lrp", "process-guid", lrpArg, "--timeout", "1")
						Eventually(sess).Should(gexec.Exit(0))
					})
				})
			})
		})

		Context("as a file", func() {
			var lrpArg string

			BeforeEach(func() {
				f, err := os.CreateTemp(os.TempDir(), "update_lrp_spec")
				Expect(err).NotTo(HaveOccurred())
				defer f.Close()
				Expect(json.NewEncoder(f).Encode(lrpUpdate)).To(Succeed())

				lrpArg = "@" + f.Name()
			})

			It("exits with status code 0", func() {
				sess := RunCFDot("update-desired-lrp", "process-guid", lrpArg)
				Eventually(sess).Should(gexec.Exit(0))
			})
		})

		Context("empty spec", func() {
			It("exits with status code of 3", func() {
				sess := RunCFDot("update-desired-lrp", "process-guid")
				Eventually(sess).Should(gexec.Exit(3))
			})
		})

		Context("invalid spec", func() {
			It("exits with status code of 3 and prints the error", func() {
				sess := RunCFDot("update-desired-lrp", "process-guid", "foo")
				Eventually(sess).Should(gexec.Exit(3))
				Expect(sess.Err).To(gbytes.Say("Invalid JSON:"))
			})
		})

		Context("non-existing spec file", func() {
			It("exits with status 3 and prints the error", func() {
				sess := RunCFDot("update-desired-lrp", "process-guid1", "@/path/to/non/existing/file")
				Eventually(sess).Should(gexec.Exit(3))
				Expect(sess.Err).To(gbytes.Say("no such file"))
			})
		})
	})

	Context("when bbs responds with non-200 status code", func() {
		JustBeforeEach(func() {
			bbsServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/v1/desired_lrp/update"),
					ghttp.RespondWithProto(500, &models.DesiredLRPLifecycleResponse{
						Error: &models.Error{
							Type:    models.Error_Deadlock,
							Message: "deadlock detected",
						},
					}),
				),
			)
		})

		It("exits with status 4 and prints the error", func() {
			sess := RunCFDot("update-desired-lrp", "process-guid1", "{}")
			Eventually(sess).Should(gexec.Exit(4))
			Expect(sess.Err).To(gbytes.Say("Deadlock"))
		})
	})
})
